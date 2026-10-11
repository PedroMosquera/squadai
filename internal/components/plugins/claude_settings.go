package plugins

import (
	"encoding/json"
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

// Claude Code reads plugin declarations from the project's .claude/settings.json
// (https://code.claude.com/docs/en/settings-reference). Users enable their own
// plugins and marketplaces in the same maps, so SquadAI owns individual
// entries, recorded per key in the managed sidecar, never the maps as a whole.
// Claude Code itself fetches the marketplaces and installs the plugins.
const (
	keyEnabledPlugins = "enabledPlugins"
	keyMarketplaces   = "extraKnownMarketplaces"

	claudeSettingsDescription = "plugin:claude-settings"
)

var claudeOwnedKeys = []string{keyEnabledPlugins, keyMarketplaces}

type claudeMerge struct {
	existing  map[string]any
	doc       map[string]any
	owned     map[string][]string
	conflicts []fileutil.MergeConflict
}

// desiredClaude returns, per owned key, the entries the active claude_plugin
// declarations call for. When two declarations name the same marketplace the
// first by plugin name wins; the validator reports the clash.
func (i *Installer) desiredClaude() (map[string]map[string]any, error) {
	plugins := map[string]any{}
	markets := map[string]any{}
	for _, name := range sortedPluginNames(i.plugins) {
		def := i.plugins[name]
		if def.InstallMethod != "claude_plugin" || def.PluginID == "" || !i.active(def, domain.AgentClaudeCode) {
			continue
		}
		plugins[def.PluginID] = true
		_, market, ok := strings.Cut(def.PluginID, "@")
		if _, dup := markets[market]; !ok || dup || def.Marketplace == nil {
			continue
		}
		src, err := toJSONValue(def.Marketplace)
		if err != nil {
			return nil, fmt.Errorf("plugin %s marketplace: %w", name, err)
		}
		markets[market] = map[string]any{"source": src}
	}
	return map[string]map[string]any{keyEnabledPlugins: plugins, keyMarketplaces: markets}, nil
}

// mergeClaude computes the settings document Apply would write: declared
// entries added or updated where SquadAI owns them, entries it owned but no
// longer declares removed, and everything else left as found. It reads but
// never writes.
func (i *Installer) mergeClaude(path, projectDir string) (claudeMerge, error) {
	existing, err := fileutil.ReadJSONFile(path)
	if err != nil {
		return claudeMerge{}, fmt.Errorf("read %s: %w", path, err)
	}
	desired, err := i.desiredClaude()
	if err != nil {
		return claudeMerge{}, err
	}

	m := claudeMerge{existing: existing, doc: make(map[string]any, len(existing)+2), owned: map[string][]string{}}
	for k, v := range existing {
		m.doc[k] = v
	}
	for _, key := range claudeOwnedKeys {
		want := desired[key]
		var names []string
		tracked := false
		if projectDir != "" {
			names, tracked, err = managed.ReadManagedEntries(projectDir, relToProject(projectDir, path), key)
			if err != nil {
				return claudeMerge{}, fmt.Errorf("read managed entries: %w", err)
			}
		}

		current, isMap := m.doc[key].(map[string]any)
		if raw, present := m.doc[key]; present && !isMap {
			if len(want) > 0 {
				m.conflicts = append(m.conflicts, fileutil.MergeConflict{Key: key, UserValue: raw, IncomingValue: want})
			}
			continue
		}

		merged, conflicts, claimed, _ := fileutil.MergeJSON(current, want, names)
		for _, c := range conflicts {
			c.Key = key + "." + c.Key
			m.conflicts = append(m.conflicts, c)
		}
		pruned := false
		for _, n := range names {
			if _, keep := want[n]; !keep {
				if _, ok := merged[n]; ok {
					delete(merged, n)
					pruned = true
				}
			}
		}
		switch {
		case len(merged) > 0:
			m.doc[key] = merged
		case pruned:
			delete(m.doc, key)
		}
		if len(claimed) > 0 || tracked {
			m.owned[key] = append([]string{}, claimed...)
		}
	}
	sort.Slice(m.conflicts, func(a, b int) bool { return m.conflicts[a].Key < m.conflicts[b].Key })
	return m, nil
}

// planClaudeSettings plans the single settings write for every declaration.
// It also runs with nothing declared, so entries left by removed
// declarations are pruned.
func (i *Installer) planClaudeSettings(adapter domain.Adapter, projectDir string) ([]domain.PlannedAction, error) {
	path := adapter.ProjectConfigFile(projectDir)
	if path == "" {
		return nil, nil
	}
	m, err := i.mergeClaude(path, projectDir)
	if err != nil {
		return nil, err
	}
	desired, _ := i.desiredClaude()
	nothingDeclared := len(desired[keyEnabledPlugins]) == 0 && len(desired[keyMarketplaces]) == 0
	unchanged := len(m.conflicts) == 0 && reflect.DeepEqual(m.existing, m.doc)
	if m.existing == nil && nothingDeclared || unchanged && nothingDeclared {
		return nil, nil
	}

	action := domain.PlannedAction{
		ID:          fmt.Sprintf("%s-plugins", adapter.ID()),
		Agent:       adapter.ID(),
		Component:   domain.ComponentPlugins,
		TargetPath:  path,
		Description: claudeSettingsDescription,
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

func (i *Installer) applyClaudeSettings(action domain.PlannedAction) error {
	m, err := i.mergeClaude(action.TargetPath, i.projectDir)
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
	for _, key := range claudeOwnedKeys {
		names, ok := m.owned[key]
		if !ok {
			continue
		}
		if err := managed.WriteManagedEntries(i.projectDir, rel, key, names); err != nil {
			return fmt.Errorf("write managed entries: %w", err)
		}
	}
	return nil
}

func (i *Installer) renderClaudeSettings(action domain.PlannedAction) ([]byte, error) {
	m, err := i.mergeClaude(action.TargetPath, i.projectDir)
	if err != nil {
		return nil, err
	}
	return marshalSettings(m.doc)
}

// verifyClaudeSettings reports one result per active claude_plugin
// declaration: its plugin enabled and, when declared, its marketplace present
// as declared in the project settings.
func (i *Installer) verifyClaudeSettings(adapter domain.Adapter, projectDir string) ([]domain.VerifyResult, error) {
	path := adapter.ProjectConfigFile(projectDir)
	desired, err := i.desiredClaude()
	if err != nil || path == "" {
		return nil, err
	}
	doc, _ := fileutil.ReadJSONFile(path)
	enabled, _ := doc[keyEnabledPlugins].(map[string]any)
	markets, _ := doc[keyMarketplaces].(map[string]any)

	var results []domain.VerifyResult
	for _, name := range sortedPluginNames(i.plugins) {
		def := i.plugins[name]
		if def.InstallMethod != "claude_plugin" || def.PluginID == "" || !i.active(def, domain.AgentClaudeCode) {
			continue
		}
		r := domain.VerifyResult{Check: fmt.Sprintf("plugin-%s-enabled", name), Component: "plugins", Passed: enabled[def.PluginID] == true, Severity: domain.SeverityInfo}
		if !r.Passed {
			r.Severity = domain.SeverityError
			r.Message = fmt.Sprintf("plugin %s is not enabled in %s", def.PluginID, path)
		}
		results = append(results, r)

		_, market, _ := strings.Cut(def.PluginID, "@")
		want, declared := desired[keyMarketplaces][market]
		if def.Marketplace == nil || !declared {
			continue
		}
		r = domain.VerifyResult{Check: fmt.Sprintf("plugin-%s-marketplace", name), Component: "plugins", Passed: reflect.DeepEqual(markets[market], want), Severity: domain.SeverityInfo}
		if !r.Passed {
			r.Severity = domain.SeverityError
			r.Message = fmt.Sprintf("marketplace %s is not declared as configured in %s", market, path)
		}
		results = append(results, r)
	}
	return results, nil
}

// toJSONValue converts v to the generic types a parsed JSON file holds, so it
// compares with reflect.DeepEqual against what is on disk.
func toJSONValue(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(data, &out)
	return out, err
}

func marshalSettings(doc map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}
	return append(data, '\n'), nil
}

func relToProject(projectDir, path string) string {
	if rel, err := filepath.Rel(projectDir, path); err == nil {
		return rel
	}
	return path
}

func conflictsToDomain(in []fileutil.MergeConflict) []domain.Conflict {
	out := make([]domain.Conflict, 0, len(in))
	for _, c := range in {
		user, _ := json.Marshal(c.UserValue)
		incoming, _ := json.Marshal(c.IncomingValue)
		out = append(out, domain.Conflict{Key: c.Key, UserValue: string(user), IncomingValue: string(incoming)})
	}
	return out
}
