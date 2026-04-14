package cli

import "github.com/PedroMosquera/squadai/internal/domain"

// DefaultMCPServers returns the recommended MCP server configurations.
// Context7 is included and enabled by default.
func DefaultMCPServers() map[string]domain.MCPServerDef {
	return map[string]domain.MCPServerDef{
		"context7": {
			Type:    "local",
			Command: []string{"npx", "-y", "@upstash/context7-mcp@latest"},
			Enabled: true,
		},
	}
}

// AvailablePlugins returns the full catalog of available plugins.
func AvailablePlugins() map[string]domain.PluginDef {
	return map[string]domain.PluginDef{
		"superpowers": {
			Description:         "Advanced AI coding capabilities with autonomous workflows",
			Enabled:             false,
			SupportedAgents:     []string{"claude-code", "opencode", "cursor"},
			InstallMethod:       "claude_plugin",
			PluginID:            "superpowers@claude-plugins-official",
			ExcludesMethodology: "tdd",
		},
		"code-simplifier": {
			Description:     "Simplifies and refactors complex code",
			Enabled:         false,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "code-simplifier@anthropic",
		},
		"code-review": {
			Description:     "Automated code review with actionable feedback",
			Enabled:         false,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "code-review@anthropic",
		},
		"frontend-design": {
			Description:     "AI-assisted frontend design and component generation",
			Enabled:         false,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "frontend-design@anthropic",
		},
	}
}

// FilterPlugins filters the plugin catalog based on detected agents and selected methodology.
// Plugins are excluded if:
//   - Their ExcludesMethodology matches the selected methodology
//   - None of their SupportedAgents are among the detected adapters
func FilterPlugins(all map[string]domain.PluginDef, detectedAgents []domain.Adapter, methodology domain.Methodology) map[string]domain.PluginDef {
	// Build set of detected agent IDs
	agentIDs := make(map[string]bool)
	for _, a := range detectedAgents {
		agentIDs[string(a.ID())] = true
	}

	result := make(map[string]domain.PluginDef)
	for name, plugin := range all {
		// Check methodology exclusion
		if plugin.ExcludesMethodology != "" && plugin.ExcludesMethodology == string(methodology) {
			continue
		}
		// Check agent support
		hasAgent := false
		for _, agentID := range plugin.SupportedAgents {
			if agentIDs[agentID] {
				hasAgent = true
				break
			}
		}
		if !hasAgent {
			continue
		}
		result[name] = plugin
	}
	return result
}
