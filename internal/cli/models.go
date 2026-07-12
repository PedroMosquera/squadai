package cli

import (
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/model"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// ModelConfig holds the model settings for a specific agent at a specific tier.
type ModelConfig struct {
	// Settings are key-value pairs written to the agent's settings file.
	// Only for agents that support file-based model config (OpenCode, Claude Code).
	Settings map[string]interface{}
	// PromptHint is a recommendation string injected into the system prompt
	// for agents that use UI-based model selection (VS Code, Cursor, Windsurf).
	PromptHint string
}

// ModelsForTier returns the model configuration for a given tier and agent,
// resolved through the unified model catalog. The manual tier returns empty
// settings (the user picks models themselves).
func ModelsForTier(tier domain.ModelTier, agentID domain.AgentID) ModelConfig {
	return ModelsForTierWithCatalog(tier, agentID, modelcatalog.Default())
}

// ModelsForTierWithCatalog is ModelsForTier against an explicit catalog.
func ModelsForTierWithCatalog(tier domain.ModelTier, agentID domain.AgentID, cat *modelcatalog.Catalog) ModelConfig {
	if tier == domain.ModelTierManual {
		return ModelConfig{Settings: map[string]interface{}{}}
	}
	catalogTier := string(model.TierFromModelTier(tier))

	switch agentID {
	case domain.AgentOpenCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": cat.TierModel(string(agentID), catalogTier)},
		}
	case domain.AgentPi:
		return ModelConfig{
			Settings: map[string]interface{}{"defaultModel": cat.TierModel(string(agentID), catalogTier)},
		}
	case domain.AgentClaudeCode:
		return ModelConfig{
			Settings: map[string]interface{}{"model": cat.TierModel(string(agentID), catalogTier)},
		}
	case domain.AgentVSCodeCopilot, domain.AgentCursor, domain.AgentWindsurf, domain.AgentCodex:
		return ModelConfig{
			Settings:   map[string]interface{}{},
			PromptHint: cat.Hint(string(agentID), catalogTier),
		}
	default:
		return ModelConfig{Settings: map[string]interface{}{}}
	}
}
