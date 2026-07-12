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
	"strconv"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/memory"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/planner/budget"
	"github.com/PedroMosquera/squadai/internal/squadrefine"
	"github.com/PedroMosquera/squadai/internal/state"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/tokenizer"
	"github.com/PedroMosquera/squadai/internal/verify"
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

// validatePolicyResult is the JSON representation of a validate-policy run.
type validatePolicyResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
	PolicyPath string   `json:"policy_path"`
}

// RunValidatePolicy validates .squadai/policy.json in the current directory.
func RunValidatePolicy(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai validate-policy [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Validate .squadai/policy.json in the current directory. Checks that the")
			fmt.Fprintln(stdout, "schema is well-formed, that all locked component IDs are valid, and that required")
			fmt.Fprintln(stdout, "component constraints are internally consistent. Exits non-zero when issues are found.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the validation result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai validate-policy")
			fmt.Fprintln(stdout, "  squadai validate-policy --json")
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
			return exitcode.ErrPrecondition(
				fmt.Sprintf("no policy file found at %s", policyPath),
				"Run 'squadai init --with-policy' to create one.")
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
			return exitcode.ErrPolicyValidation(len(issues))
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
	return exitcode.ErrPolicyValidation(len(issues))
}

// RunPlan computes and displays the action plan.
func RunPlan(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	verbose := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOut = true
		case "--verbose", "-v":
			verbose = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai plan [--dry-run] [--json] [--verbose]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Compute the set of actions needed to bring all detected agents into the desired")
			fmt.Fprintln(stdout, "state described by .squadai/project.json. Covers all 9 components (memory,")
			fmt.Fprintln(stdout, "rules, settings, MCP, agents, skills, commands, plugins, workflows) across all 5")
			fmt.Fprintln(stdout, "supported agents. No files are written — this is always a read-only preview.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Accepted for consistency with apply; plan is inherently read-only.")
			fmt.Fprintln(stdout, "  --json     Output the planned actions as a JSON array.")
			fmt.Fprintln(stdout, "  --verbose  Show each action individually with its target path.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai plan")
			fmt.Fprintln(stdout, "  squadai plan --json")
			fmt.Fprintln(stdout, "  squadai plan --verbose")
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
		data, err := json.MarshalIndent(actions, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal plan actions: %w", err)
		}
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
	writePlannedActions(stdout, actions, verbose)

	fmt.Fprintln(stdout, "\nUse 'squadai apply' to execute.")
	return nil
}

// RunApply executes the plan with backup safety and step-level reporting.
func RunApply(args []string, stdout io.Writer) error {
	return runApplyImpl(args, stdout, nil)
}

// RunApplyWithProgress is like RunApply but forwards pipeline events to sink.
// When sink is nil it behaves identically to RunApply.
func RunApplyWithProgress(args []string, stdout io.Writer, sink pipeline.EventSink) error {
	return runApplyImpl(args, stdout, sink)
}

func desiredComponentTokens(p *planner.Planner, actions []domain.PlannedAction, homeDir, projectDir, modelName string) (map[domain.ComponentID]int, error) {
	counter := tokenizer.ForModel(modelName)
	// Each RenderAction returns the FULL rendered target document. When several
	// actions of the same component target one file (e.g. the OpenCode and Pi
	// memory sections both live in a shared AGENTS.md), summing per action would
	// count that document once per action and overstate the component, which can
	// make the budget fitter drop a component that actually fits. Keep the
	// largest render per (component, path) instead.
	type compPath struct {
		component domain.ComponentID
		path      string
	}
	tokensByPath := make(map[compPath]int)
	for _, action := range actions {
		if action.Action == domain.ActionDelete || action.TargetPath == "" {
			continue
		}
		_, desired, err := p.RenderAction(action, homeDir, projectDir)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", action.ID, err)
		}
		tokens := counter.Count(string(desired))
		key := compPath{component: action.Component, path: action.TargetPath}
		if tokens > tokensByPath[key] {
			tokensByPath[key] = tokens
		}
	}
	tokensByComponent := make(map[domain.ComponentID]int)
	for key, tokens := range tokensByPath {
		tokensByComponent[key.component] += tokens
	}
	return tokensByComponent, nil
}

// runApplyImpl is the shared implementation for RunApply and RunApplyWithProgress.
// externalSink is used when non-nil; the --verbose flag creates its own internal sink.
func runApplyImpl(args []string, stdout io.Writer, externalSink pipeline.EventSink) error {
	dryRun := false
	jsonOut := false
	force := false
	verbose := false
	noReview := false
	overwriteUnmanaged := false
	setClaudeDefaultAgent := false
	respectState := true
	noBrand := false
	maxTokens := 0
	fitModel := ""
	var explicitAgents []string
	var modelOverrides []string // raw "role=tier" pairs from --model flag
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--json":
			jsonOut = true
		case arg == "--force":
			force = true
		case arg == "--verbose":
			verbose = true
		case arg == "--no-review":
			noReview = true
		case arg == "--no-brand":
			noBrand = true
		case strings.HasPrefix(arg, "--max-tokens="):
			if v, err := strconv.Atoi(arg[len("--max-tokens="):]); err == nil {
				maxTokens = v
			}
		case strings.HasPrefix(arg, "--fit-model="):
			fitModel = arg[len("--fit-model="):]
		case arg == "--overwrite-unmanaged":
			overwriteUnmanaged = true
		case arg == "--set-claude-default-agent":
			setClaudeDefaultAgent = true
		case arg == "--respect-state" || arg == "--respect-state=true":
			respectState = true
		case arg == "--no-respect-state" || arg == "--respect-state=false":
			respectState = false
		case strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "-a="):
			val := arg[strings.Index(arg, "=")+1:]
			if val != "" {
				explicitAgents = append(explicitAgents, strings.Split(val, ",")...)
			}
		case strings.HasPrefix(arg, "--model="):
			val := strings.TrimPrefix(arg, "--model=")
			if val != "" {
				for _, pair := range strings.Split(val, ",") {
					pair = strings.TrimSpace(pair)
					if pair != "" {
						modelOverrides = append(modelOverrides, pair)
					}
				}
			}
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai apply [--dry-run] [--json] [--force] [--respect-state] [--verbose] [--model role=tier,...] [--no-brand] [--max-tokens=N] [--fit-model=<name>]")
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
			fmt.Fprintln(stdout, "  --dry-run           Preview the actions that would be executed without writing any files.")
			fmt.Fprintln(stdout, "  --json              Output the execution report as JSON (includes backup ID and step results).")
			fmt.Fprintln(stdout, "  --force             Apply with default config even when no project.json is found.")
			fmt.Fprintln(stdout, "  --verbose           Stream per-step progress to stderr as each action executes.")
			fmt.Fprintln(stdout, "  --no-review         Skip the pre-apply review screen (non-interactive / CI).")
			fmt.Fprintln(stdout, "  --no-brand          Skip the brand banner component for this apply (useful in CI).")
			fmt.Fprintln(stdout, "  --max-tokens=N      Budget cap: fit components within N tokens (drops lowest priority first).")
			fmt.Fprintln(stdout, "  --fit-model=<name>  Model to use for budget fitting (e.g. claude-sonnet-4, gpt-4o).")
			fmt.Fprintln(stdout, "  --overwrite-unmanaged  Grant blanket consent to overwrite any user-owned key")
			fmt.Fprintln(stdout, "                         SquadAI would write. Complements --no-review / CI flows;")
			fmt.Fprintln(stdout, "                         without this flag non-TTY applies halt on merge conflicts.")
			fmt.Fprintln(stdout, "  --agent=<csv>       Explicitly select agents to apply (e.g. opencode,cursor). Bypasses state filter.")
			fmt.Fprintln(stdout, "  --model=role=tier,... Override model tier per role for this run (in-memory only).")
			fmt.Fprintln(stdout, "                      Tiers: premium, standard, cheap. Example: --model=orchestrator=premium,implementer=cheap")
			fmt.Fprintln(stdout, "                      For permanent changes, edit agentmgr.yaml and re-run apply.")
			fmt.Fprintln(stdout, "  --respect-state     (default true) When state exists, restrict apply to previously-installed")
			fmt.Fprintln(stdout, "                      agents union current config. Use --no-respect-state to apply to all detected.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai apply")
			fmt.Fprintln(stdout, "  squadai apply --dry-run")
			fmt.Fprintln(stdout, "  squadai apply --json")
			fmt.Fprintln(stdout, "  squadai apply --force")
			fmt.Fprintln(stdout, "  squadai apply --verbose")
			fmt.Fprintln(stdout, "  squadai apply --no-respect-state")
			fmt.Fprintln(stdout, "  squadai apply --model=orchestrator=premium,implementer=cheap")
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
			return exitcode.ErrPrecondition(
				"no project.json found in current directory",
				"Run 'squadai init' to create one, or use --force to apply with defaults.")
		}
		fmt.Fprintln(stdout, "Warning: No project.json found. Running with default config (--force).")
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	// Apply --no-brand: disable the brand component in-memory for this run.
	if noBrand {
		if merged.Components == nil {
			merged.Components = make(map[string]domain.ComponentConfig)
		}
		merged.Components[string(domain.ComponentBrand)] = domain.ComponentConfig{Enabled: false}
	}

	// Apply --model overrides in-memory (does NOT write back to config file).
	if len(modelOverrides) > 0 {
		if err := applyModelOverrides(merged, modelOverrides); err != nil {
			return err
		}
	}

	adapters := DetectAdapters(homeDir)

	// Apply state-based filtering when --respect-state is active (default) and
	// the user did not explicitly pass --agent flags.
	if respectState && len(explicitAgents) == 0 {
		adapters = applyStateFilter(adapters, merged, homeDir)
	} else if len(explicitAgents) > 0 {
		adapters = filterAdapters(adapters, explicitAgents)
	}

	p := planner.New(planner.Options{SetClaudeDefaultAgent: setClaudeDefaultAgent})
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return exitcode.ErrPlanFailed(err)
	}

	if maxTokens > 0 {
		componentTokens, tokenErr := desiredComponentTokens(p, actions, homeDir, projectDir, fitModel)
		if tokenErr != nil {
			return fmt.Errorf("budget token estimate: %w", tokenErr)
		}
		fitResult, fitErr := budget.Fit(actions, budget.Options{
			MaxTokens:       maxTokens,
			Model:           fitModel,
			ComponentTokens: componentTokens,
		})
		if fitErr != nil {
			return fmt.Errorf("budget fit: %w", fitErr)
		}
		if !fitResult.FitAchieved {
			fmt.Fprintf(os.Stderr, "warning: could not fit within %d tokens even with all truncation. Proceeding with minimal set.\n", maxTokens)
		}
		actions = fitResult.Actions
		if !dryRun {
			if persistErr := budget.Persist(projectDir, fitResult); persistErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not persist budget: %v\n", persistErr)
			}
		}
	}

	if dryRun {
		if jsonOut {
			data, err := json.MarshalIndent(actions, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal apply actions: %w", err)
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: %d action(s) would be executed.\n", len(actions))
		for _, a := range actions {
			fmt.Fprintf(stdout, "  %-8s %s\n", a.Action, a.Description)
		}
		return nil
	}

	// Pre-apply review: show the user every file change before touching disk.
	// Skipped when --no-review, --json, or stdout is not a TTY (CI, pipes).
	var applyPolicy domain.ApplyPolicy
	if shouldRunReview(noReview, jsonOut) {
		entries, err := collectPreviewEntries(p.ComponentInstallers(), adapters, homeDir, projectDir)
		if err != nil {
			return fmt.Errorf("build review preview: %w", err)
		}
		if len(entries) > 0 {
			decision, err := ReviewPromptHook(entries)
			if err != nil {
				return fmt.Errorf("review prompt: %w", err)
			}
			if !decision.Confirmed {
				fmt.Fprintln(stdout, "Apply canceled.")
				return nil
			}
			applyPolicy = decision.Policy
		}
	}
	if overwriteUnmanaged {
		applyPolicy.OverwriteAll = true
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
	exec.WithPolicy(applyPolicy)

	// Determine the effective event sink.
	// --verbose takes precedence and creates its own channel sink.
	// If externalSink was provided (e.g. from TUI), use it when not verbose.
	var effectiveSink pipeline.EventSink
	var verboseCh chan pipeline.Event
	var verboseDone chan struct{}
	if verbose {
		verboseCh = make(chan pipeline.Event, len(actions)+4)
		effectiveSink = pipeline.NewChannelSink(verboseCh, true)
		verboseDone = make(chan struct{})
		go func() {
			defer close(verboseDone)
			for ev := range verboseCh {
				fmt.Fprintln(os.Stderr, ev.String())
			}
		}()
	} else if externalSink != nil {
		effectiveSink = externalSink
	} else {
		effectiveSink = pipeline.NopSink{}
	}

	exec.WithSink(effectiveSink)
	report, execErr := exec.Execute(actions)

	if verbose {
		// Close the channel so the drainer goroutine exits, then wait for it.
		close(verboseCh)
		<-verboseDone
	}

	if execErr != nil {
		if errors.Is(execErr, domain.ErrBackupFailed) {
			return fmt.Errorf("backup failed before apply: %w", execErr)
		}
		if errors.Is(execErr, domain.ErrRollbackFailed) {
			// Critical: rollback itself failed. Report what we can.
			fmt.Fprintln(stdout, "CRITICAL: rollback failed — manual recovery may be needed.")
			if report != nil && report.BackupID != "" {
				fmt.Fprintf(stdout, "  Backup ID: %s\n", report.BackupID)
				fmt.Fprintf(stdout, "  Try: squadai restore %s\n", report.BackupID)
			}
			return execErr
		}
		if errors.Is(execErr, domain.ErrMergeConflict) {
			fmt.Fprintln(stdout, "Apply halted: user-owned keys would be overwritten.")
			fmt.Fprintln(stdout, "Re-run without --no-review to resolve interactively, or")
			fmt.Fprintln(stdout, "pass --overwrite-unmanaged to grant blanket consent.")
			return execErr
		}
		return fmt.Errorf("apply: %w", execErr)
	}

	// Persist installed agent IDs to state (warning-only on failure).
	if report.Success {
		agentIDs := make([]string, 0, len(adapters))
		for _, a := range adapters {
			agentIDs = append(agentIDs, string(a.ID()))
		}
		if statePath, pathErr := state.DefaultPath(); pathErr == nil {
			if st, loadErr := state.Load(statePath); loadErr == nil {
				st.AddAgents(agentIDs)
				st.LastApply = timeNowUTC()
				if saveErr := state.Save(statePath, st); saveErr != nil {
					fmt.Fprintf(stdout, "Warning: could not save state: %v\n", saveErr)
				}
			} else {
				fmt.Fprintf(stdout, "Warning: could not load state: %v\n", loadErr)
			}
		}
	}

	if jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal apply report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		if !report.Success {
			return exitcode.ErrApplyFailed(fmt.Sprintf("rolled back, backup: %s — run 'squadai restore %s' if needed", report.BackupID, report.BackupID))
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
		fmt.Fprintf(stdout, "Use 'squadai restore %s' to manually restore if needed.\n", report.BackupID)
		return exitcode.ErrApplyFailed(fmt.Sprintf("run 'squadai restore %s' to manually restore if needed", report.BackupID))
	}

	fmt.Fprintln(stdout, "\nApply complete. Use 'squadai verify' to check.")
	return nil
}

// RunVerify runs compliance checks and prints the report.
func RunVerify(args []string, stdout io.Writer) error {
	jsonOut := false
	strict := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--strict":
			strict = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai verify [--json] [--strict]")
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
			fmt.Fprintln(stdout, "  --json    Output the full verification report as JSON.")
			fmt.Fprintln(stdout, "  --strict  Also run a drift check; fail if any managed file has drifted.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai verify")
			fmt.Fprintln(stdout, "  squadai verify --json")
			fmt.Fprintln(stdout, "  squadai verify --strict")
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
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal verify report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		if !report.AllPass {
			return exitcode.New(exitcode.Config, "E-301",
				"verification failed",
				"Run 'squadai diff' to see what's out of sync, then 'squadai apply' to fix.")
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
		return exitcode.New(exitcode.Config, "E-301",
			"verification failed",
			"Run 'squadai diff' to see what's out of sync, then 'squadai apply' to fix.")
	}

	if strict {
		if err := runStrictDriftCheck(projectDir, stdout, jsonOut); err != nil {
			return err
		}
	}

	return nil
}

// runStrictDriftCheck runs governance.CheckDrift and fails if any files have drifted.
func runStrictDriftCheck(projectDir string, stdout io.Writer, jsonOut bool) error {
	results, err := governance.CheckDrift(projectDir)
	if err != nil {
		return fmt.Errorf("strict drift check: %w", err)
	}

	var drifted []governance.DriftResult
	for _, r := range results {
		if r.Drifted() {
			drifted = append(drifted, r)
		}
	}

	if len(drifted) == 0 {
		if !jsonOut {
			fmt.Fprintln(stdout, "  [PASS] drift check — no managed files have drifted")
		}
		return nil
	}

	if jsonOut {
		type driftEntry struct {
			Path   string `json:"path"`
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		}
		out := make([]driftEntry, len(drifted))
		for i, r := range drifted {
			out[i] = driftEntry{Path: r.Path, Kind: string(r.Kind), Detail: r.Detail}
		}
		data, _ := json.MarshalIndent(map[string]any{"drift": out}, "", "  ")
		fmt.Fprintln(stdout, string(data))
	} else {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Drift detected (--strict):")
		for _, r := range drifted {
			fmt.Fprintf(stdout, "  [FAIL] %s — %s (%s)\n", r.Path, r.Detail, r.Kind)
		}
	}
	return exitcode.New(exitcode.Drift, "E-401",
		fmt.Sprintf("drift check failed: %d file(s) have drifted", len(drifted)),
		"Run 'squadai apply' to restore managed files to their expected state.")
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
			fmt.Fprintln(stdout, "Usage: squadai backup create [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Create a manual snapshot of all files that SquadAI manages. The backup")
			fmt.Fprintln(stdout, "includes every file that would be written by apply, even those that are already")
			fmt.Fprintln(stdout, "up to date. Backups are stored under ~/.squadai/backups by default.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the backup manifest as JSON (includes ID, timestamp, and file list).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup create")
			fmt.Fprintln(stdout, "  squadai backup create --json")
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
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal backup manifest: %w", err)
		}
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
			fmt.Fprintln(stdout, "Usage: squadai backup list [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "List all available backup snapshots. Shows the backup ID, the command that created")
			fmt.Fprintln(stdout, "the backup (apply or manual), the number of files captured, and the status.")
			fmt.Fprintln(stdout, "Use the ID with 'squadai restore <id>' to roll back to a specific snapshot.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the backup list as a JSON array.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup list")
			fmt.Fprintln(stdout, "  squadai backup list --json")
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
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
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
		data, err := json.MarshalIndent(manifests, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal backup list: %w", err)
		}
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
			fmt.Fprintln(stdout, "Usage: squadai restore <backup-id> [--dry-run] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Restore managed files from a backup snapshot. Files that existed before the backup")
			fmt.Fprintln(stdout, "are written back to their original content; files that did not exist before are")
			fmt.Fprintln(stdout, "removed. The backup ID is printed after every apply and can be listed with")
			fmt.Fprintln(stdout, "'squadai backup list'.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Show which files would be restored or removed without changing anything.")
			fmt.Fprintln(stdout, "  --json     Output the restore result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai restore 2024-01-15T10-30-00Z-abc123")
			fmt.Fprintln(stdout, "  squadai restore <id> --dry-run")
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
		return exitcode.ErrMissingArg("backup-id", "squadai restore <backup-id>")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	manifest, err := store.Get(backupID)
	if err != nil {
		return exitcode.ErrBackupNotFound(backupID)
	}

	if dryRun {
		if jsonOut {
			data, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal restore manifest: %w", err)
			}
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
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal restore result: %w", err)
		}
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

type squadRefinementInfo struct {
	Status    string   `json:"status"`
	Reasons   []string `json:"reasons"`
	LastRunAt string   `json:"last_run_at,omitempty"`
}

// RunStatus shows the current project configuration summary with health checks.
func RunStatus(args []string, stdout io.Writer) error {
	jsonOut := false
	fix := false
	daily := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--fix":
			fix = true
		case "--daily":
			daily = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai status [--json] [--fix] [--daily]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Show the current project configuration summary: detected agents, active components,")
			fmt.Fprintln(stdout, "configured MCP servers, health checks, and the most recent backup.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json   Output the status as JSON.")
			fmt.Fprintln(stdout, "  --fix    Run 'squadai apply' to fix any health issues found.")
			fmt.Fprintln(stdout, "  --daily  Show the daily-driver dashboard summary.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai status")
			fmt.Fprintln(stdout, "  squadai status --json")
			fmt.Fprintln(stdout, "  squadai status --daily")
			fmt.Fprintln(stdout, "  squadai status --fix")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for status", arg)
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

	mergedCfg, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)

	p := planner.New()
	actions, err := p.Plan(mergedCfg, adapters, homeDir, projectDir)
	if err != nil {
		return exitcode.ErrPlanFailed(err)
	}

	// Run verify to get health results.
	v := verify.New()
	verifyReport, verifyErr := v.Verify(mergedCfg, adapters, homeDir, projectDir)
	if verifyErr != nil {
		verifyReport = nil
	}

	// Count managed files per component (non-skip actions grouped by component).
	compOrder := make([]string, 0)
	compMap := make(map[string]map[string]bool)
	for _, a := range actions {
		if a.Action == domain.ActionSkip {
			continue
		}
		if a.TargetPath == "" {
			continue
		}
		comp := string(a.Component)
		if _, exists := compMap[comp]; !exists {
			compMap[comp] = make(map[string]bool)
			compOrder = append(compOrder, comp)
		}
		compMap[comp][a.TargetPath] = true
	}

	// Also include components that only have skip actions (already managed).
	for _, a := range actions {
		if a.TargetPath == "" {
			continue
		}
		comp := string(a.Component)
		if _, exists := compMap[comp]; !exists {
			compMap[comp] = make(map[string]bool)
			compOrder = append(compOrder, comp)
		}
		compMap[comp][a.TargetPath] = true
	}

	// Deduplicate compOrder preserving first appearance.
	seenComp := make(map[string]bool)
	deduped := make([]string, 0, len(compOrder))
	for _, c := range compOrder {
		if !seenComp[c] {
			seenComp[c] = true
			deduped = append(deduped, c)
		}
	}
	compOrder = deduped
	sort.Strings(compOrder)

	// Get MCP server names.
	mcpNames := make([]string, 0, len(mergedCfg.MCP))
	for name := range mergedCfg.MCP {
		mcpNames = append(mcpNames, name)
	}
	sort.Strings(mcpNames)

	// Get most recent backup.
	backupDir := backup.ResolveBackupDir(mergedCfg.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifests, listErr := store.List()
	var lastManifest *backup.Manifest
	if listErr == nil && len(manifests) > 0 {
		lastManifest = &manifests[0]
	}

	// Compute squad refinement status (skip when project not initialized).
	var refInfo *squadRefinementInfo
	projectConfigPath := config.ProjectConfigPath(projectDir)
	if _, statErr := os.Stat(projectConfigPath); statErr == nil {
		refState, refExists, _ := squadrefine.Load(projectDir)
		if !refExists {
			refInfo = &squadRefinementInfo{Status: "never-refined", Reasons: []string{}}
		} else {
			signals := sampleDriftSignals(projectDir)
			reasons := squadrefine.DriftReasons(refState, signals)
			if len(reasons) == 0 {
				refInfo = &squadRefinementInfo{Status: "fresh", Reasons: []string{}, LastRunAt: refState.LastRunAt}
			} else {
				refInfo = &squadRefinementInfo{Status: "stale", Reasons: reasons, LastRunAt: refState.LastRunAt}
			}
		}
	}

	if jsonOut {
		type adapterStatus struct {
			ID         string `json:"id"`
			Delegation string `json:"delegation,omitempty"`
		}
		type componentStatus struct {
			ID           string `json:"id"`
			ManagedFiles int    `json:"managed_files"`
		}
		type backupInfo struct {
			ID        string `json:"id"`
			Timestamp string `json:"timestamp"`
			Files     int    `json:"files"`
		}
		type healthSummary struct {
			AllPass       bool `json:"all_pass"`
			TotalChecks   int  `json:"total_checks"`
			FailingChecks int  `json:"failing_checks"`
		}
		type refinementJSON struct {
			Status    string   `json:"status"`
			Reasons   []string `json:"reasons"`
			LastRunAt string   `json:"last_run_at,omitempty"`
		}
		type statusResult struct {
			ProjectDir     string              `json:"project_dir"`
			Language       string              `json:"language,omitempty"`
			Methodology    string              `json:"methodology,omitempty"`
			Mode           string              `json:"mode,omitempty"`
			Preset         string              `json:"preset,omitempty"`
			ContextProfile string              `json:"context_profile,omitempty"`
			Memory         domain.MemoryConfig `json:"memory,omitempty"`
			Usage          domain.UsageConfig  `json:"usage,omitempty"`
			ModelTier      string              `json:"model_tier,omitempty"`
			Adapters       []adapterStatus     `json:"adapters"`
			Components     []componentStatus   `json:"components"`
			MCPServers     []string            `json:"mcp_servers"`
			LastBackup     *backupInfo         `json:"last_backup,omitempty"`
			Health         *healthSummary      `json:"health,omitempty"`
			Refinement     *refinementJSON     `json:"refinement,omitempty"`
		}

		adapterList := make([]adapterStatus, 0, len(adapters))
		for _, a := range adapters {
			adapterList = append(adapterList, adapterStatus{
				ID:         string(a.ID()),
				Delegation: string(a.DelegationStrategy()),
			})
		}

		componentList := make([]componentStatus, 0, len(compOrder))
		for _, comp := range compOrder {
			componentList = append(componentList, componentStatus{
				ID:           comp,
				ManagedFiles: len(compMap[comp]),
			})
		}

		var lastBackupJSON *backupInfo
		if lastManifest != nil {
			lastBackupJSON = &backupInfo{
				ID:        lastManifest.ID,
				Timestamp: lastManifest.Timestamp.Format("2006-01-02T15:04:05Z"),
				Files:     len(lastManifest.AffectedFiles),
			}
		}

		var health *healthSummary
		if verifyReport != nil {
			failing := 0
			for _, r := range verifyReport.Results {
				if !r.Passed && r.Severity == domain.SeverityError {
					failing++
				}
			}
			health = &healthSummary{
				AllPass:       verifyReport.AllPass,
				TotalChecks:   len(verifyReport.Results),
				FailingChecks: failing,
			}
		}

		var refJSON *refinementJSON
		if refInfo != nil {
			refJSON = &refinementJSON{
				Status:    refInfo.Status,
				Reasons:   refInfo.Reasons,
				LastRunAt: refInfo.LastRunAt,
			}
		}
		result := statusResult{
			ProjectDir:     projectDir,
			Language:       mergedCfg.Meta.Language,
			Methodology:    string(mergedCfg.Methodology),
			Mode:           string(mergedCfg.Mode),
			Preset:         string(mergedCfg.Preset),
			ContextProfile: mergedCfg.Context.DefaultProfile,
			Memory:         mergedCfg.Memory,
			Usage:          mergedCfg.Usage,
			ModelTier:      string(mergedCfg.ModelTier),
			Adapters:       adapterList,
			Components:     componentList,
			MCPServers:     mcpNames,
			LastBackup:     lastBackupJSON,
			Health:         health,
			Refinement:     refJSON,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal status result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Human-readable output.
	language := mergedCfg.Meta.Language
	if language == "" {
		language = "unknown"
	}
	methodology := string(mergedCfg.Methodology)
	if methodology == "" {
		methodology = "none"
	}
	mode := string(mergedCfg.Mode)
	if mode == "" {
		mode = "standard"
	}
	preset := string(mergedCfg.Preset)
	if preset == "" {
		preset = "custom"
	}
	contextProfile := mergedCfg.Context.DefaultProfile
	if contextProfile == "" {
		contextProfile = "default"
	}

	if daily {
		printDailyStatus(stdout, projectDir, language, methodology, mode, preset, contextProfile, mergedCfg, adapters, mcpNames, lastManifest, verifyReport, refInfo)
		return nil
	}

	fmt.Fprintf(stdout, "Project: %s (%s)\n", filepath.Base(projectDir), language)
	fmt.Fprintf(stdout, "Methodology: %s\n", methodology)
	fmt.Fprintf(stdout, "Mode: %s\n", mode)
	fmt.Fprintf(stdout, "Preset: %s\n", preset)
	fmt.Fprintf(stdout, "Context profile: %s\n", contextProfile)
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, "Agents (%d enabled):\n", len(adapters))
	for _, a := range adapters {
		fmt.Fprintf(stdout, "  %-20s %-12s %s\n", string(a.ID()), string(a.Lane()), string(a.DelegationStrategy()))
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, "Components (%d active):\n", len(compOrder))
	for _, comp := range compOrder {
		fmt.Fprintf(stdout, "  %-20s %d files managed\n", comp, len(compMap[comp]))
	}
	fmt.Fprintln(stdout)

	mcpDisplay := "none"
	if len(mcpNames) > 0 {
		mcpDisplay = strings.Join(mcpNames, ", ")
	}
	fmt.Fprintf(stdout, "MCP servers: %s\n", mcpDisplay)
	fmt.Fprintln(stdout)

	if lastManifest != nil {
		fmt.Fprintf(stdout, "Last backup: %s (%d files)\n",
			lastManifest.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			len(lastManifest.AffectedFiles))
	} else {
		fmt.Fprintln(stdout, "Last backup: none")
	}

	// Health section.
	if verifyReport != nil && len(verifyReport.Results) > 0 {
		fmt.Fprintln(stdout)
		failing := make([]domain.VerifyResult, 0)
		for _, r := range verifyReport.Results {
			if !r.Passed && r.Severity == domain.SeverityError {
				failing = append(failing, r)
			}
		}
		if len(failing) == 0 {
			fmt.Fprintf(stdout, "Health: OK (%d checks passed)\n", len(verifyReport.Results))
		} else {
			fmt.Fprintf(stdout, "Health: %d failing check(s) of %d\n", len(failing), len(verifyReport.Results))
			for _, r := range failing {
				msg := r.Check
				if r.Message != "" {
					msg = r.Message
				}
				fmt.Fprintf(stdout, "  [FAIL] %s\n", msg)
			}
			if fix {
				fmt.Fprintln(stdout)
				fmt.Fprintln(stdout, "Running 'squadai apply' to fix issues…")
				return RunApply([]string{}, stdout)
			}
			fmt.Fprintln(stdout, "\nRun 'squadai apply' to fix, or 'squadai status --fix' to fix automatically.")
		}
	}

	// Refinement section.
	if refInfo != nil {
		fmt.Fprintln(stdout)
		switch refInfo.Status {
		case "fresh":
			fmt.Fprintln(stdout, "Refinement: fresh")
		case "stale":
			fmt.Fprintf(stdout, "Refinement: stale: %s\n", strings.Join(refInfo.Reasons, ", "))
		default:
			fmt.Fprintln(stdout, "Refinement: never-refined")
		}
	}

	return nil
}

func printDailyStatus(stdout io.Writer, projectDir, language, methodology, mode, preset, contextProfile string, mergedCfg *domain.MergedConfig, adapters []domain.Adapter, mcpNames []string, lastManifest *backup.Manifest, verifyReport *domain.VerifyReport, refInfo *squadRefinementInfo) {
	fmt.Fprintf(stdout, "Daily Status: %s\n", filepath.Base(projectDir))
	fmt.Fprintf(stdout, "Project: %s (%s)\n", projectDir, language)
	fmt.Fprintf(stdout, "Setup: preset=%s methodology=%s mode=%s profile=%s\n", preset, methodology, mode, contextProfile)

	agentIDs := make([]string, 0, len(adapters))
	for _, a := range adapters {
		agentIDs = append(agentIDs, string(a.ID()))
	}
	sort.Strings(agentIDs)
	if len(agentIDs) == 0 {
		fmt.Fprintln(stdout, "Agents: none detected")
	} else {
		fmt.Fprintf(stdout, "Agents: %s\n", strings.Join(agentIDs, ", "))
	}

	mcpDisplay := "none"
	if len(mcpNames) > 0 {
		mcpDisplay = strings.Join(mcpNames, ", ")
	}
	fmt.Fprintf(stdout, "MCP: %s\n", mcpDisplay)

	memoryBackend := mergedCfg.Memory.Backend
	if memoryBackend == "" {
		memoryBackend = "docs"
	}
	memoryExport := mergedCfg.Memory.ExportPath
	if memoryExport == "" {
		memoryExport = "docs/memory"
	}
	autoCapture := "off"
	if mergedCfg.Memory.AutoCapture {
		autoCapture = "on"
	}
	fmt.Fprintf(stdout, "Memory: backend=%s auto_capture=%s export=%s\n", memoryBackend, autoCapture, memoryExport)

	enforcement := mergedCfg.Usage.Enforcement
	if enforcement == "" {
		enforcement = "off"
	}
	fmt.Fprintf(stdout, "Usage: enforcement=%s session_tokens=%d daily_tokens=%d", enforcement, mergedCfg.Usage.SessionTokenBudget, mergedCfg.Usage.DailyTokenBudget)
	if mergedCfg.Usage.SessionCostBudget > 0 || mergedCfg.Usage.DailyCostBudget > 0 {
		currency := mergedCfg.Usage.Currency
		if currency == "" {
			currency = "USD"
		}
		fmt.Fprintf(stdout, " session_cost=%.2f%s daily_cost=%.2f%s", mergedCfg.Usage.SessionCostBudget, currency, mergedCfg.Usage.DailyCostBudget, currency)
	}
	fmt.Fprintln(stdout)

	if verifyReport != nil {
		failing := 0
		for _, r := range verifyReport.Results {
			if !r.Passed && r.Severity == domain.SeverityError {
				failing++
			}
		}
		if failing == 0 {
			fmt.Fprintf(stdout, "Health: OK (%d checks)\n", len(verifyReport.Results))
		} else if lastManifest == nil {
			fmt.Fprintf(stdout, "Health: setup pending (%d missing or stale check(s) of %d)\n", failing, len(verifyReport.Results))
			fmt.Fprintln(stdout, "Next: run squadai apply --no-review")
		} else {
			fmt.Fprintf(stdout, "Health: %d failing check(s) of %d\n", failing, len(verifyReport.Results))
		}
	} else {
		fmt.Fprintln(stdout, "Health: unavailable")
	}

	if refInfo != nil {
		refinement := refInfo.Status
		if len(refInfo.Reasons) > 0 {
			refinement += " (" + strings.Join(refInfo.Reasons, ", ") + ")"
		}
		fmt.Fprintf(stdout, "Refinement: %s\n", refinement)
	}

	if lastManifest != nil {
		fmt.Fprintf(stdout, "Last backup: %s (%d files)\n", lastManifest.Timestamp.Format("2006-01-02 15:04:05 UTC"), len(lastManifest.AffectedFiles))
	} else {
		fmt.Fprintln(stdout, "Last backup: none")
	}
}

// RunSquadInitStatus returns the current squad refinement status as JSON.
// It is used by the squad_init_status MCP tool and reads from the cwd.
func RunSquadInitStatus(_ []string, stdout io.Writer) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	type result struct {
		Status    string   `json:"status"`
		Reasons   []string `json:"reasons"`
		LastRunAt string   `json:"last_run_at,omitempty"`
	}

	// When project.json does not exist, return not-initialized status.
	projectConfigPath := config.ProjectConfigPath(projectDir)
	if _, statErr := os.Stat(projectConfigPath); os.IsNotExist(statErr) {
		r := result{Status: "not-initialized", Reasons: []string{}}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	refState, exists, loadErr := squadrefine.Load(projectDir)
	if loadErr != nil {
		return fmt.Errorf("load squad-refined state: %w", loadErr)
	}

	if !exists {
		r := result{Status: "never-refined", Reasons: []string{}}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	signals := sampleDriftSignals(projectDir)
	reasons := squadrefine.DriftReasons(refState, signals)
	if len(reasons) == 0 {
		r := result{Status: "fresh", Reasons: []string{}, LastRunAt: refState.LastRunAt}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	r := result{Status: "stale", Reasons: reasons, LastRunAt: refState.LastRunAt}
	data, _ := json.MarshalIndent(r, "", "  ")
	fmt.Fprintln(stdout, string(data))
	return nil
}

// RunMemorySearchTool is the MCP tool handler for memory_search.
// It reads the cwd, calls memory.Search, and returns results as a JSON array.
func RunMemorySearchTool(args []string, stdout io.Writer) error {
	var query string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--query=") {
			query = strings.TrimPrefix(arg, "--query=")
		}
	}
	if query == "" {
		return fmt.Errorf("memory_search: query is required")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	results, err := memory.Search(projectDir, query)
	if err != nil {
		return fmt.Errorf("memory search: %w", err)
	}

	if results == nil {
		results = []memory.SearchResult{}
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal search results: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}

// RunMemoryAddTool is the MCP tool handler for memory_add.
// It reads the cwd, calls memory.AddInbox, and returns the saved path as JSON.
func RunMemoryAddTool(args []string, stdout io.Writer) error {
	var note string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--note=") {
			note = strings.TrimPrefix(arg, "--note=")
		}
	}
	if note == "" {
		return fmt.Errorf("memory_add: note is required")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	savedPath, err := memory.AddInbox(projectDir, note)
	if err != nil {
		return fmt.Errorf("memory add: %w", err)
	}

	type result struct {
		Path string `json:"path"`
	}
	data, err := json.MarshalIndent(result{Path: savedPath}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory add result: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}

// RunBackupDelete removes a backup by ID.
func RunBackupDelete(args []string, stdout io.Writer) error {
	jsonOut := false
	var id string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai backup delete <id> [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Delete a backup snapshot by its ID. The backup and all its files are permanently")
			fmt.Fprintln(stdout, "removed. Use 'squadai backup list' to see available backup IDs.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup delete 20240115T103000Z-abc123")
			fmt.Fprintln(stdout, "  squadai backup delete <id> --json")
			return nil
		default:
			if id == "" {
				id = arg
			} else {
				return fmt.Errorf("unexpected argument %q", arg)
			}
		}
	}

	if id == "" {
		return exitcode.ErrMissingArg("backup-id", "squadai backup delete <id>")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	manifest, err := store.Get(id)
	if err != nil {
		return exitcode.ErrBackupNotFound(id)
	}

	fileCount := len(manifest.AffectedFiles)

	if err := store.Delete(id); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"backup_id": id,
			"status":    "deleted",
			"files":     fileCount,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal delete result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Deleted backup %s (%d files).\n", id, fileCount)
	return nil
}

// RunBackupPrune removes all but the N most recent backups.
func RunBackupPrune(args []string, stdout io.Writer) error {
	keep := 10
	jsonOut := false

	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai backup prune [--keep=N] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Remove all but the N most recent backup snapshots. Keeps the newest N backups")
			fmt.Fprintln(stdout, "and permanently deletes the rest. Use 'squadai backup list' to see available")
			fmt.Fprintln(stdout, "backups before pruning.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --keep=N   Number of recent backups to keep (default 10).")
			fmt.Fprintln(stdout, "  --json     Output the result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup prune")
			fmt.Fprintln(stdout, "  squadai backup prune --keep=5")
			fmt.Fprintln(stdout, "  squadai backup prune --keep=3 --json")
			return nil
		case strings.HasPrefix(arg, "--keep="):
			val := strings.TrimPrefix(arg, "--keep=")
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("invalid --keep value %q: %w", val, err)
			}
			keep = n
		default:
			return fmt.Errorf("unknown flag %q for backup prune", arg)
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	// Count current backups before pruning to report accurate "kept" count.
	manifests, err := store.List()
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	total := len(manifests)

	deleted, err := store.Prune(keep)
	if err != nil {
		return fmt.Errorf("prune backups: %w", err)
	}

	kept := total - deleted

	if jsonOut {
		result := map[string]interface{}{
			"deleted": deleted,
			"kept":    keep,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal prune result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if deleted == 0 {
		fmt.Fprintf(stdout, "Nothing to prune (%d backups, keeping %d).\n", kept, keep)
		return nil
	}

	fmt.Fprintf(stdout, "Pruned %d backups (kept %d most recent).\n", deleted, kept)
	return nil
}

// RemoveOptions configures a Remove operation.
type RemoveOptions struct {
	DryRun     bool
	JSON       bool
	ProjectDir string // when empty, uses os.Getwd()
}

// RemoveReport is the result of a Remove operation.
type RemoveReport struct {
	RemovedFiles []string `json:"removed_files"` // files deleted entirely
	CleanedFiles []string `json:"cleaned_files"` // files with marker blocks stripped
	Errors       []string `json:"errors"`
	DryRun       bool     `json:"dry_run"`
}

// Remove removes all SquadAI-managed configuration from the project:
//   - Files in created_files (sidecar) are deleted entirely.
//   - Files in managed_files (sidecar) have their marker blocks stripped; if
//     the file becomes empty (or only whitespace) after stripping, it is deleted.
//   - On success (non-dry-run) the sidecar itself is removed via DeleteSidecar.
func Remove(opts RemoveOptions) (RemoveReport, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return RemoveReport{}, fmt.Errorf("resolve working directory: %w", err)
		}
	}

	createdFiles, err := managed.ListCreatedFiles(projectDir)
	if err != nil {
		return RemoveReport{}, fmt.Errorf("list created files: %w", err)
	}

	managedFiles, err := managed.ListManagedFiles(projectDir)
	if err != nil {
		return RemoveReport{}, fmt.Errorf("list managed files: %w", err)
	}

	report := RemoveReport{
		RemovedFiles: []string{},
		CleanedFiles: []string{},
		Errors:       []string{},
		DryRun:       opts.DryRun,
	}

	// --- Process created_files: delete entirely ---
	for _, relPath := range createdFiles {
		absPath := filepath.Join(projectDir, relPath)
		if opts.DryRun {
			if _, statErr := os.Stat(absPath); statErr == nil {
				report.RemovedFiles = append(report.RemovedFiles, absPath)
			}
			continue
		}
		if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
			report.Errors = append(report.Errors, fmt.Sprintf("remove %s: %v", absPath, removeErr))
		} else {
			report.RemovedFiles = append(report.RemovedFiles, absPath)
		}
	}

	// --- Process managed_files: strip marker blocks ---
	for _, relPath := range managedFiles {
		absPath := filepath.Join(projectDir, relPath)
		data, readErr := os.ReadFile(absPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			report.Errors = append(report.Errors, fmt.Sprintf("read %s: %v", absPath, readErr))
			continue
		}

		stripped, hasMarkers := marker.StripAll(string(data))
		if !hasMarkers {
			// Nothing to strip — file has no marker blocks managed by us.
			continue
		}

		if opts.DryRun {
			if strings.TrimSpace(stripped) == "" {
				report.RemovedFiles = append(report.RemovedFiles, absPath)
			} else {
				report.CleanedFiles = append(report.CleanedFiles, absPath)
			}
			continue
		}

		if strings.TrimSpace(stripped) == "" {
			// File becomes empty — delete it.
			if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
				report.Errors = append(report.Errors, fmt.Sprintf("remove %s: %v", absPath, removeErr))
			} else {
				report.RemovedFiles = append(report.RemovedFiles, absPath)
			}
		} else {
			// Preserve user content outside marker blocks.
			if _, writeErr := fileutil.WriteAtomic(absPath, []byte(stripped), 0644); writeErr != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("write %s: %v", absPath, writeErr))
			} else {
				report.CleanedFiles = append(report.CleanedFiles, absPath)
			}
		}
	}

	// Clean up sidecar unless dry-run.
	if !opts.DryRun {
		if delErr := managed.DeleteSidecar(projectDir); delErr != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("delete sidecar: %v", delErr))
		}
	}

	return report, nil
}

// removeResult is the JSON representation of a successful remove run.
type removeResult struct {
	BackupID string   `json:"backup_id"`
	Deleted  []string `json:"deleted"`
	Stripped []string `json:"stripped"`
	DryRun   bool     `json:"dry_run"`
}

// RunRemove removes all SquadAI managed files from the current project.
// Files with marker blocks that also contain user content are stripped of
// the managed sections while preserving user content.
func RunRemove(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	force := false
	respectState := true
	var explicitAgents []string

	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--json":
			jsonOut = true
		case arg == "--force":
			force = true
		case arg == "--respect-state" || arg == "--respect-state=true":
			respectState = true
		case arg == "--no-respect-state" || arg == "--respect-state=false":
			respectState = false
		case strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "-a="):
			val := arg[strings.Index(arg, "=")+1:]
			if val != "" {
				explicitAgents = append(explicitAgents, strings.Split(val, ",")...)
			}
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai remove [--force] [--dry-run] [--json] [--respect-state]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Remove all files managed by SquadAI from the current project. Files that")
			fmt.Fprintln(stdout, "contain marker blocks alongside user content are stripped of the managed sections")
			fmt.Fprintln(stdout, "only — user content is preserved. Fully managed files (no user content) are deleted.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "A backup is created automatically before any files are removed.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --force             Required to confirm removal (without it, the command errors).")
			fmt.Fprintln(stdout, "  --dry-run           Preview which files would be removed or stripped without changing anything.")
			fmt.Fprintln(stdout, "  --json              Output the result as JSON.")
			fmt.Fprintln(stdout, "  --agent=<csv>       Explicitly select agents to remove (bypasses state filter).")
			fmt.Fprintln(stdout, "  --respect-state     (default true) Restrict remove to previously-installed agents.")
			fmt.Fprintln(stdout, "                      Use --no-respect-state to operate on all detected agents.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai remove --dry-run")
			fmt.Fprintln(stdout, "  squadai remove --force")
			fmt.Fprintln(stdout, "  squadai remove --force --json")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for remove", arg)
		}
	}

	// Without --force or --dry-run, refuse to proceed.
	if !force && !dryRun {
		return fmt.Errorf("refusing to remove without confirmation — use --force to confirm or --dry-run to preview")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	mergedCfg, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)

	// Apply state-based filtering when --respect-state is active (default) and
	// the user did not explicitly pass --agent flags.
	if respectState && len(explicitAgents) == 0 {
		adapters = applyStateFilter(adapters, mergedCfg, homeDir)
	} else if len(explicitAgents) > 0 {
		adapters = filterAdapters(adapters, explicitAgents)
	}

	p := planner.New()
	actions, err := p.Plan(mergedCfg, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Collect ALL target paths (including skip actions — remove wants to clean
	// up everything managed, even files currently in sync).
	paths := collectAllTargetPaths(actions)

	if dryRun {
		result := removeResult{
			BackupID: "",
			Deleted:  []string{},
			Stripped: []string{},
			DryRun:   true,
		}

		// Classify each path as would-delete or would-strip.
		var wouldDelete []string
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("read %s: %w", path, err)
			}
			stripped, hasMarkers := marker.StripAll(string(data))
			if hasMarkers && strings.TrimSpace(stripped) != "" {
				result.Stripped = append(result.Stripped, path)
			} else {
				result.Deleted = append(result.Deleted, path)
				wouldDelete = append(wouldDelete, path)
			}
		}

		// Also check .squadai/ directory.
		squadaiDir := filepath.Join(projectDir, ".squadai")
		if info, err := os.Stat(squadaiDir); err == nil && info.IsDir() {
			result.Deleted = append(result.Deleted, squadaiDir)
		}

		// Report directories that would become empty after removing the files above.
		wouldRemoveDirs := dryRunEmptyManagedDirs(projectDir, wouldDelete)
		result.Deleted = append(result.Deleted, wouldRemoveDirs...)

		if jsonOut {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal remove result: %w", err)
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}

		if len(result.Deleted) == 0 && len(result.Stripped) == 0 {
			fmt.Fprintln(stdout, "Dry run: no managed files found.")
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: would remove %d file(s), strip %d file(s).\n", len(result.Deleted), len(result.Stripped))
		for _, p := range result.Deleted {
			fmt.Fprintf(stdout, "  delete:  %s\n", p)
		}
		for _, p := range result.Stripped {
			fmt.Fprintf(stdout, "  strip:   %s (user content preserved)\n", p)
		}
		return nil
	}

	// Create a backup before removing anything.
	backupDir := backup.ResolveBackupDir(mergedCfg.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	var backupID string
	if len(paths) > 0 {
		manifest, err := store.SnapshotFiles(paths, "remove")
		if err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		backupID = manifest.ID
	}

	var deleted []string
	var stripped []string

	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				// File already gone — skip silently.
				continue
			}
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		strippedContent, hasMarkers := marker.StripAll(string(data))

		if hasMarkers && strings.TrimSpace(strippedContent) != "" {
			// File has markers AND user content — write back the stripped version.
			if _, writeErr := fileutil.WriteAtomic(path, []byte(strippedContent), 0644); writeErr != nil {
				return fmt.Errorf("write stripped %s: %w", path, writeErr)
			}
			stripped = append(stripped, path)
		} else {
			// Fully managed file (either: markers with no user content, or no markers
			// at all meaning the whole file is ours) — delete it.
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove %s: %w", path, removeErr)
			}
			deleted = append(deleted, path)
		}
	}

	// Normalise nil slices to empty slices for consistent JSON output.
	if deleted == nil {
		deleted = []string{}
	}
	if stripped == nil {
		stripped = []string{}
	}

	if jsonOut {
		result := removeResult{
			BackupID: backupID,
			Deleted:  deleted,
			Stripped: stripped,
			DryRun:   false,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal remove result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if backupID != "" {
		fmt.Fprintf(stdout, "Backup created: %s\n", backupID)
	}

	// Clean up the .squadai/ directory (project.json, managed.json, templates, etc.).
	squadaiDir := filepath.Join(projectDir, ".squadai")
	if err := os.RemoveAll(squadaiDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove .squadai directory: %w", err)
	}
	if _, err := os.Stat(squadaiDir); os.IsNotExist(err) {
		deleted = append(deleted, squadaiDir)
	}

	// Remove any empty parent directories left behind after file deletion.
	removedDirs := removeEmptyManagedDirs(projectDir, deleted)
	deleted = append(deleted, removedDirs...)

	fmt.Fprintf(stdout, "Removed %d files, stripped markers from %d files.\n", len(deleted), len(stripped))
	for _, p := range deleted {
		fmt.Fprintf(stdout, "  deleted: %s\n", p)
	}
	for _, p := range stripped {
		fmt.Fprintf(stdout, "  stripped: %s (user content preserved)\n", p)
	}

	// Update state: remove agents that were just removed (warning-only on failure).
	agentIDs := make([]string, 0, len(adapters))
	for _, a := range adapters {
		agentIDs = append(agentIDs, string(a.ID()))
	}
	if statePath, pathErr := state.DefaultPath(); pathErr == nil {
		if st, loadErr := state.Load(statePath); loadErr == nil {
			st.RemoveAgents(agentIDs)
			if saveErr := state.Save(statePath, st); saveErr != nil {
				fmt.Fprintf(stdout, "Warning: could not save state: %v\n", saveErr)
			}
		} else {
			fmt.Fprintf(stdout, "Warning: could not load state: %v\n", loadErr)
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

// dryRunEmptyManagedDirs computes which ancestor directories of the given
// paths WOULD become empty if those paths were deleted. It does not modify
// the filesystem. Used by the dry-run branch of RunRemove.
func dryRunEmptyManagedDirs(projectDir string, wouldDeletePaths []string) []string {
	// Build a set of paths that would be deleted so we can simulate their removal.
	willDelete := make(map[string]bool, len(wouldDeletePaths))
	for _, p := range wouldDeletePaths {
		willDelete[p] = true
	}

	// Collect all ancestor directories up to projectDir.
	candidates := make(map[string]bool)
	for _, p := range wouldDeletePaths {
		dir := filepath.Dir(p)
		for dir != projectDir && dir != "/" && dir != "." && len(dir) > len(projectDir) {
			candidates[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Sort deepest-first.
	sorted := make([]string, 0, len(candidates))
	for d := range candidates {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })

	// Simulate deletion: for each candidate dir, check if all its current entries
	// are in the willDelete set (files) or in the would-be-removed dirs set (dirs).
	wouldRemove := make(map[string]bool)
	var result []string
	for _, dir := range sorted {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		allGone := true
		for _, e := range entries {
			entryPath := filepath.Join(dir, e.Name())
			if !willDelete[entryPath] && !wouldRemove[entryPath] {
				allGone = false
				break
			}
		}
		if allGone {
			wouldRemove[dir] = true
			result = append(result, dir)
		}
	}
	return result
}

// removeEmptyManagedDirs removes empty directories that were left behind after
// deleting managed files. It walks deepest-first so that nested empty dirs are
// handled correctly (.claude/agents → .claude). Only directories that are
// ancestors of deleted paths (up to projectDir) are considered.
func removeEmptyManagedDirs(projectDir string, deletedPaths []string) []string {
	// Collect all ancestor directories of deleted files, stopping at projectDir.
	candidates := make(map[string]bool)
	for _, p := range deletedPaths {
		dir := filepath.Dir(p)
		for dir != projectDir && dir != "/" && dir != "." && len(dir) > len(projectDir) {
			candidates[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Sort deepest-first (longest path first) to handle nested dirs correctly.
	sorted := make([]string, 0, len(candidates))
	for d := range candidates {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })

	var removed []string
	for _, dir := range sorted {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // dir may have already been removed by a parent removal
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err == nil {
				removed = append(removed, dir)
			}
		}
	}
	return removed
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
