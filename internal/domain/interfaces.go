package domain

import "context"

// Adapter is the core abstraction for AI agent integration.
// Each supported agent (OpenCode, Claude Code, VS Code Copilot, Cursor, Windsurf)
// implements this interface.
// No path logic should exist outside adapter implementations.
type Adapter interface {
	// ID returns the unique identifier for this agent.
	ID() AgentID

	// Lane returns whether this is a team or personal adapter.
	Lane() AdapterLane

	// Detect checks whether the agent binary and config directory exist.
	// Returns: installed (binary found), configFound (config dir exists), error.
	Detect(ctx context.Context, homeDir string) (installed bool, configFound bool, err error)

	// GlobalConfigDir returns the root config directory for this agent.
	GlobalConfigDir(homeDir string) string

	// SystemPromptFile returns the path to the agent's system prompt file.
	SystemPromptFile(homeDir string) string

	// SkillsDir returns the directory where skill files are stored.
	SkillsDir(homeDir string) string

	// SettingsPath returns the path to the agent's settings file.
	SettingsPath(homeDir string) string

	// SupportsComponent reports whether this adapter supports a given component.
	SupportsComponent(component ComponentID) bool

	// ProjectConfigFile returns the path to the project-level config file.
	// Returns empty string if this adapter has no project config.
	ProjectConfigFile(projectDir string) string

	// ProjectRulesFile returns the path to the project-level rules/instructions file.
	// Returns empty string if this adapter has no project rules file.
	ProjectRulesFile(projectDir string) string

	// ProjectAgentsDir returns the path to project-level agent definitions.
	// Returns empty string if this adapter does not support project agents.
	ProjectAgentsDir(projectDir string) string

	// ProjectSkillsDir returns the path to project-level skill definitions.
	// Returns empty string if this adapter does not support project skills.
	ProjectSkillsDir(projectDir string) string

	// ProjectCommandsDir returns the path to project-level command definitions.
	// Returns empty string if this adapter does not support project commands.
	ProjectCommandsDir(projectDir string) string

	// DelegationStrategy returns how this agent handles sub-agent delegation.
	DelegationStrategy() DelegationStrategy

	// SupportsSubAgents reports whether this adapter can create named sub-agents.
	SupportsSubAgents() bool

	// SubAgentsDir returns the global sub-agents directory for this adapter.
	// Returns empty string if sub-agents are not supported.
	SubAgentsDir(homeDir string) string

	// SupportsWorkflows reports whether this adapter supports workflow files.
	SupportsWorkflows() bool

	// WorkflowsDir returns the project-level workflows directory.
	// Returns empty string if workflows are not supported.
	WorkflowsDir(projectDir string) string
}

// ComponentInstaller handles installation and sync for a single component.
type ComponentInstaller interface {
	// ID returns which component this installer manages.
	ID() ComponentID

	// Plan computes what actions are needed for this component on a given adapter.
	Plan(adapter Adapter, homeDir, projectDir string) ([]PlannedAction, error)

	// Apply executes a single planned action. Returns an error if the action fails.
	// The caller is responsible for rollback coordination.
	Apply(action PlannedAction) error

	// Verify checks post-apply state for this component on a given adapter.
	Verify(adapter Adapter, homeDir, projectDir string) ([]VerifyResult, error)
}

// ConfigLoader loads and merges configuration from the three-layer stack.
type ConfigLoader interface {
	// LoadUser loads ~/.squadai/config.json.
	LoadUser(homeDir string) (*UserConfig, error)

	// LoadProject loads .squadai/project.json from projectDir.
	LoadProject(projectDir string) (*ProjectConfig, error)

	// LoadPolicy loads .squadai/policy.json from projectDir.
	LoadPolicy(projectDir string) (*PolicyConfig, error)

	// Merge combines all three layers with precedence rules.
	// Policy locked fields override everything.
	Merge(user *UserConfig, project *ProjectConfig, policy *PolicyConfig) (*MergedConfig, error)
}

// Planner computes the full action plan from merged config + detected adapters.
type Planner interface {
	// Plan returns the ordered list of actions to reach desired state.
	Plan(config *MergedConfig, adapters []Adapter, homeDir, projectDir string) ([]PlannedAction, error)
}

// Executor runs a plan with backup/rollback safety.
type Executor interface {
	// Execute runs all planned actions.
	// It creates a backup before mutating, and rolls back on failure.
	Execute(plan []PlannedAction) (*ApplyReport, error)
}

// Verifier checks the system state after apply.
type Verifier interface {
	// Verify runs all checks and returns a report.
	Verify(config *MergedConfig, adapters []Adapter, homeDir, projectDir string) (*VerifyReport, error)
}
