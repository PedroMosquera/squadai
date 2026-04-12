package domain

// AgentID identifies a supported AI agent/tool.
type AgentID string

const (
	AgentOpenCode  AgentID = "opencode"
	AgentClaudeCode AgentID = "claude-code"
	AgentCodex     AgentID = "codex"
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
	ComponentMemory   ComponentID = "memory"
	ComponentRules    ComponentID = "rules"
	ComponentSettings ComponentID = "settings"
	ComponentMCP      ComponentID = "mcp"
	ComponentAgents   ComponentID = "agents"
	ComponentSkills   ComponentID = "skills"
	ComponentCommands ComponentID = "commands"
)

// OperationalMode determines config precedence behavior.
type OperationalMode string

const (
	ModeTeam     OperationalMode = "team"
	ModePersonal OperationalMode = "personal"
	ModeHybrid   OperationalMode = "hybrid"
)

// StepStatus is the outcome of a single pipeline step.
type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepRunning  StepStatus = "running"
	StepSuccess  StepStatus = "success"
	StepFailed   StepStatus = "failed"
	StepRolledBack StepStatus = "rolled_back"
)

// ActionType describes what a planned step intends to do.
type ActionType string

const (
	ActionCreate  ActionType = "create"
	ActionUpdate  ActionType = "update"
	ActionDelete  ActionType = "delete"
	ActionSkip    ActionType = "skip"
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

// VerifyResult is a single check from the verifier.
type VerifyResult struct {
	Check   string `json:"check"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// VerifyReport is the full verification output.
type VerifyReport struct {
	Results []VerifyResult `json:"results"`
	AllPass bool           `json:"all_pass"`
}
