package domain

// AgentID identifies a supported AI agent/tool.
type AgentID string

const (
	AgentOpenCode      AgentID = "opencode"
	AgentClaudeCode    AgentID = "claude-code"
	AgentVSCodeCopilot AgentID = "vscode-copilot"
	AgentCursor        AgentID = "cursor"
	AgentWindsurf      AgentID = "windsurf"
)

// Methodology identifies a development methodology preset.
type Methodology string

const (
	MethodologyTDD          Methodology = "tdd"
	MethodologySDD          Methodology = "sdd"
	MethodologyConventional Methodology = "conventional"
)

// SetupPreset identifies a named setup configuration shortcut.
type SetupPreset string

const (
	PresetFullSquad SetupPreset = "full-squad"
	PresetLean      SetupPreset = "lean"
	PresetCustom    SetupPreset = "custom"
)

// ModelTier identifies the AI model cost/quality preset for agent configuration.
type ModelTier string

const (
	ModelTierBalanced    ModelTier = "balanced"
	ModelTierPerformance ModelTier = "performance"
	ModelTierStarter     ModelTier = "starter"
	ModelTierManual      ModelTier = "manual"
)

// DelegationStrategy describes how an agent handles sub-agent delegation.
type DelegationStrategy string

const (
	DelegationNativeAgents DelegationStrategy = "native" // OpenCode, Cursor
	DelegationPromptBased  DelegationStrategy = "prompt" // Claude Code
	DelegationSoloAgent    DelegationStrategy = "solo"   // VS Code, Windsurf
)

// AdapterLane classifies whether an adapter is team-required or personal-optional.
type AdapterLane string

const (
	LaneTeam     AdapterLane = "team"
	LanePersonal AdapterLane = "personal"
)

// ComponentID identifies an installable component.
type ComponentID string

const (
	ComponentMemory      ComponentID = "memory"
	ComponentRules       ComponentID = "rules"
	ComponentSettings    ComponentID = "settings"
	ComponentMCP         ComponentID = "mcp"
	ComponentAgents      ComponentID = "agents"
	ComponentSkills      ComponentID = "skills"
	ComponentCommands    ComponentID = "commands"
	ComponentPlugins     ComponentID = "plugins"
	ComponentWorkflows   ComponentID = "workflows"
	ComponentPermissions ComponentID = "permissions"
	// ComponentCleanup is used for stale-file deletion actions that are not
	// associated with a specific installable component.
	ComponentCleanup ComponentID = "cleanup"
)

// OperationalMode determines config precedence behavior.
type OperationalMode string

const (
	ModeTeam     OperationalMode = "team"
	ModePersonal OperationalMode = "personal"
)

// StepStatus is the outcome of a single pipeline step.
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepRunning    StepStatus = "running"
	StepSuccess    StepStatus = "success"
	StepFailed     StepStatus = "failed"
	StepRolledBack StepStatus = "rolled_back"
)

// ActionType describes what a planned step intends to do.
type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionUpdate ActionType = "update"
	ActionDelete ActionType = "delete"
	ActionSkip   ActionType = "skip"
)

// PlannedAction is a single step the planner produces before execution.
type PlannedAction struct {
	ID          string      `json:"id"`
	Agent       AgentID     `json:"agent"`
	Component   ComponentID `json:"component"`
	Action      ActionType  `json:"action"`
	TargetPath  string      `json:"target_path"`
	Description string      `json:"description"`
}

// StepResult records what happened when a PlannedAction was executed.
type StepResult struct {
	Action PlannedAction `json:"action"`
	Status StepStatus    `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// ApplyReport is the full result of a plan execution.
type ApplyReport struct {
	Steps    []StepResult `json:"steps"`
	BackupID string       `json:"backup_id,omitempty"`
	Success  bool         `json:"success"`
}

// Severity levels for verification results.
const (
	SeverityError   = "error"   // check failed — needs action
	SeverityWarning = "warning" // informational — policy override, detected but not configured
	SeverityInfo    = "info"    // check passed
)

// VerifyResult is a single check from the verifier.
type VerifyResult struct {
	Check     string `json:"check"`
	Passed    bool   `json:"passed"`
	Severity  string `json:"severity"`            // "error", "warning", "info"
	Component string `json:"component,omitempty"` // which component or subsystem produced this
	Message   string `json:"message,omitempty"`
}

// VerifyReport is the full verification output.
type VerifyReport struct {
	Results []VerifyResult `json:"results"`
	AllPass bool           `json:"all_pass"`
}
