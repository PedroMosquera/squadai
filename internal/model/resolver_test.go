package model

import (
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func TestResolvers_AllTiers(t *testing.T) {
	cases := []struct {
		name     string
		resolver Resolver
		tier     Tier
		want     string
	}{
		// ClaudeResolver
		{"claude-premium", NewClaudeResolver(), TierPremium, "claude-opus-4"},
		{"claude-standard", NewClaudeResolver(), TierStandard, "claude-sonnet-4"},
		{"claude-cheap", NewClaudeResolver(), TierCheap, "claude-haiku-4"},
		// OpenCodeResolver
		{"opencode-premium", NewOpenCodeResolver(), TierPremium, "anthropic/claude-opus-4"},
		{"opencode-standard", NewOpenCodeResolver(), TierStandard, "anthropic/claude-sonnet-4"},
		{"opencode-cheap", NewOpenCodeResolver(), TierCheap, "openai/gpt-4.1-mini"},
		// CursorResolver
		{"cursor-premium", NewCursorResolver(), TierPremium, "claude-opus-4"},
		{"cursor-standard", NewCursorResolver(), TierStandard, "claude-sonnet-4"},
		{"cursor-cheap", NewCursorResolver(), TierCheap, "gpt-4.1-mini"},
		// WindsurfResolver
		{"windsurf-premium", NewWindsurfResolver(), TierPremium, "claude-opus-4"},
		{"windsurf-standard", NewWindsurfResolver(), TierStandard, "claude-sonnet-4"},
		{"windsurf-cheap", NewWindsurfResolver(), TierCheap, "gpt-4.1-mini"},
		// VSCodeResolver
		{"vscode-premium", NewVSCodeResolver(), TierPremium, "gpt-4o"},
		{"vscode-standard", NewVSCodeResolver(), TierStandard, "gpt-4.1"},
		{"vscode-cheap", NewVSCodeResolver(), TierCheap, "gpt-4.1-mini"},
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
		agentID      domain.AgentID
		tier         Tier
		wantContains string // partial match to avoid tight coupling to model names
	}{
		{domain.AgentClaudeCode, TierPremium, "claude-opus-4"},
		{domain.AgentOpenCode, TierPremium, "anthropic/claude-opus-4"},
		{domain.AgentCursor, TierPremium, "claude-opus-4"},
		{domain.AgentWindsurf, TierPremium, "claude-opus-4"},
		{domain.AgentVSCodeCopilot, TierPremium, "gpt-4o"},
	}
	for _, tc := range cases {
		got := ForAgent(tc.agentID).Resolve(tc.tier)
		if got != tc.wantContains {
			t.Errorf("ForAgent(%q).Resolve(%q) = %q, want %q", tc.agentID, tc.tier, got, tc.wantContains)
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
	want := "claude-opus-4"
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
