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
	"time"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Version is the CLI version string, set by app before calling any Run* function.
var Version = "dev"

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

// RunInit creates .squadai/project.json and optionally .squadai/policy.json
// in the current working directory. It detects adapters, selects language-specific
// standards, and writes starter skill files.
func RunInit(args []string, stdout io.Writer) error {
	withPolicy := false
	withMemory := true // memory scaffold is opt-out
	force := false
	merge := false
	jsonOut := false
	global := false
	permissionsEnabled := true
	var methodology string
	methodologyExplicit := false
	var mcpFlag string
	var pluginsFlag string
	modelTier := domain.ModelTierBalanced
	modelTierExplicit := false
	var agentSelections []string
	var presetValue string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--methodology=") {
			methodology = strings.TrimPrefix(arg, "--methodology=")
			methodologyExplicit = true
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
		if strings.HasPrefix(arg, "--model-tier=") {
			val := strings.TrimPrefix(arg, "--model-tier=")
			switch domain.ModelTier(val) {
			case domain.ModelTierBalanced, domain.ModelTierPerformance,
				domain.ModelTierStarter, domain.ModelTierManual:
				modelTier = domain.ModelTier(val)
				modelTierExplicit = true
			default:
				return exitcode.ErrUnknownValue("--model-tier", val, "balanced, performance, starter, manual")
			}
			continue
		}
		if strings.HasPrefix(arg, "--agents=") {
			val := strings.TrimPrefix(arg, "--agents=")
			if val != "" {
				agentSelections = strings.Split(val, ",")
			}
			continue
		}
		if strings.HasPrefix(arg, "--preset=") {
			val := strings.TrimPrefix(arg, "--preset=")
			switch domain.SetupPreset(val) {
			case domain.PresetSoloMinimal, domain.PresetSoloPower, domain.PresetTeamStandard,
				domain.PresetEnterpriseLock, domain.PresetFullSquad, domain.PresetLean,
				domain.PresetCustom:
				presetValue = val
			default:
				return exitcode.ErrUnknownValue("--preset", val, "solo-minimal, solo-power, team-standard, enterprise-locked, full-squad, lean, custom")
			}
			continue
		}
		switch arg {
		case "--with-policy":
			withPolicy = true
		case "--with-memory":
			withMemory = true
		case "--without-memory":
			withMemory = false
		case "--force":
			force = true
		case "--merge":
			merge = true
		case "--json":
			jsonOut = true
		case "--global":
			global = true
		case "--set-claude-default-agent":
			// Accepted for TUI compatibility; handled by 'apply' step.
		case "--no-permissions":
			permissionsEnabled = false
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--model-tier=<balanced|performance|starter|manual>] [--agents=<csv>] [--preset=<solo-minimal|solo-power|team-standard|enterprise-locked|full-squad|lean|custom>] [--with-policy] [--without-memory] [--force] [--merge] [--json] [--global]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Initialize .squadai/project.json in the current directory. Detects installed")
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
			fmt.Fprintln(stdout, "  --model-tier=<balanced|performance|starter|manual>")
			fmt.Fprintln(stdout, "                 Set the model tier for agent configuration.")
			fmt.Fprintln(stdout, "                 balanced: best cost/quality ratio (default)")
			fmt.Fprintln(stdout, "                 performance: always flagship models — maximum quality, higher cost")
			fmt.Fprintln(stdout, "                 starter: capable models at lowest cost")
			fmt.Fprintln(stdout, "                 manual: configure models yourself — no defaults applied")
			fmt.Fprintln(stdout, "  --agents=<csv> Comma-separated list of agent IDs to configure (e.g. opencode,cursor).")
			fmt.Fprintln(stdout, "                 OpenCode is always included. Omit to configure all detected agents.")
			fmt.Fprintln(stdout, "  --preset=<solo-minimal|solo-power|team-standard|enterprise-locked|full-squad|lean|custom>")
			fmt.Fprintln(stdout, "                 Apply a named setup preset:")
			fmt.Fprintln(stdout, "                 solo-minimal: conventional workflow, starter models, low context budget")
			fmt.Fprintln(stdout, "                 solo-power: TDD workflow, balanced models, daily-driver defaults")
			fmt.Fprintln(stdout, "                 team-standard: TDD workflow, balanced models, shared governance")
			fmt.Fprintln(stdout, "                 enterprise-locked: SDD workflow, performance models, strict profile")
			fmt.Fprintln(stdout, "                 full-squad: SDD methodology, balanced models, all components")
			fmt.Fprintln(stdout, "                 lean: conventional methodology, starter models, core only")
			fmt.Fprintln(stdout, "                 custom: explicit flags or wizard defaults")
			fmt.Fprintln(stdout, "  --global       Apply configuration globally (home directory) instead of the current project.")
			fmt.Fprintln(stdout, "  --with-policy  Also create .squadai/policy.json with a starter template.")
			fmt.Fprintln(stdout, "  --without-memory  Skip creating the docs/memory/ scaffold (memory is created by default).")
			fmt.Fprintln(stdout, "  --force        Overwrite existing template and skill files (project.json is")
			fmt.Fprintln(stdout, "                 always overwritten when it already exists with --force).")
			fmt.Fprintln(stdout, "  --merge        Re-run init, merging new config on top of existing (preserves user customizations).")
			fmt.Fprintln(stdout, "  --json         Output the init result as JSON instead of human-readable text.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai init")
			fmt.Fprintln(stdout, "  squadai init --methodology=tdd --with-policy")
			fmt.Fprintln(stdout, "  squadai init --methodology=sdd --mcp=context7 --plugins=code-review")
			fmt.Fprintln(stdout, "  squadai init --model-tier=performance")
			fmt.Fprintln(stdout, "  squadai init --agents=opencode,cursor")
			fmt.Fprintln(stdout, "  squadai init --preset=solo-power")
			fmt.Fprintln(stdout, "  squadai init --preset=full-squad")
			fmt.Fprintln(stdout, "  squadai init --without-memory")
			fmt.Fprintln(stdout, "  squadai init --force")
			fmt.Fprintln(stdout, "  squadai init --merge")
			fmt.Fprintln(stdout, "  squadai init --json")
			return nil
		}
	}

	// Apply preset AFTER flag parsing but BEFORE building config.
	// Preset sets methodology/modelTier only when not already explicitly set.
	switch domain.SetupPreset(presetValue) {
	case domain.PresetSoloMinimal:
		if !methodologyExplicit {
			methodology = string(domain.MethodologyConventional)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierStarter
		}
	case domain.PresetSoloPower:
		if !methodologyExplicit {
			methodology = string(domain.MethodologyTDD)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierBalanced
		}
	case domain.PresetTeamStandard:
		if !methodologyExplicit {
			methodology = string(domain.MethodologyTDD)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierBalanced
		}
	case domain.PresetEnterpriseLock:
		if !methodologyExplicit {
			methodology = string(domain.MethodologySDD)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierPerformance
		}
	case domain.PresetFullSquad:
		if !methodologyExplicit {
			methodology = string(domain.MethodologySDD)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierBalanced
		}
	case domain.PresetLean:
		if !methodologyExplicit {
			methodology = string(domain.MethodologyConventional)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierStarter
		}
	}

	if merge && force {
		return exitcode.ErrFlagConflict("--merge and --force")
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
			return exitcode.ErrUnknownValue("--methodology", methodology, "tdd, sdd, conventional")
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

	// When --global is set, target the home directory instead of the project directory.
	if global {
		if homeDir == "" {
			return fmt.Errorf("--global: could not determine home directory")
		}
		projectDir = homeDir
	}

	// Detect project metadata.
	meta := DetectProjectMeta(projectDir)

	// Detect installed adapters.
	var detectedAdapters []domain.Adapter
	if homeDir != "" {
		detectedAdapters = DetectAdapters(homeDir)
	}

	// Filter adapters to user-selected subset (--agents= flag).
	// OpenCode is always preserved regardless of selection.
	selectedAdapters := filterAdapters(detectedAdapters, agentSelections)

	// When --json is set, suppress all human-readable writes to stdout by
	// redirecting the human output writer to a discard sink.
	humanOut := stdout
	if jsonOut {
		humanOut = io.Discard
	}

	// Create project config.
	projectPath := config.ProjectConfigPath(projectDir)
	_, projectExists := os.Stat(projectPath)

	// proj holds the final config used for both human-readable and JSON output.
	var proj *domain.ProjectConfig
	if projectExists == nil && !force && !merge {
		// Existing config, no overwrite requested — skip.
		fmt.Fprintf(humanOut, "  exists  %s\n", relPath(projectDir, projectPath))
	} else if projectExists == nil && merge {
		// Merge mode: load existing, build fresh detection, merge together.
		existing, loadErr := config.LoadProject(projectDir)
		if loadErr != nil {
			return fmt.Errorf("load existing project config for merge: %w", loadErr)
		}
		fresh := buildSmartProjectConfig(meta, selectedAdapters, meth, mcpSelections, pluginSelections, modelTier)
		if presetValue != "" {
			fresh.Preset = domain.SetupPreset(presetValue)
		}
		if !permissionsEnabled {
			fresh.Components[string(domain.ComponentPermissions)] = domain.ComponentConfig{Enabled: false}
		}
		proj = mergeProjectConfigs(existing, fresh, methodologyExplicit, modelTierExplicit)
		if err := config.WriteJSON(projectPath, proj); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}
		fmt.Fprintf(humanOut, "  merged  %s\n", relPath(projectDir, projectPath))
	} else {
		// New or force: build fresh config.
		proj = buildSmartProjectConfig(meta, selectedAdapters, meth, mcpSelections, pluginSelections, modelTier)
		if presetValue != "" {
			proj.Preset = domain.SetupPreset(presetValue)
		}
		if !permissionsEnabled {
			proj.Components[string(domain.ComponentPermissions)] = domain.ComponentConfig{Enabled: false}
		}
		if err := config.WriteJSON(projectPath, proj); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}
		if projectExists == nil && force {
			fmt.Fprintf(humanOut, "  overwritten %s\n", relPath(projectDir, projectPath))
		} else {
			fmt.Fprintf(humanOut, "  created %s\n", relPath(projectDir, projectPath))
		}
	}

	// When proj was not written (exists + no-op skip), build it for JSON output.
	if proj == nil {
		proj = buildSmartProjectConfig(meta, selectedAdapters, meth, mcpSelections, pluginSelections, modelTier)
		if presetValue != "" {
			proj.Preset = domain.SetupPreset(presetValue)
		}
		if !permissionsEnabled {
			proj.Components[string(domain.ComponentPermissions)] = domain.ComponentConfig{Enabled: false}
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

	// Write .gitignore suggestion file.
	agentManagerDir := filepath.Join(projectDir, config.ProjectConfigDir)
	gitignoreSuggestion := `# Files to add to your .gitignore for SquadAI
# ================================================

# Always ignore backups (contain file snapshots, can be large)
.squadai/backups/

# Ignore user-specific config (each developer has their own)
.squadai/user.json

# ------------------------------------------------
# Files to COMMIT (team-shared configuration)
# ------------------------------------------------
# .squadai/project.json    — project-level agent config
# .squadai/policy.json     — team policy enforcement
# AGENTS.md                      — agent system prompt
# CLAUDE.md                      — Claude Code system prompt
# .cursorrules                   — Cursor rules
# .instructions.md               — VS Code Copilot instructions
`
	writeInitFile(humanOut, projectDir, filepath.Join(agentManagerDir, ".gitignore-suggestion"), gitignoreSuggestion, force)

	// Create memory scaffold unless --without-memory was passed.
	if withMemory {
		if err := writeMemoryScaffold(humanOut, projectDir); err != nil {
			fmt.Fprintf(humanOut, "  warning: memory scaffold: %v\n", err)
		}
	}

	if jsonOut {
		// Build adapter ID list.
		adapterIDs := make([]string, 0, len(selectedAdapters))
		for _, a := range selectedAdapters {
			adapterIDs = append(adapterIDs, string(a.ID()))
		}

		// Build component/MCP/plugin state from the computed project config.
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

	// Print post-init report.
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Initialized:")
	if meta.Language != "" {
		fmt.Fprintf(stdout, "  Language:   %s\n", meta.Language)
	}
	if meta.Name != "" {
		fmt.Fprintf(stdout, "  Project:    %s\n", meta.Name)
	}
	adapterNames := adapterSummary(selectedAdapters)
	if adapterNames != "" {
		fmt.Fprintf(stdout, "  Agents:     %s\n", adapterNames)
	}
	if meth != "" {
		team := domain.DefaultTeam(meth)
		roleNames := make([]string, 0, len(team))
		for name := range team {
			roleNames = append(roleNames, name)
		}
		sort.Strings(roleNames)
		fmt.Fprintf(stdout, "  Methodology: %s — %s\n", meth, methodologyDescription(meth))
		fmt.Fprintf(stdout, "  Team roles:  %s\n", strings.Join(roleNames, ", "))
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
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. Review .squadai/project.json and adjust settings if needed.")
	fmt.Fprintln(stdout, "  2. Run 'squadai apply' to configure your agent environment.")
	fmt.Fprintln(stdout, "  3. Run 'squadai verify' to confirm everything is correctly applied.")
	return nil
}

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

// DetectAdapters returns all registered adapters that are installed or have config.
// OpenCode (team lane) is always included. Personal-lane adapters (Claude Code,
// VS Code Copilot, Cursor, Windsurf, Pi) are included only when detected on the system.
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

	piAgent := pi.New()
	if installed, configFound, err := piAgent.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, piAgent)
	}

	return adapters
}

// filterAdapters returns only adapters whose ID is in the selections set.
// If selections is nil/empty, returns detected unchanged (backward-compatible).
// If selections is non-empty but no detected adapter matches, returns an empty slice.
func filterAdapters(detected []domain.Adapter, selections []string) []domain.Adapter {
	if len(selections) == 0 {
		return detected
	}
	allowed := make(map[string]bool, len(selections))
	for _, s := range selections {
		allowed[strings.TrimSpace(s)] = true
	}

	var result []domain.Adapter
	for _, a := range detected {
		if allowed[string(a.ID())] {
			result = append(result, a)
		}
	}
	return result
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

// timeNowUTC returns the current UTC time. Extracted for testability.
var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}
