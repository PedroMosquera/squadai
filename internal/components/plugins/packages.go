package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// Pi and OpenCode read plugin packages from an array the user also edits, so
// SquadAI owns individual entries, recorded in the managed sidecar as the
// exact strings it wrote. User entries keep their place; SquadAI's entries
// are replaced in place when the declared version changes and appended when
// new. The harness fetches and installs the packages itself.
const packagesDescription = "plugin:packages"

type packageMerge struct {
	key       string
	existing  map[string]any
	doc       map[string]any
	owned     []string
	tracked   bool
	conflicts []fileutil.MergeConflict
}

// desiredPackages returns the entries the active package declarations call
// for, in harness form and plugin-name order.
func (i *Installer) desiredPackages(agent domain.AgentID) []string {
	var out []string
	for _, name := range sortedPluginNames(i.plugins) {
		def := i.plugins[name]
		if def.InstallMethod != "package" || !i.active(def, agent) {
			continue
		}
		spec, err := domain.ParsePackageSpec(def.Package)
		if err != nil {
			continue
		}
		if entry, ok := domain.HarnessPackageEntry(agent, spec); ok {
			out = append(out, entry)
		}
	}
	return out
}

// packageIdentity is what two entries share when they are versions of one
// package. Entries SquadAI cannot parse identify as themselves.
func packageIdentity(agent domain.AgentID, entry any) string {
	var s string
	switch e := entry.(type) {
	case string:
		s = e
	case []any:
		if len(e) > 0 {
			s, _ = e[0].(string)
		}
	case map[string]any:
		s, _ = e["source"].(string)
	}
	if spec, ok := domain.ParseHarnessPackage(agent, s); ok {
		return spec.Identity()
	}
	return s
}

// mergePackages computes the settings document Apply would write. It reads
// but never writes.
func (i *Installer) mergePackages(agent domain.AgentID, path, key, projectDir string) (packageMerge, error) {
	existing, err := fileutil.ReadJSONFile(path)
	if err != nil {
		return packageMerge{}, fmt.Errorf("read %s: %w", path, err)
	}
	m := packageMerge{key: key, existing: existing, doc: make(map[string]any, len(existing)+1)}
	for k, v := range existing {
		m.doc[k] = v
	}
	var ownedBefore []string
	if projectDir != "" {
		ownedBefore, m.tracked, err = managed.ReadManagedEntries(projectDir, relToProject(projectDir, path), key)
		if err != nil {
			return packageMerge{}, fmt.Errorf("read managed entries: %w", err)
		}
	}

	want := i.desiredPackages(agent)
	raw, present := m.doc[key]
	current, isList := raw.([]any)
	if present && !isList {
		if len(want) > 0 {
			m.conflicts = append(m.conflicts, fileutil.MergeConflict{Key: key, UserValue: raw, IncomingValue: want})
		}
		return m, nil
	}

	wantByID := make(map[string]string, len(want))
	for _, w := range want {
		if id := packageIdentity(agent, w); wantByID[id] == "" {
			wantByID[id] = w
		}
	}
	placed := map[string]bool{}
	out := make([]any, 0, len(current)+len(want))
	pruned := false
	for _, e := range current {
		id := packageIdentity(agent, e)
		w, wanted := wantByID[id]
		s, isString := e.(string)
		switch {
		case isString && slices.Contains(ownedBefore, s):
			if wanted && !placed[id] {
				out = append(out, w)
				m.owned = append(m.owned, w)
				placed[id] = true
			} else {
				pruned = true
			}
			continue
		case !wanted || placed[id]:
		case isString && s == w:
			m.owned = append(m.owned, w)
			placed[id] = true
		default:
			m.conflicts = append(m.conflicts, fileutil.MergeConflict{Key: key + "." + id, UserValue: e, IncomingValue: w})
			placed[id] = true
		}
		out = append(out, e)
	}
	for _, w := range want {
		if id := packageIdentity(agent, w); !placed[id] {
			out = append(out, w)
			m.owned = append(m.owned, w)
			placed[id] = true
		}
	}
	switch {
	case len(out) > 0:
		m.doc[key] = out
	case pruned:
		delete(m.doc, key)
	}
	return m, nil
}

// planPackages plans the single package write for the agent. It also runs
// with nothing declared, so entries left by removed declarations are pruned.
func (i *Installer) planPackages(adapter domain.Adapter, projectDir string) ([]domain.PlannedAction, error) {
	path, key := domain.PackageSettings(adapter, projectDir)
	if path == "" {
		return nil, nil
	}
	m, err := i.mergePackages(adapter.ID(), path, key, projectDir)
	if err != nil {
		return nil, err
	}
	nothingDeclared := len(i.desiredPackages(adapter.ID())) == 0
	unchanged := len(m.conflicts) == 0 && reflect.DeepEqual(m.existing, m.doc)
	if nothingDeclared && (m.existing == nil || unchanged) {
		return nil, nil
	}

	action := domain.PlannedAction{
		ID:          fmt.Sprintf("%s-plugin-packages", adapter.ID()),
		Agent:       adapter.ID(),
		Component:   domain.ComponentPlugins,
		TargetPath:  path,
		Description: packagesDescription,
	}
	switch {
	case unchanged:
		action.Action = domain.ActionSkip
	case m.existing == nil:
		action.Action = domain.ActionCreate
	default:
		action.Action = domain.ActionUpdate
	}
	return []domain.PlannedAction{action}, nil
}

func (i *Installer) applyPackages(action domain.PlannedAction) error {
	m, err := i.mergePackages(action.Agent, action.TargetPath, domain.PackageSettingsKey(action.Agent), i.projectDir)
	if err != nil {
		return err
	}
	if len(m.conflicts) > 0 {
		return &domain.ConflictError{TargetPath: action.TargetPath, Conflicts: conflictsToDomain(m.conflicts)}
	}
	data, err := marshalSettings(m.doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(action.TargetPath), 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if i.projectDir == "" {
		return nil
	}
	rel := relToProject(i.projectDir, action.TargetPath)
	if len(m.owned) > 0 {
		if err := managed.WriteManagedEntries(i.projectDir, rel, m.key, m.owned); err != nil {
			return fmt.Errorf("write managed entries: %w", err)
		}
		return nil
	}
	if m.tracked {
		return forgetManagedKey(i.projectDir, rel, m.key)
	}
	return nil
}

// forgetManagedKey drops key from the file's sidecar record once SquadAI
// owns nothing under it, so drift checks stop expecting the key.
func forgetManagedKey(projectDir, rel, key string) error {
	keys, err := managed.ReadManagedKeys(projectDir, rel)
	if err != nil {
		return fmt.Errorf("read managed keys: %w", err)
	}
	remaining := slices.DeleteFunc(keys, func(k string) bool { return k == key })
	if len(remaining) == 0 {
		err = managed.RemoveManagedFile(projectDir, rel)
	} else {
		err = managed.WriteManagedKeys(projectDir, rel, remaining)
	}
	if err != nil {
		return fmt.Errorf("write managed keys: %w", err)
	}
	return nil
}

func (i *Installer) renderPackages(action domain.PlannedAction) ([]byte, error) {
	m, err := i.mergePackages(action.Agent, action.TargetPath, domain.PackageSettingsKey(action.Agent), i.projectDir)
	if err != nil {
		return nil, err
	}
	return marshalSettings(m.doc)
}

// verifyPackages reports one result per active package declaration: its
// entry present in the agent's settings array.
func (i *Installer) verifyPackages(adapter domain.Adapter, projectDir string) []domain.VerifyResult {
	path, key := domain.PackageSettings(adapter, projectDir)
	if path == "" {
		return nil
	}
	doc, _ := fileutil.ReadJSONFile(path)
	list, _ := doc[key].([]any)

	var results []domain.VerifyResult
	for _, name := range sortedPluginNames(i.plugins) {
		def := i.plugins[name]
		if def.InstallMethod != "package" || !i.active(def, adapter.ID()) {
			continue
		}
		spec, err := domain.ParsePackageSpec(def.Package)
		if err != nil {
			continue
		}
		entry, ok := domain.HarnessPackageEntry(adapter.ID(), spec)
		if !ok {
			continue
		}
		r := domain.VerifyResult{Check: fmt.Sprintf("plugin-%s-package", name), Component: "plugins", Passed: slices.Contains(list, any(entry)), Severity: domain.SeverityInfo}
		if !r.Passed {
			r.Severity = domain.SeverityError
			r.Message = fmt.Sprintf("package %s is not in %s of %s", entry, key, path)
		}
		results = append(results, r)
	}
	return results
}
