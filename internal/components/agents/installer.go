package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

const teamSectionID = "team"

// Installer implements domain.ComponentInstaller for agent definitions.
// It writes .opencode/agents/<name>.md files with YAML frontmatter.
// When config.Team is non-empty, it also renders team orchestrator and sub-agent
// templates based on the adapter's delegation strategy.
type Installer struct {
	agents     map[string]domain.AgentDef // custom agents from config
	config     *domain.MergedConfig       // team config for template rendering (nil = V1 behavior)
	projectDir string
}

// New returns an agents installer configured from the merged agent definitions.
// cfg may be nil for backward compatibility (V1 behavior: only custom agents).
func New(agents map[string]domain.AgentDef, cfg *domain.MergedConfig, projectDir string) *Installer {
	resolved := make(map[string]domain.AgentDef)
	for name, def := range agents {
		// Resolve prompt content from file if needed.
		if def.Prompt == "" && def.PromptFile != "" && projectDir != "" {
			filePath := filepath.Join(projectDir, ".squadai", def.PromptFile)
			data, err := os.ReadFile(filePath)
			if err == nil {
				def.Prompt = string(data)
			}
		}
		resolved[name] = def
	}
	return &Installer{
		agents:     resolved,
		config:     cfg,
		projectDir: projectDir,
	}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentAgents
}

// Plan determines what agent file actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	var actions []domain.PlannedAction

	// Phase 1: Team-based agents (V2 behavior).
	// Prompt/solo delegation strategies write to the rules file rather than an
	// agents directory, so they bypass the SupportsComponent(ComponentAgents) guard.
	if i.config != nil && len(i.config.Team) > 0 && i.config.Methodology != "" {
		strategy := adapter.DelegationStrategy()
		if adapter.SupportsComponent(domain.ComponentAgents) ||
			strategy == domain.DelegationPromptBased ||
			strategy == domain.DelegationSoloAgent {
			teamActions, err := i.planTeamAgents(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			actions = append(actions, teamActions...)
		}
	}

	// Phase 2: Custom agents from config (V1 behavior).
	if !adapter.SupportsComponent(domain.ComponentAgents) {
		return actions, nil
	}

	agentsDir := adapter.ProjectAgentsDir(projectDir)
	if agentsDir != "" && len(i.agents) > 0 {
		names := sortedKeys(i.agents)
		for _, name := range names {
			def := i.agents[name]
			targetPath := filepath.Join(agentsDir, name+".md")
			content := renderAgent(name, def)
			actionID := fmt.Sprintf("%s-agent-%s", adapter.ID(), name)

			existing, err := fileutil.ReadFileOrEmpty(targetPath)
			if err != nil {
				return nil, fmt.Errorf("read agent %s: %w", name, err)
			}

			if string(existing) == content {
				actions = append(actions, domain.PlannedAction{
					ID:          actionID,
					Agent:       adapter.ID(),
					Component:   domain.ComponentAgents,
					Action:      domain.ActionSkip,
					TargetPath:  targetPath,
					Description: fmt.Sprintf("agent %s already up to date", name),
				})
				continue
			}

			action := domain.ActionCreate
			if len(existing) > 0 {
				action = domain.ActionUpdate
			}

			actions = append(actions, domain.PlannedAction{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentAgents,
				Action:      action,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("%s agent %s", action, name),
			})
		}
	}

	return actions, nil
}

// planTeamAgents dispatches to the appropriate delegation strategy planner.
func (i *Installer) planTeamAgents(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	switch adapter.DelegationStrategy() {
	case domain.DelegationNativeAgents:
		return i.planNativeAgents(adapter, homeDir, projectDir)
	case domain.DelegationPromptBased:
		return i.planPromptDelegation(adapter, homeDir, projectDir)
	case domain.DelegationSoloAgent:
		return i.planSoloAgent(adapter, homeDir, projectDir)
	default:
		return nil, nil
	}
}

// planNativeAgents plans file creation for each team role as a native agent .md file.
// Used for adapters like OpenCode and Cursor.
func (i *Installer) planNativeAgents(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	agentsDir := adapter.ProjectAgentsDir(projectDir)
	if agentsDir == "" {
		return nil, nil
	}

	methodology := string(i.config.Methodology)
	data := buildTemplateData(adapter, i.config, homeDir, projectDir)

	var actions []domain.PlannedAction

	// Orchestrator first.
	orchestratorContent, err := renderTemplate("orchestrator",
		assets.MustRead("teams/"+methodology+"/orchestrator-native.md"), data)
	if err != nil {
		return nil, fmt.Errorf("render orchestrator template: %w", err)
	}

	a, err := i.planNativeAgentFile(adapter, agentsDir, "orchestrator", orchestratorContent)
	if err != nil {
		return nil, err
	}
	actions = append(actions, a)

	// Sub-agents for each non-orchestrator team role.
	roles := sortedTeamRoles(i.config.Team)
	for _, roleName := range roles {
		if roleName == "orchestrator" {
			continue
		}
		tmplContent, readErr := assets.Read("teams/" + methodology + "/" + roleName + ".md")
		if readErr != nil {
			// Some roles may not have a dedicated template; skip gracefully.
			continue
		}
		rendered, renderErr := renderTemplate(roleName, tmplContent, data)
		if renderErr != nil {
			return nil, fmt.Errorf("render sub-agent template %s: %w", roleName, renderErr)
		}

		a, err := i.planNativeAgentFile(adapter, agentsDir, roleName, rendered)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}

	return actions, nil
}

// planNativeAgentFile produces a single PlannedAction for a native agent file.
func (i *Installer) planNativeAgentFile(adapter domain.Adapter, agentsDir, roleName, content string) (domain.PlannedAction, error) {
	targetPath := filepath.Join(agentsDir, roleName+".md")
	actionID := fmt.Sprintf("%s-team-%s", adapter.ID(), roleName)
	description := fmt.Sprintf("team:native:%s", roleName)

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return domain.PlannedAction{}, fmt.Errorf("read team agent %s: %w", roleName, err)
	}

	if string(existing) == content {
		return domain.PlannedAction{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentAgents,
			Action:      domain.ActionSkip,
			TargetPath:  targetPath,
			Description: description,
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return domain.PlannedAction{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentAgents,
		Action:      action,
		TargetPath:  targetPath,
		Description: description,
	}, nil
}

// planPromptDelegation plans injection of the orchestrator template into the project rules file.
// Used for adapters like Claude Code that use the Task tool for delegation.
func (i *Installer) planPromptDelegation(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	targetPath := adapter.ProjectRulesFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	methodology := string(i.config.Methodology)
	data := buildTemplateData(adapter, i.config, homeDir, projectDir)

	rendered, err := renderTemplate("orchestrator",
		assets.MustRead("teams/"+methodology+"/orchestrator-prompt.md"), data)
	if err != nil {
		return nil, fmt.Errorf("render prompt orchestrator template: %w", err)
	}

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read rules file: %w", err)
	}

	// Use marker.InjectSection to compute the desired file content.
	desired := marker.InjectSection(string(existing), teamSectionID, rendered)

	if string(existing) == desired {
		return []domain.PlannedAction{{
			ID:          fmt.Sprintf("%s-team-orchestrator", adapter.ID()),
			Agent:       adapter.ID(),
			Component:   domain.ComponentAgents,
			Action:      domain.ActionSkip,
			TargetPath:  targetPath,
			Description: "team:prompt:orchestrator",
		}}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return []domain.PlannedAction{{
		ID:          fmt.Sprintf("%s-team-orchestrator", adapter.ID()),
		Agent:       adapter.ID(),
		Component:   domain.ComponentAgents,
		Action:      action,
		TargetPath:  targetPath,
		Description: "team:prompt:orchestrator",
	}}, nil
}

// planSoloAgent plans injection of the orchestrator template into the project rules file.
// Used for adapters like VS Code Copilot and Windsurf that run all phases inline.
func (i *Installer) planSoloAgent(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	targetPath := adapter.ProjectRulesFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	methodology := string(i.config.Methodology)
	data := buildTemplateData(adapter, i.config, homeDir, projectDir)

	rendered, err := renderTemplate("orchestrator",
		assets.MustRead("teams/"+methodology+"/orchestrator-solo.md"), data)
	if err != nil {
		return nil, fmt.Errorf("render solo orchestrator template: %w", err)
	}

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read rules file: %w", err)
	}

	desired := marker.InjectSection(string(existing), teamSectionID, rendered)

	if string(existing) == desired {
		return []domain.PlannedAction{{
			ID:          fmt.Sprintf("%s-team-orchestrator", adapter.ID()),
			Agent:       adapter.ID(),
			Component:   domain.ComponentAgents,
			Action:      domain.ActionSkip,
			TargetPath:  targetPath,
			Description: "team:solo:orchestrator",
		}}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return []domain.PlannedAction{{
		ID:          fmt.Sprintf("%s-team-orchestrator", adapter.ID()),
		Agent:       adapter.ID(),
		Component:   domain.ComponentAgents,
		Action:      action,
		TargetPath:  targetPath,
		Description: "team:solo:orchestrator",
	}}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	if action.Action == domain.ActionDelete {
		if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete agent: %w", err)
		}
		return nil
	}

	switch {
	case strings.HasPrefix(action.Description, "team:native:"):
		return i.applyNativeAgent(action)
	case strings.HasPrefix(action.Description, "team:prompt:"):
		return i.applyMarkerInjection(action, "prompt")
	case strings.HasPrefix(action.Description, "team:solo:"):
		return i.applyMarkerInjection(action, "solo")
	default:
		return i.applyCustomAgent(action)
	}
}

// applyNativeAgent re-renders and writes a native agent file.
func (i *Installer) applyNativeAgent(action domain.PlannedAction) error {
	if i.config == nil {
		return fmt.Errorf("applyNativeAgent: config is nil")
	}

	// Derive role name from the target path filename.
	roleName := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	methodology := string(i.config.Methodology)

	// Build template data. We don't have homeDir/projectDir in Apply, so derive
	// projectDir from the target path by stripping the agents subdir.
	// Target: <projectDir>/<agentsSubPath>/<roleName>.md
	// We use the stored projectDir from construction.
	agentsSubdir := strings.TrimPrefix(filepath.Dir(action.TargetPath), i.projectDir)
	_ = agentsSubdir

	data := buildTemplateDataFromAction(i.config, i.projectDir, roleName, action.Agent)

	var templateContent string
	if roleName == "orchestrator" {
		templateContent = assets.MustRead("teams/" + methodology + "/orchestrator-native.md")
	} else {
		var err error
		templateContent, err = assets.Read("teams/" + methodology + "/" + roleName + ".md")
		if err != nil {
			return fmt.Errorf("read team agent template %s: %w", roleName, err)
		}
	}

	rendered, err := renderTemplate(roleName, templateContent, data)
	if err != nil {
		return fmt.Errorf("render team agent %s: %w", roleName, err)
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("write team agent %s: %w", roleName, err)
	}

	return nil
}

// applyMarkerInjection renders the orchestrator template and injects it into the rules file.
func (i *Installer) applyMarkerInjection(action domain.PlannedAction, variant string) error {
	if i.config == nil {
		return fmt.Errorf("applyMarkerInjection: config is nil")
	}

	methodology := string(i.config.Methodology)
	data := buildTemplateDataFromAction(i.config, i.projectDir, "orchestrator", action.Agent)

	templatePath := "teams/" + methodology + "/orchestrator-" + variant + ".md"
	rendered, err := renderTemplate("orchestrator", assets.MustRead(templatePath), data)
	if err != nil {
		return fmt.Errorf("render %s orchestrator: %w", variant, err)
	}

	existing, err := fileutil.ReadFileOrEmpty(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read rules file: %w", err)
	}

	updated := marker.InjectSection(string(existing), teamSectionID, rendered)

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write rules file: %w", err)
	}

	return nil
}

// applyCustomAgent writes a custom agent definition file.
func (i *Installer) applyCustomAgent(action domain.PlannedAction) error {
	// Extract agent name from target path.
	name := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	def, ok := i.agents[name]
	if !ok {
		return fmt.Errorf("agent %q not found in config", name)
	}

	content := renderAgent(name, def)

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write agent: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the agents component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	var results []domain.VerifyResult

	// Verify team agent files if team config is present.
	if i.config != nil && len(i.config.Team) > 0 && i.config.Methodology != "" {
		strategy := adapter.DelegationStrategy()
		if adapter.SupportsComponent(domain.ComponentAgents) ||
			strategy == domain.DelegationPromptBased ||
			strategy == domain.DelegationSoloAgent {
			teamResults, err := i.verifyTeamAgents(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			results = append(results, teamResults...)
		}
	}

	// Verify custom agent files.
	if !adapter.SupportsComponent(domain.ComponentAgents) {
		return results, nil
	}
	agentsDir := adapter.ProjectAgentsDir(projectDir)
	if agentsDir == "" || len(i.agents) == 0 {
		return results, nil
	}

	for _, name := range sortedKeys(i.agents) {
		def := i.agents[name]
		targetPath := filepath.Join(agentsDir, name+".md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("agent-%s-exists", name),
				Passed:  false,
				Message: fmt.Sprintf("agent file not found: %s", targetPath),
			})
			continue
		}

		expected := renderAgent(name, def)
		if string(data) == expected {
			results = append(results, domain.VerifyResult{
				Check:  fmt.Sprintf("agent-%s-current", name),
				Passed: true,
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("agent-%s-current", name),
				Passed:  false,
				Message: fmt.Sprintf("agent %s content does not match expected", name),
			})
		}
	}

	return results, nil
}

// verifyTeamAgents checks that team-related files are in the expected state.
func (i *Installer) verifyTeamAgents(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	methodology := string(i.config.Methodology)
	data := buildTemplateData(adapter, i.config, homeDir, projectDir)

	var results []domain.VerifyResult

	switch adapter.DelegationStrategy() {
	case domain.DelegationNativeAgents:
		agentsDir := adapter.ProjectAgentsDir(projectDir)
		if agentsDir == "" {
			return nil, nil
		}

		// Verify orchestrator.
		orchestratorContent, err := renderTemplate("orchestrator",
			assets.MustRead("teams/"+methodology+"/orchestrator-native.md"), data)
		if err != nil {
			return nil, fmt.Errorf("render orchestrator for verify: %w", err)
		}
		results = append(results, verifyFileContent(
			filepath.Join(agentsDir, "orchestrator.md"),
			orchestratorContent, "team-orchestrator"))

		// Verify sub-agents.
		for _, roleName := range sortedTeamRoles(i.config.Team) {
			if roleName == "orchestrator" {
				continue
			}
			tmplContent, readErr := assets.Read("teams/" + methodology + "/" + roleName + ".md")
			if readErr != nil {
				continue
			}
			rendered, renderErr := renderTemplate(roleName, tmplContent, data)
			if renderErr != nil {
				return nil, fmt.Errorf("render %s for verify: %w", roleName, renderErr)
			}
			results = append(results, verifyFileContent(
				filepath.Join(agentsDir, roleName+".md"),
				rendered, "team-"+roleName))
		}

	case domain.DelegationPromptBased, domain.DelegationSoloAgent:
		targetPath := adapter.ProjectRulesFile(projectDir)
		if targetPath == "" {
			return nil, nil
		}
		diskContent, err := os.ReadFile(targetPath)
		if err != nil {
			results = append(results, domain.VerifyResult{
				Check:   "team-orchestrator-file-exists",
				Passed:  false,
				Message: fmt.Sprintf("rules file not found: %s", targetPath),
			})
			return results, nil
		}
		if marker.HasSection(string(diskContent), teamSectionID) {
			results = append(results, domain.VerifyResult{
				Check:  "team-orchestrator-marker-present",
				Passed: true,
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:   "team-orchestrator-marker-present",
				Passed:  false,
				Message: "team marker section not found in rules file",
			})
		}
	}

	return results, nil
}

// verifyFileContent checks that a file exists and matches the expected content.
func verifyFileContent(path, expected, checkName string) domain.VerifyResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.VerifyResult{
			Check:   fmt.Sprintf("%s-exists", checkName),
			Passed:  false,
			Message: fmt.Sprintf("file not found: %s", path),
		}
	}
	if string(data) == expected {
		return domain.VerifyResult{
			Check:  fmt.Sprintf("%s-current", checkName),
			Passed: true,
		}
	}
	return domain.VerifyResult{
		Check:   fmt.Sprintf("%s-current", checkName),
		Passed:  false,
		Message: fmt.Sprintf("%s content does not match expected", checkName),
	}
}

// buildTemplateDataFromAction constructs TemplateData using the stored projectDir.
// Used in Apply where we don't have homeDir/projectDir as parameters.
func buildTemplateDataFromAction(cfg *domain.MergedConfig, projectDir string, roleName string, agentID domain.AgentID) TemplateData {
	_, hasContext7 := cfg.MCP["context7"]

	// Derive agentsDir and skillsDir from config and a reconstructed adapter-like lookup.
	// We use projectDir stored in the installer since Apply doesn't receive it.
	var agentsDir, skillsDir string

	// Determine typical paths based on agent ID pattern.
	switch agentID {
	case domain.AgentOpenCode:
		agentsDir = filepath.Join(projectDir, ".opencode", "agents")
		skillsDir = filepath.Join(projectDir, ".opencode", "skills")
	case domain.AgentCursor:
		agentsDir = filepath.Join(projectDir, ".cursor", "agents")
		skillsDir = filepath.Join(projectDir, ".cursor", "skills")
	case domain.AgentClaudeCode:
		skillsDir = filepath.Join(projectDir, ".claude", "skills")
	case domain.AgentVSCodeCopilot:
		skillsDir = filepath.Join(projectDir, ".copilot", "skills")
	case domain.AgentWindsurf:
		skillsDir = filepath.Join(projectDir, ".windsurf", "skills")
	default:
		agentsDir = filepath.Join(projectDir, ".opencode", "agents")
		skillsDir = filepath.Join(projectDir, ".opencode", "skills")
	}

	return TemplateData{
		Methodology:        string(cfg.Methodology),
		DelegationStrategy: delegationStrategyForAgent(agentID),
		Language:           cfg.Meta.Language,
		Languages:          cfg.Meta.Languages,
		TestCommand:        cfg.Meta.TestCommand,
		BuildCommand:       cfg.Meta.BuildCommand,
		LintCommand:        cfg.Meta.LintCommand,
		SkillsDir:          skillsDir,
		AgentsDir:          agentsDir,
		TeamRoles:          cfg.Team,
		MCPServers:         cfg.MCP,
		HasContext7:        hasContext7,
		Framework:          cfg.Meta.Framework,
		PackageManager:     cfg.Meta.PackageManager,
		ModelTier:          string(cfg.ModelTier),
		ModelHint:          promptHintForTier(string(cfg.ModelTier)),
	}
}

// delegationStrategyForAgent returns the delegation strategy string for a known agent ID.
func delegationStrategyForAgent(id domain.AgentID) string {
	switch id {
	case domain.AgentClaudeCode:
		return "prompt"
	case domain.AgentVSCodeCopilot, domain.AgentWindsurf:
		return "solo"
	default:
		return "native"
	}
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) (string, error) {
	switch {
	case strings.HasPrefix(action.Description, "team:native:"):
		return i.renderNativeAgentContent(action)
	case strings.HasPrefix(action.Description, "team:prompt:"):
		return i.renderMarkerInjectionContent(action, "prompt")
	case strings.HasPrefix(action.Description, "team:solo:"):
		return i.renderMarkerInjectionContent(action, "solo")
	default:
		return i.renderCustomAgentContent(action)
	}
}

// renderNativeAgentContent computes the content for a native agent file.
func (i *Installer) renderNativeAgentContent(action domain.PlannedAction) (string, error) {
	if i.config == nil {
		return "", fmt.Errorf("renderNativeAgentContent: config is nil")
	}
	roleName := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	methodology := string(i.config.Methodology)
	data := buildTemplateDataFromAction(i.config, i.projectDir, roleName, action.Agent)

	var templateContent string
	if roleName == "orchestrator" {
		templateContent = assets.MustRead("teams/" + methodology + "/orchestrator-native.md")
	} else {
		var err error
		templateContent, err = assets.Read("teams/" + methodology + "/" + roleName + ".md")
		if err != nil {
			return "", fmt.Errorf("read team agent template %s: %w", roleName, err)
		}
	}
	return renderTemplate(roleName, templateContent, data)
}

// renderMarkerInjectionContent computes the content for a prompt/solo marker injection.
func (i *Installer) renderMarkerInjectionContent(action domain.PlannedAction, variant string) (string, error) {
	if i.config == nil {
		return "", fmt.Errorf("renderMarkerInjectionContent: config is nil")
	}
	methodology := string(i.config.Methodology)
	data := buildTemplateDataFromAction(i.config, i.projectDir, "orchestrator", action.Agent)
	templatePath := "teams/" + methodology + "/orchestrator-" + variant + ".md"
	rendered, err := renderTemplate("orchestrator", assets.MustRead(templatePath), data)
	if err != nil {
		return "", fmt.Errorf("render %s orchestrator: %w", variant, err)
	}

	existing, readErr := fileutil.ReadFileOrEmpty(action.TargetPath)
	if readErr != nil {
		return "", fmt.Errorf("read rules file: %w", readErr)
	}

	updated := marker.InjectSection(string(existing), teamSectionID, rendered)
	return updated, nil
}

// renderCustomAgentContent computes the content for a custom agent file.
func (i *Installer) renderCustomAgentContent(action domain.PlannedAction) (string, error) {
	name := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	def, ok := i.agents[name]
	if !ok {
		return "", fmt.Errorf("agent %q not found in config", name)
	}
	return renderAgent(name, def), nil
}

// renderAgent generates the markdown content for an agent definition with YAML frontmatter.
func renderAgent(name string, def domain.AgentDef) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", def.Description))
	if def.Mode != "" {
		b.WriteString(fmt.Sprintf("mode: %s\n", def.Mode))
	}
	if def.Model != "" {
		b.WriteString(fmt.Sprintf("model: %s\n", def.Model))
	}
	if len(def.Permission) > 0 {
		b.WriteString("permission:\n")
		permKeys := make([]string, 0, len(def.Permission))
		for k := range def.Permission {
			permKeys = append(permKeys, k)
		}
		sort.Strings(permKeys)
		for _, k := range permKeys {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, def.Permission[k]))
		}
	}
	b.WriteString("---\n")
	if def.Prompt != "" {
		b.WriteString("\n")
		b.WriteString(def.Prompt)
		if !strings.HasSuffix(def.Prompt, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func sortedKeys(m map[string]domain.AgentDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTeamRoles(m map[string]domain.TeamRole) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
