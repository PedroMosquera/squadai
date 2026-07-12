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

// Memory scopes supported by context profiles. "project" ("" defaults to it)
// installs the full protocol; "summary" installs the short stub; "full" adds
// the librarian/promote workflow paragraph on top of the protocol. Scope
// "none" disables the component entirely and never reaches this installer.
const (
	ScopeNone    = "none"
	ScopeSummary = "summary"
	ScopeProject = "project"
	ScopeFull    = "full"
)

// ProtocolStub is the condensed memory protocol used for the "summary" memory
// scope and for budget-fitter summary mode. It intentionally matches the
// sub-agent stub the agents installer injects into native sub-agent files.
const ProtocolStub = "## Project Memory Protocol\n\nBefore starting work, search memory: `/memory-search <query>`.\nAfter significant work, capture decisions: `/memory-add <note>`."

// fullScopeExtra is appended to the protocol for the "full" memory scope. It
// spells out the librarian/promote workflow for teams that lean on memory.
const fullScopeExtra = "**Full-scope workflow.** Treat memory as a first-class deliverable: delegate\nopen questions to `@librarian` before planning, and after each work session\nreview `docs/memory/_inbox/` and run `/memory-promote` (or\n`squadai memory promote`) so accepted notes graduate into topic folders."

// Options controls optional memory installer behavior.
type Options struct {
	// Scope is the context-profile memory scope: "", "project", "summary", or
	// "full". Empty behaves like "project" (the standard protocol).
	Scope string
}

// Installer implements domain.ComponentInstaller for the memory component.
type Installer struct {
	opts Options
}

// New returns a memory component installer.
func New(opts ...Options) *Installer {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Installer{opts: o}
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

	desiredContent := i.ContentForAgentID(adapter.ID(), "")

	sectionID := SectionIDForAgentID(adapter.ID())

	// Check if the adapter-scoped section already matches. Single-adapter
	// installs may still carry the legacy unscoped marker; a matching legacy
	// section is treated as current for backward compatibility.
	for _, sid := range []string{sectionID, SectionID} {
		if !marker.HasSection(string(existing), sid) {
			continue
		}
		current := marker.ExtractSection(string(existing), sid)
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

	content := i.ContentForAgentID(action.Agent, action.Mode)
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
	expected := i.ContentForAgentID(adapter.ID(), "")
	switch current {
	case expected:
		results = append(results, domain.VerifyResult{
			Check:  "memory-content-current",
			Passed: true,
		})
	case ProtocolStub:
		// The budget fitter may have degraded the section to the summary
		// stub — a legitimate installed state, not drift.
		results = append(results, domain.VerifyResult{
			Check:    "memory-content-current",
			Passed:   true,
			Severity: domain.SeverityWarning,
			Message:  "memory protocol installed in summary mode (token budget)",
		})
	default:
		results = append(results, domain.VerifyResult{
			Check:   "memory-content-current",
			Passed:  false,
			Message: "memory section content is outdated",
		})
	}

	return results, nil
}

// ContentForAgentID returns the memory content this installer would write for
// the given agent under the given action mode ("" or "summary"), honoring the
// installer's configured memory scope.
func (i *Installer) ContentForAgentID(agentID domain.AgentID, mode string) string {
	if mode == "summary" {
		return ProtocolStub
	}
	switch i.opts.Scope {
	case ScopeSummary:
		return ProtocolStub
	case ScopeFull:
		return templateForAgentID(agentID) + "\n\n" + fullScopeExtra
	default: // "", ScopeProject
		return templateForAgentID(agentID)
	}
}

// templateForAgentID returns the memory protocol template for a given agent ID.
func templateForAgentID(agentID domain.AgentID) string {
	switch agentID {
	case domain.AgentOpenCode:
		return assets.MustRead("memory/opencode.md")
	case domain.AgentClaudeCode:
		return assets.MustRead("memory/claude.md")
	default:
		return assets.MustRead("memory/generic.md")
	}
}

// ProtocolTemplate returns the generic memory protocol template.
// Kept for backward compatibility with external callers.
func ProtocolTemplate() string {
	return assets.MustRead("memory/generic.md")
}

// TemplateForAgentID returns the agent-specific memory protocol template
// (project scope). Use this to get the expected content for a specific agent
// in tests and callers that don't carry installer options.
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
