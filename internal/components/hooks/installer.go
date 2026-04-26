// Package hooks installs Claude Code hook definitions into .claude/settings.json.
//
// Hooks are additive: SquadAI-managed entries are merged into any existing user
// hooks. Deduplication is by Command string within a matched Matcher group.
// Only the Claude Code adapter emits actions; all other adapters are silently
// skipped — hooks are a Claude Code-specific feature.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	claudeadapter "github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Installer implements domain.ComponentInstaller for Claude Code hooks.
type Installer struct {
	desired domain.HooksConfig
}

// New returns an Installer configured with the desired hooks state.
func New(desired domain.HooksConfig) *Installer {
	return &Installer{desired: desired}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentHooks
}

// Plan computes whether the hooks need to be added to settings.json.
// Returns at most one action, only for the Claude Code adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if adapter.ID() != domain.AgentClaudeCode {
		return nil, nil
	}
	if len(i.desired) == 0 {
		return nil, nil
	}

	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	installed, err := claudeadapter.HooksInstalled(projectDir, i.desired)
	if err != nil {
		return nil, fmt.Errorf("check hooks installed: %w", err)
	}

	actionID := "claude-hooks"
	if installed {
		return []domain.PlannedAction{{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentHooks,
			Action:      domain.ActionSkip,
			TargetPath:  settingsPath,
			Description: "hooks already installed",
		}}, nil
	}

	actionType := domain.ActionUpdate
	if _, statErr := os.Stat(settingsPath); os.IsNotExist(statErr) {
		actionType = domain.ActionCreate
	}

	return []domain.PlannedAction{{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentHooks,
		Action:      actionType,
		TargetPath:  settingsPath,
		Description: fmt.Sprintf("merge %d hook event(s) into .claude/settings.json", len(i.desired)),
	}}, nil
}

// Apply merges the desired hooks into .claude/settings.json.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}
	projectDir := filepath.Dir(filepath.Dir(action.TargetPath)) // strip /.claude/settings.json
	if _, err := claudeadapter.SetHooks(projectDir, i.desired); err != nil {
		return fmt.Errorf("set hooks: %w", err)
	}
	return nil
}

// Verify reports whether every desired hook entry is present in settings.json.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if adapter.ID() != domain.AgentClaudeCode {
		return nil, nil
	}
	if len(i.desired) == 0 {
		return nil, nil
	}

	installed, err := claudeadapter.HooksInstalled(projectDir, i.desired)
	if err != nil {
		return []domain.VerifyResult{{
			Check:     "hooks-installed",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: string(domain.ComponentHooks),
			Message:   fmt.Sprintf("read hooks state: %v", err),
		}}, nil
	}

	if !installed {
		return []domain.VerifyResult{{
			Check:     "hooks-installed",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: string(domain.ComponentHooks),
			Message:   "one or more required hooks are missing from .claude/settings.json",
		}}, nil
	}

	return []domain.VerifyResult{{
		Check:     "hooks-installed",
		Passed:    true,
		Severity:  domain.SeverityInfo,
		Component: string(domain.ComponentHooks),
	}}, nil
}

// RenderContent returns the JSON bytes that Apply would write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	existing := make(map[string]any)
	data, err := os.ReadFile(action.TargetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read claude settings: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("parse claude settings: %w", err)
		}
	}

	// Simulate the additive merge that SetHooks performs.
	existingHooks := claudeadapter.DecodeHooksMap(existing["hooks"])
	for event, wantMatchers := range i.desired {
		for _, wm := range wantMatchers {
			existingMatchers := existingHooks[event]
			idx := claudeadapter.FindMatcherIdx(existingMatchers, wm.Matcher)
			if idx < 0 {
				existingHooks[event] = append(existingHooks[event], wm)
				continue
			}
			for _, wantEntry := range wm.Hooks {
				if !claudeadapter.HasCommand(existingMatchers[idx].Hooks, wantEntry.Command) {
					existingHooks[event][idx].Hooks = append(existingHooks[event][idx].Hooks, wantEntry)
				}
			}
		}
	}
	existing["hooks"] = claudeadapter.EncodeHooksMap(existingHooks)

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal claude settings: %w", err)
	}
	return append(out, '\n'), nil
}
