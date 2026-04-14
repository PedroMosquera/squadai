package agents

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// TemplateData holds all variables for rendering team orchestrator and sub-agent templates.
type TemplateData struct {
	Methodology        string
	DelegationStrategy string
	Language           string
	Languages          []string
	TestCommand        string
	BuildCommand       string
	LintCommand        string
	SkillsDir          string
	AgentsDir          string
	TeamRoles          map[string]domain.TeamRole
	MCPServers         map[string]domain.MCPServerDef
	HasContext7        bool
}

// renderTemplate renders Go text/template content against TemplateData.
// The "missingkey=zero" option causes missing template keys to render as
// their zero value rather than returning an error.
func renderTemplate(name, content string, data TemplateData) (string, error) {
	t, err := template.New(name).Option("missingkey=zero").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// buildTemplateData constructs TemplateData from adapter + merged config.
func buildTemplateData(adapter domain.Adapter, cfg *domain.MergedConfig, homeDir, projectDir string) TemplateData {
	_, hasContext7 := cfg.MCP["context7"]
	return TemplateData{
		Methodology:        string(cfg.Methodology),
		DelegationStrategy: string(adapter.DelegationStrategy()),
		Language:           cfg.Meta.Language,
		Languages:          cfg.Meta.Languages,
		TestCommand:        cfg.Meta.TestCommand,
		BuildCommand:       cfg.Meta.BuildCommand,
		LintCommand:        cfg.Meta.LintCommand,
		SkillsDir:          adapter.ProjectSkillsDir(projectDir),
		AgentsDir:          adapter.ProjectAgentsDir(projectDir),
		TeamRoles:          cfg.Team,
		MCPServers:         cfg.MCP,
		HasContext7:        hasContext7,
	}
}
