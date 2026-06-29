package memory

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
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

	sectionID := SectionIDForAgentID(adapter.ID())

	// Check if the adapter-scoped section already matches.
	if marker.HasSection(string(existing), sectionID) {
		current := marker.ExtractSection(string(existing), sectionID)
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

	// Backward compatibility: single-adapter installs may still have the
	// legacy unscoped marker. Treat a matching legacy section as current.
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
	updated := InjectContent(string(existing), action.Agent, content)

	_, err = fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644)
	if err != nil {
		return fmt.Errorf("write target: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the memory component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentMemory) {
		return nil, nil
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

	sectionID := SectionIDForAgentID(adapter.ID())
	if !marker.HasSection(string(content), sectionID) && !marker.HasSection(string(content), SectionID) {
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
	current := marker.ExtractSection(string(content), sectionID)
	if current == "" {
		current = marker.ExtractSection(string(content), SectionID)
	}
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

// SectionIDForAgentID returns the adapter-scoped marker section used in shared
// files such as AGENTS.md. Scoped markers prevent different adapters from
// overwriting each other's memory content when they target the same file.
func SectionIDForAgentID(agentID domain.AgentID) string {
	if agentID == "" {
		return SectionID
	}
	return SectionID + ":" + string(agentID)
}

// InjectContent writes the canonical adapter-scoped memory section and removes
// the legacy unscoped section when present so migrated files do not keep stale
// duplicate memory blocks.
func InjectContent(document string, agentID domain.AgentID, content string) string {
	if marker.HasSection(document, SectionID) {
		document = marker.InjectSection(document, SectionID, "")
	}
	return marker.InjectSection(document, SectionIDForAgentID(agentID), content)
}

func openCodeMemoryTemplate() string {
	return assets.MustRead("memory/opencode.md")
}

func claudeCodeMemoryTemplate() string {
	return assets.MustRead("memory/claude.md")
}

func genericMemoryTemplate() string {
	return assets.MustRead("memory/generic.md")
}
