package config

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/model"
)

// hybridWarnOnce ensures the deprecation warning is printed at most once per
// process invocation, even when ResolveHybridMode is called multiple times
// (e.g. by TUI flows that reload config on every navigation).
var hybridWarnOnce sync.Once

// ResolveHybridMode resolves a deprecated "hybrid" mode to the appropriate
// canonical mode based on whether a policy.json exists in policyDir.
// If the mode is not "hybrid", it is returned unchanged.
// A deprecation warning is written to stderr at most once per process.
func ResolveHybridMode(mode domain.OperationalMode, policyDir string) domain.OperationalMode {
	if mode != "hybrid" {
		return mode
	}
	policyPath := policyDir + "/policy.json"
	_, err := os.Stat(policyPath)
	var resolved domain.OperationalMode
	if err == nil {
		resolved = domain.ModeTeam
	} else {
		resolved = domain.ModePersonal
	}
	hybridWarnOnce.Do(func() {
		fmt.Fprintf(os.Stderr,
			"Warning: mode %q is deprecated. Resolved to %q based on policy.json presence. "+
				"Update your config to use %q explicitly.\n",
			"hybrid", resolved, resolved)
	})
	return resolved
}

// ValidateUser checks a UserConfig for structural issues.
func ValidateUser(cfg *domain.UserConfig) []string {
	var issues []string

	if cfg.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}

	switch cfg.Mode {
	case domain.ModeTeam, domain.ModePersonal:
		// valid
	case "hybrid":
		// deprecated alias — accepted without error; callers should use ResolveHybridMode
	case "":
		issues = append(issues, "mode is required")
	default:
		issues = append(issues, fmt.Sprintf("unknown mode %q (expected: team, personal)", cfg.Mode))
	}

	for name := range cfg.Adapters {
		if !isKnownAdapter(name) {
			issues = append(issues, fmt.Sprintf("unknown adapter %q", name))
		}
	}

	for name := range cfg.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q", name))
		}
	}

	return issues
}

// ValidateProject checks a ProjectConfig for structural issues.
func ValidateProject(cfg *domain.ProjectConfig) []string {
	var issues []string

	if cfg.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}

	for name := range cfg.Adapters {
		if !isKnownAdapter(name) {
			issues = append(issues, fmt.Sprintf("unknown adapter %q", name))
		}
	}

	for name := range cfg.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q", name))
		}
	}

	issues = append(issues, validateAgentDefs(cfg.Agents)...)
	issues = append(issues, validateSkillDefs(cfg.Skills)...)
	issues = append(issues, validateCommandDefs(cfg.Commands)...)
	issues = append(issues, validateMCPDefs(cfg.MCP)...)
	issues = append(issues, validatePreset(cfg.Preset)...)
	issues = append(issues, validateMethodology(cfg.Methodology)...)
	issues = append(issues, validateMemoryConfig(cfg.Memory)...)
	issues = append(issues, validateContextConfig(cfg.Context)...)
	issues = append(issues, validateUsageConfig(cfg.Usage)...)
	issues = append(issues, validateModelsConfig(cfg.Models)...)
	issues = append(issues, validateTeamRoles(cfg.Team)...)
	issues = append(issues, validatePluginDefs(cfg.Plugins, cfg.Methodology)...)

	return issues
}

// ValidatePolicy checks a PolicyConfig for structural and consistency issues.
// This powers the `validate-policy` command.
func ValidatePolicy(cfg *domain.PolicyConfig) []string {
	var issues []string

	if cfg.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}

	switch cfg.Mode {
	case domain.ModeTeam, domain.ModePersonal:
		// valid
	case "hybrid":
		// deprecated alias — accepted without error for policy files too
	case "":
		issues = append(issues, "mode is required")
	default:
		issues = append(issues, fmt.Sprintf("unknown mode %q", cfg.Mode))
	}

	// Check that every locked field has a corresponding value in required block.
	for _, field := range cfg.Locked {
		if !hasRequiredValue(cfg, field) {
			issues = append(issues, fmt.Sprintf("locked field %q has no corresponding value in required block", field))
		}
	}

	// Check that required adapters are known.
	for name := range cfg.Required.Adapters {
		if !isKnownAdapter(name) {
			issues = append(issues, fmt.Sprintf("unknown adapter %q in required block", name))
		}
	}

	// Check that required components are known.
	for name := range cfg.Required.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q in required block", name))
		}
	}

	// Validate required agents, MCP definitions.
	issues = append(issues, validateAgentDefs(cfg.Required.Agents)...)
	issues = append(issues, validateMCPDefs(cfg.Required.MCP)...)
	issues = append(issues, validateAllowLists(cfg.Allowed)...)

	return issues
}

// validateAllowLists rejects empty patterns: "" only matches an empty value,
// so it is almost always a typo that silently denies everything.
func validateAllowLists(a domain.AllowLists) []string {
	lists := map[string][]string{
		"models":         a.Models,
		"hook_commands":  a.HookCommands,
		"plugin_sources": a.PluginSources,
	}
	if a.MCPServers != nil {
		lists["mcp_servers.names"] = a.MCPServers.Names
		lists["mcp_servers.commands"] = a.MCPServers.Commands
		lists["mcp_servers.urls"] = a.MCPServers.URLs
	}
	var issues []string
	for _, key := range slices.Sorted(maps.Keys(lists)) {
		if slices.Contains(lists[key], "") {
			issues = append(issues, fmt.Sprintf("allowed.%s contains an empty pattern", key))
		}
	}
	return issues
}

// hasRequiredValue checks whether a locked field path has a corresponding
// value in the policy's required block.
func hasRequiredValue(cfg *domain.PolicyConfig, field string) bool {
	parts := strings.Split(field, ".")
	if len(parts) < 2 {
		return false
	}

	switch parts[0] {
	case "adapters":
		if len(parts) < 3 {
			return false
		}
		_, exists := cfg.Required.Adapters[parts[1]]
		return exists

	case "components":
		if len(parts) < 3 {
			return false
		}
		_, exists := cfg.Required.Components[parts[1]]
		return exists

	case "copilot":
		return cfg.Required.Copilot.InstructionsTemplate != ""

	case "rules":
		return cfg.Required.Rules.TeamStandards != "" || cfg.Required.Rules.TeamStandardsFile != ""

	case "agents":
		if len(parts) < 2 {
			return false
		}
		_, exists := cfg.Required.Agents[parts[1]]
		return exists

	case "mcp":
		if len(parts) < 2 {
			return false
		}
		_, exists := cfg.Required.MCP[parts[1]]
		return exists

	case "plugins":
		if len(parts) < 2 {
			return false
		}
		_, exists := cfg.Required.Plugins[parts[1]]
		return exists

	case "claude":
		// Only "claude.agent_teams.enabled" is locked-field-supported today.
		// The required block must explicitly set the desired value, even if
		// it is the zero value, so we treat presence under "claude.*" as
		// satisfied whenever the structure is non-zero. A bool zero-value is
		// indistinguishable from "unset," but locking false is a legitimate
		// policy ("nobody on this team is allowed to enable Agent Teams").
		if len(parts) < 3 {
			return false
		}
		if parts[1] == "agent_teams" && parts[2] == "enabled" {
			return true
		}
		return false

	case "hooks":
		// "hooks" as a whole — valid when the required block has any hook entries.
		return len(cfg.Required.Hooks) > 0

	default:
		return false
	}
}

// validateAgentDefs checks that agent definitions have required fields.
func validateAgentDefs(agents map[string]domain.AgentDef) []string {
	var issues []string
	for name, def := range agents {
		if def.Description == "" {
			issues = append(issues, fmt.Sprintf("agent %q must have a description", name))
		}
		if def.Mode == "" {
			issues = append(issues, fmt.Sprintf("agent %q must have a mode", name))
		} else {
			switch def.Mode {
			case "subagent", "byoa":
				// valid modes
			default:
				issues = append(issues, fmt.Sprintf("agent %q has unknown mode %q (expected: subagent, byoa)", name, def.Mode))
			}
		}
		if def.Prompt == "" && def.PromptFile == "" {
			issues = append(issues, fmt.Sprintf("agent %q must have prompt or prompt_file", name))
		}
	}
	return issues
}

// validateSkillDefs checks that skill definitions have required fields.
func validateSkillDefs(skills map[string]domain.SkillDef) []string {
	var issues []string
	for name, def := range skills {
		if def.Description == "" {
			issues = append(issues, fmt.Sprintf("skill %q must have a description", name))
		}
		if def.Content == "" && def.ContentFile == "" {
			issues = append(issues, fmt.Sprintf("skill %q must have content or content_file", name))
		}
	}
	return issues
}

// validateCommandDefs checks that command definitions have required fields.
func validateCommandDefs(commands map[string]domain.CommandDef) []string {
	var issues []string
	for name, def := range commands {
		if def.Description == "" {
			issues = append(issues, fmt.Sprintf("command %q must have a description", name))
		}
	}
	return issues
}

// validateMCPDefs checks that MCP server definitions have required fields.
func validateMCPDefs(mcpServers map[string]domain.MCPServerDef) []string {
	var issues []string
	for name, def := range mcpServers {
		if def.Type == "" {
			issues = append(issues, fmt.Sprintf("MCP server %q must have a type", name))
		} else {
			switch def.Type {
			case "local", "remote":
				// valid types
			default:
				issues = append(issues, fmt.Sprintf("MCP server %q has unknown type %q (expected: local, remote)", name, def.Type))
			}
		}
		if def.Type == "local" && len(def.Command) == 0 {
			issues = append(issues, fmt.Sprintf("MCP server %q (type=local) must have a command", name))
		}
		if def.Type == "remote" && def.URL == "" {
			issues = append(issues, fmt.Sprintf("MCP server %q (type=remote) must have a url", name))
		}
	}
	return issues
}

// validatePreset checks that preset is a supported solo/team setup shortcut.
func validatePreset(p domain.SetupPreset) []string {
	if p == "" {
		return nil
	}
	switch p {
	case domain.PresetSoloMinimal, domain.PresetSoloPower, domain.PresetTeamStandard,
		domain.PresetEnterpriseLock, domain.PresetFullSquad, domain.PresetLean,
		domain.PresetCustom:
		return nil
	default:
		return []string{fmt.Sprintf("unknown preset %q (expected: solo-minimal, solo-power, team-standard, enterprise-locked, full-squad, lean, custom)", p)}
	}
}

// validateMethodology checks that methodology is a known value or empty.
func validateMethodology(m domain.Methodology) []string {
	if m == "" {
		return nil
	}
	switch m {
	case domain.MethodologyTDD, domain.MethodologySDD, domain.MethodologyConventional:
		return nil
	default:
		return []string{fmt.Sprintf("unknown methodology %q (expected: tdd, sdd, conventional)", m)}
	}
}

func validateMemoryConfig(memory domain.MemoryConfig) []string {
	var issues []string
	if memory.Backend != "" {
		switch memory.Backend {
		case "native", "docs":
			// valid
		default:
			issues = append(issues, fmt.Sprintf("memory.backend has unknown value %q (expected: native, docs)", memory.Backend))
		}
	}
	if memory.ProjectKeyStrategy != "" {
		switch memory.ProjectKeyStrategy {
		case "git-remote", "repo-name":
			// valid
		default:
			issues = append(issues, fmt.Sprintf("memory.project_key_strategy has unknown value %q (expected: git-remote, repo-name)", memory.ProjectKeyStrategy))
		}
	}
	return issues
}

func validateContextConfig(contextCfg domain.ContextConfig) []string {
	var issues []string
	if contextCfg.DefaultProfile != "" && len(contextCfg.Profiles) > 0 {
		if _, ok := contextCfg.Profiles[contextCfg.DefaultProfile]; !ok {
			issues = append(issues, fmt.Sprintf("context.default_profile %q is not defined in context.profiles", contextCfg.DefaultProfile))
		}
	}
	for name, profile := range contextCfg.Profiles {
		if profile.MaxApproxTokens < 0 {
			issues = append(issues, fmt.Sprintf("context profile %q max_approx_tokens must be >= 0", name))
		}
		switch profile.MemoryScope {
		case "", "none", "summary", "project", "full":
			// valid
		default:
			issues = append(issues, fmt.Sprintf("context profile %q has unknown memory_scope %q (expected: none, summary, project, full)", name, profile.MemoryScope))
		}
	}
	return issues
}

func validateUsageConfig(usage domain.UsageConfig) []string {
	var issues []string
	if usage.DailyTokenBudget < 0 {
		issues = append(issues, "usage.daily_token_budget must be >= 0")
	}
	if usage.SessionTokenBudget < 0 {
		issues = append(issues, "usage.session_token_budget must be >= 0")
	}
	if usage.DailyCostBudget < 0 {
		issues = append(issues, "usage.daily_cost_budget must be >= 0")
	}
	if usage.SessionCostBudget < 0 {
		issues = append(issues, "usage.session_cost_budget must be >= 0")
	}
	if usage.Enforcement != "" {
		switch usage.Enforcement {
		case "off", "warn", "ask", "block":
			// valid
		default:
			issues = append(issues, fmt.Sprintf("usage.enforcement has unknown value %q (expected: off, warn, ask, block)", usage.Enforcement))
		}
	}
	switch usage.PriceCatalogSource {
	case "", "embedded":
		// valid
	default:
		issues = append(issues, fmt.Sprintf("usage.price_catalog_source has unknown value %q (expected: embedded)", usage.PriceCatalogSource))
	}
	return issues
}

func validateModelsConfig(models domain.ModelsConfig) []string {
	var issues []string
	for name, profile := range models.Profiles {
		switch profile.Tier {
		case "", "cheap", "balanced", "premium":
			// valid
		default:
			issues = append(issues, fmt.Sprintf("models profile %q has unknown tier %q (expected: cheap, balanced, premium)", name, profile.Tier))
		}
		for adapterName, concreteModel := range profile.Adapters {
			if !isKnownAdapter(adapterName) {
				issues = append(issues, fmt.Sprintf("models profile %q has unknown adapter %q in adapters override", name, adapterName))
			}
			if strings.TrimSpace(concreteModel) == "" {
				issues = append(issues, fmt.Sprintf("models profile %q adapter %q override must be a non-empty model id", name, adapterName))
			}
		}
	}
	return issues
}

// validateTeamRoles checks that team role definitions have valid fields.
func validateTeamRoles(team map[string]domain.TeamRole) []string {
	var issues []string
	for name, role := range team {
		if role.Mode == "" {
			issues = append(issues, fmt.Sprintf("team role %q must have a mode", name))
		} else {
			switch role.Mode {
			case "subagent", "inline":
				// valid
			default:
				issues = append(issues, fmt.Sprintf("team role %q has unknown mode %q (expected: subagent, inline)", name, role.Mode))
			}
		}
		if role.Model != "" {
			if _, err := model.ParseTier(role.Model); err != nil {
				issues = append(issues, fmt.Sprintf("team role %q has unknown_model_tier %q (expected: premium, standard, cheap)", name, role.Model))
			}
		}
	}
	return issues
}

// validatePluginDefs checks that plugin definitions have valid fields and
// that no plugin's excludes_methodology matches the current methodology.
func validatePluginDefs(plugins map[string]domain.PluginDef, methodology domain.Methodology) []string {
	var issues []string
	for name, def := range plugins {
		if def.InstallMethod == "" {
			issues = append(issues, fmt.Sprintf("plugin %q must have an install_method", name))
		} else {
			switch def.InstallMethod {
			case "claude_plugin", "skill_files":
				// valid
			case "package":
				issues = append(issues, validatePackagePlugin(name, def)...)
			default:
				issues = append(issues, fmt.Sprintf("plugin %q has unknown install_method %q (expected: claude_plugin, package, skill_files)", name, def.InstallMethod))
			}
		}
		if def.ExcludesMethodology != "" && def.Enabled && methodology != "" && string(methodology) == def.ExcludesMethodology {
			issues = append(issues, fmt.Sprintf("plugin %q is incompatible with methodology %q", name, methodology))
		}
		if def.Marketplace != nil {
			issues = append(issues, validatePluginMarketplace(name, def)...)
		}
	}
	issues = append(issues, conflictingMarketplaces(plugins)...)
	return append(issues, duplicatePackages(plugins)...)
}

// validatePackagePlugin checks a package declaration. The spec must be pinned
// because the agent runs the package's code: an unpinned spec would load
// whatever release is current when the agent next starts.
func validatePackagePlugin(name string, def domain.PluginDef) []string {
	var issues []string
	if len(def.SupportedAgents) == 0 {
		issues = append(issues, fmt.Sprintf("plugin %q needs supported_agents (pi, opencode)", name))
	}
	for _, agent := range def.SupportedAgents {
		if domain.PackageSettingsKey(domain.AgentID(agent)) == "" {
			issues = append(issues, fmt.Sprintf("plugin %q lists %q, but packages install only for pi and opencode", name, agent))
		}
	}
	if def.Package == "" {
		return append(issues, fmt.Sprintf("plugin %q needs a package (npm:<name>@<version> or git:<host>/<repo>@<ref>)", name))
	}
	spec, err := domain.ParsePackageSpec(def.Package)
	if err != nil {
		return append(issues, fmt.Sprintf("plugin %q %v", name, err))
	}
	switch {
	case spec.Pinned():
	case spec.Kind == "npm":
		issues = append(issues, fmt.Sprintf("plugin %q package %q must pin an exact version, as npm:<name>@1.2.3", name, def.Package))
	default:
		issues = append(issues, fmt.Sprintf("plugin %q package %q must pin a ref (a tag or commit), as git:<host>/<repo>@v1.0.0", name, def.Package))
	}
	if _, ok := domain.HarnessPackageEntry(domain.AgentOpenCode, spec); !ok && slices.Contains(def.SupportedAgents, string(domain.AgentOpenCode)) {
		issues = append(issues, fmt.Sprintf("plugin %q is a git package, which opencode cannot load; list it for pi only", name))
	}
	return issues
}

// duplicatePackages reports two declarations of one package: the harness
// loads a package once, so only one of the versions could apply.
func duplicatePackages(plugins map[string]domain.PluginDef) []string {
	var issues []string
	seen := make(map[string]string)
	for _, name := range slices.Sorted(maps.Keys(plugins)) {
		def := plugins[name]
		if def.InstallMethod != "package" {
			continue
		}
		spec, err := domain.ParsePackageSpec(def.Package)
		if err != nil {
			continue
		}
		if prev, ok := seen[spec.Identity()]; ok {
			issues = append(issues, fmt.Sprintf("plugin %q and plugin %q both declare package %q", prev, name, spec.Identity()))
			continue
		}
		seen[spec.Identity()] = name
	}
	return issues
}

// conflictingMarketplaces reports a marketplace name declared with two
// different sources: the settings file can hold only one.
func conflictingMarketplaces(plugins map[string]domain.PluginDef) []string {
	var issues []string
	seen := make(map[string]domain.PluginMarketplace)
	for _, name := range slices.Sorted(maps.Keys(plugins)) {
		def := plugins[name]
		_, market, _ := strings.Cut(def.PluginID, "@")
		if def.Marketplace == nil || market == "" {
			continue
		}
		if prev, ok := seen[market]; ok && prev != *def.Marketplace {
			issues = append(issues, fmt.Sprintf("plugin %q declares marketplace %q with a different source than another plugin", name, market))
			continue
		}
		seen[market] = *def.Marketplace
	}
	return issues
}

// validatePluginMarketplace checks a declared marketplace against the source
// types Claude Code documents for extraKnownMarketplaces.
func validatePluginMarketplace(name string, def domain.PluginDef) []string {
	var issues []string
	if def.InstallMethod != "claude_plugin" {
		issues = append(issues, fmt.Sprintf("plugin %q declares a marketplace, which needs install_method claude_plugin", name))
	}
	if _, market, ok := strings.Cut(def.PluginID, "@"); !ok || market == "" {
		issues = append(issues, fmt.Sprintf("plugin %q declares a marketplace, so plugin_id must be <plugin>@<marketplace>", name))
	}
	m := def.Marketplace
	required := map[string]string{"github": m.Repo, "git": m.URL, "url": m.URL, "file": m.Path, "directory": m.Path}
	field := map[string]string{"github": "repo", "git": "url", "url": "url", "file": "path", "directory": "path"}
	value, known := required[m.Source]
	switch {
	case !known:
		issues = append(issues, fmt.Sprintf("plugin %q marketplace has unknown source %q (expected: github, git, url, file, directory)", name, m.Source))
	case value == "":
		issues = append(issues, fmt.Sprintf("plugin %q marketplace source %q needs %s", name, m.Source, field[m.Source]))
	}
	return issues
}

var knownAdapters = map[string]struct{}{
	string(domain.AgentOpenCode):      {},
	string(domain.AgentClaudeCode):    {},
	string(domain.AgentVSCodeCopilot): {},
	string(domain.AgentCursor):        {},
	string(domain.AgentWindsurf):      {},
	string(domain.AgentPi):            {},
	string(domain.AgentCodex):         {},
}

var knownComponents = map[string]struct{}{
	string(domain.ComponentMemory):      {},
	string(domain.ComponentRules):       {},
	string(domain.ComponentSettings):    {},
	string(domain.ComponentMCP):         {},
	string(domain.ComponentAgents):      {},
	string(domain.ComponentSkills):      {},
	string(domain.ComponentCommands):    {},
	string(domain.ComponentPlugins):     {},
	string(domain.ComponentWorkflows):   {},
	string(domain.ComponentPermissions): {},
	string(domain.ComponentHooks):       {},
	string(domain.ComponentBrand):       {},
	string(domain.ComponentEfficiency):  {},
}

func isKnownAdapter(name string) bool {
	_, ok := knownAdapters[name]
	return ok
}

func isKnownComponent(name string) bool {
	_, ok := knownComponents[name]
	return ok
}
