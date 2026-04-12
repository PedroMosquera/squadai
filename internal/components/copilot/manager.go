package copilot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
	"github.com/PedroMosquera/agent-manager-pro/internal/templates"
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
func (m *Manager) Plan(projectDir string, cfg domain.CopilotConfig) (domain.PlannedAction, error) {
	targetPath := filepath.Join(projectDir, CopilotInstructionsPath)

	content := TemplateContentWithContext(cfg, projectDir)

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
func (m *Manager) Apply(projectDir string, cfg domain.CopilotConfig) error {
	targetPath := filepath.Join(projectDir, CopilotInstructionsPath)
	content := TemplateContentWithContext(cfg, projectDir)

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
func (m *Manager) Verify(projectDir string, cfg domain.CopilotConfig) []domain.VerifyResult {
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
	expected := TemplateContentWithContext(cfg, projectDir)
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

// TemplateContent returns the copilot instructions content for a given template reference.
// Uses an empty ProjectMeta so the standard template renders a generic fallback.
func TemplateContent(templateRef string) string {
	cfg := domain.CopilotConfig{InstructionsTemplate: templateRef}
	return TemplateContentWithContext(cfg, "")
}

// TemplateContentWithContext resolves a template reference with full context.
// If cfg.CustomContent is non-empty and templateRef is "custom", uses CustomContent.
// If templateRef is "file:<path>", reads from .agent-manager/<path> in projectDir.
// Falls back to built-in standard template for "standard" or empty.
// When the standard template is used, cfg.Meta is used for project-aware rendering.
func TemplateContentWithContext(cfg domain.CopilotConfig, projectDir string) string {
	resolved, err := templates.ResolveTemplate(cfg.InstructionsTemplate, cfg.CustomContent, projectDir)
	if err != nil {
		// On resolution error, fall back to standard template.
		return standardTemplate(cfg.Meta)
	}
	if resolved == "" {
		return standardTemplate(cfg.Meta)
	}
	return resolved
}

// standardTemplate renders project-aware copilot instructions using ProjectMeta.
// When Meta fields are empty, the template omits those sections gracefully.
func standardTemplate(meta domain.ProjectMeta) string {
	const tmpl = `## Team Standards

{{- if .Name}}

### Project: {{.Name}}
{{- end}}
{{- if or .Language .Framework}}

**Stack**:
{{- if .Language}} {{.Language}}{{end}}
{{- if .Framework}} / {{.Framework}}{{end}}
{{- end}}

### Verification Commands
{{- if or .TestCommand .BuildCommand .LintCommand}}
{{- if .TestCommand}}
- Tests: ` + "`{{.TestCommand}}`" + `
{{- end}}
{{- if .BuildCommand}}
- Build: ` + "`{{.BuildCommand}}`" + `
{{- end}}
{{- if .LintCommand}}
- Lint: ` + "`{{.LintCommand}}`" + `
{{- end}}
{{- else}}
- No verification commands configured — add them to project.json meta section.
{{- end}}

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

	t, err := template.New("copilot").Parse(tmpl)
	if err != nil {
		// Should never happen with a hardcoded template; fall back to minimal.
		return "## Team Standards\n\nManaged by agent-manager-pro."
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, meta); err != nil {
		return "## Team Standards\n\nManaged by agent-manager-pro."
	}

	return buf.String()
}
