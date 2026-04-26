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
	if len(want) == 0 {
		return false, nil
	}

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

	// Decode existing hooks map into a normalised Go structure.
	existingHooks := DecodeHooksMap(existing["hooks"])
	changed := false

	for event, wantMatchers := range want {
		for _, wm := range wantMatchers {
			existingMatchers := existingHooks[event]
			idx := FindMatcherIdx(existingMatchers, wm.Matcher)
			if idx < 0 {
				existingHooks[event] = append(existingHooks[event], wm)
				changed = true
				continue
			}
			for _, wantEntry := range wm.Hooks {
				if !HasCommand(existingMatchers[idx].Hooks, wantEntry.Command) {
					existingHooks[event][idx].Hooks = append(existingHooks[event][idx].Hooks, wantEntry)
					changed = true
				}
			}
		}
	}

	if !changed {
		return false, nil
	}

	existing["hooks"] = EncodeHooksMap(existingHooks)
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
