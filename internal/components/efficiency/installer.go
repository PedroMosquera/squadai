// Package efficiency installs the session-efficiency protocol — a short,
// always-on block of context-discipline rules (search-before-read, output
// summarization, delegation/timeboxing, checkpoint-at-60%) injected into each
// adapter's project rules file. It is the first context-discipline prompting
// solo (non-team) users receive.
package efficiency

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// SectionID is the marker section identifier for efficiency content. Sections
// in shared files (e.g. AGENTS.md used by both OpenCode and Pi) are adapter-
// scoped via SectionIDForAgentID so adapters do not overwrite each other.
const SectionID = "efficiency"

// Options controls efficiency template rendering.
type Options struct {
	// MemoryEnabled renders the memory-search-first rule when true.
	MemoryEnabled bool
}

// templateData is the data passed to the session.md text/template.
type templateData struct {
	// Delegation is true for adapters that can delegate work to sub-agents
	// (native or prompt-based delegation); false for solo adapters, which get
	// the timebox-and-checkpoint variant instead.
	Delegation    bool
	MemoryEnabled bool
}

// Installer implements domain.ComponentInstaller for the efficiency component.
type Installer struct {
	opts Options
}

// New returns an efficiency component installer.
func New(opts ...Options) *Installer {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Installer{opts: o}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentEfficiency
}

// targetPath returns the file where efficiency content is injected: the
// project rules file when available, else the global system prompt file.
func targetPath(adapter domain.Adapter, homeDir, projectDir string) string {
	if projectDir != "" {
		if p := adapter.ProjectRulesFile(projectDir); p != "" {
			return p
		}
	}
	return adapter.SystemPromptFile(homeDir)
}

// delegationForAgentID reports whether the agent can delegate to sub-agents.
// Mirrors each adapter's DelegationStrategy: solo adapters (VS Code Copilot,
// Windsurf) run everything inline; the rest delegate natively or via prompts.
func delegationForAgentID(agentID domain.AgentID) bool {
	switch agentID {
	case domain.AgentVSCodeCopilot, domain.AgentWindsurf:
		return false
	default:
		return true
	}
}

// SectionIDForAgentID returns the adapter-scoped marker section ID.
func SectionIDForAgentID(agentID domain.AgentID) string {
	if agentID == "" {
		return SectionID
	}
	return SectionID + ":" + string(agentID)
}

// ContentForAgentID renders the session-efficiency protocol for an agent.
func (i *Installer) ContentForAgentID(agentID domain.AgentID) (string, error) {
	tmpl, err := template.New("session").Parse(assets.MustRead("efficiency/session.md"))
	if err != nil {
		return "", fmt.Errorf("parse efficiency template: %w", err)
	}
	var b strings.Builder
	err = tmpl.Execute(&b, templateData{
		Delegation:    delegationForAgentID(agentID),
		MemoryEnabled: i.opts.MemoryEnabled,
	})
	if err != nil {
		return "", fmt.Errorf("render efficiency template: %w", err)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// InjectContent writes the adapter-scoped efficiency section, clearing any
// legacy unscoped section so migrated files never carry duplicates.
func InjectContent(document string, agentID domain.AgentID, content string) string {
	if marker.HasSection(document, SectionID) {
		document = marker.InjectSection(document, SectionID, "")
	}
	return marker.InjectSection(document, SectionIDForAgentID(agentID), content)
}

// Plan determines what actions are needed for this adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentEfficiency) {
		return nil, nil
	}

	promptPath := targetPath(adapter, homeDir, projectDir)

	existing, err := fileutil.ReadFileOrEmpty(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read rules file: %w", err)
	}

	desired, err := i.ContentForAgentID(adapter.ID())
	if err != nil {
		return nil, err
	}

	actionID := fmt.Sprintf("%s-prompt-efficiency", adapter.ID())
	base := domain.PlannedAction{
		ID:         actionID,
		Agent:      adapter.ID(),
		Component:  domain.ComponentEfficiency,
		TargetPath: promptPath,
	}

	for _, sid := range []string{SectionIDForAgentID(adapter.ID()), SectionID} {
		if !marker.HasSection(string(existing), sid) {
			continue
		}
		if marker.ExtractSection(string(existing), sid) == desired {
			base.Action = domain.ActionSkip
			base.Description = "efficiency section already up to date"
			return []domain.PlannedAction{base}, nil
		}
		base.Action = domain.ActionUpdate
		base.Description = "update session-efficiency protocol"
		return []domain.PlannedAction{base}, nil
	}

	base.Action = domain.ActionCreate
	if len(existing) > 0 {
		base.Action = domain.ActionUpdate
	}
	base.Description = "inject session-efficiency protocol"
	return []domain.PlannedAction{base}, nil
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

	content, err := i.ContentForAgentID(action.Agent)
	if err != nil {
		return err
	}
	updated := InjectContent(string(existing), action.Agent, content)

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write target: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the efficiency component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentEfficiency) {
		return nil, nil
	}

	promptPath := targetPath(adapter, homeDir, projectDir)
	var results []domain.VerifyResult

	content, err := os.ReadFile(promptPath)
	if err != nil {
		results = append(results, domain.VerifyResult{
			Check:   "efficiency-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("rules file not found: %s", promptPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "efficiency-file-exists",
		Passed: true,
	})

	sectionID := SectionIDForAgentID(adapter.ID())
	if !marker.HasSection(string(content), sectionID) && !marker.HasSection(string(content), SectionID) {
		results = append(results, domain.VerifyResult{
			Check:   "efficiency-markers-present",
			Passed:  false,
			Message: "efficiency marker section not found in rules file",
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "efficiency-markers-present",
		Passed: true,
	})

	current := marker.ExtractSection(string(content), sectionID)
	if current == "" {
		current = marker.ExtractSection(string(content), SectionID)
	}
	expected, err := i.ContentForAgentID(adapter.ID())
	if err != nil {
		return nil, err
	}
	if current != expected {
		results = append(results, domain.VerifyResult{
			Check:   "efficiency-content-current",
			Passed:  false,
			Message: "efficiency section content is outdated",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:  "efficiency-content-current",
			Passed: true,
		})
	}

	return results, nil
}
