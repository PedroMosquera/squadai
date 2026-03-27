package memory

import (
	"fmt"
	"os"

	"github.com/alexmosquera/agent-manager-pro/internal/domain"
	"github.com/alexmosquera/agent-manager-pro/internal/fileutil"
	"github.com/alexmosquera/agent-manager-pro/internal/marker"
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

// Plan determines what actions are needed for this adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentMemory) {
		return nil, nil
	}

	promptPath := adapter.SystemPromptFile(homeDir)

	existing, err := fileutil.ReadFileOrEmpty(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read system prompt: %w", err)
	}

	desiredContent := ProtocolTemplate()

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

	updated := marker.InjectSection(string(existing), SectionID, ProtocolTemplate())

	_, err = fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644)
	if err != nil {
		return fmt.Errorf("write target: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the memory component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentMemory) {
		return []domain.VerifyResult{
			{
				Check:   "memory-supported",
				Passed:  false,
				Message: fmt.Sprintf("adapter %s does not support memory", adapter.ID()),
			},
		}, nil
	}

	promptPath := adapter.SystemPromptFile(homeDir)
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
	if current != ProtocolTemplate() {
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

// ProtocolTemplate returns the memory protocol instructions injected into
// the agent's system prompt. This defines the behavioral contract for
// how the agent should use memory tools.
func ProtocolTemplate() string {
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
