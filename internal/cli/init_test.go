package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/config"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── buildSmartProjectConfig ───────────────────────────────────────────────

func TestBuildSmartProjectConfig_GoProject(t *testing.T) {
	meta := domain.ProjectMeta{
		Name:         "my-app",
		Language:     "Go",
		TestCommand:  "go test ./...",
		BuildCommand: "go build ./...",
	}

	adapters := []domain.Adapter{}
	proj := buildSmartProjectConfig(meta, adapters, "", nil, nil)

	if proj.Version != 1 {
		t.Errorf("Version = %d, want 1", proj.Version)
	}
	if proj.Meta.Name != "my-app" {
		t.Errorf("Meta.Name = %q, want %q", proj.Meta.Name, "my-app")
	}
	if !proj.Adapters[string(domain.AgentOpenCode)].Enabled {
		t.Error("OpenCode adapter should be enabled")
	}
	if proj.Components[string(domain.ComponentMemory)].Enabled != true {
		t.Error("Memory component should be enabled")
	}
	if proj.Skills["code-review"].Description == "" {
		t.Error("code-review skill should be defined")
	}
	if proj.Skills["testing"].ContentFile == "" {
		t.Error("testing skill should have content_file")
	}
}

func TestBuildSmartProjectConfig_DetectedAdapters(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}

	// Create mock adapters that look like detected ones.
	adapters := DetectAdapters(t.TempDir()) // Will at least include OpenCode

	proj := buildSmartProjectConfig(meta, adapters, "", nil, nil)

	// OpenCode should always be enabled.
	if !proj.Adapters[string(domain.AgentOpenCode)].Enabled {
		t.Error("OpenCode adapter should be enabled")
	}
}

func TestBuildSmartProjectConfig_WithMethodology_TDD(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyTDD, nil, nil)

	if proj.Methodology != domain.MethodologyTDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologyTDD)
	}
	if len(proj.Team) != 6 {
		t.Errorf("Team len = %d, want 6 for TDD", len(proj.Team))
	}
}

func TestBuildSmartProjectConfig_WithMethodology_SDD(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologySDD, nil, nil)

	if proj.Methodology != domain.MethodologySDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologySDD)
	}
	if len(proj.Team) != 8 {
		t.Errorf("Team len = %d, want 8 for SDD", len(proj.Team))
	}
}

func TestBuildSmartProjectConfig_WithMethodology_Conventional(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyConventional, nil, nil)

	if proj.Methodology != domain.MethodologyConventional {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologyConventional)
	}
	if len(proj.Team) != 4 {
		t.Errorf("Team len = %d, want 4 for Conventional", len(proj.Team))
	}
}

func TestBuildSmartProjectConfig_WithMethodology_Empty(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	if proj.Methodology != "" {
		t.Errorf("Methodology = %q, want empty", proj.Methodology)
	}
	if proj.Team != nil {
		t.Errorf("Team should be nil when no methodology, got %v", proj.Team)
	}
}

// ─── selectStandards ────────────────────────────────────────────────────────

func TestSelectStandards_Go(t *testing.T) {
	content := selectStandards("Go")
	if !strings.Contains(content, "Error Handling") {
		t.Error("Go standards should contain Error Handling section")
	}
	if !strings.Contains(content, "gofmt") {
		t.Error("Go standards should mention gofmt")
	}
}

func TestSelectStandards_TypeScript(t *testing.T) {
	content := selectStandards("TypeScript")
	if !strings.Contains(content, "TypeScript") {
		t.Error("JS standards should mention TypeScript")
	}
}

func TestSelectStandards_Python(t *testing.T) {
	content := selectStandards("Python")
	if !strings.Contains(content, "Type Hints") {
		t.Error("Python standards should contain Type Hints section")
	}
}

func TestSelectStandards_Unknown(t *testing.T) {
	content := selectStandards("Haskell")
	if !strings.Contains(content, "Code Quality") {
		t.Error("unknown language should get generic standards")
	}
}

func TestSelectStandards_Empty(t *testing.T) {
	content := selectStandards("")
	if !strings.Contains(content, "Code Quality") {
		t.Error("empty language should get generic standards")
	}
}

// ─── writeInitFile ──────────────────────────────────────────────────────────

func TestWriteInitFile_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.md")
	var buf bytes.Buffer

	writeInitFile(&buf, dir, path, "hello world", false)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", string(data), "hello world")
	}
	if !strings.Contains(buf.String(), "created") {
		t.Errorf("output should contain 'created', got %q", buf.String())
	}
}

func TestWriteInitFile_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	writeInitFile(&buf, dir, path, "new content", false)

	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Error("existing file should not be overwritten without --force")
	}
	if !strings.Contains(buf.String(), "exists") {
		t.Errorf("output should contain 'exists', got %q", buf.String())
	}
}

func TestWriteInitFile_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	writeInitFile(&buf, dir, path, "new content", true)

	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Errorf("content = %q, want %q", string(data), "new content")
	}
	if !strings.Contains(buf.String(), "overwritten") {
		t.Errorf("output should contain 'overwritten', got %q", buf.String())
	}
}

// ─── adapterSummary ─────────────────────────────────────────────────────────

func TestAdapterSummary_Empty(t *testing.T) {
	result := adapterSummary(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestAdapterSummary_SingleAdapter(t *testing.T) {
	home := t.TempDir()
	adapters := DetectAdapters(home) // at least opencode

	result := adapterSummary(adapters)
	if !strings.Contains(result, "opencode") {
		t.Errorf("expected 'opencode' in summary, got %q", result)
	}
}

// ─── Init writes team standards file ────────────────────────────────────────

func TestInit_WritesTeamStandards_GoProject(t *testing.T) {
	dir := t.TempDir()

	// Create a Go project.
	goMod := "module github.com/example/test-project\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)
	content := selectStandards(meta.Language)

	standardsPath := filepath.Join(dir, config.ProjectConfigDir, "templates", "team-standards.md")
	var buf bytes.Buffer
	writeInitFile(&buf, dir, standardsPath, content, false)

	data, err := os.ReadFile(standardsPath)
	if err != nil {
		t.Fatalf("standards file not created: %v", err)
	}
	if !strings.Contains(string(data), "Error Handling") {
		t.Error("Go project should get Go-specific standards")
	}
}

func TestInit_WritesSkillFiles(t *testing.T) {
	dir := t.TempDir()

	skillNames := []string{"code-review", "testing", "pr-description"}
	for _, name := range skillNames {
		path := filepath.Join(dir, config.ProjectConfigDir, "skills", name+".md")
		var buf bytes.Buffer
		writeInitFile(&buf, dir, path, "skill content for "+name, false)

		if _, err := os.Stat(path); err != nil {
			t.Errorf("skill file %s not created: %v", name, err)
		}
	}
}

// ─── Init help flags ─────────────────────────────────────────────────────────

func TestRunInit_HelpIncludesForce(t *testing.T) {
	var buf bytes.Buffer
	err := RunInit([]string{"--help"}, &buf)
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "--force") {
		t.Error("help output should mention --force flag")
	}
}

func TestRunInit_HelpIncludesMethodology(t *testing.T) {
	var buf bytes.Buffer
	err := RunInit([]string{"--help"}, &buf)
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "--methodology") {
		t.Error("help output should mention --methodology flag")
	}
}

// ─── RunInit --methodology flag ──────────────────────────────────────────────

func TestRunInit_MethodologyFlag_TDD(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=tdd"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	// Read back the written project.json.
	projPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	data, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}

	var proj domain.ProjectConfig
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("unmarshal project.json: %v", err)
	}

	if proj.Methodology != domain.MethodologyTDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologyTDD)
	}
	if len(proj.Team) != 6 {
		t.Errorf("Team len = %d, want 6 for TDD", len(proj.Team))
	}
}

func TestRunInit_MethodologyFlag_SDD(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=sdd"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	projPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	data, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}

	var proj domain.ProjectConfig
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("unmarshal project.json: %v", err)
	}

	if proj.Methodology != domain.MethodologySDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologySDD)
	}
	if len(proj.Team) != 8 {
		t.Errorf("Team len = %d, want 8 for SDD", len(proj.Team))
	}
}

func TestRunInit_MethodologyFlag_Unknown_Error(t *testing.T) {
	var buf bytes.Buffer
	err := RunInit([]string{"--methodology=invalid"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown methodology, got nil")
	}
	if !strings.Contains(err.Error(), "unknown methodology") {
		t.Errorf("error = %q, want to contain 'unknown methodology'", err.Error())
	}
}

func TestRunInit_MethodologySummaryPrinted(t *testing.T) {
	dir := t.TempDir()
	// Create a go.mod so we get project metadata (triggers the summary block).
	goMod := "module github.com/example/test\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=tdd"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Methodology: tdd") {
		t.Errorf("output should contain 'Methodology: tdd', got:\n%s", out)
	}
	if !strings.Contains(out, "Team roles:") {
		t.Errorf("output should contain 'Team roles:', got:\n%s", out)
	}
}

// ─── RunInit MCP integration ─────────────────────────────────────────────────

func TestRunInit_MCPWrittenToProjectJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=tdd"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	projPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	data, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}

	var proj domain.ProjectConfig
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("unmarshal project.json: %v", err)
	}

	if _, ok := proj.MCP["context7"]; !ok {
		t.Errorf("project.json MCP should contain 'context7', got: %v", proj.MCP)
	}
}

func TestRunInit_MCPContext7EnabledByDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=tdd"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	projPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	data, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}

	var proj domain.ProjectConfig
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("unmarshal project.json: %v", err)
	}

	c7, ok := proj.MCP["context7"]
	if !ok {
		t.Fatal("context7 not found in project.json MCP")
	}
	if !c7.Enabled {
		t.Error("context7 should be Enabled by default")
	}
}

func TestRunInit_MCPSummaryPrinted(t *testing.T) {
	dir := t.TempDir()
	// Create a go.mod so we get project metadata (triggers the summary block).
	goMod := "module github.com/example/test\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=tdd"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MCP servers: context7") {
		t.Errorf("output should contain 'MCP servers: context7', got:\n%s", out)
	}
}

// ─── buildSmartProjectConfig MCP ─────────────────────────────────────────────

func TestBuildSmartProjectConfig_HasDefaultMCP(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	if len(proj.MCP) == 0 {
		t.Error("buildSmartProjectConfig should always include default MCP servers")
	}
	if _, ok := proj.MCP["context7"]; !ok {
		t.Errorf("MCP should contain 'context7', got: %v", proj.MCP)
	}
}

func TestBuildSmartProjectConfig_MCPAndMethodology(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyTDD, nil, nil)

	if proj.Methodology != domain.MethodologyTDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologyTDD)
	}
	if len(proj.Team) == 0 {
		t.Error("Team should be populated for TDD methodology")
	}
	if _, ok := proj.MCP["context7"]; !ok {
		t.Errorf("MCP should contain 'context7' even with methodology set, got: %v", proj.MCP)
	}
}

// ─── Item 1.1: All 9 component keys ──────────────────────────────────────────

func TestBuildSmartProjectConfig_AllNineComponents_WithMethodology(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyTDD, nil, nil)

	wantComponents := []string{
		string(domain.ComponentMemory),
		"copilot",
		string(domain.ComponentRules),
		string(domain.ComponentSettings),
		string(domain.ComponentSkills),
		string(domain.ComponentWorkflows),
		string(domain.ComponentMCP),
		string(domain.ComponentAgents),
		string(domain.ComponentCommands),
	}
	for _, comp := range wantComponents {
		cfg, ok := proj.Components[comp]
		if !ok {
			t.Errorf("component %q missing from Components map", comp)
			continue
		}
		if !cfg.Enabled {
			t.Errorf("component %q should be enabled, got Enabled=false", comp)
		}
	}
}

func TestBuildSmartProjectConfig_ComponentsWithoutMethodology(t *testing.T) {
	// Without methodology: agents and commands should NOT be present.
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	// Always-on components must be present and enabled.
	alwaysOn := []string{
		string(domain.ComponentMemory),
		"copilot",
		string(domain.ComponentRules),
		string(domain.ComponentSettings),
		string(domain.ComponentSkills),
		string(domain.ComponentWorkflows),
		string(domain.ComponentMCP),
	}
	for _, comp := range alwaysOn {
		cfg, ok := proj.Components[comp]
		if !ok {
			t.Errorf("always-on component %q missing", comp)
			continue
		}
		if !cfg.Enabled {
			t.Errorf("always-on component %q should be enabled", comp)
		}
	}

	// Methodology-conditional components must NOT be present.
	for _, comp := range []string{string(domain.ComponentAgents), string(domain.ComponentCommands)} {
		if _, ok := proj.Components[comp]; ok {
			t.Errorf("component %q should not be enabled without a methodology", comp)
		}
	}
}

func TestBuildSmartProjectConfig_PluginsComponent_WhenPluginsSelected(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, []string{"code-review"})

	cfg, ok := proj.Components[string(domain.ComponentPlugins)]
	if !ok {
		t.Fatal("plugins component should be present when plugins are selected")
	}
	if !cfg.Enabled {
		t.Error("plugins component should be enabled when plugins are selected")
	}
	if len(proj.Plugins) == 0 {
		t.Error("proj.Plugins should be populated when plugins are selected")
	}
}

func TestBuildSmartProjectConfig_PluginsComponent_WhenNoPluginsSelected(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	if _, ok := proj.Components[string(domain.ComponentPlugins)]; ok {
		t.Error("plugins component should not be present when no plugins are selected")
	}
}

// ─── Item 1.2: All detected adapters enabled ─────────────────────────────────

func TestBuildSmartProjectConfig_AllDetectedAdaptersEnabled(t *testing.T) {
	// Use a home dir that won't find any personal adapters —
	// only OpenCode (always included) will be in the slice.
	home := t.TempDir()
	adapters := DetectAdapters(home)

	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, adapters, "", nil, nil)

	for _, a := range adapters {
		cfg, ok := proj.Adapters[string(a.ID())]
		if !ok {
			t.Errorf("adapter %q missing from project Adapters", a.ID())
			continue
		}
		if !cfg.Enabled {
			t.Errorf("adapter %q should be enabled, got Enabled=false", a.ID())
		}
	}
}

// ─── Item 1.3: --mcp flag ──────────────────────────────────────────────────

func TestRunInit_MCPFlag_FiltersToSelected(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=tdd", "--mcp=context7"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	projPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	data, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}

	var proj domain.ProjectConfig
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("unmarshal project.json: %v", err)
	}

	if _, ok := proj.MCP["context7"]; !ok {
		t.Errorf("project.json MCP should contain 'context7', got: %v", proj.MCP)
	}
	if proj.Components[string(domain.ComponentMCP)].Enabled != true {
		t.Error("mcp component should be enabled when --mcp flag provided")
	}
}

func TestBuildSmartProjectConfig_MCPFlag_FiltersMCP(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", []string{"context7"}, nil)

	if _, ok := proj.MCP["context7"]; !ok {
		t.Errorf("MCP should contain 'context7' when selected, got: %v", proj.MCP)
	}
	// Verify only the selected server is present.
	if len(proj.MCP) != 1 {
		t.Errorf("MCP should have exactly 1 entry when 1 selected, got: %d", len(proj.MCP))
	}
}

func TestBuildSmartProjectConfig_MCPFlag_EmptySelectionUsesDefaults(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	defaults := DefaultMCPServers()
	if len(proj.MCP) != len(defaults) {
		t.Errorf("empty mcp selection should use all defaults: got %d, want %d", len(proj.MCP), len(defaults))
	}
}

// ─── Item 1.3: --plugins flag ──────────────────────────────────────────────

func TestRunInit_PluginsFlag_PopulatesPlugins(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--methodology=sdd", "--plugins=code-review"}, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	projPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	data, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatalf("project.json not found: %v", err)
	}

	var proj domain.ProjectConfig
	if err := json.Unmarshal(data, &proj); err != nil {
		t.Fatalf("unmarshal project.json: %v", err)
	}

	if _, ok := proj.Plugins["code-review"]; !ok {
		t.Errorf("project.json Plugins should contain 'code-review', got: %v", proj.Plugins)
	}
	if proj.Components[string(domain.ComponentPlugins)].Enabled != true {
		t.Error("plugins component should be enabled when --plugins flag provided")
	}
}

func TestBuildSmartProjectConfig_PluginsFlag_SetsEnabled(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, []string{"code-review"})

	plugin, ok := proj.Plugins["code-review"]
	if !ok {
		t.Fatal("code-review plugin should be present")
	}
	if !plugin.Enabled {
		t.Error("selected plugin should have Enabled=true")
	}
}

func TestBuildSmartProjectConfig_PluginsFlag_UnknownPluginIgnored(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	// "nonexistent" is not in AvailablePlugins() — should be silently ignored.
	proj := buildSmartProjectConfig(meta, nil, "", nil, []string{"nonexistent"})

	if len(proj.Plugins) != 0 {
		t.Errorf("unknown plugin should not populate Plugins, got: %v", proj.Plugins)
	}
	if _, ok := proj.Components[string(domain.ComponentPlugins)]; ok {
		t.Error("plugins component should not be enabled for unknown plugin")
	}
}

// ─── find-skills skill file ───────────────────────────────────────────────────

func TestRunInit_WritesFindSkillsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit(nil, &buf); err != nil {
		t.Fatalf("RunInit error: %v", err)
	}

	skillPath := filepath.Join(dir, config.ProjectConfigDir, "skills", "find-skills.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf("find-skills.md skill file not created: %v", err)
	}
}

func TestBuildSmartProjectConfig_FindSkillsInSkillsMap(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	if _, ok := proj.Skills["find-skills"]; !ok {
		t.Error("find-skills should be in the Skills map")
	}
}

// ─── RunInit --json ───────────────────────────────────────────────────────────

func TestRunInit_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--json"}, &buf); err != nil {
		t.Fatalf("RunInit --json error: %v", err)
	}

	var result struct {
		ProjectDir    string          `json:"project_dir"`
		Methodology   string          `json:"methodology"`
		Adapters      []string        `json:"adapters"`
		Components    map[string]bool `json:"components"`
		SkillsWritten []string        `json:"skills_written"`
		MCPServers    []string        `json:"mcp_servers"`
		Plugins       []string        `json:"plugins"`
		PolicyCreated bool            `json:"policy_created"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.ProjectDir == "" {
		t.Error("project_dir field should not be empty")
	}
	if result.Adapters == nil {
		t.Error("adapters field should be an array (not null)")
	}
	if result.Components == nil {
		t.Error("components field should be an object (not null)")
	}
	if result.SkillsWritten == nil {
		t.Error("skills_written field should be an array (not null)")
	}
	if len(result.SkillsWritten) == 0 {
		t.Error("skills_written should contain at least one entry")
	}
	if result.MCPServers == nil {
		t.Error("mcp_servers field should be an array (not null)")
	}
	if result.Plugins == nil {
		t.Error("plugins field should be an array (not null)")
	}
}

func TestRunInit_JSONOutput_NoHumanText(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--json"}, &buf); err != nil {
		t.Fatalf("RunInit --json error: %v", err)
	}

	out := buf.String()
	// Human-readable output lines start with "  created", "  exists", or the
	// "Run 'agent-manager apply'" prompt — none of these should appear in JSON mode.
	for _, forbidden := range []string{"  created", "  exists", "  overwritten", "Run 'agent-manager"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("--json should suppress human-readable output %q, got:\n%s", forbidden, out)
		}
	}
	// Entire output must parse as a single JSON object.
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
}

func TestRunInit_JSONOutput_WithMethodology(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--json", "--methodology=tdd"}, &buf); err != nil {
		t.Fatalf("RunInit --json --methodology=tdd error: %v", err)
	}

	var result struct {
		Methodology string          `json:"methodology"`
		Components  map[string]bool `json:"components"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.Methodology != "tdd" {
		t.Errorf("methodology = %q, want %q", result.Methodology, "tdd")
	}
	if !result.Components[string(domain.ComponentAgents)] {
		t.Error("components[agents] should be true with --methodology=tdd")
	}
	if !result.Components[string(domain.ComponentCommands)] {
		t.Error("components[commands] should be true with --methodology=tdd")
	}
}

func TestRunInit_JSONOutput_PolicyCreatedFalseWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--json"}, &buf); err != nil {
		t.Fatalf("RunInit --json error: %v", err)
	}

	var result struct {
		PolicyCreated bool `json:"policy_created"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if result.PolicyCreated {
		t.Error("policy_created should be false when --with-policy is not set")
	}
}

func TestRunInit_JSONOutput_PolicyCreatedTrueWithFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--json", "--with-policy"}, &buf); err != nil {
		t.Fatalf("RunInit --json --with-policy error: %v", err)
	}

	var result struct {
		PolicyCreated bool `json:"policy_created"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if !result.PolicyCreated {
		t.Error("policy_created should be true when --with-policy is set")
	}
}

// ─── selectMultiStandards ────────────────────────────────────────────────────

func TestSelectMultiStandards_SingleLanguage_IdenticalToSelectStandards(t *testing.T) {
	single := selectMultiStandards([]string{"Go"})
	direct := selectStandards("Go")
	if single != direct {
		t.Error("selectMultiStandards with a single language should return the same content as selectStandards")
	}
}

func TestSelectMultiStandards_EmptyLanguages_ReturnsGeneric(t *testing.T) {
	content := selectMultiStandards([]string{})
	if !strings.Contains(content, "Code Quality") {
		t.Error("empty languages should return generic standards (contains 'Code Quality')")
	}
}

func TestSelectMultiStandards_TwoLanguages_ContainsBothSections(t *testing.T) {
	content := selectMultiStandards([]string{"Go", "Python"})

	if !strings.Contains(content, "## Go Standards") {
		t.Error("multi-standards for Go+Python should contain '## Go Standards' heading")
	}
	if !strings.Contains(content, "## Python Standards") {
		t.Error("multi-standards for Go+Python should contain '## Python Standards' heading")
	}
	if !strings.Contains(content, "---") {
		t.Error("multi-standards for Go+Python should contain a '---' separator between sections")
	}
	// Both standard bodies should be present too.
	if !strings.Contains(content, "gofmt") {
		t.Error("multi-standards should contain Go body (mentions 'gofmt')")
	}
	if !strings.Contains(content, "Type Hints") {
		t.Error("multi-standards should contain Python body (mentions 'Type Hints')")
	}
}

func TestSelectMultiStandards_ThreeLanguages_AllSectionsPresent(t *testing.T) {
	content := selectMultiStandards([]string{"Go", "TypeScript/JavaScript", "Rust"})

	for _, lang := range []string{"Go", "TypeScript/JavaScript", "Rust"} {
		heading := "## " + lang + " Standards"
		if !strings.Contains(content, heading) {
			t.Errorf("multi-standards should contain heading %q", heading)
		}
	}
}

// ─── Init writes multi-language standards for monorepos ───────────────────────

func TestInit_WritesMultiStandards_GoNodeMonorepo(t *testing.T) {
	dir := t.TempDir()

	// Create a Go+Node monorepo.
	goMod := "module github.com/example/mono\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	pkg := map[string]interface{}{
		"name":    "frontend",
		"scripts": map[string]string{"test": "jest"},
	}
	pkgData, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkgData, 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)
	if len(meta.Languages) < 2 {
		t.Fatalf("expected monorepo to detect ≥2 languages, got %v", meta.Languages)
	}

	standardsContent := selectMultiStandards(meta.Languages)

	standardsPath := filepath.Join(dir, config.ProjectConfigDir, "templates", "team-standards.md")
	var buf bytes.Buffer
	writeInitFile(&buf, dir, standardsPath, standardsContent, false)

	data, err := os.ReadFile(standardsPath)
	if err != nil {
		t.Fatalf("standards file not created: %v", err)
	}
	content := string(data)

	// Both language sections must appear in the combined file.
	if !strings.Contains(content, "## Go Standards") {
		t.Error("monorepo standards should contain '## Go Standards' section")
	}
	if !strings.Contains(content, "## TypeScript/JavaScript Standards") {
		t.Error("monorepo standards should contain '## TypeScript/JavaScript Standards' section")
	}
}
