package memory

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

const (
	// SectionID is the marker section identifier for memory content.
	SectionID = "memory"
)

// Installer implements domain.ComponentInstaller for the memory component.
type Installer struct{}

// New returns a memory component installer.
func New() *Installer {
	return &Installer{}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentMemory
}

// memoryTargetPath returns the file path where memory content should be injected
// for the given adapter. Uses the project-level rules file if available, otherwise
// falls back to the global system prompt file.
func memoryTargetPath(adapter domain.Adapter, homeDir, projectDir string) string {
	if projectDir != "" {
		if p := adapter.ProjectRulesFile(projectDir); p != "" {
			return p
		}
	}
	return adapter.SystemPromptFile(homeDir)
}

// Plan determines what actions are needed for this adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentMemory) {
		return nil, nil
	}

	promptPath := memoryTargetPath(adapter, homeDir, projectDir)

	existing, err := fileutil.ReadFileOrEmpty(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read system prompt: %w", err)
	}

	desiredContent := templateForAdapter(adapter)

	// Check if the section already matches.
	if marker.HasSection(string(existing), SectionID) {
		current := marker.ExtractSection(string(existing), SectionID)
		if current == desiredContent {
			return []domain.PlannedAction{
				{
					ID:          fmt.Sprintf("%s-%s-memory", adapter.ID(), "prompt"),
					Agent:       adapter.ID(),
					Component:   domain.ComponentMemory,
					Action:      domain.ActionSkip,
					TargetPath:  promptPath,
					Description: "memory section already up to date",
				},
			}, nil
		}
		return []domain.PlannedAction{
			{
				ID:          fmt.Sprintf("%s-%s-memory", adapter.ID(), "prompt"),
				Agent:       adapter.ID(),
				Component:   domain.ComponentMemory,
				Action:      domain.ActionUpdate,
				TargetPath:  promptPath,
				Description: "update memory protocol in system prompt",
			},
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return []domain.PlannedAction{
		{
			ID:          fmt.Sprintf("%s-%s-memory", adapter.ID(), "prompt"),
			Agent:       adapter.ID(),
			Component:   domain.ComponentMemory,
			Action:      action,
			TargetPath:  promptPath,
			Description: "inject memory protocol into system prompt",
		},
	}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	existing, err := fileutil.ReadFileOrEmpty(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}

	content := templateForAgentID(action.Agent)
	updated := marker.InjectSection(string(existing), SectionID, content)

	_, err = fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644)
	if err != nil {
		return fmt.Errorf("write target: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the memory component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentMemory) {
		return []domain.VerifyResult{
			{
				Check:   "memory-supported",
				Passed:  false,
				Message: fmt.Sprintf("adapter %s does not support memory", adapter.ID()),
			},
		}, nil
	}

	promptPath := memoryTargetPath(adapter, homeDir, projectDir)
	var results []domain.VerifyResult

	// Check file exists.
	_, err := os.Stat(promptPath)
	if err != nil {
		results = append(results, domain.VerifyResult{
			Check:   "memory-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("system prompt file not found: %s", promptPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "memory-file-exists",
		Passed: true,
	})

	// Check marker section exists.
	content, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read prompt: %w", err)
	}

	if !marker.HasSection(string(content), SectionID) {
		results = append(results, domain.VerifyResult{
			Check:   "memory-markers-present",
			Passed:  false,
			Message: "memory marker section not found in system prompt",
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "memory-markers-present",
		Passed: true,
	})

	// Check content matches expected.
	current := marker.ExtractSection(string(content), SectionID)
	expected := templateForAdapter(adapter)
	if current != expected {
		results = append(results, domain.VerifyResult{
			Check:   "memory-content-current",
			Passed:  false,
			Message: "memory section content is outdated",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:  "memory-content-current",
			Passed: true,
		})
	}

	return results, nil
}

// templateForAdapter returns the agent-specific memory protocol template
// for the given adapter.
func templateForAdapter(adapter domain.Adapter) string {
	return templateForAgentID(adapter.ID())
}

// templateForAgentID returns the memory protocol template for a given agent ID.
// Used by Apply which only has access to the action's Agent field.
func templateForAgentID(agentID domain.AgentID) string {
	switch agentID {
	case domain.AgentOpenCode:
		return openCodeMemoryTemplate()
	case domain.AgentClaudeCode:
		return claudeCodeMemoryTemplate()
	case domain.AgentCodex:
		return codexMemoryTemplate()
	default:
		return genericMemoryTemplate()
	}
}

// ProtocolTemplate returns the generic memory protocol template.
// Kept for backward compatibility with external callers.
func ProtocolTemplate() string {
	return genericMemoryTemplate()
}

// TemplateForAgentID returns the agent-specific memory protocol template.
// Use this to get the expected content for a specific agent in tests and callers.
func TemplateForAgentID(agentID domain.AgentID) string {
	return templateForAgentID(agentID)
}

func openCodeMemoryTemplate() string {
	return `## Memory Protocol

### Session Start
- Read the project's AGENTS.md for context
- Check ` + "`.agent-manager/`" + ` for project configuration

### Save Triggers
Save important context to AGENTS.md or relevant documentation after:
- Architecture decisions
- Bug discoveries and fixes
- New conventions or patterns established
- Configuration changes

### Session End
Update AGENTS.md with a brief summary of significant changes made during this session.`
}

func claudeCodeMemoryTemplate() string {
	return `## Memory Protocol

### Session Start
- Review CLAUDE.md for project context and prior decisions

### Save Triggers
When you discover important project context, update CLAUDE.md with:
- Architecture decisions and rationale
- Bug patterns and solutions
- Project-specific conventions
- Environment setup notes

### Session End
Append a session summary to the relevant section of CLAUDE.md.`
}

func codexMemoryTemplate() string {
	return `## Memory Protocol

### Session Start
- Check project documentation for context and prior decisions

### Save Triggers
Save context after any of these events:
- Important decisions or architecture choices
- Bug discoveries and their fixes
- New conventions or patterns established

### Session End
Save a brief summary of significant changes made during this session.`
}

func genericMemoryTemplate() string {
	return `## Memory Protocol

You have access to persistent memory tools. Follow these rules:

### Save Triggers
Save context after any of these events:
- Important decisions or architecture choices
- Bug discoveries and their fixes
- New conventions or patterns established
- Configuration changes
- Dependency additions or removals

### Search Protocol
- At session start, search memory for relevant context
- Before making architectural decisions, check for prior decisions
- Use keyword search for specific topics

### Session Summary
At the end of each session, save a summary including:
- Goal: what was the objective
- Accomplished: what was completed
- Discoveries: what was learned
- Next Steps: what remains to be done`
}
