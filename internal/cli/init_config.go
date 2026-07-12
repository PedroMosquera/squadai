package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// methodologyDescription returns a one-line description for a methodology.
func methodologyDescription(m domain.Methodology) string {
	switch m {
	case domain.MethodologyTDD:
		return "Test-Driven Development — red/green/refactor cycle with 6 specialized roles"
	case domain.MethodologySDD:
		return "Spec-Driven Development — spec-first approach with 8 roles including reviewers"
	case domain.MethodologyConventional:
		return "Conventional workflow — 4 balanced roles for general software development"
	default:
		return string(m)
	}
}

// buildSmartProjectConfig creates a rich project.json from detected metadata, adapters,
// an optional methodology selection, optional MCP server selections, optional plugin selections,
// and an optional model tier selection.
// mcpSelections is a list of MCP server IDs to enable (nil/empty = all defaults).
// pluginSelections is a list of plugin IDs to enable (nil/empty = none).
func buildSmartProjectConfig(meta domain.ProjectMeta, adapters []domain.Adapter, methodology domain.Methodology, mcpSelections []string, pluginSelections []string, modelTier domain.ModelTier) *domain.ProjectConfig {
	proj := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			"copilot":                      {Enabled: true},
			string(domain.ComponentRules): {
				Enabled: true,
				Settings: map[string]interface{}{
					"team_standards_file": "templates/team-standards.md",
				},
			},
			string(domain.ComponentSkills):      {Enabled: true},
			string(domain.ComponentWorkflows):   {Enabled: true},
			string(domain.ComponentPermissions): {Enabled: true},
			string(domain.ComponentBrand):       {Enabled: true},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
		Memory:  domain.DefaultMemoryConfig(),
		Context: domain.DefaultContextConfig(),
		Usage:   domain.DefaultUsageConfig(),
		Models:  domain.DefaultModelsConfig(),
		Skills: map[string]domain.SkillDef{
			"code-review": {
				Description: "Structured code review",
				ContentFile: "skills/code-review.md",
			},
			"testing": {
				Description: "Test writing protocol",
				ContentFile: "skills/testing.md",
			},
			"pr-description": {
				Description: "PR description generation",
				ContentFile: "skills/pr-description.md",
			},
			"find-skills": {
				Description: "Find and load available skills",
				ContentFile: "skills/find-skills.md",
			},
		},
	}

	// Set model tier when explicitly provided (non-empty).
	if modelTier != "" {
		proj.ModelTier = modelTier
	}

	// Enable ALL detected personal-lane adapters.
	for _, a := range adapters {
		proj.Adapters[string(a.ID())] = domain.AdapterConfig{Enabled: true}
	}

	// Enable settings component only when at least one adapter has non-empty Settings.
	hasSettings := false
	for _, ac := range proj.Adapters {
		if len(ac.Settings) > 0 {
			hasSettings = true
			break
		}
	}
	if hasSettings {
		proj.Components[string(domain.ComponentSettings)] = domain.ComponentConfig{Enabled: true}
	}

	// Apply methodology and generate team composition if specified.
	if methodology != "" {
		proj.Methodology = methodology
		proj.Team = domain.DefaultTeam(methodology)
		proj.Commands = defaultCommandsForMethodology(methodology)
		// Enable agent and command components when methodology is active.
		proj.Components[string(domain.ComponentAgents)] = domain.ComponentConfig{Enabled: true}
		proj.Components[string(domain.ComponentCommands)] = domain.ComponentConfig{Enabled: true}
	}

	// Always include recommended MCP servers; filter to selections if provided.
	allMCP := DefaultMCPServers()
	if len(mcpSelections) > 0 {
		filtered := make(map[string]domain.MCPServerDef, len(mcpSelections))
		selSet := make(map[string]bool, len(mcpSelections))
		for _, s := range mcpSelections {
			selSet[s] = true
		}
		for name, def := range allMCP {
			if selSet[name] {
				filtered[name] = def
			}
		}
		proj.MCP = filtered
	} else {
		proj.MCP = allMCP
	}
	// Enable MCP component when servers are configured.
	if len(proj.MCP) > 0 {
		proj.Components[string(domain.ComponentMCP)] = domain.ComponentConfig{Enabled: true}
	}

	// Apply plugin selections if provided.
	if len(pluginSelections) > 0 {
		allPlugins := AvailablePlugins()
		selSet := make(map[string]bool, len(pluginSelections))
		for _, s := range pluginSelections {
			selSet[s] = true
		}
		selected := make(map[string]domain.PluginDef)
		for name, def := range allPlugins {
			if selSet[name] {
				d := def
				d.Enabled = true
				selected[name] = d
			}
		}
		if len(selected) > 0 {
			proj.Plugins = selected
			proj.Components[string(domain.ComponentPlugins)] = domain.ComponentConfig{Enabled: true}
		}
	}

	return proj
}

// mergeProjectConfigs merges a freshly-detected config on top of an existing one,
// preserving user customizations while adding newly-detected items.
//
// Merge rules:
//   - Version, Meta: always taken from fresh (latest version, re-detected metadata)
//   - Adapters, Components, Skills, MCP, Plugins: map-merge — new keys from fresh are
//     added, but existing keys are never overwritten (user customizations preserved)
//   - Methodology, Team, Commands: if methodologyExplicit is true, overwrite from fresh;
//     otherwise preserve existing values
//   - ModelTier: if modelTierExplicit is true, overwrite from fresh; otherwise preserve
//   - Preset and roadmap blocks: preserve existing values unless fresh explicitly sets them
//   - Copilot, Rules: always preserved from existing (user-managed)
func mergeProjectConfigs(existing, fresh *domain.ProjectConfig, methodologyExplicit bool, modelTierExplicit bool) *domain.ProjectConfig {
	result := *existing

	// Always overwrite version and meta from fresh.
	result.Version = fresh.Version
	result.Meta = fresh.Meta
	if fresh.Preset != "" {
		result.Preset = fresh.Preset
	}

	// Map-merge Adapters: add new keys from fresh, never overwrite existing.
	result.Adapters = make(map[string]domain.AdapterConfig, len(existing.Adapters))
	for k, v := range existing.Adapters {
		result.Adapters[k] = v
	}
	for k, v := range fresh.Adapters {
		if _, exists := result.Adapters[k]; !exists {
			result.Adapters[k] = v
		}
	}

	// Map-merge Components: add new keys from fresh, never overwrite existing.
	result.Components = make(map[string]domain.ComponentConfig, len(existing.Components))
	for k, v := range existing.Components {
		result.Components[k] = v
	}
	for k, v := range fresh.Components {
		if _, exists := result.Components[k]; !exists {
			result.Components[k] = v
		}
	}

	// Map-merge Skills: default skills added if not present, user-added preserved.
	result.Skills = make(map[string]domain.SkillDef, len(existing.Skills))
	for k, v := range existing.Skills {
		result.Skills[k] = v
	}
	for k, v := range fresh.Skills {
		if _, exists := result.Skills[k]; !exists {
			result.Skills[k] = v
		}
	}

	// Map-merge MCP: default servers added if not present, user servers preserved.
	result.MCP = make(map[string]domain.MCPServerDef, len(existing.MCP))
	for k, v := range existing.MCP {
		result.MCP[k] = v
	}
	for k, v := range fresh.MCP {
		if _, exists := result.MCP[k]; !exists {
			result.MCP[k] = v
		}
	}

	// Map-merge Plugins: new plugins added, existing preserved.
	result.Plugins = make(map[string]domain.PluginDef, len(existing.Plugins))
	for k, v := range existing.Plugins {
		result.Plugins[k] = v
	}
	for k, v := range fresh.Plugins {
		if _, exists := result.Plugins[k]; !exists {
			result.Plugins[k] = v
		}
	}

	// Methodology-aware: if explicit flag given, overwrite; otherwise preserve.
	if methodologyExplicit {
		result.Methodology = fresh.Methodology
		result.Team = fresh.Team
		result.Commands = fresh.Commands
	}
	// If not explicit, result.Methodology/Team/Commands are already the existing
	// values from the struct copy above.

	// ModelTier-aware: if explicit flag given, overwrite; otherwise preserve.
	if modelTierExplicit {
		result.ModelTier = fresh.ModelTier
	}

	if isZeroMemoryConfig(result.Memory) && !isZeroMemoryConfig(fresh.Memory) {
		result.Memory = fresh.Memory
	}
	if isZeroContextConfig(result.Context) && !isZeroContextConfig(fresh.Context) {
		result.Context = fresh.Context
	}
	if isZeroUsageConfig(result.Usage) && !isZeroUsageConfig(fresh.Usage) {
		result.Usage = fresh.Usage
	}
	if isZeroModelsConfig(result.Models) && !isZeroModelsConfig(fresh.Models) {
		result.Models = fresh.Models
	}

	// Copilot and Rules are always preserved from existing (user-managed).
	result.Copilot = existing.Copilot
	result.Rules = existing.Rules

	// Agents are preserved from existing (user-managed definitions).
	// Map-merge to add fresh agent defs if not already present.
	result.Agents = make(map[string]domain.AgentDef, len(existing.Agents))
	for k, v := range existing.Agents {
		result.Agents[k] = v
	}
	for k, v := range fresh.Agents {
		if _, exists := result.Agents[k]; !exists {
			result.Agents[k] = v
		}
	}

	return &result
}

func isZeroMemoryConfig(c domain.MemoryConfig) bool {
	return c.Backend == "" && !c.AutoCapture && c.ProjectKeyStrategy == "" && c.ExportPath == ""
}

func isZeroContextConfig(c domain.ContextConfig) bool {
	return c.DefaultProfile == "" && len(c.Profiles) == 0
}

func isZeroUsageConfig(c domain.UsageConfig) bool {
	return c.DailyTokenBudget == 0 && c.SessionTokenBudget == 0 &&
		c.DailyCostBudget == 0 && c.SessionCostBudget == 0 &&
		c.Enforcement == "" && c.Currency == "" && c.PriceCatalogSource == "" &&
		len(c.ProfileTiers) == 0
}

func isZeroModelsConfig(c domain.ModelsConfig) bool {
	return len(c.Profiles) == 0 && len(c.Overrides) == 0
}

// defaultCommandsForMethodology returns a set of command definitions appropriate
// for the given methodology. All methodologies include a base "review" command;
// TDD adds "run-tests" and "tdd-cycle"; SDD adds "spec"; Conventional adds "implement".
// Empty or unknown methodology returns only the base review command.
func defaultCommandsForMethodology(m domain.Methodology) map[string]domain.CommandDef {
	base := map[string]domain.CommandDef{
		"review": {
			Description: "Run structured code review on staged changes",
			Template:    "Review the staged changes using the code-review skill.",
		},
	}
	switch m {
	case domain.MethodologyTDD:
		base["run-tests"] = domain.CommandDef{
			Description: "Run full test suite and report failures",
			Template:    "Run the test suite. Report each failure with file, line, and a fix suggestion.",
		}
		base["tdd-cycle"] = domain.CommandDef{
			Description: "Execute one red-green-refactor cycle",
			Template:    "Using the test-driven-development skill, execute one complete red-green-refactor cycle.",
		}
	case domain.MethodologySDD:
		base["spec"] = domain.CommandDef{
			Description: "Generate a formal spec document for a feature",
			Template:    "Using the sdd-spec skill, write a formal specification for the requested feature.",
		}
	case domain.MethodologyConventional:
		base["implement"] = domain.CommandDef{
			Description: "Implement a feature with inline review",
			Template:    "Implement the requested feature, then self-review before presenting the result.",
		}
	}
	return base
}

// selectStandards returns the content of the language-specific standards asset.
func selectStandards(language string) string {
	switch language {
	case "Go":
		return assets.MustRead("standards/go.md")
	case "TypeScript", "TypeScript/JavaScript":
		return assets.MustRead("standards/javascript.md")
	case "Python":
		return assets.MustRead("standards/python.md")
	case "Rust":
		return assets.MustRead("standards/rust.md")
	case "Java", "Kotlin":
		return assets.MustRead("standards/java.md")
	case "Ruby":
		return assets.MustRead("standards/ruby.md")
	case "C#":
		return assets.MustRead("standards/csharp.md")
	case "PHP":
		return assets.MustRead("standards/php.md")
	case "Swift":
		return assets.MustRead("standards/swift.md")
	case "C/C++":
		return assets.MustRead("standards/cpp.md")
	case "Dart":
		return assets.MustRead("standards/dart.md")
	case "Elixir":
		return assets.MustRead("standards/elixir.md")
	case "Scala":
		return assets.MustRead("standards/scala.md")
	default:
		return assets.MustRead("standards/generic.md")
	}
}

// selectMultiStandards composes standards for all detected languages.
// When multiple languages are detected each section is prefixed with a
// "## {Language} Standards" heading and the sections are joined with a
// horizontal rule separator. When only one language is detected the output
// is identical to selectStandards(). When languages is empty the generic
// standards are returned.
func selectMultiStandards(languages []string) string {
	if len(languages) == 0 {
		return assets.MustRead("standards/generic.md")
	}
	if len(languages) == 1 {
		return selectStandards(languages[0])
	}
	var parts []string
	for _, lang := range languages {
		content := selectStandards(lang)
		parts = append(parts, "## "+lang+" Standards\n\n"+content)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// writeInitFile writes content to path, respecting the force flag.
// Reports status to stdout.
func writeInitFile(stdout io.Writer, projectDir, path, content string, force bool) {
	rel := relPath(projectDir, path)
	_, existsErr := os.Stat(path)
	existed := existsErr == nil

	if existed && !force {
		fmt.Fprintf(stdout, "  exists  %s\n", rel)
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(stdout, "  error   %s: %v\n", rel, err)
		return
	}

	if _, err := fileutil.WriteAtomic(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(stdout, "  error   %s: %v\n", rel, err)
		return
	}

	if existed && force {
		fmt.Fprintf(stdout, "  overwritten %s\n", rel)
	} else {
		fmt.Fprintf(stdout, "  created %s\n", rel)
	}
}

// writeMemoryScaffold creates the docs/memory/ directory structure from embedded scaffold files.
// Files that already exist are skipped (no overwrites). Prints a single summary line on creation.
func writeMemoryScaffold(stdout io.Writer, projectDir string) error {
	type scaffoldEntry struct {
		asset string // embedded asset path (relative to assets FS root)
		dest  string // destination path relative to projectDir
	}
	entries := []scaffoldEntry{
		{"projectmemory/scaffold/memory-README.md", filepath.Join("docs", "memory", "README.md")},
		{"projectmemory/scaffold/inbox-README.md", filepath.Join("docs", "memory", "_inbox", "README.md")},
		{"projectmemory/scaffold/decisions-README.md", filepath.Join("docs", "memory", "decisions", "README.md")},
		{"projectmemory/scaffold/learnings-README.md", filepath.Join("docs", "memory", "learnings", "README.md")},
		{"projectmemory/scaffold/incidents-README.md", filepath.Join("docs", "memory", "incidents", "README.md")},
	}

	anyCreated := false
	for _, e := range entries {
		destPath := filepath.Join(projectDir, e.dest)
		if _, statErr := os.Stat(destPath); statErr == nil {
			// File already exists — preserve it.
			continue
		}
		content, readErr := assets.Read(e.asset)
		if readErr != nil {
			return fmt.Errorf("read scaffold asset %s: %w", e.asset, readErr)
		}
		if mkErr := os.MkdirAll(filepath.Dir(destPath), 0755); mkErr != nil {
			return fmt.Errorf("create scaffold dir for %s: %w", e.dest, mkErr)
		}
		if _, writeErr := fileutil.WriteAtomic(destPath, []byte(content), 0644); writeErr != nil {
			return fmt.Errorf("write scaffold file %s: %w", e.dest, writeErr)
		}
		anyCreated = true
	}

	if anyCreated {
		fmt.Fprintf(stdout, "  Memory scaffold created at docs/memory/\n")
	}
	return nil
}

// adapterSummary returns a comma-separated list of adapter names.
func adapterSummary(adapters []domain.Adapter) string {
	if len(adapters) == 0 {
		return ""
	}
	names := ""
	for i, a := range adapters {
		if i > 0 {
			names += ", "
		}
		names += string(a.ID())
	}
	return names
}

// relPath returns a relative path from base, falling back to abs on error.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
