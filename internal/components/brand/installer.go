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

// Plan determines what actions are needed for this adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentBrand) {
		return nil, nil
	}

	promptPath := brandTargetPath(adapter, homeDir, projectDir)

	existing, err := fileutil.ReadFileOrEmpty(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read target file: %w", err)
	}

	desiredContent := fence(templateForAdapter(adapter))

	// Check if the section already matches.
	if marker.HasSection(string(existing), SectionID) {
		current := marker.ExtractSection(string(existing), SectionID)
		if current == desiredContent {
			return []domain.PlannedAction{
				{
					ID:          fmt.Sprintf("%s-%s-brand", adapter.ID(), "banner"),
					Agent:       adapter.ID(),
					Component:   domain.ComponentBrand,
					Action:      domain.ActionSkip,
					TargetPath:  promptPath,
					Description: "brand banner already up to date",
				},
			}, nil
		}
		return []domain.PlannedAction{
			{
				ID:          fmt.Sprintf("%s-%s-brand", adapter.ID(), "banner"),
				Agent:       adapter.ID(),
				Component:   domain.ComponentBrand,
				Action:      domain.ActionUpdate,
				TargetPath:  promptPath,
				Description: "update brand banner in target file",
			},
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return []domain.PlannedAction{
		{
			ID:          fmt.Sprintf("%s-%s-brand", adapter.ID(), "banner"),
			Agent:       adapter.ID(),
			Component:   domain.ComponentBrand,
			Action:      action,
			TargetPath:  promptPath,
			Description: "inject brand banner into target file",
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

	banner := templateForAgentID(action.Agent)
	// The banner asset does not include a markdown fence; wrap it so the
	// ASCII art renders verbatim inside agent rules/prompt files.
	fenced := "```text\n" + banner + "\n```\n"
	updated := marker.InjectSection(string(existing), SectionID, fenced)

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

	promptPath := brandTargetPath(adapter, homeDir, projectDir)
	var results []domain.VerifyResult

	// Check file exists.
	_, err := os.Stat(promptPath)
	if err != nil {
		results = append(results, domain.VerifyResult{
			Check:   "brand-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("target file not found: %s", promptPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "brand-file-exists",
		Passed: true,
	})

	// Check marker section exists.
	content, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}

	if !marker.HasSection(string(content), SectionID) {
		results = append(results, domain.VerifyResult{
			Check:   "brand-markers-present",
			Passed:  false,
			Message: "brand marker section not found in target file",
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "brand-markers-present",
		Passed: true,
	})

	// Check content matches expected.
	current := marker.ExtractSection(string(content), SectionID)
	expected := fence(templateForAdapter(adapter))
	if current != expected {
		results = append(results, domain.VerifyResult{
			Check:   "brand-content-current",
			Passed:  false,
			Message: "brand banner content is outdated",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:  "brand-content-current",
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
