package cli

import (
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── ModelsForTier ────────────────────────────────────────────────────────────

func TestModelsForTier_Balanced_OpenCode(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierBalanced, domain.AgentOpenCode)

	model, ok := cfg.Settings["model"]
	if !ok {
		t.Fatal("balanced/opencode should have 'model' in Settings")
	}
	if model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("Settings[model] = %q, want %q", model, "anthropic/claude-sonnet-4-20250514")
	}
}

func TestModelsForTier_Performance_OpenCode(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierPerformance, domain.AgentOpenCode)

	model, ok := cfg.Settings["model"]
	if !ok {
		t.Fatal("performance/opencode should have 'model' in Settings")
	}
	if model != "anthropic/claude-sonnet-4-5" {
		t.Errorf("Settings[model] = %q, want %q", model, "anthropic/claude-sonnet-4-5")
	}
}

func TestModelsForTier_Starter_OpenCode(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierStarter, domain.AgentOpenCode)

	model, ok := cfg.Settings["model"]
	if !ok {
		t.Fatal("starter/opencode should have 'model' in Settings")
	}
	if model != "anthropic/claude-haiku-3-5" {
		t.Errorf("Settings[model] = %q, want %q", model, "anthropic/claude-haiku-3-5")
	}
}

func TestModelsForTier_Manual_ReturnsEmpty(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierManual, domain.AgentOpenCode)

	if len(cfg.Settings) != 0 {
		t.Errorf("manual tier should return empty Settings, got: %v", cfg.Settings)
	}
	if cfg.PromptHint != "" {
		t.Errorf("manual tier should return empty PromptHint, got: %q", cfg.PromptHint)
	}
}

func TestModelsForTier_Balanced_VSCode_HasPromptHint(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierBalanced, domain.AgentVSCodeCopilot)

	if cfg.PromptHint == "" {
		t.Error("balanced/vscode-copilot should return a non-empty PromptHint")
	}
}

func TestModelsForTier_Performance_Claude(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierPerformance, domain.AgentClaudeCode)

	model, ok := cfg.Settings["model"]
	if !ok {
		t.Fatal("performance/claude-code should have 'model' in Settings")
	}
	// Must be bare (no "anthropic/" prefix) for Claude Code.
	if model != "claude-sonnet-4-5" {
		t.Errorf("Settings[model] = %q, want %q", model, "claude-sonnet-4-5")
	}
}

func TestModelsForTier_Starter_Claude(t *testing.T) {
	cfg := ModelsForTier(domain.ModelTierStarter, domain.AgentClaudeCode)

	model, ok := cfg.Settings["model"]
	if !ok {
		t.Fatal("starter/claude-code should have 'model' in Settings")
	}
	if model != "claude-haiku-3-5" {
		t.Errorf("Settings[model] = %q, want %q", model, "claude-haiku-3-5")
	}
}

func TestModelsForTier_AllTiers_AllAgents_NoNilMaps(t *testing.T) {
	tiers := []domain.ModelTier{
		domain.ModelTierBalanced,
		domain.ModelTierPerformance,
		domain.ModelTierStarter,
		domain.ModelTierManual,
	}
	agents := []domain.AgentID{
		domain.AgentOpenCode,
		domain.AgentClaudeCode,
		domain.AgentVSCodeCopilot,
		domain.AgentCursor,
		domain.AgentWindsurf,
	}

	for _, tier := range tiers {
		for _, agent := range agents {
			cfg := ModelsForTier(tier, agent)
			if cfg.Settings == nil {
				t.Errorf("tier=%q agent=%q: Settings is nil (should be non-nil map)", tier, agent)
			}
		}
	}
}
