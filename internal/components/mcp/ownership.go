package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// SquadAI owns individual servers under the adapter's root key, never the key
// as a whole: users add their own servers to the same file (.mcp.json is
// shared with every other tool that reads it). Ownership is recorded per
// server name in the managed sidecar.

// serverKey names one server in conflicts and review-screen overrides, so
// consent to overwrite one server never covers another. A bare root-key
// override (OverwriteAll) covers every server SquadAI configures.
func serverKey(rootKey, name string) string { return rootKey + "." + name }

type serverMerge struct {
	doc       map[string]any
	conflicts []fileutil.MergeConflict
	owned     []string
}

// desiredServers returns the configured servers in the adapter's schema,
// normalized to the generic JSON types a parsed file holds so values compare
// with reflect.DeepEqual.
func (i *Installer) desiredServers(agent domain.AgentID) (map[string]any, error) {
	out := make(map[string]any, len(i.servers))
	for name, def := range i.servers {
		v, err := normalizeJSON(i.serverToMap(def, agent))
		if err != nil {
			return nil, err
		}
		out[name] = v
	}
	return out, nil
}

// ownedServers returns the server names SquadAI owns in the file, the names it
// owned but no longer configures (to be removed), and whether it owns the
// root key. A sidecar written before per-server tracking records only the
// root key; then every server SquadAI configures counts as owned and nothing
// is removed, so the migration never deletes a name SquadAI does not
// configure.
func ownedServers(projectDir, path, rootKey string, desired map[string]any) (owned map[string]bool, prunable []string, keyOwned bool, err error) {
	owned = make(map[string]bool)
	if projectDir == "" {
		return owned, nil, false, nil
	}
	rel := relToProject(projectDir, path)
	keys, err := managed.ReadManagedKeys(projectDir, rel)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read managed keys sidecar: %w", err)
	}
	for _, k := range keys {
		if k == rootKey {
			keyOwned = true
		}
	}
	names, tracked, err := managed.ReadManagedEntries(projectDir, rel, rootKey)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read managed servers sidecar: %w", err)
	}
	switch {
	case tracked:
		for _, n := range names {
			owned[n] = true
			if _, ok := desired[n]; !ok {
				prunable = append(prunable, n)
			}
		}
	case keyOwned:
		for n := range desired {
			owned[n] = true
		}
	}
	return owned, prunable, keyOwned, nil
}

// mergeServers computes the document Apply would write: SquadAI's servers
// written or updated where it owns them (or overrides grant consent), servers
// it owned but no longer configures removed, and every other server and
// top-level key left as found. It reads but never writes.
func (i *Installer) mergeServers(agent domain.AgentID, path, projectDir string, overrides []string) (serverMerge, error) {
	rootKey := i.rootKeyForAgent(agent)
	existing, err := fileutil.ReadJSONFile(path)
	if err != nil {
		return serverMerge{}, fmt.Errorf("read %s: %w", path, err)
	}
	desired, err := i.desiredServers(agent)
	if err != nil {
		return serverMerge{}, err
	}
	owned, prunable, keyOwned, err := ownedServers(projectDir, path, rootKey, desired)
	if err != nil {
		return serverMerge{}, err
	}

	overrideAll := false
	for _, k := range overrides {
		switch {
		case k == rootKey:
			overrideAll = true
		case strings.HasPrefix(k, rootKey+"."):
			owned[strings.TrimPrefix(k, rootKey+".")] = true
		}
	}
	if overrideAll {
		for n := range desired {
			owned[n] = true
		}
	}

	current, isMap := existing[rootKey].(map[string]any)
	if raw, present := existing[rootKey]; present && !isMap {
		if !keyOwned && !overrideAll {
			return serverMerge{conflicts: []fileutil.MergeConflict{{Key: rootKey, UserValue: raw, IncomingValue: desired}}}, nil
		}
		current = nil
	}

	ownedList := make([]string, 0, len(owned))
	for n := range owned {
		ownedList = append(ownedList, n)
	}
	merged, conflicts, claimed, err := fileutil.MergeJSON(current, desired, ownedList)
	if err != nil {
		return serverMerge{}, err
	}
	if len(conflicts) > 0 {
		sort.Slice(conflicts, func(a, b int) bool { return conflicts[a].Key < conflicts[b].Key })
		for idx := range conflicts {
			conflicts[idx].Key = serverKey(rootKey, conflicts[idx].Key)
		}
		return serverMerge{conflicts: conflicts}, nil
	}
	if merged == nil {
		merged = make(map[string]any)
	}
	for _, n := range prunable {
		delete(merged, n)
	}

	doc := make(map[string]any, len(existing)+1)
	for k, v := range existing {
		doc[k] = v
	}
	doc[rootKey] = merged
	return serverMerge{doc: doc, owned: claimed}, nil
}

// serversCurrent reports whether applying would leave the root key unchanged:
// every SquadAI server is on disk as configured and none it dropped remains.
// Servers SquadAI does not own never make this false.
func (i *Installer) serversCurrent(agent domain.AgentID, path, projectDir string) (bool, error) {
	existing, err := fileutil.ReadJSONFile(path)
	if err != nil || existing == nil {
		return false, err
	}
	m, err := i.mergeServers(agent, path, projectDir, nil)
	if err != nil || len(m.conflicts) > 0 {
		return false, err
	}
	rootKey := i.rootKeyForAgent(agent)
	return reflect.DeepEqual(existing[rootKey], m.doc[rootKey]), nil
}

// applyServers writes SquadAI's servers into the action's target file and
// records which server names it now owns.
func (i *Installer) applyServers(action domain.PlannedAction) error {
	rootKey := i.rootKeyForAgent(action.Agent)
	overrides := i.policy.EffectiveOverrides(action.TargetPath, []string{rootKey})
	m, err := i.mergeServers(action.Agent, action.TargetPath, i.projectDir, overrides)
	if err != nil {
		return err
	}
	if len(m.conflicts) > 0 {
		return &domain.ConflictError{
			TargetPath: action.TargetPath,
			Conflicts:  conflictsToDomain(m.conflicts),
		}
	}

	if err := os.MkdirAll(filepath.Dir(action.TargetPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := marshalJSONDoc(m.doc)
	if err != nil {
		return err
	}
	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", action.TargetPath, err)
	}

	if i.projectDir != "" {
		rel := relToProject(i.projectDir, action.TargetPath)
		if err := managed.WriteManagedEntries(i.projectDir, rel, rootKey, m.owned); err != nil {
			return fmt.Errorf("write managed servers sidecar: %w", err)
		}
	}
	return nil
}

// renderServersContent computes what applyServers would write, showing
// SquadAI's value for any conflicting server; the conflicts themselves are
// reported separately by Preview.
func (i *Installer) renderServersContent(action domain.PlannedAction) ([]byte, error) {
	m, err := i.mergeServers(action.Agent, action.TargetPath, i.projectDir, []string{i.rootKeyForAgent(action.Agent)})
	if err != nil {
		return nil, err
	}
	return marshalJSONDoc(m.doc)
}
