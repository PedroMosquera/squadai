package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
