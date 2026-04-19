package doctor

import (
	"context"
	"fmt"
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
	}
	return results
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
