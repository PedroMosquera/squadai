package copilot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

const (
	// SectionID is the marker section for managed copilot instructions.
	SectionID = "copilot-instructions"

	// CopilotInstructionsPath is the relative path within a project.
	CopilotInstructionsPath = ".github/copilot-instructions.md"
)

// Manager handles writing and updating .github/copilot-instructions.md.
// It uses marker blocks to manage its section without clobbering
// user-authored content outside the markers.
type Manager struct{}

// New returns a copilot instructions Manager.
func New() *Manager {
	return &Manager{}
}

// Plan determines what action is needed for copilot instructions.
func (m *Manager) Plan(projectDir, template string) (domain.PlannedAction, error) {
	targetPath := filepath.Join(projectDir, CopilotInstructionsPath)

	content := TemplateContent(template)

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return domain.PlannedAction{}, fmt.Errorf("read copilot instructions: %w", err)
	}

	if marker.HasSection(string(existing), SectionID) {
		current := marker.ExtractSection(string(existing), SectionID)
		if current == content {
			return domain.PlannedAction{
				ID:          "copilot-instructions",
				Agent:       "", // not adapter-specific
				Component:   "",
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: "copilot instructions already up to date",
			}, nil
		}
		return domain.PlannedAction{
			ID:          "copilot-instructions",
			Action:      domain.ActionUpdate,
			TargetPath:  targetPath,
			Description: "update managed copilot instructions section",
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return domain.PlannedAction{
		ID:          "copilot-instructions",
		Action:      action,
		TargetPath:  targetPath,
		Description: "inject managed copilot instructions section",
	}, nil
}

// Apply writes the copilot instructions using marker blocks.
func (m *Manager) Apply(projectDir, template string) error {
	targetPath := filepath.Join(projectDir, CopilotInstructionsPath)
	content := TemplateContent(template)

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return fmt.Errorf("read copilot instructions: %w", err)
	}

	updated := marker.InjectSection(string(existing), SectionID, content)

	_, err = fileutil.WriteAtomic(targetPath, []byte(updated), 0644)
	if err != nil {
		return fmt.Errorf("write copilot instructions: %w", err)
	}

	return nil
}

// Verify checks that copilot instructions are correctly installed.
func (m *Manager) Verify(projectDir, template string) []domain.VerifyResult {
	targetPath := filepath.Join(projectDir, CopilotInstructionsPath)
	var results []domain.VerifyResult

	data, err := os.ReadFile(targetPath)
	if err != nil {
		results = append(results, domain.VerifyResult{
			Check:   "copilot-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("copilot instructions not found: %s", targetPath),
		})
		return results
	}
	results = append(results, domain.VerifyResult{
		Check:  "copilot-file-exists",
		Passed: true,
	})

	doc := string(data)

	if !marker.HasSection(doc, SectionID) {
		results = append(results, domain.VerifyResult{
			Check:   "copilot-markers-present",
			Passed:  false,
			Message: "managed section markers not found",
		})
		return results
	}
	results = append(results, domain.VerifyResult{
		Check:  "copilot-markers-present",
		Passed: true,
	})

	current := marker.ExtractSection(doc, SectionID)
	expected := TemplateContent(template)
	if current != expected {
		results = append(results, domain.VerifyResult{
			Check:   "copilot-content-current",
			Passed:  false,
			Message: "managed section content is outdated",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:  "copilot-content-current",
			Passed: true,
		})
	}

	return results
}

// TemplateContent returns the copilot instructions content for a given template name.
// Currently only "standard" is supported.
func TemplateContent(template string) string {
	switch template {
	case "standard":
		return standardTemplate()
	default:
		return standardTemplate()
	}
}

func standardTemplate() string {
	return `## Team Standards

This project uses agent-manager-pro for consistent AI agent configuration.

### Code Style
- Follow existing patterns in the codebase
- Write tests for new functionality
- Use clear, descriptive naming

### Commit Messages
- Use conventional commit format (feat:, fix:, docs:, etc.)
- Keep the first line under 72 characters

### Architecture
- Respect package boundaries and import rules
- Keep domain logic free of infrastructure concerns
- All mutating operations must support dry-run mode`
}
