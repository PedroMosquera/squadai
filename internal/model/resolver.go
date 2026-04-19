package model

import "github.com/PedroMosquera/squadai/internal/domain"

// Resolver maps a Tier to a concrete model name for a specific adapter.
// Each adapter implementation has different model IDs (namespacing, versions, etc.)
type Resolver interface {
	Resolve(tier Tier) string
}

type mapResolver struct {
	models map[Tier]string
}

// Resolve returns the concrete model name for the given tier.
// Falls back to the standard tier model if the tier is not found.
func (r *mapResolver) Resolve(tier Tier) string {
	if m, ok := r.models[tier]; ok {
		return m
	}
	return r.models[TierStandard]
}

// NewClaudeResolver returns a Resolver for Claude Code (claude-code adapter).
// Uses direct model names without provider prefix.
func NewClaudeResolver() Resolver {
	return &mapResolver{models: map[Tier]string{
		TierPremium:  "claude-opus-4",
		TierStandard: "claude-sonnet-4",
		TierCheap:    "claude-haiku-4",
	}}
}

// NewOpenCodeResolver returns a Resolver for OpenCode.
// Uses provider-qualified names (anthropic/..., openai/...).
func NewOpenCodeResolver() Resolver {
	return &mapResolver{models: map[Tier]string{
		TierPremium:  "anthropic/claude-opus-4",
		TierStandard: "anthropic/claude-sonnet-4",
		TierCheap:    "openai/gpt-4.1-mini",
	}}
}

// NewCursorResolver returns a Resolver for Cursor.
func NewCursorResolver() Resolver {
	return &mapResolver{models: map[Tier]string{
		TierPremium:  "claude-opus-4",
		TierStandard: "claude-sonnet-4",
		TierCheap:    "gpt-4.1-mini",
	}}
}

// NewWindsurfResolver returns a Resolver for Windsurf.
func NewWindsurfResolver() Resolver {
	return &mapResolver{models: map[Tier]string{
		TierPremium:  "claude-opus-4",
		TierStandard: "claude-sonnet-4",
		TierCheap:    "gpt-4.1-mini",
	}}
}

// NewVSCodeResolver returns a Resolver for VS Code Copilot.
func NewVSCodeResolver() Resolver {
	return &mapResolver{models: map[Tier]string{
		TierPremium:  "gpt-4o",
		TierStandard: "gpt-4.1",
		TierCheap:    "gpt-4.1-mini",
	}}
}

// ForAgent returns the Resolver appropriate for the given adapter ID.
// Unknown agent IDs fall back to the OpenCode resolver.
func ForAgent(id domain.AgentID) Resolver {
	switch id {
	case domain.AgentClaudeCode:
		return NewClaudeResolver()
	case domain.AgentOpenCode:
		return NewOpenCodeResolver()
	case domain.AgentCursor:
		return NewCursorResolver()
	case domain.AgentWindsurf:
		return NewWindsurfResolver()
	case domain.AgentVSCodeCopilot:
		return NewVSCodeResolver()
	default:
		return NewOpenCodeResolver()
	}
}

// ResolveRoleModel resolves the concrete model name for a role given its tier string
// (from domain.TeamRole.Model) and an adapter ID.
// An empty tier string falls back to DefaultTier().
// An unrecognised tier string also falls back to DefaultTier() (parse errors are
// surfaced at validation time, not here).
func ResolveRoleModel(roleModelField string, agentID domain.AgentID) string {
	tier := DefaultTier()
	if roleModelField != "" {
		if parsed, err := ParseTier(roleModelField); err == nil {
			tier = parsed
		}
	}
	return ForAgent(agentID).Resolve(tier)
}
