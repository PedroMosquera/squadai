package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// AgentTeamsEnvVar is the environment variable Claude Code reads to opt into
// the experimental Agent Teams runtime. SquadAI writes it under .claude/settings.json's
// "env" map when ProjectConfig.Claude.AgentTeams.Enabled is true.
const AgentTeamsEnvVar = "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"

// SetAgentTeamsEnv toggles the Agent Teams opt-in env var inside the project's
// .claude/settings.json. The function is idempotent and preserves all other
// keys in the file, including sibling entries inside the "env" map.
//
// When enabled is true, env[CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS] = "1" is set.
// When enabled is false, that single key is removed; if the env map becomes
// empty as a result, the env key itself is removed too. The settings file is
// only written when changes are needed.
//
// Returns true if the file was written (or created), false if no changes were
// required (idempotent skip).
func SetAgentTeamsEnv(projectDir string, enabled bool) (changed bool, err error) {
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	existing := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read claude settings: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return false, fmt.Errorf("parse claude settings: %w", err)
		}
	}

	envMap, _ := existing["env"].(map[string]any)
	currentVal, _ := envMap[AgentTeamsEnvVar].(string)
	currentlyOn := currentVal == "1"

	if currentlyOn == enabled {
		return false, nil
	}

	if enabled {
		if envMap == nil {
			envMap = make(map[string]any)
		}
		envMap[AgentTeamsEnvVar] = "1"
		existing["env"] = envMap
	} else {
		delete(envMap, AgentTeamsEnvVar)
		if len(envMap) == 0 {
			delete(existing, "env")
		} else {
			existing["env"] = envMap
		}
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal claude settings: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, fmt.Errorf("create .claude dir: %w", err)
	}
	if err := writeAtomicFile(settingsPath, out); err != nil {
		return false, err
	}
	return true, nil
}

// AgentTeamsEnabled reports whether Claude Code's settings.json in projectDir
// currently has the Agent Teams env var set. Used by the doctor check.
func AgentTeamsEnabled(projectDir string) (bool, error) {
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read claude settings: %w", err)
	}
	if len(data) == 0 {
		return false, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, fmt.Errorf("parse claude settings: %w", err)
	}

	envMap, _ := doc["env"].(map[string]any)
	val, _ := envMap[AgentTeamsEnvVar].(string)
	return val == "1", nil
}

// writeAtomicFile writes data to path via temp-file + rename. Local helper to
// keep the package free of cross-package deps for this small write.
func writeAtomicFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".squadai-settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp settings: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename settings file: %w", err)
	}
	tmpName = ""
	return nil
}

// SetHooks merges SquadAI-managed hooks into .claude/settings.json.
// Each hook entry is uniquely identified by its Command string. Existing user
// hooks are preserved. Matcher groups are matched by their Matcher field
// (empty matches empty). Returns true when the file was written.
func SetHooks(projectDir string, want domain.HooksConfig) (bool, error) {
	return SyncHooks(projectDir, want, nil)
}

// SyncHooks is SetHooks plus ownership: entries whose command satisfies owned
// are rewritten to match want and removed when want no longer lists them.
// A nil owned behaves exactly like SetHooks.
func SyncHooks(projectDir string, want domain.HooksConfig, owned func(command string) bool) (bool, error) {
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	existing := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read claude settings: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return false, fmt.Errorf("parse claude settings: %w", err)
		}
	}

	merged, changed := ReconcileHooks(DecodeHooksMap(existing["hooks"]), want, owned)
	if !changed {
		return false, nil
	}

	if len(merged) == 0 {
		delete(existing, "hooks")
	} else {
		existing["hooks"] = EncodeHooksMap(merged)
	}
	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal claude settings: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, fmt.Errorf("create .claude dir: %w", err)
	}
	return true, writeAtomicFile(settingsPath, out)
}

// HooksInstalled returns true when every hook entry in want is present in
// .claude/settings.json. An absent or unparseable settings file returns false.
func HooksInstalled(projectDir string, want domain.HooksConfig) (bool, error) {
	if len(want) == 0 {
		return true, nil
	}

	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read claude settings: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, nil
	}

	installed := DecodeHooksMap(doc["hooks"])
	for event, wantMatchers := range want {
		for _, wm := range wantMatchers {
			existingMatchers := installed[event]
			idx := FindMatcherIdx(existingMatchers, wm.Matcher)
			if idx < 0 {
				return false, nil
			}
			for _, we := range wm.Hooks {
				if !HasCommand(existingMatchers[idx].Hooks, we.Command) {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

// HooksInSync reports whether SyncHooks with the same arguments would leave
// .claude/settings.json unchanged. An unparseable settings file is out of sync
// only when want is non-empty, so a broken file nobody asked SquadAI to touch
// does not turn into a failing apply.
func HooksInSync(projectDir string, want domain.HooksConfig, owned func(command string) bool) (bool, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, ".claude", "settings.json"))
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read claude settings: %w", err)
	}
	doc := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return len(want) == 0, nil
		}
	}
	_, changed := ReconcileHooks(DecodeHooksMap(doc["hooks"]), want, owned)
	return !changed, nil
}

// ReconcileHooks merges want into existing without mutating it. Entries are
// keyed by command within a matcher group. Commands not satisfying owned are
// only ever added; owned ones are also updated in place and removed when want
// drops them, along with any group or event left empty by that removal.
func ReconcileHooks(existing map[string][]domain.HookMatcher, want domain.HooksConfig, owned func(command string) bool) (map[string][]domain.HookMatcher, bool) {
	isOwned := func(cmd string) bool { return owned != nil && owned(cmd) }
	out := make(map[string][]domain.HookMatcher, len(existing))
	for ev, ms := range existing {
		cp := make([]domain.HookMatcher, len(ms))
		for i, m := range ms {
			cp[i] = domain.HookMatcher{Matcher: m.Matcher, Hooks: append([]domain.HookEntry(nil), m.Hooks...)}
		}
		out[ev] = cp
	}

	changed := false
	wanted := map[[3]string]bool{}
	for event, wantMatchers := range want {
		for _, wm := range wantMatchers {
			for _, we := range wm.Hooks {
				wanted[[3]string{event, wm.Matcher, we.Command}] = true
			}
			idx := FindMatcherIdx(out[event], wm.Matcher)
			if idx < 0 {
				out[event] = append(out[event], domain.HookMatcher{Matcher: wm.Matcher, Hooks: append([]domain.HookEntry(nil), wm.Hooks...)})
				changed = true
				continue
			}
			group := &out[event][idx]
			for _, we := range wm.Hooks {
				i := commandIdx(group.Hooks, we.Command)
				switch {
				case i < 0:
					group.Hooks = append(group.Hooks, we)
					changed = true
				case isOwned(we.Command) && !sameHook(group.Hooks[i], we):
					group.Hooks[i] = we
					changed = true
				}
			}
		}
	}

	if owned == nil {
		return out, changed
	}
	for event, ms := range out {
		keptGroups := ms[:0]
		for _, m := range ms {
			kept := m.Hooks[:0]
			for _, h := range m.Hooks {
				if isOwned(h.Command) && !wanted[[3]string{event, m.Matcher, h.Command}] {
					changed = true
					continue
				}
				kept = append(kept, h)
			}
			if len(kept) == 0 && len(m.Hooks) > 0 {
				continue
			}
			m.Hooks = kept
			keptGroups = append(keptGroups, m)
		}
		if len(keptGroups) == 0 {
			delete(out, event)
			continue
		}
		out[event] = keptGroups
	}
	return out, changed
}

func commandIdx(entries []domain.HookEntry, cmd string) int {
	for i, e := range entries {
		if e.Command == cmd {
			return i
		}
	}
	return -1
}

func sameHook(a, b domain.HookEntry) bool {
	return a.Type == b.Type && a.Command == b.Command && a.Async == b.Async &&
		a.Timeout == b.Timeout && a.OnFailure == b.OnFailure
}

// DecodeHooksMap normalises the raw JSON hooks value into a typed map.
// Exported so the hooks component installer can reuse it for RenderContent.
func DecodeHooksMap(raw any) map[string][]domain.HookMatcher {
	result := make(map[string][]domain.HookMatcher)
	if raw == nil {
		return result
	}
	// Re-marshal then unmarshal to convert map[string]any → typed structs.
	b, err := json.Marshal(raw)
	if err != nil {
		return result
	}
	var typed map[string][]domain.HookMatcher
	if err := json.Unmarshal(b, &typed); err != nil {
		return result
	}
	return typed
}

// EncodeHooksMap converts a typed hooks map back to the any-typed form needed
// for the JSON settings document.
// Exported so the hooks component installer can reuse it for RenderContent.
func EncodeHooksMap(m map[string][]domain.HookMatcher) any {
	b, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// FindMatcherIdx returns the index of the HookMatcher in ms whose Matcher
// field equals target, or -1 if none is found.
// Exported so the hooks component installer can reuse it for RenderContent.
func FindMatcherIdx(ms []domain.HookMatcher, target string) int {
	for i, m := range ms {
		if m.Matcher == target {
			return i
		}
	}
	return -1
}

// HasCommand reports whether any HookEntry in entries has the given command.
// Exported so the hooks component installer can reuse it for RenderContent.
func HasCommand(entries []domain.HookEntry, cmd string) bool {
	for _, e := range entries {
		if e.Command == cmd {
			return true
		}
	}
	return false
}

// WriteDefaultAgentSettings writes (or merges) .claude/settings.json in projectDir
// to set the given agentName as the default Claude Code agent.
//
// If the file already exists, existing keys are preserved. Only the "agent" key
// is added or updated.
func WriteDefaultAgentSettings(projectDir, agentName string) error {
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	// Read existing settings if present.
	existing := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read claude settings: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse claude settings: %w", err)
		}
	}

	// Set (or overwrite) the agent key.
	existing["agent"] = agentName

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claude settings: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(settingsPath), ".squadai-settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp settings: %w", err)
	}
	if err := os.Rename(tmpName, settingsPath); err != nil {
		return fmt.Errorf("rename settings file: %w", err)
	}
	tmpName = "" // prevent deferred cleanup
	return nil
}
