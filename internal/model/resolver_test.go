package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

func TestResolvers_AllTiers(t *testing.T) {
	cases := []struct {
		name     string
		resolver Resolver
		tier     Tier
		want     string
	}{
		// ClaudeResolver
		{"claude-premium", NewClaudeResolver(), TierPremium, "claude-fable-5"},
		{"claude-standard", NewClaudeResolver(), TierStandard, "claude-sonnet-4-6"},
		{"claude-cheap", NewClaudeResolver(), TierCheap, "claude-haiku-4-5"},
		// OpenCodeResolver
		{"opencode-premium", NewOpenCodeResolver(), TierPremium, "anthropic/claude-fable-5"},
		{"opencode-standard", NewOpenCodeResolver(), TierStandard, "anthropic/claude-sonnet-4-6"},
		{"opencode-cheap", NewOpenCodeResolver(), TierCheap, "anthropic/claude-haiku-4-5"},
		// CursorResolver
		{"cursor-premium", NewCursorResolver(), TierPremium, "claude-fable-5"},
		{"cursor-standard", NewCursorResolver(), TierStandard, "claude-sonnet-4-6"},
		{"cursor-cheap", NewCursorResolver(), TierCheap, "claude-haiku-4-5"},
		// WindsurfResolver
		{"windsurf-premium", NewWindsurfResolver(), TierPremium, "claude-fable-5"},
		{"windsurf-standard", NewWindsurfResolver(), TierStandard, "claude-sonnet-4-6"},
		{"windsurf-cheap", NewWindsurfResolver(), TierCheap, "claude-haiku-4-5"},
		// VSCodeResolver
		{"vscode-premium", NewVSCodeResolver(), TierPremium, "gpt-5.2"},
		{"vscode-standard", NewVSCodeResolver(), TierStandard, "claude-sonnet-4-6"},
		{"vscode-cheap", NewVSCodeResolver(), TierCheap, "gpt-5-mini"},
		// PiResolver
		{"pi-premium", NewPiResolver(), TierPremium, "anthropic/claude-fable-5"},
		{"pi-standard", NewPiResolver(), TierStandard, "anthropic/claude-sonnet-4-6"},
		{"pi-cheap", NewPiResolver(), TierCheap, "anthropic/claude-haiku-4-5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.resolver.Resolve(tc.tier)
			if got != tc.want {
				t.Errorf("Resolve(%q) = %q, want %q", tc.tier, got, tc.want)
			}
		})
	}
}

func TestForAgent_MapsToCorrectResolver(t *testing.T) {
	cases := []struct {
		agentID domain.AgentID
		tier    Tier
		want    string
	}{
		{domain.AgentClaudeCode, TierPremium, "claude-fable-5"},
		{domain.AgentOpenCode, TierPremium, "anthropic/claude-fable-5"},
		{domain.AgentCursor, TierPremium, "claude-fable-5"},
		{domain.AgentWindsurf, TierPremium, "claude-fable-5"},
		{domain.AgentVSCodeCopilot, TierPremium, "gpt-5.2"},
		{domain.AgentPi, TierPremium, "anthropic/claude-fable-5"},
	}
	for _, tc := range cases {
		got := ForAgent(tc.agentID).Resolve(tc.tier)
		if got != tc.want {
			t.Errorf("ForAgent(%q).Resolve(%q) = %q, want %q", tc.agentID, tc.tier, got, tc.want)
		}
	}
}

func TestForAgent_UnknownAgentFallsBackToOpenCode(t *testing.T) {
	resolver := ForAgent("unknown-agent")
	got := resolver.Resolve(TierStandard)
	want := NewOpenCodeResolver().Resolve(TierStandard)
	if got != want {
		t.Errorf("ForAgent(unknown).Resolve(standard) = %q, want %q", got, want)
	}
}

func TestResolveRoleModel_EmptyField_UsesDefault(t *testing.T) {
	got := ResolveRoleModel("", domain.AgentOpenCode)
	want := NewOpenCodeResolver().Resolve(TierStandard)
	if got != want {
		t.Errorf("ResolveRoleModel(\"\", opencode) = %q, want %q", got, want)
	}
}

func TestResolveRoleModel_ValidTier(t *testing.T) {
	got := ResolveRoleModel("premium", domain.AgentClaudeCode)
	want := "claude-fable-5"
	if got != want {
		t.Errorf("ResolveRoleModel(\"premium\", claude-code) = %q, want %q", got, want)
	}
}

func TestResolveRoleModel_InvalidTier_FallsBackToDefault(t *testing.T) {
	// Invalid tier at resolve time falls back — validation catches this earlier.
	got := ResolveRoleModel("invalid-tier", domain.AgentOpenCode)
	want := NewOpenCodeResolver().Resolve(TierStandard)
	if got != want {
		t.Errorf("ResolveRoleModel(\"invalid-tier\", opencode) = %q, want %q", got, want)
	}
}

// ─── ResolveRoleModelFor ─────────────────────────────────────────────────────

// cfgWith returns a config with one profile and one role override pointing at it.
func cfgWith(profile domain.ModelProfile) *domain.MergedConfig {
	return &domain.MergedConfig{
		Models: domain.ModelsConfig{
			Profiles:  map[string]domain.ModelProfile{"special": profile},
			Overrides: map[string]string{"tdd.planner": "special"},
		},
	}
}

func TestResolveRoleModelFor_Precedence(t *testing.T) {
	cases := []struct {
		name        string
		cfg         *domain.MergedConfig
		methodology string
		roleName    string
		roleTier    string
		agentID     domain.AgentID
		want        string
	}{
		{
			name: "adapters concrete override wins over everything",
			cfg: cfgWith(domain.ModelProfile{
				Tier:     "cheap",
				Adapters: map[string]string{"claude-code": "my-custom-model"},
			}),
			methodology: "tdd", roleName: "planner", roleTier: "premium",
			agentID: domain.AgentClaudeCode,
			want:    "my-custom-model",
		},
		{
			name: "adapters override for another adapter is ignored",
			cfg: cfgWith(domain.ModelProfile{
				Tier:     "premium",
				Adapters: map[string]string{"opencode": "my-custom-model"},
			}),
			methodology: "tdd", roleName: "planner", roleTier: "cheap",
			agentID: domain.AgentClaudeCode,
			want:    "claude-fable-5", // profile tier premium beats role tier cheap
		},
		{
			name:        "profile tier beats role tier",
			cfg:         cfgWith(domain.ModelProfile{Tier: "cheap"}),
			methodology: "tdd", roleName: "planner", roleTier: "premium",
			agentID: domain.AgentClaudeCode,
			want:    "claude-haiku-4-5",
		},
		{
			name:        "no override entry falls back to role tier",
			cfg:         cfgWith(domain.ModelProfile{Tier: "cheap"}),
			methodology: "tdd", roleName: "implementer", roleTier: "premium",
			agentID: domain.AgentClaudeCode,
			want:    "claude-fable-5",
		},
		{
			name:        "override points at missing profile falls back to role tier",
			cfg:         &domain.MergedConfig{Models: domain.ModelsConfig{Overrides: map[string]string{"tdd.planner": "ghost"}}},
			methodology: "tdd", roleName: "planner", roleTier: "cheap",
			agentID: domain.AgentClaudeCode,
			want:    "claude-haiku-4-5",
		},
		{
			name:        "empty tier everywhere resolves catalog default",
			cfg:         &domain.MergedConfig{},
			methodology: "tdd", roleName: "planner", roleTier: "",
			agentID: domain.AgentClaudeCode,
			want:    "claude-sonnet-4-6",
		},
		{
			name:        "nil config degrades to role tier",
			cfg:         nil,
			methodology: "tdd", roleName: "planner", roleTier: "premium",
			agentID: domain.AgentOpenCode,
			want:    "anthropic/claude-fable-5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveRoleModelFor(tc.cfg, tc.methodology, tc.roleName, tc.roleTier, tc.agentID, nil)
			if got != tc.want {
				t.Errorf("ResolveRoleModelFor(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveRoleModelFor_OverrideFileFlipsResolution(t *testing.T) {
	// An override file that remaps the claude-code standard tier must flip
	// what the resolver returns, proving the resolvers are catalog-driven.
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	override := `{
		"schema_version": 1,
		"updated": "2027-01-01",
		"adapters": {
			"claude-code": {"tiers": {"standard": "claude-flipped-9"}}
		}
	}`
	if err := os.WriteFile(filepath.Join(home, ".squadai", "models.json"), []byte(override), 0644); err != nil {
		t.Fatal(err)
	}
	cat, err := modelcatalog.Load(home, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := ResolveRoleModelFor(nil, "tdd", "planner", "", domain.AgentClaudeCode, cat)
	if got != "claude-flipped-9" {
		t.Errorf("with override file, resolved %q, want %q", got, "claude-flipped-9")
	}
	// Sanity: premium tier is untouched by the partial override.
	if got := ResolveRoleModelFor(nil, "tdd", "planner", "premium", domain.AgentClaudeCode, cat); got != "claude-fable-5" {
		t.Errorf("premium tier = %q, want %q", got, "claude-fable-5")
	}
}
