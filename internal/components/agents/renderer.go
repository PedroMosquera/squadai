package agents

import (
	"bytes"
	"fmt"
	"strings"
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
	Framework          string
	PackageManager     string
	ModelTier          string
	ModelHint          string
	// RoleModel is the concrete model name resolved for the current role.
	// Populated per-role during planning/apply and injected into frontmatter.
	RoleModel string
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
	tier := string(cfg.ModelTier)
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
		Framework:          cfg.Meta.Framework,
		PackageManager:     cfg.Meta.PackageManager,
		ModelTier:          tier,
		ModelHint:          promptHintForTier(tier),
	}
}

// promptHintForTier returns a human-readable model recommendation for the given tier.
func promptHintForTier(tier string) string {
	switch tier {
	case "performance":
		return "Use the most capable models available (Claude Sonnet 4.5, GPT-4.1). Prioritize quality over cost."
	case "starter":
		return "Use cost-effective models (Claude Haiku 3.5, GPT-4.1-mini). Prioritize budget over capability."
	case "balanced":
		return "Use Claude Sonnet 4 for complex tasks and architecture decisions. Use GPT-4.1-mini for quick edits and simple fixes."
	default:
		return "" // manual or unknown — no hint
	}
}

// injectModelIntoFrontmatter inserts or replaces the "model:" line in the YAML
// frontmatter of a rendered agent file.  The frontmatter is delimited by leading
// and trailing "---" lines.  If modelName is empty the content is returned unchanged.
func injectModelIntoFrontmatter(content, modelName string) string {
	if modelName == "" {
		return content
	}
	// Locate the opening "---".
	if !strings.HasPrefix(content, "---") {
		return content
	}
	// Find the closing "---".
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}

	frontmatter := rest[:idx]
	body := rest[idx+1:] // starts with "---\n..."

	// Remove any existing "model:" line.
	lines := strings.Split(frontmatter, "\n")
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "model:") {
			filtered = append(filtered, l)
		}
	}
	newFrontmatter := strings.Join(filtered, "\n")
	newFrontmatter = strings.TrimRight(newFrontmatter, "\n")
	newFrontmatter += "\nmodel: " + modelName

	return "---" + newFrontmatter + "\n" + body
}
