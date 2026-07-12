package cli

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/components/rules"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/model"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/tokenizer"
)

// resolveActiveProfile picks the context profile for this run.
// Precedence: --profile flag > Context.DefaultProfile > none.
//
// An unknown name passed via --profile is an error that lists the available
// profiles. A stale Context.DefaultProfile pointing at a profile that no
// longer exists is treated as "no profile" so plain applies keep working;
// the config validator reports the inconsistency separately.
func resolveActiveProfile(merged *domain.MergedConfig, flagProfile string) (string, *domain.ContextProfile, error) {
	name := flagProfile
	if name == "" {
		name = merged.Context.DefaultProfile
	}
	if name == "" {
		return "", nil, nil
	}
	prof, ok := merged.Context.Profiles[name]
	if !ok {
		if flagProfile == "" {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("unknown context profile %q (available: %s)",
			name, strings.Join(profileNames(merged), ", "))
	}
	if isLegacyInertDefaultProfile(name, prof) {
		return "", nil, nil
	}
	return name, &prof, nil
}

// isLegacyInertDefaultProfile reports whether the active profile is the
// exact "default" profile that squadai <= v0.6.0 wrote into every
// project.json as scaffolding. Context profiles were inert in those
// releases, so users never opted into that profile's restrictions (12k
// token cap, context7-only MCP, shared-only skills); enforcing them on
// upgrade would silently cap and degrade existing installs, and diff/verify
// (which do not run the budget fitter) would never converge with apply.
// A default profile the user has modified in any way does not match and is
// enforced normally.
func isLegacyInertDefaultProfile(name string, prof domain.ContextProfile) bool {
	if name != "default" {
		return false
	}
	legacy := domain.ContextProfile{
		MemoryScope:     "project",
		MCPServers:      []string{"context7"},
		SkillScopes:     []string{"shared"},
		MaxApproxTokens: 12000,
		Include:         []string{"**/*"},
		Exclude:         []string{".git/**", "node_modules/**", "dist/**"},
	}
	return reflect.DeepEqual(prof, legacy)
}

// profileNames returns the sorted names of the configured context profiles.
func profileNames(merged *domain.MergedConfig) []string {
	names := make([]string, 0, len(merged.Context.Profiles))
	for n := range merged.Context.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// applyProfileToConfig mutates the merged config in place so the active
// context profile shapes what bundle.Build constructs:
//
//   - MCPServers: nil keeps every configured server; a present (even empty)
//     list is a strict filter — servers not listed are removed, and the MCP
//     installer prunes previously managed ones.
//   - MemoryScope: "none" disables the memory component; "summary", "project",
//     and "full" flow to the memory and agents installers via
//     ActiveContextProfile so both the rules-file protocol and native agent
//     files pick the stub or full variant.
//   - SkillScopes: flow to skills.New via ActiveContextProfile (prefix or
//     exact matching; nil means no filter).
//
// Include, Exclude, and AdapterOverrides are intentionally NOT enforced yet.
func applyProfileToConfig(merged *domain.MergedConfig, name string, prof *domain.ContextProfile) {
	if prof == nil {
		return
	}
	merged.ActiveContextProfile = prof
	merged.ActiveProfileName = name

	if prof.MCPServers != nil {
		allowed := make(map[string]bool, len(prof.MCPServers))
		for _, s := range prof.MCPServers {
			allowed[s] = true
		}
		filtered := make(map[string]domain.MCPServerDef)
		for k, v := range merged.MCP {
			if allowed[k] {
				filtered[k] = v
			}
		}
		merged.MCP = filtered
	}

	if prof.MemoryScope == memory.ScopeNone {
		// Copy-on-write: callers may hold a pre-profile view of the config,
		// so never mutate the shared components map in place.
		components := make(map[string]domain.ComponentConfig, len(merged.Components)+1)
		for k, v := range merged.Components {
			components[k] = v
		}
		components[string(domain.ComponentMemory)] = domain.ComponentConfig{Enabled: false}
		merged.Components = components
	}
}

// applyDefaultProfile resolves and applies the project's default context
// profile (no flag override) so plan, diff, verify, and token-budget see the
// same effective config as apply. Resolution errors cannot occur without a
// flag, so the config is left untouched in that case.
func applyDefaultProfile(merged *domain.MergedConfig) {
	if merged == nil {
		return
	}
	name, prof, err := resolveActiveProfile(merged, "")
	if err != nil {
		return
	}
	applyProfileToConfig(merged, name, prof)
}

// effectiveTokenCap returns the budget cap for this run:
// --max-tokens flag > profile MaxApproxTokens > 0 (no cap).
func effectiveTokenCap(flagMaxTokens int, prof *domain.ContextProfile) int {
	if flagMaxTokens > 0 {
		return flagMaxTokens
	}
	if prof != nil && prof.MaxApproxTokens > 0 {
		return prof.MaxApproxTokens
	}
	return 0
}

// resolveFitModel returns the concrete model used for budget-fit token
// counting. Chain: --fit-model flag > Usage.ProfileTiers[profile] via the
// tier bridge and model catalog > the catalog's standard-tier default.
// Whenever fitting runs, token counts therefore come from a real tokenizer,
// never the chars/4 heuristic.
func resolveFitModel(merged *domain.MergedConfig, profileName, flagFitModel string) string {
	if flagFitModel != "" {
		return flagFitModel
	}
	cat := modelcatalog.Default()
	adapterID := fitAdapterID(merged)
	if profileName != "" && merged != nil {
		if tier, ok := merged.Usage.ProfileTiers[profileName]; ok && tier != "" {
			if m := cat.TierModel(adapterID, string(model.TierFromProfile(tier))); m != "" {
				return m
			}
		}
	}
	return cat.TierModel(adapterID, string(model.DefaultTier()))
}

// fitAdapterID picks the adapter whose catalog tier table anchors fit-model
// resolution: the first enabled adapter in stable order. An empty return
// falls back to the catalog's default adapter entry.
func fitAdapterID(merged *domain.MergedConfig) string {
	if merged == nil {
		return ""
	}
	ids := make([]string, 0, len(merged.Adapters))
	for id, a := range merged.Adapters {
		if a.Enabled {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// lazyLoadingAdapters lists agents that lazy-load skills and commands: only
// the YAML frontmatter (name + description) costs session tokens; the body is
// read on invocation.
var lazyLoadingAdapters = map[domain.AgentID]bool{
	domain.AgentClaudeCode: true,
	domain.AgentOpenCode:   true,
}

// lazyLoadedComponents are the components whose files lazy-loading adapters
// index by frontmatter only.
var lazyLoadedComponents = map[domain.ComponentID]bool{
	domain.ComponentSkills:   true,
	domain.ComponentCommands: true,
}

// frontmatterOnly returns the YAML frontmatter block (including delimiters)
// of a markdown document, or the full content when no frontmatter exists.
func frontmatterOnly(content []byte) []byte {
	const sep = "---\n"
	s := string(content)
	if !strings.HasPrefix(s, sep) {
		return content
	}
	end := strings.Index(s[len(sep):], sep)
	if end < 0 {
		return content
	}
	return []byte(s[:len(sep)+end+len(sep)])
}

// desiredComponentTokens renders every planned action and returns per-
// component token counts for budget fitting. Two cost-model corrections keep
// the fitter honest:
//
//   - Skills and commands on lazy-loading adapters (Claude Code, OpenCode)
//     count only their frontmatter bytes — the session cost of a lazy-loaded
//     file — so the fitter no longer drops components that are cheap in
//     practice.
//   - Marker-injection components (memory, efficiency, brand, rules) count
//     only their own section content instead of the whole shared document, so
//     three components sharing AGENTS.md don't each get billed for the full
//     file.
// The second return value is the subtotal for agents-component actions that
// are NOT prompt/solo rules-file injections (native agent files, settings):
// these are unchanged in the fitter's summary mode, so the summary estimate
// for agents is that subtotal plus the compact digest per injection target.
func desiredComponentTokens(p *planner.Planner, actions []domain.PlannedAction, homeDir, projectDir, modelName string) (map[domain.ComponentID]int, int, error) {
	counter := tokenizer.ForModel(modelName)
	// Each RenderAction returns the FULL rendered target document. When several
	// actions of the same component target one file (e.g. the OpenCode and Pi
	// memory sections both live in a shared AGENTS.md), summing per action would
	// count that document once per action and overstate the component, which can
	// make the budget fitter drop a component that actually fits. Keep the
	// largest render per (component, agent, path) instead; agent matters for
	// section-based components, which have one adapter-scoped section each.
	type compPath struct {
		component domain.ComponentID
		agent     domain.AgentID
		path      string
	}
	tokensByPath := make(map[compPath]int)
	injectionKey := make(map[compPath]bool)
	for _, action := range actions {
		if action.Action == domain.ActionDelete || action.TargetPath == "" {
			continue
		}
		_, desired, err := p.RenderAction(action, homeDir, projectDir)
		if err != nil {
			return nil, 0, fmt.Errorf("render %s: %w", action.ID, err)
		}
		key := compPath{component: action.Component, path: action.TargetPath}
		if section, ok := plannedSectionContent(action, desired); ok {
			desired = section
			key.agent = action.Agent
		}
		if lazyLoadedComponents[action.Component] && lazyLoadingAdapters[action.Agent] {
			desired = frontmatterOnly(desired)
		}
		tokens := counter.Count(string(desired))
		if tokens > tokensByPath[key] {
			tokensByPath[key] = tokens
		}
		if action.Component == domain.ComponentAgents && isAgentsInjectionAction(action) {
			injectionKey[key] = true
		}
	}
	tokensByComponent := make(map[domain.ComponentID]int)
	nativeAgentsTokens := 0
	for key, tokens := range tokensByPath {
		tokensByComponent[key.component] += tokens
		if key.component == domain.ComponentAgents && !injectionKey[key] {
			nativeAgentsTokens += tokens
		}
	}
	return tokensByComponent, nativeAgentsTokens, nil
}

// isAgentsInjectionAction reports whether an agents-component action injects
// the orchestrator into a shared rules file (prompt/solo delegation) rather
// than writing a standalone native agent file.
func isAgentsInjectionAction(action domain.PlannedAction) bool {
	return strings.HasPrefix(action.Description, "team:prompt:") ||
		strings.HasPrefix(action.Description, "team:solo:")
}

// summaryComponentTokens returns real token counts for the summary renders of
// the summarizable components present in the plan (the memory stub and the
// condensed team standards), sized per distinct target path. The fitter uses
// these instead of the tokens/2 guess.
// nativeAgentsTokens is the full-render subtotal for non-injection agents
// actions from desiredComponentTokens: native agent files are unchanged in
// summary mode, so the agents summary is that subtotal plus one compact
// orchestrator digest per prompt/solo rules-file target.
func summaryComponentTokens(actions []domain.PlannedAction, modelName string, nativeAgentsTokens int) map[domain.ComponentID]int {
	counter := tokenizer.ForModel(modelName)
	paths := map[domain.ComponentID]map[string]bool{
		domain.ComponentMemory: {},
		domain.ComponentRules:  {},
	}
	injectionTargets := map[string]bool{}
	hasAgents := false
	for _, a := range actions {
		if a.TargetPath == "" || a.Action == domain.ActionDelete {
			continue
		}
		if set, ok := paths[a.Component]; ok {
			set[a.TargetPath] = true
		}
		if a.Component == domain.ComponentAgents {
			hasAgents = true
			if isAgentsInjectionAction(a) {
				injectionTargets[a.TargetPath] = true
			}
		}
	}
	out := make(map[domain.ComponentID]int)
	if n := len(paths[domain.ComponentMemory]); n > 0 {
		out[domain.ComponentMemory] = n * counter.Count(memory.ProtocolStub)
	}
	if n := len(paths[domain.ComponentRules]); n > 0 {
		out[domain.ComponentRules] = n * counter.Count(rules.SummaryContent())
	}
	if hasAgents {
		digest := counter.Count(agents.OrchestratorDigest(""))
		out[domain.ComponentAgents] = nativeAgentsTokens + len(injectionTargets)*digest
	}
	return out
}
