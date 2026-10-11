// Package hooks installs Claude Code hook definitions into .claude/settings.json.
//
// Hooks are additive: SquadAI-managed entries are merged into any existing user
// hooks. Deduplication is by Command string within a matched Matcher group.
// Budget hooks (budget.go) and the memory capture hook (memory.go) are the
// exception: SquadAI owns them outright, so they are also updated and removed
// to follow usage.enforcement and memory.capture_on_stop.
// Codex gets budget and memory capture hooks (codex_budget.go), Pi budget
// enforcement only (pi_budget.go); all other adapters are silently skipped.
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
	usage   domain.UsageConfig
	// captureOnStop drives the Codex memory hook, see WithMemoryCapture.
	captureOnStop bool
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
	if budgetHarness(adapter.ID()) {
		return i.planHarnessBudget(adapter, projectDir)
	}
	if adapter.ID() != domain.AgentClaudeCode {
		return nil, nil
	}
	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")

	installed, err := claudeadapter.HooksInSync(projectDir, i.desired, ownedCommand)
	if err != nil {
		return nil, fmt.Errorf("check hooks installed: %w", err)
	}
	if len(i.desired) == 0 && installed {
		return nil, nil
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
	desc := fmt.Sprintf("merge %d hook event(s) into .claude/settings.json", len(i.desired))
	if len(i.desired) == 0 {
		desc = "remove SquadAI-owned hooks from .claude/settings.json"
	}

	return []domain.PlannedAction{{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentHooks,
		Action:      actionType,
		TargetPath:  settingsPath,
		Description: desc,
	}}, nil
}

// Apply merges the desired hooks into .claude/settings.json.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}
	if budgetHarness(action.Agent) {
		return i.applyHarnessBudget(action)
	}
	projectDir := filepath.Dir(filepath.Dir(action.TargetPath)) // strip /.claude/settings.json
	if _, err := claudeadapter.SyncHooks(projectDir, i.desired, ownedCommand); err != nil {
		return fmt.Errorf("set hooks: %w", err)
	}
	return nil
}

// Verify reports whether every desired hook entry is present in settings.json.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if budgetHarness(adapter.ID()) {
		return i.verifyHarnessBudget(adapter, projectDir), nil
	}
	if adapter.ID() != domain.AgentClaudeCode {
		return nil, nil
	}
	if len(i.desired) == 0 {
		return nil, nil
	}

	installed, err := claudeadapter.HooksInSync(projectDir, i.desired, ownedCommand)
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
	if budgetHarness(action.Agent) {
		return i.renderHarnessBudget(action)
	}
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

	merged, _ := claudeadapter.ReconcileHooks(claudeadapter.DecodeHooksMap(existing["hooks"]), i.desired, ownedCommand)
	if len(merged) == 0 {
		delete(existing, "hooks")
	} else {
		existing["hooks"] = claudeadapter.EncodeHooksMap(merged)
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal claude settings: %w", err)
	}
	return append(out, '\n'), nil
}
