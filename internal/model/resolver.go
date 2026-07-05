package model

import (
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// Resolver maps a Tier to a concrete model name for a specific adapter.
// Each adapter implementation has different model IDs (namespacing, versions, etc.)
type Resolver interface {
	Resolve(tier Tier) string
}

// catalogResolver resolves tiers through the unified model catalog.
type catalogResolver struct {
	adapterID domain.AgentID
	cat       *modelcatalog.Catalog
}

// Resolve returns the concrete model name for the given tier.
// Falls back to the standard tier model if the tier is not found.
func (r catalogResolver) Resolve(tier Tier) string {
	return r.cat.TierModel(string(r.adapterID), string(tier))
}

// NewClaudeResolver returns a Resolver for Claude Code (claude-code adapter).
// Uses direct model names without provider prefix.
func NewClaudeResolver() Resolver { return ForAgent(domain.AgentClaudeCode) }

// NewOpenCodeResolver returns a Resolver for OpenCode.
// Uses provider-qualified names (anthropic/..., openai/...).
func NewOpenCodeResolver() Resolver { return ForAgent(domain.AgentOpenCode) }

// NewCursorResolver returns a Resolver for Cursor.
func NewCursorResolver() Resolver { return ForAgent(domain.AgentCursor) }

// NewWindsurfResolver returns a Resolver for Windsurf.
func NewWindsurfResolver() Resolver { return ForAgent(domain.AgentWindsurf) }

// NewVSCodeResolver returns a Resolver for VS Code Copilot.
func NewVSCodeResolver() Resolver { return ForAgent(domain.AgentVSCodeCopilot) }

// NewPiResolver returns a Resolver for Pi Agent.
// Uses provider-qualified names like OpenCode (anthropic/...).
func NewPiResolver() Resolver { return ForAgent(domain.AgentPi) }

// ForAgent returns the Resolver appropriate for the given adapter ID, backed
// by the process-default model catalog. Unknown agent IDs fall back to the
// OpenCode tier mapping inside the catalog.
func ForAgent(id domain.AgentID) Resolver {
	return ForAgentWithCatalog(id, modelcatalog.Default())
}

// ForAgentWithCatalog returns a Resolver for the given adapter backed by an
// explicit catalog (e.g. one loaded with explicit homeDir/projectDir).
func ForAgentWithCatalog(id domain.AgentID, cat *modelcatalog.Catalog) Resolver {
	return catalogResolver{adapterID: id, cat: cat}
}

// ResolveRoleModel resolves the concrete model name for a role given its tier string
// (from domain.TeamRole.Model) and an adapter ID.
// An empty tier string falls back to DefaultTier().
// An unrecognised tier string also falls back to DefaultTier() (parse errors are
// surfaced at validation time, not here).
func ResolveRoleModel(roleModelField string, agentID domain.AgentID) string {
	return resolveRoleTier(roleModelField, agentID, modelcatalog.Default())
}

// resolveRoleTier resolves a role tier string against an explicit catalog.
func resolveRoleTier(roleModelField string, agentID domain.AgentID, cat *modelcatalog.Catalog) string {
	tier := DefaultTier()
	if roleModelField != "" {
		if parsed, err := ParseTier(roleModelField); err == nil {
			tier = parsed
		}
	}
	return ForAgentWithCatalog(agentID, cat).Resolve(tier)
}

// ResolveRoleModelFor resolves the concrete model for a team role with full
// config awareness. Precedence, highest first:
//
//  1. ModelProfile.Adapters[agentID] — a concrete per-adapter model override
//     on the profile selected by cfg.Models.Overrides["<methodology>.<role>"].
//  2. The selected profile's Tier (cheap/balanced/premium).
//  3. The role's own tier string (premium/standard/cheap).
//  4. The catalog default (standard tier) for the adapter.
//
// A nil cat falls back to the process-default catalog; a nil cfg degrades to
// ResolveRoleModel semantics.
func ResolveRoleModelFor(cfg *domain.MergedConfig, methodology, roleName, roleTier string, agentID domain.AgentID, cat *modelcatalog.Catalog) string {
	if cat == nil {
		cat = modelcatalog.Default()
	}
	if cfg != nil {
		if profileName, ok := cfg.Models.Overrides[methodology+"."+roleName]; ok {
			if profile, ok := cfg.Models.Profiles[profileName]; ok {
				if concrete := profile.Adapters[string(agentID)]; concrete != "" {
					return concrete
				}
				if profile.Tier != "" {
					return ForAgentWithCatalog(agentID, cat).Resolve(TierFromProfile(profile.Tier))
				}
			}
		}
	}
	return resolveRoleTier(roleTier, agentID, cat)
}
