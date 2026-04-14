package cli

import "github.com/PedroMosquera/squadai/internal/domain"

// ModelConfig holds the model settings for a specific agent at a specific tier.
type ModelConfig struct {
	// Settings are key-value pairs written to the agent's settings file.
	// Only for agents that support file-based model config (OpenCode, Claude Code).
	Settings map[string]interface{}
	// PromptHint is a recommendation string injected into the system prompt
	// for agents that use UI-based model selection (VS Code, Cursor, Windsurf).
	PromptHint string
}

// ModelsForTier returns the model configuration for a given tier and agent.
func ModelsForTier(tier domain.ModelTier, agentID domain.AgentID) ModelConfig {
	switch tier {
	case domain.ModelTierPerformance:
		return performanceModels(agentID)
	case domain.ModelTierStarter:
		return starterModels(agentID)
	case domain.ModelTierManual:
		return ModelConfig{Settings: map[string]interface{}{}}
	default: // balanced
		return balancedModels(agentID)
	}
}

func balancedModels(agentID domain.AgentID) ModelConfig {
	switch agentID {
	case domain.AgentOpenCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": "anthropic/claude-sonnet-4-20250514"},
		}
	case domain.AgentClaudeCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": "claude-sonnet-4-20250514"},
		}
	case domain.AgentVSCodeCopilot:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use Claude Sonnet 4 or GPT-4.1-mini. Use flagship models only for architecture decisions.",
		}
	case domain.AgentCursor:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use Claude Sonnet 4 for complex tasks, GPT-4.1-mini for edits and quick fixes.",
		}
	case domain.AgentWindsurf:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use the default Cascade model. Switch to premium only for complex refactors.",
		}
	default:
		return ModelConfig{Settings: map[string]interface{}{}}
	}
}

func performanceModels(agentID domain.AgentID) ModelConfig {
	switch agentID {
	case domain.AgentOpenCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": "anthropic/claude-sonnet-4-5"},
		}
	case domain.AgentClaudeCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": "claude-sonnet-4-5"},
		}
	case domain.AgentVSCodeCopilot:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use Claude Sonnet 4.5 or GPT-4.1 for all tasks",
		}
	case domain.AgentCursor:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use Claude Sonnet 4.5 or GPT-4.1 for all tasks",
		}
	case domain.AgentWindsurf:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use the most capable model available (Claude Sonnet 4.5)",
		}
	default:
		return ModelConfig{Settings: map[string]interface{}{}}
	}
}

func starterModels(agentID domain.AgentID) ModelConfig {
	switch agentID {
	case domain.AgentOpenCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": "anthropic/claude-haiku-3-5"},
		}
	case domain.AgentClaudeCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": "claude-haiku-3-5"},
		}
	case domain.AgentVSCodeCopilot:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use GPT-4.1-mini or Claude Haiku for all tasks to minimize cost.",
		}
	case domain.AgentCursor:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use GPT-4.1-mini for all tasks. Avoid premium models to stay within budget.",
		}
	case domain.AgentWindsurf:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: "Use the free/default model tier. Premium models are not needed for most tasks.",
		}
	default:
		return ModelConfig{Settings: map[string]interface{}{}}
	}
}
