package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── Config builders ──────────────────────────────────────────────────────────

// buildClaudeNodeReactConfig writes and loads a config for Claude Code + Node/React project.
func buildClaudeNodeReactConfig(t *testing.T, home, project string, meta domain.ProjectMeta) *domain.MergedConfig {
	t.Helper()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	return loadMerged(t, home, project)
}

// buildVSCodeNodeReactConfig writes and loads a config for VS Code Copilot + Node/React project.
func buildVSCodeNodeReactConfig(t *testing.T, home, project string, meta domain.ProjectMeta) *domain.MergedConfig {
	t.Helper()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentVSCodeCopilot)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentVSCodeCopilot): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	return loadMerged(t, home, project)
}

// buildCursorNodeReactConfig writes and loads a config for Cursor + Node/React project.
func buildCursorNodeReactConfig(t *testing.T, home, project string, meta domain.ProjectMeta) *domain.MergedConfig {
	t.Helper()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentCursor)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentCursor): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	return loadMerged(t, home, project)
}

// buildWindsurfNodeReactConfig writes and loads a config for Windsurf + Node/React project.
func buildWindsurfNodeReactConfig(t *testing.T, home, project string, meta domain.ProjectMeta) *domain.MergedConfig {
	t.Helper()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentWindsurf)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentWindsurf): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	return loadMerged(t, home, project)
}

// loadMerged is a shared helper that loads user + project configs and merges them.
func loadMerged(t *testing.T, home, project string) *domain.MergedConfig {
	t.Helper()
	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	return config.Merge(user, proj, nil)
}

// ─── Claude Code / Node React ─────────────────────────────────────────────────

// TestMultiAdapter_ClaudeCode_NodeReact_MCPFormat verifies Claude Code uses
// .mcp.json at project root (MCPConfigFile strategy) for a Node/React project.
func TestMultiAdapter_ClaudeCode_NodeReact_MCPFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildClaudeNodeReactConfig(t, home, dir, meta)

	adapter := claude.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("ClaudeCode/NodeReact/MCPFormat: apply should succeed")
	}

	// Claude Code uses MCPConfigFile strategy: <project>/.mcp.json with "mcpServers".
	mcpFile := filepath.Join(dir, ".mcp.json")
	assertFileExists(t, mcpFile, "ClaudeCode/NodeReact: .mcp.json")
	assertJSONKey(t, mcpFile, "mcpServers", "ClaudeCode/NodeReact: .mcp.json has mcpServers key")

	// Must NOT write opencode.json or .vscode/mcp.json.
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); err == nil {
		t.Error("ClaudeCode/NodeReact: must not write opencode.json (wrong adapter)")
	}
}

// TestMultiAdapter_ClaudeCode_NodeReact_RulesFormat verifies Claude Code writes
// CLAUDE.md with a squadai memory marker block.
func TestMultiAdapter_ClaudeCode_NodeReact_RulesFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildClaudeNodeReactConfig(t, home, dir, meta)

	adapter := claude.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("ClaudeCode/NodeReact/RulesFormat: apply should succeed")
	}

	// Claude Code memory target is CLAUDE.md in the project root.
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	assertFileExists(t, claudeMD, "ClaudeCode/NodeReact: CLAUDE.md")
	assertFileContains(t, claudeMD, "squadai", "ClaudeCode/NodeReact: CLAUDE.md has squadai marker")
}

// TestMultiAdapter_ClaudeCode_NodeReact_TemplateRendering verifies CLAUDE.md is
// written with the expected squadai memory section (no stack rendering for Claude Code).
func TestMultiAdapter_ClaudeCode_NodeReact_TemplateRendering(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildClaudeNodeReactConfig(t, home, dir, meta)

	adapter := claude.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("ClaudeCode/NodeReact/TemplateRendering: apply should succeed")
	}

	claudeMD := filepath.Join(dir, "CLAUDE.md")
	assertFileExists(t, claudeMD, "ClaudeCode/NodeReact: CLAUDE.md")

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Must have the agent-manager marker section injected.
	if !strings.Contains(content, "<!-- squadai:memory") {
		t.Error("ClaudeCode/NodeReact: CLAUDE.md missing squadai:memory marker")
	}
	// Must not be empty or have a plaintext-only stub.
	if len(strings.TrimSpace(content)) == 0 {
		t.Error("ClaudeCode/NodeReact: CLAUDE.md must not be empty")
	}
}

// TestMultiAdapter_ClaudeCode_NodeReact_Reversibility validates that Remove cleans up
// Claude Code files and leaves the original Node/React project files untouched.
func TestMultiAdapter_ClaudeCode_NodeReact_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildClaudeNodeReactConfig(t, home, dir, meta)

	adapter := claude.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("ClaudeCode/NodeReact/Reversibility: apply should succeed before remove test")
	}

	assertFileExists(t, filepath.Join(dir, "CLAUDE.md"), "ClaudeCode/NodeReact: CLAUDE.md before remove")
	assertFileExists(t, filepath.Join(dir, ".mcp.json"), "ClaudeCode/NodeReact: .mcp.json before remove")

	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	totalRemoved := len(removeReport.RemovedFiles) + len(removeReport.CleanedFiles)
	if totalRemoved == 0 {
		t.Error("ClaudeCode/NodeReact: Remove should have removed or cleaned at least one file")
	}

	// Original Node/React project files must be untouched.
	assertFileExists(t, filepath.Join(dir, "package.json"), "ClaudeCode/NodeReact: package.json untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "App.tsx"), "ClaudeCode/NodeReact: src/App.tsx untouched after remove")
}

// ─── VS Code Copilot / Node React ─────────────────────────────────────────────

// TestMultiAdapter_VSCode_NodeReact_MCPFormat verifies VS Code writes .vscode/mcp.json
// with a "servers" root key for a Node/React project.
func TestMultiAdapter_VSCode_NodeReact_MCPFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildVSCodeNodeReactConfig(t, home, dir, meta)

	adapter := vscode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("VSCode/NodeReact/MCPFormat: apply should succeed")
	}

	// VS Code uses MCPConfigFile strategy: .vscode/mcp.json with "servers" key.
	mcpJSON := filepath.Join(dir, ".vscode", "mcp.json")
	assertFileExists(t, mcpJSON, "VSCode/NodeReact: .vscode/mcp.json")
	assertJSONKey(t, mcpJSON, "servers", "VSCode/NodeReact: .vscode/mcp.json has 'servers' key")

	data, err := os.ReadFile(mcpJSON)
	if err != nil {
		t.Fatalf("read .vscode/mcp.json: %v", err)
	}
	jsonStr := string(data)
	if strings.Contains(jsonStr, `"mcpServers"`) {
		t.Error("VSCode/NodeReact: .vscode/mcp.json must not have 'mcpServers' key (should be 'servers')")
	}
	if strings.Contains(jsonStr, `"mcp":`) {
		t.Error("VSCode/NodeReact: .vscode/mcp.json must not have 'mcp' key (should be 'servers')")
	}
}

// TestMultiAdapter_VSCode_NodeReact_RulesFormat verifies VS Code writes .instructions.md
// (plain text, no frontmatter) with a squadai memory marker.
func TestMultiAdapter_VSCode_NodeReact_RulesFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildVSCodeNodeReactConfig(t, home, dir, meta)

	adapter := vscode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("VSCode/NodeReact/RulesFormat: apply should succeed")
	}

	// VS Code memory target is .instructions.md at project root (ProjectRulesFile).
	instructionsMD := filepath.Join(dir, ".instructions.md")
	assertFileExists(t, instructionsMD, "VSCode/NodeReact: .instructions.md")
	assertFileContains(t, instructionsMD, "squadai", "VSCode/NodeReact: .instructions.md has squadai marker")

	data, err := os.ReadFile(instructionsMD)
	if err != nil {
		t.Fatalf("read .instructions.md: %v", err)
	}
	// Plain text — must NOT start with YAML frontmatter.
	if strings.HasPrefix(string(data), "---\n") {
		t.Error("VSCode/NodeReact: .instructions.md must not have YAML frontmatter (plain text only)")
	}
}

// TestMultiAdapter_VSCode_NodeReact_TemplateRendering verifies the memory marker section
// is present and the file is non-trivially populated for a Node/React project.
func TestMultiAdapter_VSCode_NodeReact_TemplateRendering(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildVSCodeNodeReactConfig(t, home, dir, meta)

	adapter := vscode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("VSCode/NodeReact/TemplateRendering: apply should succeed")
	}

	instructionsMD := filepath.Join(dir, ".instructions.md")
	assertFileExists(t, instructionsMD, "VSCode/NodeReact: .instructions.md")

	data, err := os.ReadFile(instructionsMD)
	if err != nil {
		t.Fatalf("read .instructions.md: %v", err)
	}
	content := string(data)

	// Must have the agent-manager marker section injected.
	if !strings.Contains(content, "<!-- squadai:memory") {
		t.Error("VSCode/NodeReact: .instructions.md missing squadai:memory marker")
	}
	// Must not be empty.
	if len(strings.TrimSpace(content)) == 0 {
		t.Error("VSCode/NodeReact: .instructions.md must not be empty")
	}
}

// TestMultiAdapter_VSCode_NodeReact_Reversibility validates that Remove cleans up
// VS Code files and leaves the original Node/React project files untouched.
func TestMultiAdapter_VSCode_NodeReact_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildVSCodeNodeReactConfig(t, home, dir, meta)

	adapter := vscode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("VSCode/NodeReact/Reversibility: apply should succeed before remove test")
	}

	assertFileExists(t, filepath.Join(dir, ".instructions.md"), "VSCode/NodeReact: .instructions.md before remove")
	assertFileExists(t, filepath.Join(dir, ".vscode", "mcp.json"), "VSCode/NodeReact: .vscode/mcp.json before remove")

	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	totalRemoved := len(removeReport.RemovedFiles) + len(removeReport.CleanedFiles)
	if totalRemoved == 0 {
		t.Error("VSCode/NodeReact: Remove should have removed or cleaned at least one file")
	}

	// Original Node/React project files must be untouched.
	assertFileExists(t, filepath.Join(dir, "package.json"), "VSCode/NodeReact: package.json untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "App.tsx"), "VSCode/NodeReact: src/App.tsx untouched after remove")
}

// ─── Cursor / Node React ──────────────────────────────────────────────────────

// TestMultiAdapter_Cursor_NodeReact_MCPFormat verifies Cursor writes .cursor/mcp.json
// with "mcpServers" key for a Node/React project.
func TestMultiAdapter_Cursor_NodeReact_MCPFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildCursorNodeReactConfig(t, home, dir, meta)

	adapter := cursor.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("Cursor/NodeReact/MCPFormat: apply should succeed")
	}

	// Cursor uses MCPConfigFile strategy: .cursor/mcp.json with "mcpServers" key.
	mcpJSON := filepath.Join(dir, ".cursor", "mcp.json")
	assertFileExists(t, mcpJSON, "Cursor/NodeReact: .cursor/mcp.json")
	assertJSONKey(t, mcpJSON, "mcpServers", "Cursor/NodeReact: .cursor/mcp.json has 'mcpServers' key")

	data, err := os.ReadFile(mcpJSON)
	if err != nil {
		t.Fatalf("read .cursor/mcp.json: %v", err)
	}
	if strings.Contains(string(data), `"servers":`) {
		t.Error("Cursor/NodeReact: .cursor/mcp.json must not have 'servers' key (should be 'mcpServers')")
	}
}

// TestMultiAdapter_Cursor_NodeReact_RulesFormat verifies Cursor writes
// .cursor/rules/squadai.mdc with alwaysApply: true YAML frontmatter.
func TestMultiAdapter_Cursor_NodeReact_RulesFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)

	// Cursor memory targets .cursor/rules/squadai.mdc (ProjectRulesFile).
	// We only need memory enabled for the marker injection.
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentCursor)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentCursor): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentRules):  {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		Rules: domain.RulesConfig{
			TeamStandards: "## Node/React Team Standards\n\nUse TypeScript strictly.\n",
		},
	}
	if err := config.WriteJSON(config.ProjectConfigPath(dir), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	merged := loadMerged(t, home, dir)

	adapter := cursor.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("Cursor/NodeReact/RulesFormat: apply should succeed")
	}

	// .cursor/rules/squadai.mdc must exist with alwaysApply: true frontmatter.
	rulesFile := filepath.Join(dir, ".cursor", "rules", "squadai.mdc")
	assertFileExists(t, rulesFile, "Cursor/NodeReact: .cursor/rules/squadai.mdc")

	data, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatalf("read squadai.mdc: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("Cursor/NodeReact: squadai.mdc must start with YAML frontmatter (---)")
	}
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("Cursor/NodeReact: squadai.mdc must contain 'alwaysApply: true'")
	}
	if !strings.Contains(content, "Node/React Team Standards") {
		t.Error("Cursor/NodeReact: squadai.mdc must contain the injected team standards content")
	}
}

// TestMultiAdapter_Cursor_NodeReact_TemplateRendering verifies Cursor writes an
// orchestrator agent file (DelegationNativeAgents) that references TypeScript.
func TestMultiAdapter_Cursor_NodeReact_TemplateRendering(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildCursorNodeReactConfig(t, home, dir, meta)

	adapter := cursor.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("Cursor/NodeReact/TemplateRendering: apply should succeed")
	}

	// Cursor uses DelegationNativeAgents — agent files go to .cursor/agents/.
	orchestratorPath := filepath.Join(dir, ".cursor", "agents", "orchestrator.md")
	assertFileExists(t, orchestratorPath, "Cursor/NodeReact: .cursor/agents/orchestrator.md")
	assertFileContains(t, orchestratorPath, "TypeScript", "Cursor/NodeReact: orchestrator.md mentions TypeScript")

	// Must NOT mention incorrect languages.
	data, err := os.ReadFile(orchestratorPath)
	if err != nil {
		t.Fatalf("read orchestrator.md: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "- Language: Python") {
		t.Error("Cursor/NodeReact: orchestrator.md must not reference Python")
	}
	if strings.Contains(content, "- Language: Java") {
		t.Error("Cursor/NodeReact: orchestrator.md must not reference Java")
	}
}

// TestMultiAdapter_Cursor_NodeReact_Reversibility validates that Remove cleans up
// Cursor files and leaves the original Node/React project files untouched.
func TestMultiAdapter_Cursor_NodeReact_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildCursorNodeReactConfig(t, home, dir, meta)

	adapter := cursor.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("Cursor/NodeReact/Reversibility: apply should succeed before remove test")
	}

	assertFileExists(t, filepath.Join(dir, ".cursor", "mcp.json"), "Cursor/NodeReact: .cursor/mcp.json before remove")
	assertFileExists(t, filepath.Join(dir, ".cursor", "agents", "orchestrator.md"), "Cursor/NodeReact: orchestrator.md before remove")

	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	totalRemoved := len(removeReport.RemovedFiles) + len(removeReport.CleanedFiles)
	if totalRemoved == 0 {
		t.Error("Cursor/NodeReact: Remove should have removed or cleaned at least one file")
	}

	// Orchestrator file should be removed.
	orchestratorPath := filepath.Join(dir, ".cursor", "agents", "orchestrator.md")
	if _, err := os.Stat(orchestratorPath); err == nil {
		t.Error("Cursor/NodeReact: orchestrator.md should be removed after Remove")
	}

	// Original Node/React project files must be untouched.
	assertFileExists(t, filepath.Join(dir, "package.json"), "Cursor/NodeReact: package.json untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "App.tsx"), "Cursor/NodeReact: src/App.tsx untouched after remove")
}

// ─── Windsurf / Node React ────────────────────────────────────────────────────

// TestMultiAdapter_Windsurf_NodeReact_MCPFormat verifies Windsurf writes
// .windsurf/mcp_config.json with "mcpServers" key for a Node/React project.
func TestMultiAdapter_Windsurf_NodeReact_MCPFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildWindsurfNodeReactConfig(t, home, dir, meta)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("Windsurf/NodeReact/MCPFormat: apply should succeed")
	}

	// Windsurf uses MCPConfigFile strategy: .windsurf/mcp_config.json with "mcpServers" key.
	mcpJSON := filepath.Join(dir, ".windsurf", "mcp_config.json")
	assertFileExists(t, mcpJSON, "Windsurf/NodeReact: .windsurf/mcp_config.json")
	assertJSONKey(t, mcpJSON, "mcpServers", "Windsurf/NodeReact: .windsurf/mcp_config.json has 'mcpServers' key")

	data, err := os.ReadFile(mcpJSON)
	if err != nil {
		t.Fatalf("read .windsurf/mcp_config.json: %v", err)
	}
	if strings.Contains(string(data), `"servers":`) {
		t.Error("Windsurf/NodeReact: mcp_config.json must not have 'servers' key (should be 'mcpServers')")
	}
}

// TestMultiAdapter_Windsurf_NodeReact_RulesFormat verifies Windsurf writes
// .windsurf/rules/squadai.md with trigger: always_on YAML frontmatter.
func TestMultiAdapter_Windsurf_NodeReact_RulesFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentWindsurf)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentWindsurf): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentRules):  {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		Rules: domain.RulesConfig{
			TeamStandards: "## Node/React Team Standards\n\nUse pnpm for package management.\n",
		},
	}
	if err := config.WriteJSON(config.ProjectConfigPath(dir), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	merged := loadMerged(t, home, dir)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("Windsurf/NodeReact/RulesFormat: apply should succeed")
	}

	// .windsurf/rules/squadai.md must exist with trigger: always_on frontmatter.
	rulesFile := filepath.Join(dir, ".windsurf", "rules", "squadai.md")
	assertFileExists(t, rulesFile, "Windsurf/NodeReact: .windsurf/rules/squadai.md")

	data, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatalf("read squadai.md: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("Windsurf/NodeReact: squadai.md must start with YAML frontmatter (---)")
	}
	if !strings.Contains(content, "trigger: always_on") {
		t.Error("Windsurf/NodeReact: squadai.md must contain 'trigger: always_on'")
	}
	if !strings.Contains(content, "Node/React Team Standards") {
		t.Error("Windsurf/NodeReact: squadai.md must contain the injected team standards content")
	}
}

// TestMultiAdapter_Windsurf_NodeReact_TemplateRendering verifies memory marker section
// is written to the rules file for Windsurf (solo agent, no orchestrator template).
func TestMultiAdapter_Windsurf_NodeReact_TemplateRendering(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildWindsurfNodeReactConfig(t, home, dir, meta)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("Windsurf/NodeReact/TemplateRendering: apply should succeed")
	}

	// Windsurf memory targets .windsurf/rules/squadai.md (ProjectRulesFile).
	rulesFile := filepath.Join(dir, ".windsurf", "rules", "squadai.md")
	assertFileExists(t, rulesFile, "Windsurf/NodeReact: .windsurf/rules/squadai.md")

	data, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatalf("read squadai.md: %v", err)
	}
	content := string(data)

	// Must have the agent-manager memory marker injected.
	if !strings.Contains(content, "<!-- squadai:memory") {
		t.Error("Windsurf/NodeReact: squadai.md missing squadai:memory marker")
	}
	if len(strings.TrimSpace(content)) == 0 {
		t.Error("Windsurf/NodeReact: squadai.md must not be empty")
	}
}

// TestMultiAdapter_Windsurf_NodeReact_Reversibility validates that Remove cleans up
// Windsurf files and leaves the original Node/React project files untouched.
func TestMultiAdapter_Windsurf_NodeReact_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildWindsurfNodeReactConfig(t, home, dir, meta)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("Windsurf/NodeReact/Reversibility: apply should succeed before remove test")
	}

	assertFileExists(t, filepath.Join(dir, ".windsurf", "mcp_config.json"), "Windsurf/NodeReact: .windsurf/mcp_config.json before remove")

	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	totalRemoved := len(removeReport.RemovedFiles) + len(removeReport.CleanedFiles)
	if totalRemoved == 0 {
		t.Error("Windsurf/NodeReact: Remove should have removed or cleaned at least one file")
	}

	// Original Node/React project files must be untouched.
	assertFileExists(t, filepath.Join(dir, "package.json"), "Windsurf/NodeReact: package.json untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "App.tsx"), "Windsurf/NodeReact: src/App.tsx untouched after remove")
}

// ─── Multi-Adapter Simultaneous (OpenCode + Claude Code + Cursor) ─────────────

// TestMultiAdapter_Simultaneous_OpenCodeClaudeCursor runs init with OpenCode + Claude Code + Cursor
// on the same Node/React project and verifies:
//   - All 3 generate their own configs without conflicts.
//   - Each MCP config uses the correct format for its agent.
//   - Each rules/memory file is at the correct path.
//   - Remove cleans up all 3 without touching project files.
func TestMultiAdapter_Simultaneous_OpenCodeClaudeCursor(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	userCfg.Adapters[string(domain.AgentCursor)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode):   {Enabled: true},
			string(domain.AgentClaudeCode): {Enabled: true},
			string(domain.AgentCursor):     {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(dir), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	merged := loadMerged(t, home, dir)

	ocAdapter := opencode.New()
	ccAdapter := claude.New()
	csAdapter := cursor.New()
	adapters := []domain.Adapter{ocAdapter, ccAdapter, csAdapter}

	// Plan + execute for all 3 adapters simultaneously.
	for _, adapter := range adapters {
		report := runPlanExecute(t, merged, adapter, home, dir)
		if !report.Success {
			for _, s := range report.Steps {
				if s.Status == domain.StepFailed {
					t.Errorf("adapter %s step %q failed: %s", adapter.ID(), s.Action.ID, s.Error)
				}
			}
			t.Fatalf("Simultaneous: apply should succeed for adapter %s", adapter.ID())
		}
	}

	// ── Verify each adapter produced its own correct output ──────────────────

	// OpenCode: MergeIntoSettings → opencode.json with "mcp" key.
	ocJSON := filepath.Join(dir, "opencode.json")
	assertFileExists(t, ocJSON, "Simultaneous/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "Simultaneous/OpenCode: opencode.json has 'mcp' key")
	ocData, err := os.ReadFile(ocJSON)
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	if strings.Contains(string(ocData), `"mcpServers"`) {
		t.Error("Simultaneous/OpenCode: opencode.json must not have 'mcpServers' key")
	}

	// OpenCode: memory in AGENTS.md.
	assertFileExists(t, filepath.Join(dir, "AGENTS.md"), "Simultaneous/OpenCode: AGENTS.md")
	assertFileContains(t, filepath.Join(dir, "AGENTS.md"), "squadai", "Simultaneous/OpenCode: AGENTS.md has marker")

	// OpenCode: agents in .opencode/agents/.
	ocAgentsDir := filepath.Join(dir, ".opencode", "agents")
	assertFileExists(t, filepath.Join(ocAgentsDir, "orchestrator.md"), "Simultaneous/OpenCode: .opencode/agents/orchestrator.md")

	// Claude Code: MCPConfigFile → <project>/.mcp.json with "mcpServers".
	mcpFile := filepath.Join(dir, ".mcp.json")
	assertFileExists(t, mcpFile, "Simultaneous/Claude: .mcp.json")
	assertJSONKey(t, mcpFile, "mcpServers", "Simultaneous/Claude: .mcp.json has mcpServers key")

	// Claude Code: memory in CLAUDE.md.
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	assertFileExists(t, claudeMD, "Simultaneous/Claude: CLAUDE.md")
	assertFileContains(t, claudeMD, "squadai", "Simultaneous/Claude: CLAUDE.md has marker")

	// Cursor: MCPConfigFile → .cursor/mcp.json with "mcpServers".
	cursorMCP := filepath.Join(dir, ".cursor", "mcp.json")
	assertFileExists(t, cursorMCP, "Simultaneous/Cursor: .cursor/mcp.json")
	assertJSONKey(t, cursorMCP, "mcpServers", "Simultaneous/Cursor: .cursor/mcp.json has 'mcpServers'")

	// Cursor: agents in .cursor/agents/.
	cursorAgentsDir := filepath.Join(dir, ".cursor", "agents")
	assertFileExists(t, filepath.Join(cursorAgentsDir, "orchestrator.md"), "Simultaneous/Cursor: .cursor/agents/orchestrator.md")

	// ── Verify no cross-contamination ────────────────────────────────────────

	// opencode.json must NOT have "servers" or "mcpServers" keys.
	if strings.Contains(string(ocData), `"servers"`) {
		t.Error("Simultaneous: opencode.json must not have VS Code 'servers' key")
	}

	// .cursor/mcp.json must NOT have "mcp" or "servers" keys.
	cursorMCPData, err := os.ReadFile(cursorMCP)
	if err != nil {
		t.Fatalf("read .cursor/mcp.json: %v", err)
	}
	if strings.Contains(string(cursorMCPData), `"mcp":`) {
		t.Error("Simultaneous: .cursor/mcp.json must not have OpenCode 'mcp' key")
	}

	// ── Remove cleans up all 3 adapters ──────────────────────────────────────
	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	totalRemoved := len(removeReport.RemovedFiles) + len(removeReport.CleanedFiles)
	if totalRemoved == 0 {
		t.Error("Simultaneous: Remove should have removed or cleaned files for all 3 adapters")
	}

	// OpenCode agent files should be gone.
	if _, statErr := os.Stat(filepath.Join(ocAgentsDir, "orchestrator.md")); statErr == nil {
		t.Error("Simultaneous: .opencode/agents/orchestrator.md should be removed")
	}

	// Cursor agent files should be gone.
	if _, statErr := os.Stat(filepath.Join(cursorAgentsDir, "orchestrator.md")); statErr == nil {
		t.Error("Simultaneous: .cursor/agents/orchestrator.md should be removed")
	}

	// Original Node/React project files must survive.
	assertFileExists(t, filepath.Join(dir, "package.json"), "Simultaneous: package.json untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "App.tsx"), "Simultaneous: src/App.tsx untouched after remove")
}
