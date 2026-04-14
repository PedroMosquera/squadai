package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
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
	// "Run 'squadai apply'" prompt — none of these should appear in JSON mode.
	for _, forbidden := range []string{"  created", "  exists", "  overwritten", "Run 'squadai"} {
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

// ─── P4-B: init --merge ───────────────────────────────────────────────────────

// TestRunInit_Merge_MutuallyExclusiveWithForce verifies that --merge and --force
// together return an error containing "mutually exclusive".
func TestRunInit_Merge_MutuallyExclusiveWithForce(t *testing.T) {
	var buf bytes.Buffer
	err := RunInit([]string{"--merge", "--force"}, &buf)
	if err == nil {
		t.Fatal("expected error when --merge and --force are combined, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want to contain 'mutually exclusive'", err.Error())
	}
}

// TestRunInit_HelpIncludesMerge verifies that --merge appears in help output.
func TestRunInit_HelpIncludesMerge(t *testing.T) {
	var buf bytes.Buffer
	if err := RunInit([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "--merge") {
		t.Error("help output should mention --merge flag")
	}
}

// TestRunInit_Merge_PreservesExistingConfig verifies that --merge preserves
// user-added skills that are not in the default set.
func TestRunInit_Merge_PreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create a project.json with a custom skill.
	initial := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Skills: map[string]domain.SkillDef{
			"my-custom-skill": {
				Description: "A custom user skill",
				ContentFile: "skills/my-custom.md",
			},
		},
		MCP: map[string]domain.MCPServerDef{},
	}
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, initial); err != nil {
		t.Fatalf("write initial project config: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--merge"}, &buf); err != nil {
		t.Fatalf("RunInit --merge error: %v", err)
	}

	proj, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("load project after merge: %v", err)
	}

	if _, ok := proj.Skills["my-custom-skill"]; !ok {
		t.Error("--merge should preserve user-added skill 'my-custom-skill'")
	}
}

// TestRunInit_Merge_AddsNewAdapters verifies that --merge adds newly-detected
// adapters (e.g., cursor) without removing existing ones.
func TestRunInit_Merge_AddsNewAdapters(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Start with a project.json that only has opencode.
	initial := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{},
	}
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, initial); err != nil {
		t.Fatalf("write initial project config: %v", err)
	}

	// Create a fake .cursor/ directory inside the temp home so cursor is detected.
	// Since RunInit uses os.UserHomeDir(), we can't inject a custom home.
	// Instead, simulate by directly testing the merge logic with a fake fresh config
	// that includes cursor.
	freshWithCursor := buildSmartProjectConfig(
		domain.ProjectMeta{Language: "Go"},
		nil, // no adapters from detection (temp dir won't find cursor)
		"",
		nil,
		nil,
	)
	// Add cursor to the fresh config to simulate detection.
	freshWithCursor.Adapters["cursor"] = domain.AdapterConfig{Enabled: true}

	merged := mergeProjectConfigs(initial, freshWithCursor, false)

	// Original opencode adapter must still be present.
	if _, ok := merged.Adapters["opencode"]; !ok {
		t.Error("merged config should preserve existing opencode adapter")
	}
	// Newly-detected cursor adapter should be added.
	if _, ok := merged.Adapters["cursor"]; !ok {
		t.Error("merged config should add newly-detected cursor adapter")
	}
}

// TestRunInit_Merge_PreservesMethodology verifies that without --methodology,
// --merge preserves the existing methodology.
func TestRunInit_Merge_PreservesMethodology(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Start with tdd methodology.
	initial := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
		MCP:         map[string]domain.MCPServerDef{},
	}
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, initial); err != nil {
		t.Fatalf("write initial project config: %v", err)
	}

	var buf bytes.Buffer
	// Run --merge without --methodology flag.
	if err := RunInit([]string{"--merge"}, &buf); err != nil {
		t.Fatalf("RunInit --merge error: %v", err)
	}

	proj, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("load project after merge: %v", err)
	}

	if proj.Methodology != domain.MethodologyTDD {
		t.Errorf("Methodology = %q, want %q (should be preserved)", proj.Methodology, domain.MethodologyTDD)
	}
}

// TestRunInit_Merge_OverridesMethodologyWhenExplicit verifies that when
// --methodology is explicitly given, --merge overwrites the existing methodology.
func TestRunInit_Merge_OverridesMethodologyWhenExplicit(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Start with tdd methodology.
	initial := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
		MCP:         map[string]domain.MCPServerDef{},
	}
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, initial); err != nil {
		t.Fatalf("write initial project config: %v", err)
	}

	var buf bytes.Buffer
	// Run --merge --methodology=sdd — explicit flag should override.
	if err := RunInit([]string{"--merge", "--methodology=sdd"}, &buf); err != nil {
		t.Fatalf("RunInit --merge --methodology=sdd error: %v", err)
	}

	proj, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("load project after merge: %v", err)
	}

	if proj.Methodology != domain.MethodologySDD {
		t.Errorf("Methodology = %q, want %q (should be overridden by explicit flag)", proj.Methodology, domain.MethodologySDD)
	}
}

// TestRunInit_Merge_UpdatesMeta verifies that --merge re-detects and updates Meta.
func TestRunInit_Merge_UpdatesMeta(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Start with a project.json that has Language="python".
	initial := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Meta: domain.ProjectMeta{Language: "python"},
		MCP:  map[string]domain.MCPServerDef{},
	}
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, initial); err != nil {
		t.Fatalf("write initial project config: %v", err)
	}

	// Create a go.mod to trigger Go language detection.
	goMod := "module github.com/example/test\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--merge"}, &buf); err != nil {
		t.Fatalf("RunInit --merge error: %v", err)
	}

	proj, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("load project after merge: %v", err)
	}

	if proj.Meta.Language != "Go" {
		t.Errorf("Meta.Language = %q, want %q (should be updated by merge)", proj.Meta.Language, "Go")
	}
}

// TestRunInit_Merge_NoExistingConfig verifies that --merge behaves like normal
// init when no project.json exists (creates a new config).
func TestRunInit_Merge_NoExistingConfig(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--merge"}, &buf); err != nil {
		t.Fatalf("RunInit --merge with no existing config should not error: %v", err)
	}

	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		t.Error("--merge with no existing config should create project.json")
	}

	proj, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("load project after merge init: %v", err)
	}
	if proj.Version != 1 {
		t.Errorf("Version = %d, want 1", proj.Version)
	}
}

// TestRunInit_Merge_OutputSaysMerged verifies that the human output says "merged"
// when --merge is used and config already exists.
func TestRunInit_Merge_OutputSaysMerged(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create initial project.json.
	initial := domain.DefaultProjectConfig()
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, initial); err != nil {
		t.Fatalf("write initial project config: %v", err)
	}

	var buf bytes.Buffer
	if err := RunInit([]string{"--merge"}, &buf); err != nil {
		t.Fatalf("RunInit --merge error: %v", err)
	}

	if !strings.Contains(buf.String(), "merged") {
		t.Errorf("output should contain 'merged', got:\n%s", buf.String())
	}
}

// ─── TestMergeProjectConfigs (unit tests for the merge function) ──────────────

// TestMergeProjectConfigs_AdditiveAdapterMerge verifies new adapters are added
// and existing adapters are not overwritten.
func TestMergeProjectConfigs_AdditiveAdapterMerge(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{},
	}
	fresh := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: false}, // existing value should be kept
			"cursor":   {Enabled: true},  // new adapter should be added
		},
		Components: map[string]domain.ComponentConfig{},
	}

	result := mergeProjectConfigs(existing, fresh, false)

	if !result.Adapters["opencode"].Enabled {
		t.Error("existing opencode adapter Enabled=true should be preserved")
	}
	if _, ok := result.Adapters["cursor"]; !ok {
		t.Error("new cursor adapter from fresh should be added")
	}
}

// TestMergeProjectConfigs_ExistingSkillsPreserved verifies that user-added skills
// are not removed.
func TestMergeProjectConfigs_ExistingSkillsPreserved(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{},
		Adapters:   map[string]domain.AdapterConfig{},
		Skills: map[string]domain.SkillDef{
			"my-skill": {Description: "My custom skill"},
		},
	}
	fresh := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{},
		Adapters:   map[string]domain.AdapterConfig{},
		Skills: map[string]domain.SkillDef{
			"code-review": {Description: "Default code review skill"},
		},
	}

	result := mergeProjectConfigs(existing, fresh, false)

	if _, ok := result.Skills["my-skill"]; !ok {
		t.Error("user skill 'my-skill' should be preserved after merge")
	}
	if _, ok := result.Skills["code-review"]; !ok {
		t.Error("default skill 'code-review' from fresh should be added")
	}
}

// TestMergeProjectConfigs_MethodologyPreservation verifies that when
// methodologyExplicit=false, the existing methodology is kept.
func TestMergeProjectConfigs_MethodologyPreservation(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{},
		Adapters:    map[string]domain.AdapterConfig{},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
	}
	fresh := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{},
		Adapters:    map[string]domain.AdapterConfig{},
		Methodology: domain.MethodologySDD,
		Team:        domain.DefaultTeam(domain.MethodologySDD),
	}

	result := mergeProjectConfigs(existing, fresh, false /* not explicit */)

	if result.Methodology != domain.MethodologyTDD {
		t.Errorf("Methodology = %q, want %q (should be preserved when not explicit)", result.Methodology, domain.MethodologyTDD)
	}
	// Team should also be TDD (6 roles), not SDD (8 roles).
	if len(result.Team) != 6 {
		t.Errorf("Team len = %d, want 6 (TDD team, preserved)", len(result.Team))
	}
}

// TestMergeProjectConfigs_MethodologyOverride verifies that when
// methodologyExplicit=true, the methodology from fresh overwrites existing.
func TestMergeProjectConfigs_MethodologyOverride(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{},
		Adapters:    map[string]domain.AdapterConfig{},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
	}
	fresh := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{},
		Adapters:    map[string]domain.AdapterConfig{},
		Methodology: domain.MethodologySDD,
		Team:        domain.DefaultTeam(domain.MethodologySDD),
	}

	result := mergeProjectConfigs(existing, fresh, true /* explicit */)

	if result.Methodology != domain.MethodologySDD {
		t.Errorf("Methodology = %q, want %q (should be overwritten by explicit flag)", result.Methodology, domain.MethodologySDD)
	}
	if len(result.Team) != 8 {
		t.Errorf("Team len = %d, want 8 (SDD team, from fresh)", len(result.Team))
	}
}

// TestMergeProjectConfigs_MapIsolation verifies that modifying the result's maps
// does not affect the original existing config (no shared map references).
func TestMergeProjectConfigs_MapIsolation(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Skills: map[string]domain.SkillDef{
			"skill-a": {Description: "Skill A"},
		},
		MCP: map[string]domain.MCPServerDef{
			"context7": {Enabled: true},
		},
		Plugins: map[string]domain.PluginDef{
			"plugin-a": {Description: "Plugin A"},
		},
	}
	fresh := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
	}

	result := mergeProjectConfigs(existing, fresh, false)

	// Add a new entry to result's maps.
	result.Adapters["new-adapter"] = domain.AdapterConfig{Enabled: true}
	result.Components["new-comp"] = domain.ComponentConfig{Enabled: true}
	result.Skills["new-skill"] = domain.SkillDef{Description: "New"}
	result.MCP["new-mcp"] = domain.MCPServerDef{Enabled: true}
	result.Plugins["new-plugin"] = domain.PluginDef{Description: "New"}

	// Existing maps must be unchanged.
	if _, ok := existing.Adapters["new-adapter"]; ok {
		t.Error("modifying result.Adapters should not affect existing.Adapters")
	}
	if _, ok := existing.Components["new-comp"]; ok {
		t.Error("modifying result.Components should not affect existing.Components")
	}
	if _, ok := existing.Skills["new-skill"]; ok {
		t.Error("modifying result.Skills should not affect existing.Skills")
	}
	if _, ok := existing.MCP["new-mcp"]; ok {
		t.Error("modifying result.MCP should not affect existing.MCP")
	}
	if _, ok := existing.Plugins["new-plugin"]; ok {
		t.Error("modifying result.Plugins should not affect existing.Plugins")
	}
}

// TestMergeProjectConfigs_MetaAlwaysUpdated verifies Version and Meta come from fresh.
func TestMergeProjectConfigs_MetaAlwaysUpdated(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Meta:       domain.ProjectMeta{Language: "python", Name: "old-project"},
	}
	fresh := &domain.ProjectConfig{
		Version:    2,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Meta:       domain.ProjectMeta{Language: "Go", Name: "new-project"},
	}

	result := mergeProjectConfigs(existing, fresh, false)

	if result.Version != 2 {
		t.Errorf("Version = %d, want 2 (always from fresh)", result.Version)
	}
	if result.Meta.Language != "Go" {
		t.Errorf("Meta.Language = %q, want %q (always from fresh)", result.Meta.Language, "Go")
	}
	if result.Meta.Name != "new-project" {
		t.Errorf("Meta.Name = %q, want %q (always from fresh)", result.Meta.Name, "new-project")
	}
}

// TestMergeProjectConfigs_CopilotPreserved verifies Copilot config is always
// preserved from existing.
func TestMergeProjectConfigs_CopilotPreserved(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "custom",
			CustomContent:        "# My custom instructions",
		},
	}
	fresh := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}

	result := mergeProjectConfigs(existing, fresh, false)

	if result.Copilot.InstructionsTemplate != "custom" {
		t.Errorf("Copilot.InstructionsTemplate = %q, want %q (should be preserved)", result.Copilot.InstructionsTemplate, "custom")
	}
	if result.Copilot.CustomContent != "# My custom instructions" {
		t.Errorf("Copilot.CustomContent = %q (should be preserved)", result.Copilot.CustomContent)
	}
}

// TestMergeProjectConfigs_MCPAdditive verifies that new MCP servers are added
// without removing existing user-configured servers.
func TestMergeProjectConfigs_MCPAdditive(t *testing.T) {
	existing := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		MCP: map[string]domain.MCPServerDef{
			"my-custom-server": {Type: "http", Enabled: true},
			"context7":         {Type: "stdio", Enabled: false}, // user disabled it
		},
	}
	fresh := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		MCP: map[string]domain.MCPServerDef{
			"context7":   {Type: "stdio", Enabled: true}, // default: enabled
			"new-server": {Type: "http", Enabled: true},  // new default server
		},
	}

	result := mergeProjectConfigs(existing, fresh, false)

	// Existing custom server must be preserved.
	if _, ok := result.MCP["my-custom-server"]; !ok {
		t.Error("user-added 'my-custom-server' should be preserved")
	}
	// Existing user-disabled context7 must stay disabled (not overwritten by fresh).
	if result.MCP["context7"].Enabled {
		t.Error("user-disabled 'context7' should remain disabled (not overwritten by fresh)")
	}
	// New server from fresh must be added.
	if _, ok := result.MCP["new-server"]; !ok {
		t.Error("new 'new-server' from fresh should be added")
	}
}
