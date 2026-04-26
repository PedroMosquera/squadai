package doctor

import (
	"context"
	"errors"
	"fmt"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

const catAgents = "AI Agents"

// runAgents checks each configured adapter for installation and config presence.
func (d *Doctor) runAgents(ctx context.Context) []CheckResult {
	if len(d.adapters) == 0 {
		return []CheckResult{skip(catAgents, "agents", "no adapters configured")}
	}

	var results []CheckResult
	for _, adapter := range d.adapters {
		agentID := string(adapter.ID())
		installed, configFound, err := adapter.Detect(ctx, d.homeDir)
		if err != nil {
			results = append(results, warn(catAgents, agentID,
				fmt.Sprintf("%s detection error: %v", agentID, err), "", ""))
			continue
		}

		if !installed {
			results = append(results, skip(catAgents, agentID,
				fmt.Sprintf("%s not installed", agentID)))
			continue
		}

		// Look up binary path for detail.
		binaryName := agentBinaryName(agentID)
		binPath := ""
		if binaryName != "" {
			if p, lookErr := d.looker.LookPath(binaryName); lookErr == nil {
				binPath = p
			}
		}

		if !configFound {
			msg := fmt.Sprintf("%s binary found but config dir missing", agentID)
			if binPath != "" {
				msg = fmt.Sprintf("%s found at %s but config dir missing", agentID, binPath)
			}
			results = append(results, warn(catAgents, agentID, msg, binPath,
				fmt.Sprintf("Run 'squadai apply' to create the config for %s", agentID)))
			continue
		}

		msg := fmt.Sprintf("%s detected", agentID)
		if binPath != "" {
			msg = fmt.Sprintf("%s detected at %s", agentID, binPath)
		}
		results = append(results, pass(catAgents, agentID, msg, binPath))

		// Per-agent feature checks.
		if adapter.ID() == domain.AgentClaudeCode {
			results = append(results, d.checkAgentTeams())
			results = append(results, d.checkHooks()...)
		}
	}
	return results
}

// checkAgentTeams reports whether the Agent Teams runtime opt-in matches the
// project's configured desired state. The check has three outcomes:
//
//	pass: desired state matches what's in .claude/settings.json
//	warn: drift detected (config wants enabled but env var missing, or vice versa)
//	skip: project config not loadable (a separate check already flags this)
func (d *Doctor) checkAgentTeams() CheckResult {
	proj, err := config.LoadProject(d.projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return skip(catAgents, "claude.agent_teams",
				"Agent Teams check skipped — project config missing")
		}
		return warn(catAgents, "claude.agent_teams",
			fmt.Sprintf("Agent Teams check failed: %v", err), "", "")
	}

	desired := proj.Claude.AgentTeams.Enabled
	got, err := claude.AgentTeamsEnabled(d.projectDir)
	if err != nil {
		return warn(catAgents, "claude.agent_teams",
			fmt.Sprintf("read .claude/settings.json: %v", err), "", "")
	}

	switch {
	case desired && got:
		return pass(catAgents, "claude.agent_teams",
			"Agent Teams enabled (CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1)",
			"enabled")
	case !desired && !got:
		return pass(catAgents, "claude.agent_teams",
			"Agent Teams disabled (default)",
			"disabled")
	case desired && !got:
		return warn(catAgents, "claude.agent_teams",
			"Agent Teams enabled in config but env var missing in .claude/settings.json",
			"drift",
			"Run 'squadai apply' to inject CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	default: // !desired && got
		return warn(catAgents, "claude.agent_teams",
			"Agent Teams env var present in .claude/settings.json but not enabled in config",
			"drift",
			"Run 'squadai apply' to remove the stale env var, or set claude.agent_teams.enabled=true in project.json")
	}
}

// checkHooks reports whether all hooks declared in project.json are present in
// .claude/settings.json. Returns nil when no hooks are configured.
func (d *Doctor) checkHooks() []CheckResult {
	proj, err := config.LoadProject(d.projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil
		}
		return []CheckResult{warn(catAgents, "claude.hooks",
			fmt.Sprintf("hooks check failed: %v", err), "", "")}
	}

	if len(proj.Hooks) == 0 {
		return nil
	}

	installed, err := claude.HooksInstalled(d.projectDir, proj.Hooks)
	if err != nil {
		return []CheckResult{warn(catAgents, "claude.hooks",
			fmt.Sprintf("read .claude/settings.json: %v", err), "", "")}
	}

	if installed {
		return []CheckResult{pass(catAgents, "claude.hooks",
			fmt.Sprintf("%d hook event(s) installed in .claude/settings.json", len(proj.Hooks)),
			"installed")}
	}
	return []CheckResult{warn(catAgents, "claude.hooks",
		"one or more hooks from project.json are missing from .claude/settings.json",
		"drift",
		"Run 'squadai apply' to merge required hooks into .claude/settings.json")}
}

// agentBinaryName returns the primary binary name for a given agent ID string.
func agentBinaryName(agentID string) string {
	names := map[string]string{
		"claude":   "claude",
		"cursor":   "cursor",
		"opencode": "opencode",
		"windsurf": "windsurf",
		"vscode":   "code",
	}
	if name, ok := names[agentID]; ok {
		return name
	}
	return agentID
}
