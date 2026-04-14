package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/cursor"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/vscode"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/windsurf"
	"github.com/PedroMosquera/agent-manager-pro/internal/assets"
	"github.com/PedroMosquera/agent-manager-pro/internal/backup"
	"github.com/PedroMosquera/agent-manager-pro/internal/config"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/pipeline"
	"github.com/PedroMosquera/agent-manager-pro/internal/planner"
	"github.com/PedroMosquera/agent-manager-pro/internal/verify"
)

// initResult is the JSON representation of a successful init run.
type initResult struct {
	ProjectDir    string          `json:"project_dir"`
	Methodology   string          `json:"methodology"`
	Adapters      []string        `json:"adapters"`
	Components    map[string]bool `json:"components"`
	SkillsWritten []string        `json:"skills_written"`
	MCPServers    []string        `json:"mcp_servers"`
	Plugins       []string        `json:"plugins"`
	PolicyCreated bool            `json:"policy_created"`
}

// RunInit creates .agent-manager/project.json and optionally .agent-manager/policy.json
// in the current working directory. It detects adapters, selects language-specific
// standards, and writes starter skill files.
func RunInit(args []string, stdout io.Writer) error {
	withPolicy := false
	force := false
	jsonOut := false
	var methodology string
	var mcpFlag string
	var pluginsFlag string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--methodology=") {
			methodology = strings.TrimPrefix(arg, "--methodology=")
			continue
		}
		if strings.HasPrefix(arg, "--mcp=") {
			mcpFlag = strings.TrimPrefix(arg, "--mcp=")
			continue
		}
		if strings.HasPrefix(arg, "--plugins=") {
			pluginsFlag = strings.TrimPrefix(arg, "--plugins=")
			continue
		}
		switch arg {
		case "--with-policy":
			withPolicy = true
		case "--force":
			force = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--with-policy] [--force] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Initialize .agent-manager/project.json in the current directory. Detects installed")
			fmt.Fprintln(stdout, "agents (Claude Code, Cursor, VS Code Copilot, Windsurf, OpenCode), identifies the")
			fmt.Fprintln(stdout, "project language, and writes language-specific team standards and starter skill files.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --methodology=<tdd|sdd|conventional>")
			fmt.Fprintln(stdout, "                 Set the development methodology. Generates a team composition")
			fmt.Fprintln(stdout, "                 (TDD: 6 roles, SDD: 8 roles, Conventional: 4 roles) and enables")
			fmt.Fprintln(stdout, "                 the agents and commands components.")
			fmt.Fprintln(stdout, "  --mcp=<csv>    Comma-separated list of MCP server IDs to enable (e.g. context7).")
			fmt.Fprintln(stdout, "                 Omit to include all recommended servers.")
			fmt.Fprintln(stdout, "  --plugins=<csv>")
			fmt.Fprintln(stdout, "                 Comma-separated list of plugin IDs to enable (e.g. code-review).")
			fmt.Fprintln(stdout, "                 Omit to skip plugin installation.")
			fmt.Fprintln(stdout, "  --with-policy  Also create .agent-manager/policy.json with a starter template.")
			fmt.Fprintln(stdout, "  --force        Overwrite existing template and skill files (project.json is")
			fmt.Fprintln(stdout, "                 always overwritten when it already exists with --force).")
			fmt.Fprintln(stdout, "  --json         Output the init result as JSON instead of human-readable text.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager init")
			fmt.Fprintln(stdout, "  agent-manager init --methodology=tdd --with-policy")
			fmt.Fprintln(stdout, "  agent-manager init --methodology=sdd --mcp=context7 --plugins=code-review")
			fmt.Fprintln(stdout, "  agent-manager init --force")
			fmt.Fprintln(stdout, "  agent-manager init --json")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for init", arg)
		}
	}

	// Parse MCP selections from flag.
	var mcpSelections []string
	if mcpFlag != "" {
		for _, s := range strings.Split(mcpFlag, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				mcpSelections = append(mcpSelections, s)
			}
		}
	}

	// Parse plugin selections from flag.
	var pluginSelections []string
	if pluginsFlag != "" {
		for _, s := range strings.Split(pluginsFlag, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				pluginSelections = append(pluginSelections, s)
			}
		}
	}

	// Validate methodology if provided.
	var meth domain.Methodology
	if methodology != "" {
		meth = domain.Methodology(methodology)
		switch meth {
		case domain.MethodologyTDD, domain.MethodologySDD, domain.MethodologyConventional:
			// valid
		default:
			return fmt.Errorf("unknown methodology %q; use tdd, sdd, or conventional", methodology)
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "" // non-fatal, adapter detection will be limited
	}

	// Detect project metadata.
	meta := DetectProjectMeta(projectDir)

	// Detect installed adapters.
	var detectedAdapters []domain.Adapter
	if homeDir != "" {
		detectedAdapters = DetectAdapters(homeDir)
	}

	// When --json is set, suppress all human-readable writes to stdout by
	// redirecting the human output writer to a discard sink.
	humanOut := stdout
	if jsonOut {
		humanOut = io.Discard
	}

	// Create project config.
	projectPath := config.ProjectConfigPath(projectDir)
	_, projectExists := os.Stat(projectPath)
	if projectExists == nil && !force {
		fmt.Fprintf(humanOut, "  exists  %s\n", relPath(projectDir, projectPath))
	} else {
		proj := buildSmartProjectConfig(meta, detectedAdapters, meth, mcpSelections, pluginSelections)
		if err := config.WriteJSON(projectPath, proj); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}
		if projectExists == nil && force {
			fmt.Fprintf(humanOut, "  overwritten %s\n", relPath(projectDir, projectPath))
		} else {
			fmt.Fprintf(humanOut, "  created %s\n", relPath(projectDir, projectPath))
		}
	}

	// Create policy config if requested.
	policyCreated := false
	if withPolicy {
		policyPath := config.PolicyConfigPath(projectDir)
		if _, err := os.Stat(policyPath); err == nil {
			fmt.Fprintf(humanOut, "  exists  %s\n", relPath(projectDir, policyPath))
		} else {
			pol := domain.DefaultPolicyConfig()
			if err := config.WriteJSON(policyPath, pol); err != nil {
				return fmt.Errorf("write policy config: %w", err)
			}
			fmt.Fprintf(humanOut, "  created %s\n", relPath(projectDir, policyPath))
			policyCreated = true
		}
	}

	// Create user config if it doesn't exist.
	if homeDir != "" {
		userPath := config.UserConfigPath(homeDir)
		if _, statErr := os.Stat(userPath); statErr != nil {
			userCfg := domain.DefaultUserConfig()
			if writeErr := config.WriteJSON(userPath, userCfg); writeErr == nil {
				fmt.Fprintf(humanOut, "  created %s\n", userPath)
			}
		}
	}

	// Write language-specific team standards.
	// When multiple languages are detected, compose standards for all of them.
	standardsContent := selectMultiStandards(meta.Languages)
	if len(meta.Languages) == 0 {
		standardsContent = selectStandards(meta.Language)
	}
	standardsPath := filepath.Join(projectDir, config.ProjectConfigDir, "templates", "team-standards.md")
	writeInitFile(humanOut, projectDir, standardsPath, standardsContent, force)

	// Write starter skill files.
	skillFiles := []struct {
		name string
		path string
	}{
		{"skills/shared/code-review/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "code-review.md")},
		{"skills/shared/testing/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "testing.md")},
		{"skills/shared/pr-description/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "pr-description.md")},
		{"skills/shared/find-skills/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "find-skills.md")},
	}
	skillNames := make([]string, 0, len(skillFiles))
	for _, sf := range skillFiles {
		content := assets.MustRead(sf.name)
		writeInitFile(humanOut, projectDir, sf.path, content, force)
		skillNames = append(skillNames, sf.name)
	}

	if jsonOut {
		// Build adapter ID list.
		adapterIDs := make([]string, 0, len(detectedAdapters))
		for _, a := range detectedAdapters {
			adapterIDs = append(adapterIDs, string(a.ID()))
		}

		// Build component/MCP/plugin state from the written project config.
		proj := buildSmartProjectConfig(meta, detectedAdapters, meth, mcpSelections, pluginSelections)

		componentMap := make(map[string]bool, len(proj.Components))
		for k, v := range proj.Components {
			componentMap[k] = v.Enabled
		}

		mcpIDs := make([]string, 0, len(proj.MCP))
		for k := range proj.MCP {
			mcpIDs = append(mcpIDs, k)
		}
		sort.Strings(mcpIDs)

		pluginIDs := make([]string, 0, len(proj.Plugins))
		for k := range proj.Plugins {
			pluginIDs = append(pluginIDs, k)
		}
		sort.Strings(pluginIDs)

		result := initResult{
			ProjectDir:    projectDir,
			Methodology:   string(meth),
			Adapters:      adapterIDs,
			Components:    componentMap,
			SkillsWritten: skillNames,
			MCPServers:    mcpIDs,
			Plugins:       pluginIDs,
			PolicyCreated: policyCreated,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal init result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Print summary.
	fmt.Fprintln(stdout)
	if meta.Name != "" || meta.Language != "" {
		fmt.Fprintln(stdout, "Detected:")
		if meta.Language != "" {
			fmt.Fprintf(stdout, "  Language: %s\n", meta.Language)
		}
		if meta.Name != "" {
			fmt.Fprintf(stdout, "  Project:  %s\n", meta.Name)
		}
		adapterNames := adapterSummary(detectedAdapters)
		if adapterNames != "" {
			fmt.Fprintf(stdout, "  Agents:   %s\n", adapterNames)
		}
		if meth != "" {
			fmt.Fprintf(stdout, "  Methodology: %s\n", meth)
			fmt.Fprintf(stdout, "  Team roles:  %d\n", len(domain.DefaultTeam(meth)))
		}
		mcpServers := DefaultMCPServers()
		if len(mcpServers) > 0 {
			mcpNames := make([]string, 0, len(mcpServers))
			for name := range mcpServers {
				mcpNames = append(mcpNames, name)
			}
			sort.Strings(mcpNames)
			fmt.Fprintf(stdout, "  MCP servers: %s\n", strings.Join(mcpNames, ", "))
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprintln(stdout, "Run 'agent-manager apply' to configure your environment.")
	return nil
}

// buildSmartProjectConfig creates a rich project.json from detected metadata, adapters,
// an optional methodology selection, optional MCP server selections, and optional plugin selections.
// mcpSelections is a list of MCP server IDs to enable (nil/empty = all defaults).
// pluginSelections is a list of plugin IDs to enable (nil/empty = none).
func buildSmartProjectConfig(meta domain.ProjectMeta, adapters []domain.Adapter, methodology domain.Methodology, mcpSelections []string, pluginSelections []string) *domain.ProjectConfig {
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
			string(domain.ComponentSettings):  {Enabled: true},
			string(domain.ComponentSkills):    {Enabled: true},
			string(domain.ComponentWorkflows): {Enabled: true},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
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

	// Enable ALL detected personal-lane adapters.
	for _, a := range adapters {
		proj.Adapters[string(a.ID())] = domain.AdapterConfig{Enabled: true}
	}

	// Apply methodology and generate team composition if specified.
	if methodology != "" {
		proj.Methodology = methodology
		proj.Team = domain.DefaultTeam(methodology)
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

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(stdout, "  error   %s: %v\n", rel, err)
		return
	}

	if existed && force {
		fmt.Fprintf(stdout, "  overwritten %s\n", rel)
	} else {
		fmt.Fprintf(stdout, "  created %s\n", rel)
	}
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

// validatePolicyResult is the JSON representation of a validate-policy run.
type validatePolicyResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
	PolicyPath string   `json:"policy_path"`
}

// RunValidatePolicy validates .agent-manager/policy.json in the current directory.
func RunValidatePolicy(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager validate-policy [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Validate .agent-manager/policy.json in the current directory. Checks that the")
			fmt.Fprintln(stdout, "schema is well-formed, that all locked component IDs are valid, and that required")
			fmt.Fprintln(stdout, "component constraints are internally consistent. Exits non-zero when issues are found.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the validation result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager validate-policy")
			fmt.Fprintln(stdout, "  agent-manager validate-policy --json")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	policyPath := config.PolicyConfigPath(projectDir)

	policy, err := config.LoadPolicy(projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return fmt.Errorf("no policy file found at %s", policyPath)
		}
		return fmt.Errorf("load policy: %w", err)
	}

	issues := config.ValidatePolicy(policy)

	if jsonOut {
		violations := issues
		if violations == nil {
			violations = []string{}
		}
		result := validatePolicyResult{
			Valid:      len(issues) == 0,
			Violations: violations,
			PolicyPath: policyPath,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal validate-policy result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		if len(issues) > 0 {
			return fmt.Errorf("policy validation failed with %d issue(s)", len(issues))
		}
		return nil
	}

	if len(issues) == 0 {
		fmt.Fprintln(stdout, "Policy is valid. No issues found.")
		return nil
	}

	fmt.Fprintf(stdout, "Policy validation found %d issue(s):\n", len(issues))
	for i, issue := range issues {
		fmt.Fprintf(stdout, "  %d. %s\n", i+1, issue)
	}
	return fmt.Errorf("policy validation failed with %d issue(s)", len(issues))
}

// RunPlan computes and displays the action plan.
func RunPlan(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager plan [--dry-run] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Compute the set of actions needed to bring all detected agents into the desired")
			fmt.Fprintln(stdout, "state described by .agent-manager/project.json. Covers all 9 components (memory,")
			fmt.Fprintln(stdout, "rules, settings, MCP, agents, skills, commands, plugins, workflows) across all 5")
			fmt.Fprintln(stdout, "supported agents. No files are written — this is always a read-only preview.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Accepted for consistency with apply; plan is inherently read-only.")
			fmt.Fprintln(stdout, "  --json     Output the planned actions as a JSON array.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager plan")
			fmt.Fprintln(stdout, "  agent-manager plan --json")
			return nil
		}
	}

	_ = dryRun // plan is inherently dry-run; flag accepted for consistency

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(actions, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Report violations first.
	if len(merged.Violations) > 0 {
		fmt.Fprintln(stdout, "Policy overrides:")
		for _, v := range merged.Violations {
			fmt.Fprintf(stdout, "  - %s\n", v)
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprintf(stdout, "Mode: %s\n\n", merged.Mode)

	if len(actions) == 0 {
		fmt.Fprintln(stdout, "No actions needed. Everything is up to date.")
		return nil
	}

	fmt.Fprintf(stdout, "Planned actions (%d):\n", len(actions))
	for _, a := range actions {
		fmt.Fprintf(stdout, "  %-8s %-40s %s\n", a.Action, a.Description, a.TargetPath)
	}

	fmt.Fprintln(stdout, "\nUse 'agent-manager apply' to execute.")
	return nil
}

// RunApply executes the plan with backup safety and step-level reporting.
func RunApply(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	force := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOut = true
		case "--force":
			force = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager apply [--dry-run] [--json] [--force]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Apply the planned configuration changes to your project. Creates or updates agent")
			fmt.Fprintln(stdout, "config files, MCP server settings, skill files, and team definitions for all")
			fmt.Fprintln(stdout, "detected agents (Claude Code, Cursor, VS Code Copilot, Windsurf, OpenCode).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "All managed files are backed up automatically before any changes are written.")
			fmt.Fprintln(stdout, "If any step fails, all completed changes are rolled back using the backup.")
			fmt.Fprintln(stdout, "The backup ID is printed so you can restore manually if needed.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Preview the actions that would be executed without writing any files.")
			fmt.Fprintln(stdout, "  --json     Output the execution report as JSON (includes backup ID and step results).")
			fmt.Fprintln(stdout, "  --force    Apply with default config even when no project.json is found.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager apply")
			fmt.Fprintln(stdout, "  agent-manager apply --dry-run")
			fmt.Fprintln(stdout, "  agent-manager apply --json")
			fmt.Fprintln(stdout, "  agent-manager apply --force")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	// Guard: require project.json to exist unless --force is given.
	projectConfigPath := config.ProjectConfigPath(projectDir)
	if _, statErr := os.Stat(projectConfigPath); os.IsNotExist(statErr) {
		if !force {
			fmt.Fprintln(stdout, "Error: No project.json found in current directory.")
			fmt.Fprintln(stdout, "Run 'agent-manager init' to create one, or use --force to apply with defaults.")
			return fmt.Errorf("no project.json found in current directory")
		}
		fmt.Fprintln(stdout, "Warning: No project.json found. Running with default config (--force).")
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if dryRun {
		if jsonOut {
			data, _ := json.MarshalIndent(actions, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: %d action(s) would be executed.\n", len(actions))
		for _, a := range actions {
			fmt.Fprintf(stdout, "  %-8s %s\n", a.Action, a.Description)
		}
		return nil
	}

	// Create backup store for apply safety.
	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		projectDir,
		merged.Copilot,
		store,
	)

	report, execErr := exec.Execute(actions)
	if execErr != nil {
		if errors.Is(execErr, domain.ErrBackupFailed) {
			return fmt.Errorf("backup failed before apply: %w", execErr)
		}
		if errors.Is(execErr, domain.ErrRollbackFailed) {
			// Critical: rollback itself failed. Report what we can.
			fmt.Fprintln(stdout, "CRITICAL: rollback failed — manual recovery may be needed.")
			if report != nil && report.BackupID != "" {
				fmt.Fprintf(stdout, "  Backup ID: %s\n", report.BackupID)
				fmt.Fprintf(stdout, "  Try: agent-manager restore %s\n", report.BackupID)
			}
			return execErr
		}
		return fmt.Errorf("apply: %w", execErr)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if !report.Success {
			return fmt.Errorf("apply completed with failures (rolled back, backup: %s)", report.BackupID)
		}
		return nil
	}

	if report.BackupID != "" {
		fmt.Fprintf(stdout, "Backup: %s\n\n", report.BackupID)
	}

	for _, s := range report.Steps {
		icon := "ok"
		switch s.Status {
		case domain.StepFailed:
			icon = "FAIL"
		case domain.StepRolledBack:
			icon = "SKIP"
		}
		fmt.Fprintf(stdout, "  [%s] %s\n", icon, s.Action.Description)
		if s.Error != "" {
			fmt.Fprintf(stdout, "        error: %s\n", s.Error)
		}
	}

	// Print summary line.
	printApplySummary(stdout, report.Steps)

	if !report.Success {
		fmt.Fprintf(stdout, "\nApply failed. All changes rolled back (backup: %s).\n", report.BackupID)
		fmt.Fprintf(stdout, "Use 'agent-manager restore %s' to manually restore if needed.\n", report.BackupID)
		return fmt.Errorf("apply completed with failures")
	}

	fmt.Fprintln(stdout, "\nApply complete. Use 'agent-manager verify' to check.")
	return nil
}

// RunSync performs idempotent reconciliation (same as apply — plan then execute).
func RunSync(args []string, stdout io.Writer) error {
	// Intercept help before delegating to apply.
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(stdout, "Usage: agent-manager sync [--dry-run] [--json] [--force]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Idempotent reconciliation: plan and apply in one step. Identical to apply but")
			fmt.Fprintln(stdout, "emphasizes idempotency — running sync multiple times produces the same result.")
			fmt.Fprintln(stdout, "Actions are skipped automatically when the target file already matches the desired")
			fmt.Fprintln(stdout, "state, so it is safe to run on a schedule or in CI.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Preview the actions that would be executed without writing any files.")
			fmt.Fprintln(stdout, "  --json     Output the execution report as JSON.")
			fmt.Fprintln(stdout, "  --force    Sync with default config even when no project.json is found.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager sync")
			fmt.Fprintln(stdout, "  agent-manager sync --dry-run")
			fmt.Fprintln(stdout, "  agent-manager sync --force")
			return nil
		}
	}
	// Sync is semantically identical to apply — it plans and executes.
	// The idempotency comes from the planner returning Skip for up-to-date items.
	return RunApply(args, stdout)
}

// RunVerify runs compliance checks and prints the report.
func RunVerify(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager verify [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Run compliance and health checks against the current project configuration.")
			fmt.Fprintln(stdout, "Verifies that all enabled components are correctly installed for each detected")
			fmt.Fprintln(stdout, "agent: expected files exist, marker blocks are present, and settings are valid.")
			fmt.Fprintln(stdout, "Exits non-zero if any check fails.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Each check is reported as PASS, FAIL, or WARN. Warnings do not cause a non-zero")
			fmt.Fprintln(stdout, "exit. Results are grouped by component when there are more than 5 checks.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the full verification report as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager verify")
			fmt.Fprintln(stdout, "  agent-manager verify --json")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)
	v := verify.New()
	report, err := v.Verify(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if !report.AllPass {
			return fmt.Errorf("verification failed")
		}
		return nil
	}

	if len(report.Results) == 0 {
		fmt.Fprintln(stdout, "No checks to run (no components or adapters enabled).")
		return nil
	}

	// Group results by component if there are enough.
	if len(report.Results) > 5 {
		printGroupedResults(stdout, report.Results)
	} else {
		for _, r := range report.Results {
			printVerifyResult(stdout, r)
		}
	}

	// Print summary line.
	printVerifySummary(stdout, report.Results)

	if !report.AllPass {
		return fmt.Errorf("verification failed")
	}

	return nil
}

// printVerifyResult prints a single verification result line.
func printVerifyResult(stdout io.Writer, r domain.VerifyResult) {
	icon := "PASS"
	if !r.Passed {
		icon = "FAIL"
	}
	if r.Severity == domain.SeverityWarning {
		icon = "WARN"
	}
	line := fmt.Sprintf("  [%s] %s", icon, r.Check)
	if r.Message != "" {
		line += " — " + r.Message
	}
	fmt.Fprintln(stdout, line)
}

// printGroupedResults groups verification results by Component field and prints them.
func printGroupedResults(stdout io.Writer, results []domain.VerifyResult) {
	// Collect groups in order of first appearance.
	type group struct {
		name    string
		results []domain.VerifyResult
	}
	var groups []group
	seen := make(map[string]int)

	for _, r := range results {
		comp := r.Component
		if comp == "" {
			comp = "General"
		}
		if idx, ok := seen[comp]; ok {
			groups[idx].results = append(groups[idx].results, r)
		} else {
			seen[comp] = len(groups)
			groups = append(groups, group{name: comp, results: []domain.VerifyResult{r}})
		}
	}

	for i, g := range groups {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "%s:\n", g.name)
		for _, r := range g.results {
			printVerifyResult(stdout, r)
		}
	}
}

// printApplySummary counts written/skipped/failed steps and prints a one-line summary.
func printApplySummary(stdout io.Writer, steps []domain.StepResult) {
	var written, skipped, failed int
	for _, s := range steps {
		switch {
		case s.Status == domain.StepSuccess:
			if s.Action.Action == domain.ActionSkip {
				skipped++
			} else {
				written++
			}
		case s.Status == domain.StepFailed:
			failed++
		case s.Status == domain.StepRolledBack:
			failed++
		default:
			written++
		}
	}
	fmt.Fprintf(stdout, "\nApplied %d action(s): %d written, %d skipped, %d failed\n", len(steps), written, skipped, failed)
}

// printVerifySummary counts passed/failed/warning results and prints a one-line summary.
func printVerifySummary(stdout io.Writer, results []domain.VerifyResult) {
	var passed, failedCount, warnings int
	for _, r := range results {
		if r.Severity == domain.SeverityWarning {
			warnings++
		} else if r.Passed {
			passed++
		} else {
			failedCount++
		}
	}
	fmt.Fprintf(stdout, "\n%d checks: %d passed, %d failed, %d warnings\n", len(results), passed, failedCount, warnings)
}

// RunBackupCreate creates a manual backup snapshot of all managed files.
func RunBackupCreate(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager backup create [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Create a manual snapshot of all files that agent-manager manages. The backup")
			fmt.Fprintln(stdout, "includes every file that would be written by apply, even those that are already")
			fmt.Fprintln(stdout, "up to date. Backups are stored under ~/.agent-manager/backups by default.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the backup manifest as JSON (includes ID, timestamp, and file list).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager backup create")
			fmt.Fprintln(stdout, "  agent-manager backup create --json")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	// Plan to discover which files would be affected.
	adapters := DetectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Collect all target paths (including skip — we want a full snapshot).
	paths := collectAllTargetPaths(actions)
	if len(paths) == 0 {
		fmt.Fprintln(stdout, "No managed files found to back up.")
		return nil
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifest, err := store.SnapshotFiles(paths, "manual")
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(manifest, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Backup created: %s\n", manifest.ID)
	fmt.Fprintf(stdout, "  Files: %d\n", len(manifest.AffectedFiles))
	fmt.Fprintf(stdout, "  Time:  %s\n", manifest.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	return nil
}

// RunBackupList lists available backups.
func RunBackupList(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager backup list [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "List all available backup snapshots. Shows the backup ID, the command that created")
			fmt.Fprintln(stdout, "the backup (apply or manual), the number of files captured, and the status.")
			fmt.Fprintln(stdout, "Use the ID with 'agent-manager restore <id>' to roll back to a specific snapshot.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the backup list as a JSON array.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager backup list")
			fmt.Fprintln(stdout, "  agent-manager backup list --json")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		// If no project config, use default backup dir.
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.agent-manager/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifests, err := store.List()
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}

	if len(manifests) == 0 {
		fmt.Fprintln(stdout, "No backups found.")
		return nil
	}

	if jsonOut {
		data, _ := json.MarshalIndent(manifests, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Backups (%d):\n\n", len(manifests))
	fmt.Fprintf(stdout, "  %-36s  %-10s  %-5s  %s\n", "ID", "COMMAND", "FILES", "STATUS")
	for _, m := range manifests {
		fmt.Fprintf(stdout, "  %-36s  %-10s  %-5d  %s\n",
			m.ID, m.Command, len(m.AffectedFiles), m.Status)
	}
	return nil
}

// RunRestore restores files from a backup.
func RunRestore(args []string, stdout io.Writer) error {
	jsonOut := false
	dryRun := false
	var backupID string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager restore <backup-id> [--dry-run] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Restore managed files from a backup snapshot. Files that existed before the backup")
			fmt.Fprintln(stdout, "are written back to their original content; files that did not exist before are")
			fmt.Fprintln(stdout, "removed. The backup ID is printed after every apply and can be listed with")
			fmt.Fprintln(stdout, "'agent-manager backup list'.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Show which files would be restored or removed without changing anything.")
			fmt.Fprintln(stdout, "  --json     Output the restore result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  agent-manager restore 2024-01-15T10-30-00Z-abc123")
			fmt.Fprintln(stdout, "  agent-manager restore <id> --dry-run")
			return nil
		default:
			if backupID == "" {
				backupID = arg
			} else {
				return fmt.Errorf("unexpected argument %q", arg)
			}
		}
	}

	if backupID == "" {
		return fmt.Errorf("backup ID is required — usage: agent-manager restore <backup-id>")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.agent-manager/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	manifest, err := store.Get(backupID)
	if err != nil {
		return fmt.Errorf("load backup: %w", err)
	}

	if dryRun {
		if jsonOut {
			data, _ := json.MarshalIndent(manifest, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: would restore %d file(s) from backup %s\n", len(manifest.AffectedFiles), backupID)
		for _, f := range manifest.AffectedFiles {
			if f.ExistedBefore {
				fmt.Fprintf(stdout, "  restore %s\n", f.Path)
			} else {
				fmt.Fprintf(stdout, "  remove  %s\n", f.Path)
			}
		}
		return nil
	}

	if err := store.Restore(backupID); err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"backup_id": backupID,
			"restored":  len(manifest.AffectedFiles),
			"status":    "restored",
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Restored %d file(s) from backup %s.\n", len(manifest.AffectedFiles), backupID)
	for _, f := range manifest.AffectedFiles {
		if f.ExistedBefore {
			fmt.Fprintf(stdout, "  restored %s\n", f.Path)
		} else {
			fmt.Fprintf(stdout, "  removed  %s\n", f.Path)
		}
	}
	return nil
}

// collectAllTargetPaths extracts unique target paths from all actions (including skips).
func collectAllTargetPaths(actions []domain.PlannedAction) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, a := range actions {
		if a.TargetPath != "" && !seen[a.TargetPath] {
			seen[a.TargetPath] = true
			paths = append(paths, a.TargetPath)
		}
	}
	return paths
}

// DetectAdapters returns all registered adapters that are installed or have config.
// OpenCode (team lane) is always included. Personal-lane adapters (Claude Code,
// VS Code Copilot, Cursor, Windsurf) are included only when detected on the system.
func DetectAdapters(homeDir string) []domain.Adapter {
	ctx := context.Background()
	var adapters []domain.Adapter

	// OpenCode is always included — team baseline.
	oc := opencode.New()
	adapters = append(adapters, oc)

	// Personal-lane adapters: include only if binary or config is found.
	cc := claude.New()
	if installed, configFound, err := cc.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, cc)
	}

	vs := vscode.New()
	if installed, configFound, err := vs.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, vs)
	}

	cu := cursor.New()
	if installed, configFound, err := cu.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, cu)
	}

	ws := windsurf.New()
	if installed, configFound, err := ws.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, ws)
	}

	return adapters
}

// LoadAndMerge is the shared config loading logic for commands that need merged config.
func LoadAndMerge(homeDir, projectDir string) (*domain.MergedConfig, error) {
	return loadAndMerge(homeDir, projectDir)
}

// loadAndMerge is the shared config loading logic for commands that need merged config.
func loadAndMerge(homeDir, projectDir string) (*domain.MergedConfig, error) {
	user, err := config.LoadUser(homeDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load user config: %w", err)
	}

	project, err := config.LoadProject(projectDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	policy, err := config.LoadPolicy(projectDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	return config.Merge(user, project, policy), nil
}
