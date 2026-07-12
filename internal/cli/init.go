package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
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

// RunInit creates .squadai/project.json and optionally .squadai/policy.json
// in the current working directory. It detects adapters, selects language-specific
// standards, and writes starter skill files.
func RunInit(args []string, stdout io.Writer) error {
	withPolicy := false
	withMemory := true     // memory scaffold is opt-out
	memoryScaffold := true // --no-memory-scaffold: keep memory enabled but skip the docs/memory scaffold
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
		case "--no-memory-scaffold":
			memoryScaffold = false
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
			fmt.Fprintln(stdout, "Usage: squadai init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--model-tier=<balanced|performance|starter|manual>] [--agents=<csv>] [--preset=<solo-minimal|solo-power|team-standard|enterprise-locked|full-squad|lean|custom>] [--with-policy] [--without-memory] [--no-memory-scaffold] [--force] [--merge] [--json] [--global]")
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
			fmt.Fprintln(stdout, "                 Omit to include all recommended servers; pass --mcp=none to enable none.")
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
			fmt.Fprintln(stdout, "                 solo-minimal: conventional workflow, starter models")
			fmt.Fprintln(stdout, "                 solo-power: TDD workflow, balanced models, daily-driver defaults")
			fmt.Fprintln(stdout, "                 team-standard: TDD workflow, balanced models, team policy (policy.json)")
			fmt.Fprintln(stdout, "                 enterprise-locked: SDD workflow, performance models, team policy (policy.json)")
			fmt.Fprintln(stdout, "                 full-squad: SDD methodology, balanced models, all components")
			fmt.Fprintln(stdout, "                 lean: conventional methodology, starter models, core only")
			fmt.Fprintln(stdout, "                 custom: explicit flags or wizard defaults")
			fmt.Fprintln(stdout, "  --global       Apply configuration globally (home directory) instead of the current project.")
			fmt.Fprintln(stdout, "  --with-policy  Also create .squadai/policy.json with a starter template.")
			fmt.Fprintln(stdout, "  --without-memory  Skip creating the docs/memory/ scaffold (memory is created by default).")
			fmt.Fprintln(stdout, "  --no-memory-scaffold  Keep project memory enabled but do not scaffold docs/memory/.")
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
		// Team presets carry shared governance: generate policy.json.
		withPolicy = true
	case domain.PresetEnterpriseLock:
		if !methodologyExplicit {
			methodology = string(domain.MethodologySDD)
			methodologyExplicit = true
		}
		if !modelTierExplicit {
			modelTier = domain.ModelTierPerformance
		}
		// Team presets carry shared governance: generate policy.json.
		withPolicy = true
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

	// Create memory scaffold unless --without-memory or --no-memory-scaffold was passed.
	if withMemory && memoryScaffold {
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
