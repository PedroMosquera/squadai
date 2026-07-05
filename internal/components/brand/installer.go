package brand

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

const (
	// SectionID is the marker section identifier for brand banner content.
	SectionID = "brand"
)

// Installer implements domain.ComponentInstaller for the brand component.
// It injects an ASCII-art banner block into an agent's rules/prompt file so
// developers get a visual indicator that SquadAI is active at session start.
type Installer struct{}

// New returns a brand component installer.
func New() *Installer {
	return &Installer{}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentBrand
}

// brandTargetPath returns the file path where the brand banner should be
// injected for the given adapter. Uses the project-level rules file if
// available, otherwise falls back to the global system prompt file.
func brandTargetPath(adapter domain.Adapter, homeDir, projectDir string) string {
	if projectDir != "" {
		if p := adapter.ProjectRulesFile(projectDir); p != "" {
			return p
		}
	}
	return adapter.SystemPromptFile(homeDir)
}

// brandTarget describes one file the brand banner is injected into.
type brandTarget struct {
	path     string
	idSuffix string // distinguishes multiple actions per adapter
	global   bool   // true for Pi's extra home-directory target
}

// brandTargets returns every file the banner should land in for this adapter.
// All adapters get the primary target (project rules file, falling back to the
// global system prompt). Pi additionally gets its global ~/.pi/agent/AGENTS.md
// so every Pi session carries the banner, not just this project's.
func brandTargets(adapter domain.Adapter, homeDir, projectDir string) []brandTarget {
	targets := []brandTarget{{path: brandTargetPath(adapter, homeDir, projectDir), idSuffix: "banner"}}
	if adapter.ID() == domain.AgentPi && homeDir != "" {
		global := adapter.SystemPromptFile(homeDir)
		if global != "" && global != targets[0].path {
			targets = append(targets, brandTarget{path: global, idSuffix: "banner-global", global: true})
		}
	}
	return targets
}

// Plan determines what actions are needed for this adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentBrand) {
		return nil, nil
	}

	var actions []domain.PlannedAction
	for _, tgt := range brandTargets(adapter, homeDir, projectDir) {
		action, err := planTarget(adapter, tgt)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// planTarget computes the planned action for a single banner target file.
func planTarget(adapter domain.Adapter, tgt brandTarget) (domain.PlannedAction, error) {
	existing, err := fileutil.ReadFileOrEmpty(tgt.path)
	if err != nil {
		return domain.PlannedAction{}, fmt.Errorf("read target file: %w", err)
	}

	base := domain.PlannedAction{
		ID:         fmt.Sprintf("%s-%s-brand", adapter.ID(), tgt.idSuffix),
		Agent:      adapter.ID(),
		Component:  domain.ComponentBrand,
		TargetPath: tgt.path,
	}

	desiredContent := fence(templateForAdapter(adapter))
	sectionID := SectionIDForAgentID(adapter.ID())

	// Check the adapter-scoped section first; fall back to a legacy unscoped
	// section for single-adapter installs (new writes migrate to scoped markers).
	for _, sid := range []string{sectionID, SectionID} {
		if !marker.HasSection(string(existing), sid) {
			continue
		}
		if marker.ExtractSection(string(existing), sid) == desiredContent {
			base.Action = domain.ActionSkip
			base.Description = "brand banner already up to date"
		} else {
			base.Action = domain.ActionUpdate
			base.Description = "update brand banner in target file"
		}
		return base, nil
	}

	base.Action = domain.ActionCreate
	if len(existing) > 0 {
		base.Action = domain.ActionUpdate
	}
	base.Description = "inject brand banner into target file"
	return base, nil
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

	banner := templateForAgentID(action.Agent)
	// The banner asset does not include a markdown fence; wrap it so the
	// ASCII art renders verbatim inside agent rules/prompt files.
	fenced := "```text\n" + banner + "\n```\n"
	updated := InjectContent(string(existing), action.Agent, fenced)

	_, err = fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644)
	if err != nil {
		return fmt.Errorf("write target: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the brand component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentBrand) {
		return nil, nil
	}

	var results []domain.VerifyResult
	for _, tgt := range brandTargets(adapter, homeDir, projectDir) {
		checkPrefix := "brand"
		if tgt.global {
			checkPrefix = "brand-global"
		}
		targetResults, err := verifyTarget(adapter, tgt.path, checkPrefix)
		if err != nil {
			return nil, err
		}
		results = append(results, targetResults...)
	}
	return results, nil
}

// verifyTarget checks a single banner target file.
func verifyTarget(adapter domain.Adapter, promptPath, checkPrefix string) ([]domain.VerifyResult, error) {
	var results []domain.VerifyResult

	// Check file exists.
	_, err := os.Stat(promptPath)
	if err != nil {
		results = append(results, domain.VerifyResult{
			Check:   checkPrefix + "-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("target file not found: %s", promptPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  checkPrefix + "-file-exists",
		Passed: true,
	})

	// Check marker section exists.
	content, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}

	sectionID := SectionIDForAgentID(adapter.ID())
	if !marker.HasSection(string(content), sectionID) && !marker.HasSection(string(content), SectionID) {
		results = append(results, domain.VerifyResult{
			Check:   checkPrefix + "-markers-present",
			Passed:  false,
			Message: "brand marker section not found in target file",
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  checkPrefix + "-markers-present",
		Passed: true,
	})

	// Check content matches expected.
	current := marker.ExtractSection(string(content), sectionID)
	if current == "" {
		current = marker.ExtractSection(string(content), SectionID)
	}
	expected := fence(templateForAdapter(adapter))
	if current != expected {
		results = append(results, domain.VerifyResult{
			Check:   checkPrefix + "-content-current",
			Passed:  false,
			Message: "brand banner content is outdated",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:  checkPrefix + "-content-current",
			Passed: true,
		})
	}

	return results, nil
}

// fence wraps a banner in a markdown fenced code block. This is the canonical
// form stored in the marker section and compared against marker.ExtractSection
// output (which strips the surrounding newlines added during injection).
func fence(banner string) string {
	return "```text\n" + banner + "\n```"
}

// templateForAdapter returns the agent-specific brand banner for the given adapter.
func templateForAdapter(adapter domain.Adapter) string {
	return templateForAgentID(adapter.ID())
}

// templateForAgentID returns the brand banner for a given agent ID.
// Used by Apply which only has access to the action's Agent field.
func templateForAgentID(agentID domain.AgentID) string {
	switch agentID {
	case domain.AgentOpenCode:
		return assets.MustRead("brand/banner-opencode.txt")
	case domain.AgentPi:
		return assets.MustRead("brand/banner-pi.txt")
	case domain.AgentClaudeCode:
		return assets.MustRead("brand/banner-claude-code.txt")
	case domain.AgentCursor:
		return assets.MustRead("brand/banner-cursor.txt")
	default:
		return assets.MustRead("brand/banner-squadai.txt")
	}
}

// ProtocolTemplate returns the generic brand banner.
func ProtocolTemplate() string {
	return assets.MustRead("brand/banner-squadai.txt")
}

// TemplateForAgentID returns the agent-specific brand banner.
// Use this to get the expected banner for a specific agent in tests and callers.
func TemplateForAgentID(agentID domain.AgentID) string {
	return templateForAgentID(agentID)
}

// SectionIDForAgentID returns the adapter-scoped brand marker section. OpenCode
// and Pi both target AGENTS.md, so brand sections must not share one marker.
func SectionIDForAgentID(agentID domain.AgentID) string {
	if agentID == "" {
		return SectionID
	}
	return SectionID + ":" + string(agentID)
}

// InjectContent writes the canonical adapter-scoped brand section and removes
// any legacy unscoped section during migration.
func InjectContent(document string, agentID domain.AgentID, content string) string {
	if marker.HasSection(document, SectionID) {
		document = marker.InjectSection(document, SectionID, "")
	}
	return marker.InjectSection(document, SectionIDForAgentID(agentID), content)
}
