// Package agent_teams installs the Claude Code Agent Teams runtime opt-in.
//
// When MergedConfig.Claude.AgentTeams.Enabled is true, the installer ensures
// .claude/settings.json contains env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1.
// When the flag is false, the installer removes that env var so toggling off
// is a clean teardown.
//
// The installer only emits actions for the Claude Code adapter. Other adapters
// are silently skipped — Agent Teams is a Claude Code feature.
package agent_teams

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Installer implements domain.ComponentInstaller for the Agent Teams opt-in.
type Installer struct {
	desired bool
}

// New returns an Installer driven by the merged Claude.AgentTeams.Enabled flag.
func New(enabled bool) *Installer {
	return &Installer{desired: enabled}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentAgentTeams
}

// Plan computes whether the env var needs to be added, removed, or left alone.
// Always returns at most one action and only for the Claude Code adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if adapter.ID() != domain.AgentClaudeCode {
		return nil, nil
	}

	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	current, err := readAgentTeamsState(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("read claude settings: %w", err)
	}

	actionID := "claude-agent-teams"
	if current == i.desired {
		return []domain.PlannedAction{{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentAgentTeams,
			Action:      domain.ActionSkip,
			TargetPath:  settingsPath,
			Description: agentTeamsDescription(i.desired, true),
		}}, nil
	}

	actionType := domain.ActionUpdate
	// File does not exist yet → we'll create it (only meaningful when enabling).
	if _, statErr := os.Stat(settingsPath); os.IsNotExist(statErr) && i.desired {
		actionType = domain.ActionCreate
	}

	return []domain.PlannedAction{{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentAgentTeams,
		Action:      actionType,
		TargetPath:  settingsPath,
		Description: agentTeamsDescription(i.desired, false),
	}}, nil
}

// Apply toggles the env var to match the desired state. Idempotent.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}
	projectDir := filepath.Dir(filepath.Dir(action.TargetPath)) // strip /.claude/settings.json
	if _, err := claude.SetAgentTeamsEnv(projectDir, i.desired); err != nil {
		return fmt.Errorf("set agent teams env: %w", err)
	}
	return nil
}

// Verify reports whether the env var matches the desired state.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if adapter.ID() != domain.AgentClaudeCode {
		return nil, nil
	}

	got, err := claude.AgentTeamsEnabled(projectDir)
	if err != nil {
		return []domain.VerifyResult{{
			Check:     "agent-teams-state",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: string(domain.ComponentAgentTeams),
			Message:   fmt.Sprintf("read claude settings: %v", err),
		}}, nil
	}

	if got != i.desired {
		return []domain.VerifyResult{{
			Check:     "agent-teams-state",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: string(domain.ComponentAgentTeams),
			Message:   fmt.Sprintf("agent teams env var mismatch: want %v, got %v", i.desired, got),
		}}, nil
	}

	return []domain.VerifyResult{{
		Check:     "agent-teams-state",
		Passed:    true,
		Severity:  domain.SeverityInfo,
		Component: string(domain.ComponentAgentTeams),
	}}, nil
}

// RenderContent returns the bytes Apply would write, computed from the current
// on-disk file. Used by the diff renderer.
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

	envMap, _ := existing["env"].(map[string]any)
	if i.desired {
		if envMap == nil {
			envMap = make(map[string]any)
		}
		envMap[claude.AgentTeamsEnvVar] = "1"
		existing["env"] = envMap
	} else {
		delete(envMap, claude.AgentTeamsEnvVar)
		if len(envMap) == 0 {
			delete(existing, "env")
		} else {
			existing["env"] = envMap
		}
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal claude settings: %w", err)
	}
	return append(out, '\n'), nil
}

// readAgentTeamsState reads the current value of the Agent Teams env var from
// the Claude settings file. Returns false if the file does not exist or the
// var is missing.
func readAgentTeamsState(settingsPath string) (bool, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	envMap, _ := doc["env"].(map[string]any)
	val, _ := envMap[claude.AgentTeamsEnvVar].(string)
	return val == "1", nil
}

// agentTeamsDescription returns the Description string used in PlannedActions.
// settled indicates ActionSkip — the desired state already matches disk.
func agentTeamsDescription(desired, settled bool) string {
	switch {
	case settled && desired:
		return "Agent Teams already enabled"
	case settled && !desired:
		return "Agent Teams already disabled"
	case desired:
		return "enable Claude Code Agent Teams (set " + claude.AgentTeamsEnvVar + "=1)"
	default:
		return "disable Claude Code Agent Teams (remove " + claude.AgentTeamsEnvVar + ")"
	}
}
