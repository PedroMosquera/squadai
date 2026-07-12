package domain

// UserConfig represents ~/.squadai/config.json.
type UserConfig struct {
	Version    int                        `json:"version"`
	Mode       OperationalMode            `json:"mode"`
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Paths      PathsConfig                `json:"paths"`
}

// ProjectConfig represents .squadai/project.json.
type ProjectConfig struct {
	Version     int                        `json:"version"`
	Preset      SetupPreset                `json:"preset,omitempty"`
	Adapters    map[string]AdapterConfig   `json:"adapters,omitempty"`
	Components  map[string]ComponentConfig `json:"components"`
	Copilot     CopilotConfig              `json:"copilot"`
	Rules       RulesConfig                `json:"rules,omitempty"`
	Agents      map[string]AgentDef        `json:"agents,omitempty"`
	Skills      map[string]SkillDef        `json:"skills,omitempty"`
	Commands    map[string]CommandDef      `json:"commands,omitempty"`
	MCP         map[string]MCPServerDef    `json:"mcp,omitempty"`
	Meta        ProjectMeta                `json:"meta,omitempty"`
	Methodology Methodology                `json:"methodology,omitempty"`
	ModelTier   ModelTier                  `json:"model_tier,omitempty"`
	Memory      MemoryConfig               `json:"memory,omitempty"`
	Context     ContextConfig              `json:"context,omitempty"`
	Usage       UsageConfig                `json:"usage,omitempty"`
	Models      ModelsConfig               `json:"models,omitempty"`
	Team        map[string]TeamRole        `json:"team,omitempty"`
	Plugins     map[string]PluginDef       `json:"plugins,omitempty"`
	Claude      ClaudeConfig               `json:"claude,omitempty"`
	Hooks       HooksConfig                `json:"hooks,omitempty"`
	Marketplace MarketplaceConfig          `json:"marketplace,omitempty"`
}

// MemoryConfig controls SquadAI's local-first project memory backend.
type MemoryConfig struct {
	Backend            string `json:"backend,omitempty"` // native, docs
	AutoCapture        bool   `json:"auto_capture,omitempty"`
	ProjectKeyStrategy string `json:"project_key_strategy,omitempty"` // git-remote, repo-name
	ExportPath         string `json:"export_path,omitempty"`
}

// ContextConfig defines named context profiles for daily agent sessions.
type ContextConfig struct {
	DefaultProfile string                    `json:"default_profile,omitempty"`
	Profiles       map[string]ContextProfile `json:"profiles,omitempty"`
}

// ContextProfile controls which local context is assembled for an agent run.
//
// MCPServers and SkillScopes deliberately have no omitempty: nil means "no
// filter" while a present-but-empty list is a strict filter that admits
// nothing, and that distinction must survive project.json round trips.
type ContextProfile struct {
	MemoryScope      string                 `json:"memory_scope,omitempty"`
	MCPServers       []string               `json:"mcp_servers"`
	SkillScopes      []string               `json:"skill_scopes"`
	MaxApproxTokens  int                    `json:"max_approx_tokens,omitempty"`
	Include          []string               `json:"include,omitempty"`
	Exclude          []string               `json:"exclude,omitempty"`
	AdapterOverrides map[string]interface{} `json:"adapter_overrides,omitempty"`
}

// UsageConfig stores approximate budget controls. Enforcement is best-effort
// until an adapter exposes exact token and cost telemetry.
type UsageConfig struct {
	DailyTokenBudget   int               `json:"daily_token_budget,omitempty"`
	SessionTokenBudget int               `json:"session_token_budget,omitempty"`
	DailyCostBudget    float64           `json:"daily_cost_budget,omitempty"`
	SessionCostBudget  float64           `json:"session_cost_budget,omitempty"`
	Enforcement        string            `json:"enforcement,omitempty"` // off, warn, ask, block
	Currency           string            `json:"currency,omitempty"`
	PriceCatalogSource string            `json:"price_catalog_source,omitempty"`
	ProfileTiers       map[string]string `json:"profile_tiers,omitempty"`
}

// ModelsConfig defines abstract model profiles and phase/role overrides.
type ModelsConfig struct {
	Profiles  map[string]ModelProfile `json:"profiles,omitempty"`
	Overrides map[string]string       `json:"overrides,omitempty"`
}

// ModelProfile is an adapter-neutral model routing preset.
type ModelProfile struct {
	Tier        string            `json:"tier,omitempty"`
	Description string            `json:"description,omitempty"`
	Adapters    map[string]string `json:"adapters,omitempty"`
}

// MarketplaceConfig tracks which plugin marketplace plugins are installed in
// this project. Recorded here so it is version-controlled and policy-lockable.
type MarketplaceConfig struct {
	// Source is the upstream registry (default: "github.com/wshobson/agents").
	Source string `json:"source,omitempty"`
	// Plugins maps plugin name to its installed version string.
	Plugins map[string]string `json:"plugins,omitempty"`
}

// ClaudeConfig holds Claude Code-specific feature toggles. Generic adapter
// behavior lives under Adapters[claude]; this struct only carries options
// that don't fit the generic AdapterConfig shape (e.g., feature opt-ins
// requiring policy-locked fields and special apply pipelines).
type ClaudeConfig struct {
	// AgentTeams opts the project into Claude Code's experimental Agent Teams
	// runtime by injecting CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 into the
	// project's .claude/settings.json env map.
	AgentTeams AgentTeamsConfig `json:"agent_teams,omitempty"`
}

// AgentTeamsConfig is the opt-in toggle for the experimental Agent Teams runtime.
type AgentTeamsConfig struct {
	Enabled bool `json:"enabled"`
}

// HookEntry is a single hook action — currently only "command" type is supported.
type HookEntry struct {
	Type    string `json:"type"` // always "command"
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // seconds; 0 = agent default (60 s)
}

// HookMatcher is a group of hooks optionally scoped to a specific tool.
// Matcher values match Claude Code's matcher field: "Bash", "Write", "Edit", etc.
// Empty Matcher means the hook fires for every tool in the event.
type HookMatcher struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []HookEntry `json:"hooks"`
}

// HooksConfig maps Claude Code hook event names to their matcher/hook lists.
// Supported events: PreToolUse, PostToolUse, Stop, UserPromptSubmit,
// SubagentStart, SubagentStop.
// Each event value is a list of HookMatcher groups, merged additively with
// any hooks the user has configured independently.
type HooksConfig map[string][]HookMatcher

// PolicyConfig represents .squadai/policy.json.
type PolicyConfig struct {
	Version  int             `json:"version"`
	Mode     OperationalMode `json:"mode"`
	Locked   []string        `json:"locked"`
	Required RequiredBlock   `json:"required"`
}

// RequiredBlock defines the enforced values in a policy.
type RequiredBlock struct {
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Copilot    CopilotConfig              `json:"copilot"`
	Rules      RulesConfig                `json:"rules,omitempty"`
	Agents     map[string]AgentDef        `json:"agents,omitempty"`
	MCP        map[string]MCPServerDef    `json:"mcp,omitempty"`
	Plugins    map[string]PluginDef       `json:"plugins,omitempty"`
	Claude     ClaudeConfig               `json:"claude,omitempty"`
	Hooks      HooksConfig                `json:"hooks,omitempty"`
}

// AdapterConfig is the per-adapter configuration in config files.
type AdapterConfig struct {
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// ComponentConfig is the per-component configuration in config files.
type ComponentConfig struct {
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// CopilotConfig controls shared copilot-instructions behavior.
type CopilotConfig struct {
	InstructionsTemplate string `json:"instructions_template"`
	CustomContent        string `json:"custom_content,omitempty"`

	// Meta is populated by the merger from ProjectConfig.Meta. It is not
	// serialized because the canonical source is ProjectConfig.Meta.
	Meta ProjectMeta `json:"-"`
}

// RulesConfig defines team standards content for instruction files.
type RulesConfig struct {
	// TeamStandards is inline markdown content injected into AGENTS.md / CLAUDE.md.
	TeamStandards string `json:"team_standards,omitempty"`
	// TeamStandardsFile is a path (relative to .squadai/) to load content from.
	TeamStandardsFile string `json:"team_standards_file,omitempty"`
	// Instructions lists additional instruction file paths to reference.
	Instructions []string `json:"instructions,omitempty"`
}

// PluginDef defines a third-party plugin that can be installed for supported agents.
type PluginDef struct {
	Description         string   `json:"description"`
	Enabled             bool     `json:"enabled"`
	SupportedAgents     []string `json:"supported_agents"`
	InstallMethod       string   `json:"install_method"`                 // "claude_plugin" or "skill_files"
	PluginID            string   `json:"plugin_id,omitempty"`            // e.g., "superpowers@claude-plugins-official"
	ExcludesMethodology string   `json:"excludes_methodology,omitempty"` // blocked when this methodology is active
}

// TeamRole defines a role within a methodology team.
type TeamRole struct {
	Description string   `json:"description"`
	Mode        string   `json:"mode"`                   // "subagent" or "inline"
	SkillRef    string   `json:"skill_ref,omitempty"`    // e.g., "tdd/brainstorming"
	DelegatesTo []string `json:"delegates_to,omitempty"` // roles this role can delegate to
	// Model is the abstract tier for this role: "premium", "standard", or "cheap".
	// Empty means use the default tier (standard). Each adapter resolves this to
	// a concrete model name at install time.
	Model string `json:"model,omitempty"`
}

// AgentDef defines a custom agent for OpenCode's .opencode/agents/ directory.
// Claude Code-specific fields (Skills, MaxTurns, Memory, Effort) are ignored
// by non-Claude adapters and are passed through to .claude/agents/*.md frontmatter.
type AgentDef struct {
	Description string            `json:"description"`
	Mode        string            `json:"mode"`
	Model       string            `json:"model,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	PromptFile  string            `json:"prompt_file,omitempty"`
	Permission  map[string]string `json:"permission,omitempty"`

	// Tools is the allowlist of tool names this agent may use (e.g. ["Read","Bash"]).
	// Empty means all tools are permitted. For Claude Code the list is rendered as a
	// comma-separated string; for OpenCode it is rendered as a YAML bool map.
	Tools []string `json:"tools,omitempty"`

	// Skills lists skill file paths relative to the project's skills directory.
	// Claude Code only; ignored for other adapters.
	Skills []string `json:"skills,omitempty"`

	// MaxTurns caps the number of agentic turns. 0 means no explicit cap (agent default).
	// Claude Code only; ignored for other adapters.
	MaxTurns int `json:"max_turns,omitempty"`

	// Memory lists explicit memory file paths to load into context.
	// When non-empty these override the default memory-scope injection.
	// Claude Code only; ignored for other adapters.
	Memory []string `json:"memory,omitempty"`

	// Effort sets the reasoning budget: "low", "normal", or "high".
	// Claude Code only; ignored for other adapters.
	Effort string `json:"effort,omitempty"`
}

// SkillDef defines a custom skill for OpenCode's .opencode/skills/ directory.
type SkillDef struct {
	Description string `json:"description"`
	Content     string `json:"content,omitempty"`
	ContentFile string `json:"content_file,omitempty"`
}

// CommandDef defines a custom command for OpenCode's .opencode/commands/ directory.
type CommandDef struct {
	Description string `json:"description"`
	Template    string `json:"template,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Model       string `json:"model,omitempty"`
}

// MCPServerDef defines an MCP server configuration.
type MCPServerDef struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Enabled     bool              `json:"enabled"`
	Environment map[string]string `json:"environment,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// ProjectMeta holds project metadata used for template rendering.
type ProjectMeta struct {
	Name           string   `json:"name,omitempty"`
	Language       string   `json:"language,omitempty"`
	Languages      []string `json:"languages,omitempty"`
	Framework      string   `json:"framework,omitempty"`
	TestCommand    string   `json:"test_command,omitempty"`
	BuildCommand   string   `json:"build_command,omitempty"`
	LintCommand    string   `json:"lint_command,omitempty"`
	PackageManager string   `json:"package_manager,omitempty"`
}

// PathsConfig holds user-overridable paths.
type PathsConfig struct {
	BackupDir string `json:"backup_dir"`
}

// MergedConfig is the resolved configuration after applying precedence rules.
// Policy locked fields are final. Project overrides user. User provides defaults.
type MergedConfig struct {
	Mode        OperationalMode            `json:"mode"`
	Preset      SetupPreset                `json:"preset,omitempty"`
	Adapters    map[string]AdapterConfig   `json:"adapters"`
	Components  map[string]ComponentConfig `json:"components"`
	Copilot     CopilotConfig              `json:"copilot"`
	Rules       RulesConfig                `json:"rules,omitempty"`
	Agents      map[string]AgentDef        `json:"agents,omitempty"`
	Skills      map[string]SkillDef        `json:"skills,omitempty"`
	Commands    map[string]CommandDef      `json:"commands,omitempty"`
	MCP         map[string]MCPServerDef    `json:"mcp,omitempty"`
	Meta        ProjectMeta                `json:"meta,omitempty"`
	Paths       PathsConfig                `json:"paths"`
	Methodology Methodology                `json:"methodology,omitempty"`
	ModelTier   ModelTier                  `json:"model_tier,omitempty"`
	Memory      MemoryConfig               `json:"memory,omitempty"`
	Context     ContextConfig              `json:"context,omitempty"`
	Usage       UsageConfig                `json:"usage,omitempty"`
	Models      ModelsConfig               `json:"models,omitempty"`
	Team        map[string]TeamRole        `json:"team,omitempty"`
	Plugins     map[string]PluginDef       `json:"plugins,omitempty"`
	Claude      ClaudeConfig               `json:"claude,omitempty"`
	Hooks       HooksConfig                `json:"hooks,omitempty"`
	Marketplace MarketplaceConfig          `json:"marketplace,omitempty"`

	// Violations is populated during merge when user/project values conflicted
	// with locked policy fields. These are informational — the policy value wins.
	Violations []string `json:"violations,omitempty"`

	// ActiveContextProfile is the resolved context profile applied for this run.
	// Runtime-only: set by the CLI after profile resolution so component
	// installers (memory, agents, skills, mcp) can adjust their output. Never
	// serialized — the canonical source is Context.Profiles.
	ActiveContextProfile *ContextProfile `json:"-"`
	// ActiveProfileName is the name of ActiveContextProfile. Runtime-only.
	ActiveProfileName string `json:"-"`
}

// DefaultUserConfig returns a sensible default for first-time users.
func DefaultUserConfig() *UserConfig {
	return &UserConfig{
		Version: 1,
		Mode:    ModePersonal,
		Adapters: map[string]AdapterConfig{
			string(AgentOpenCode):   {Enabled: true},
			string(AgentClaudeCode): {Enabled: false},
			string(AgentPi):         {Enabled: false},
		},
		Components: map[string]ComponentConfig{
			string(ComponentMemory):      {Enabled: true},
			string(ComponentPermissions): {Enabled: true},
			string(ComponentBrand):       {Enabled: true},
			string(ComponentEfficiency):  {Enabled: true},
		},
		Paths: PathsConfig{
			BackupDir: "~/.squadai/backups",
		},
	}
}

// DefaultProjectConfig returns a minimal project config.
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Version: 1,
		Preset:  PresetSoloMinimal,
		Components: map[string]ComponentConfig{
			string(ComponentMemory):     {Enabled: true},
			string(ComponentBrand):      {Enabled: true},
			string(ComponentEfficiency): {Enabled: true},
		},
		Copilot: CopilotConfig{
			InstructionsTemplate: "standard",
		},
		Memory:  DefaultMemoryConfig(),
		Context: DefaultContextConfig(),
		Usage:   DefaultUsageConfig(),
		Models:  DefaultModelsConfig(),
	}
}

// DefaultMemoryConfig returns the local native memory defaults.
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		Backend:            "native",
		AutoCapture:        true,
		ProjectKeyStrategy: "git-remote",
		ExportPath:         "docs/memory",
	}
}

// DefaultContextConfig returns the built-in context profiles.
//
// The "default" profile is deliberately non-restrictive (no MCP filter, no
// skill scoping, no token cap): profiles are ACTIVE now, and the baseline
// profile must reproduce plain-apply behavior exactly. Restrictive setups are
// an explicit switch away (`squadai profile cheap`, `apply --profile=review`).
//
// Profiles with an explicit MCPServers list keep "squadai" so switching
// profiles never silently removes the agent-console access to SquadAI itself.
// "review" and "cheap" intentionally keep no MCP servers at all.
func DefaultContextConfig() ContextConfig {
	return ContextConfig{
		DefaultProfile: "default",
		Profiles: map[string]ContextProfile{
			"default":  {MemoryScope: "project", Include: []string{"**/*"}, Exclude: []string{".git/**", "node_modules/**", "dist/**"}},
			"debug":    {MemoryScope: "project", MCPServers: []string{"squadai", "context7"}, SkillScopes: []string{"shared", "tdd/systematic-debugging"}, MaxApproxTokens: 16000, Include: []string{"**/*"}, Exclude: []string{".git/**", "node_modules/**", "dist/**"}},
			"feature":  {MemoryScope: "project", MCPServers: []string{"squadai", "context7"}, SkillScopes: []string{"shared", "tdd", "sdd"}, MaxApproxTokens: 20000, Include: []string{"**/*"}, Exclude: []string{".git/**", "node_modules/**", "dist/**"}},
			"review":   {MemoryScope: "project", MCPServers: []string{}, SkillScopes: []string{"shared/code-review"}, MaxApproxTokens: 10000, Include: []string{"**/*"}, Exclude: []string{".git/**", "node_modules/**", "dist/**"}},
			"docs":     {MemoryScope: "project", MCPServers: []string{"squadai", "context7"}, SkillScopes: []string{"shared"}, MaxApproxTokens: 10000, Include: []string{"docs/**", "README.md", "*.md"}, Exclude: []string{".git/**", "node_modules/**"}},
			"incident": {MemoryScope: "project", MCPServers: []string{"squadai", "context7"}, SkillScopes: []string{"shared", "tdd/systematic-debugging"}, MaxApproxTokens: 24000, Include: []string{"**/*"}, Exclude: []string{".git/**", "node_modules/**", "dist/**"}},
			"cheap":    {MemoryScope: "summary", MCPServers: []string{}, SkillScopes: []string{"shared"}, MaxApproxTokens: 6000, Include: []string{"README.md", "docs/**"}, Exclude: []string{".git/**", "node_modules/**", "dist/**"}},
		},
	}
}

// DefaultUsageConfig returns conservative approximate budget defaults.
func DefaultUsageConfig() UsageConfig {
	return UsageConfig{
		DailyTokenBudget:   200000,
		SessionTokenBudget: 50000,
		Enforcement:        "warn",
		Currency:           "USD",
		PriceCatalogSource: "embedded",
		ProfileTiers: map[string]string{
			"cheap":    "cheap",
			"default":  "balanced",
			"debug":    "balanced",
			"feature":  "premium",
			"review":   "balanced",
			"docs":     "cheap",
			"incident": "premium",
		},
	}
}

// DefaultModelsConfig returns adapter-neutral model profile labels.
func DefaultModelsConfig() ModelsConfig {
	return ModelsConfig{
		Profiles: map[string]ModelProfile{
			"cheap":    {Tier: "cheap", Description: "lowest-cost capable model for routine edits"},
			"balanced": {Tier: "balanced", Description: "default cost/quality profile"},
			"premium":  {Tier: "premium", Description: "flagship model profile for hard planning, debugging, and review"},
			"default":  {Tier: "balanced", Description: "compatibility alias for model_tier"},
		},
		Overrides: map[string]string{
			"tdd.brainstormer": "balanced",
			"tdd.planner":      "premium",
			"tdd.implementer":  "balanced",
			"tdd.reviewer":     "premium",
			"tdd.debugger":     "premium",
			"sdd.explorer":     "balanced",
			"sdd.proposer":     "premium",
			"sdd.spec-writer":  "premium",
			"sdd.designer":     "premium",
			"sdd.implementer":  "balanced",
			"sdd.verifier":     "premium",
		},
	}
}

// DefaultTeam returns the default team composition for the given methodology.
// Each role has a description, mode ("subagent"), and a SkillRef pointing to
// the embedded skill asset. Returns nil for unknown methodologies.
func DefaultTeam(m Methodology) map[string]TeamRole {
	switch m {
	case MethodologyTDD:
		return map[string]TeamRole{
			"orchestrator": {Description: "TDD orchestrator — delegates phases to specialized sub-agents", Mode: "subagent"},
			"brainstormer": {Description: "Question-asking and requirements exploration", Mode: "subagent", SkillRef: "tdd/brainstorming"},
			"planner":      {Description: "Test plan and implementation plan creation", Mode: "subagent", SkillRef: "tdd/writing-plans"},
			"implementer":  {Description: "Red-green-refactor implementation cycles", Mode: "subagent", SkillRef: "tdd/test-driven-development"},
			"reviewer":     {Description: "Two-stage code review: automated + design", Mode: "subagent", SkillRef: "shared/code-review"},
			"debugger":     {Description: "4-phase debugging: reproduce → isolate → fix → verify", Mode: "subagent", SkillRef: "tdd/systematic-debugging"},
		}
	case MethodologySDD:
		return map[string]TeamRole{
			"orchestrator": {Description: "SDD orchestrator — manages spec-driven workflow", Mode: "subagent"},
			"explorer":     {Description: "Codebase analysis and context gathering", Mode: "subagent", SkillRef: "sdd/sdd-explore"},
			"proposer":     {Description: "Solution proposals with tradeoff analysis", Mode: "subagent", SkillRef: "sdd/sdd-propose"},
			"spec-writer":  {Description: "Formal specification document authoring", Mode: "subagent", SkillRef: "sdd/sdd-spec"},
			"designer":     {Description: "Architecture and interface design", Mode: "subagent", SkillRef: "sdd/sdd-design"},
			"task-planner": {Description: "Dependency-ordered task breakdown", Mode: "subagent", SkillRef: "sdd/sdd-tasks"},
			"implementer":  {Description: "Spec-faithful implementation", Mode: "subagent", SkillRef: "sdd/sdd-apply"},
			"verifier":     {Description: "Spec compliance verification", Mode: "subagent", SkillRef: "sdd/sdd-verify"},
		}
	case MethodologyConventional:
		return map[string]TeamRole{
			"orchestrator": {Description: "Conventional orchestrator — direct implementation with review gates", Mode: "subagent"},
			"implementer":  {Description: "General-purpose implementation", Mode: "subagent"},
			"reviewer":     {Description: "Code review checklist", Mode: "subagent", SkillRef: "shared/code-review"},
			"tester":       {Description: "Test writing and coverage", Mode: "subagent", SkillRef: "shared/testing"},
		}
	}
	return nil
}

// CuratedMCPServer describes a server in the built-in MCP catalog shown during
// the init wizard.
type CuratedMCPServer struct {
	Name        string   // unique key used in mcpSelections
	DisplayName string   // optional human-facing name; empty means Name is shown
	Description string   // human-readable description shown in TUI
	Type        string   // "remote" or "local"
	PreChecked  bool     // whether the item starts selected in the TUI
	Command     string   // primary executable for local servers
	URL         string   // endpoint for remote servers
	Args        []string // additional CLI arguments

	// Auth metadata — populated for servers that require credentials to function.
	RequiresAuth bool     // true if env vars must be set before the server will work
	AuthEnvVars  []string // e.g. ["GITHUB_PERSONAL_ACCESS_TOKEN"]
	SetupURL     string   // where to obtain credentials, e.g. "https://github.com/settings/tokens"
	SetupHint    string   // one-line human instruction shown in the post-install panel

	// Doctor metadata — used by `squadai doctor` to report server health.
	// RequiredEnvVars lists env vars that must be set for the server to authenticate.
	RequiredEnvVars []string // e.g. ["GITHUB_PERSONAL_ACCESS_TOKEN"]
	// MinNodeVersion is the minimum Node.js major version required (e.g. "20").
	MinNodeVersion string // e.g. "20"
}

// DefaultMCPCatalog returns the 6 curated MCP servers offered during init.
// The SquadAI control-plane server and Context7 are pre-checked; the others
// default to unselected. The community knowledge-graph server (config key
// "memory", kept for compatibility) is intentionally sorted last: it overlaps
// with SquadAI Project Memory and is de-emphasized in the setup flows.
func DefaultMCPCatalog() []CuratedMCPServer {
	return []CuratedMCPServer{
		{
			// SquadAI's own stdio MCP server: registered into every
			// MCP-capable agent so squadai can be driven from inside the
			// agent console. Command is the bare binary name — after install
			// "squadai" is on PATH; `squadai doctor` warns when it is not.
			Name:        "squadai",
			Description: "SquadAI control plane — plan, apply, verify, status, and project memory from inside your agent",
			Type:        "local",
			PreChecked:  true,
			Command:     "squadai",
			Args:        []string{"mcp-server"},
		},
		{
			Name:        "context7",
			Description: "Up-to-date documentation lookup for libraries and frameworks",
			Type:        "local",
			PreChecked:  true,
			Command:     "npx",
			Args:        []string{"-y", "@upstash/context7-mcp@latest"},
		},
		{
			Name:            "github",
			Description:     "Issues, PRs, code search",
			Type:            "local",
			PreChecked:      false,
			Command:         "npx",
			Args:            []string{"-y", "@modelcontextprotocol/server-github"},
			RequiresAuth:    true,
			AuthEnvVars:     []string{"GITHUB_PERSONAL_ACCESS_TOKEN"},
			RequiredEnvVars: []string{"GITHUB_PERSONAL_ACCESS_TOKEN"},
			SetupURL:        "https://github.com/settings/tokens",
			SetupHint:       "Create a personal access token with repo scope",
		},
		{
			Name:            "sentry",
			Description:     "Error monitoring and issue tracking",
			Type:            "local",
			PreChecked:      false,
			Command:         "npx",
			Args:            []string{"-y", "@sentry/mcp-server"},
			RequiresAuth:    true,
			AuthEnvVars:     []string{"SENTRY_AUTH_TOKEN"},
			RequiredEnvVars: []string{"SENTRY_AUTH_TOKEN"},
			SetupURL:        "https://sentry.io/settings/account/api/auth-tokens/",
			SetupHint:       "Create an auth token with project:read scope",
		},
		{
			Name:        "sequential-thinking",
			Description: "Structured reasoning enhancement for complex tasks",
			Type:        "local",
			PreChecked:  false,
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
		},
		{
			// Config key stays "memory" for compatibility (--mcp=memory and
			// existing project.json entries keep working); only the display
			// name and description are de-emphasized.
			Name:        "memory",
			DisplayName: "knowledge-graph (community)",
			Description: "Community knowledge-graph MCP server. Overlaps with SquadAI Project Memory — most people should use Project Memory instead.",
			Type:        "local",
			PreChecked:  false,
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-memory"},
		},
	}
}

// Display returns the human-facing name for a curated MCP server.
func (s CuratedMCPServer) Display() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return s.Name
}

// DefaultPolicyConfig returns a team policy that locks the baseline.
func DefaultPolicyConfig() *PolicyConfig {
	return &PolicyConfig{
		Version: 1,
		Mode:    ModeTeam,
		Locked: []string{
			"adapters.opencode.enabled",
			"components.memory.enabled",
			"copilot.instructions_template",
		},
		Required: RequiredBlock{
			Adapters: map[string]AdapterConfig{
				string(AgentOpenCode): {Enabled: true},
			},
			Components: map[string]ComponentConfig{
				string(ComponentMemory): {Enabled: true},
			},
			Copilot: CopilotConfig{
				InstructionsTemplate: "standard",
			},
		},
	}
}
