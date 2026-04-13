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
	proj := buildSmartProjectConfig(meta, adapters, "")

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

	proj := buildSmartProjectConfig(meta, adapters, "")

	// OpenCode should always be enabled.
	if !proj.Adapters[string(domain.AgentOpenCode)].Enabled {
		t.Error("OpenCode adapter should be enabled")
	}
}

func TestBuildSmartProjectConfig_WithMethodology_TDD(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyTDD)

	if proj.Methodology != domain.MethodologyTDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologyTDD)
	}
	if len(proj.Team) != 6 {
		t.Errorf("Team len = %d, want 6 for TDD", len(proj.Team))
	}
}

func TestBuildSmartProjectConfig_WithMethodology_SDD(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologySDD)

	if proj.Methodology != domain.MethodologySDD {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologySDD)
	}
	if len(proj.Team) != 8 {
		t.Errorf("Team len = %d, want 8 for SDD", len(proj.Team))
	}
}

func TestBuildSmartProjectConfig_WithMethodology_Conventional(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyConventional)

	if proj.Methodology != domain.MethodologyConventional {
		t.Errorf("Methodology = %q, want %q", proj.Methodology, domain.MethodologyConventional)
	}
	if len(proj.Team) != 4 {
		t.Errorf("Team len = %d, want 4 for Conventional", len(proj.Team))
	}
}

func TestBuildSmartProjectConfig_WithMethodology_Empty(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "")

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
	content := selectStandards("Rust")
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
