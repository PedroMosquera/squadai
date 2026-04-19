package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/verify"
)

// ─── Scaffold helpers ────────────────────────────────────────────────────────

// scaffoldPythonAPI creates a minimal Python/FastAPI project in a temp dir.
func scaffoldPythonAPI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "requirements.txt"), "fastapi>=0.100.0\nuvicorn>=0.23.0\npytest>=7.0.0\n")
	writeFile(t, filepath.Join(dir, "main.py"), "from fastapi import FastAPI\napp = FastAPI()\n")
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	writeFile(t, filepath.Join(dir, "tests", "__init__.py"), "")

	return dir
}

// scaffoldNodeReact creates a minimal Node/React/pnpm project in a temp dir.
func scaffoldNodeReact(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":           "react-frontend",
		"packageManager": "pnpm@8.0.0",
		"dependencies": map[string]string{
			"react":     "^18.0.0",
			"react-dom": "^18.0.0",
		},
		"scripts": map[string]string{
			"test":  "vitest",
			"build": "vite build",
		},
	}
	writePkgJSON(t, filepath.Join(dir, "package.json"), pkg)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: '6.0'\n")
	writeFile(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions":{"target":"ESNext","jsx":"react-jsx"}}`)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	writeFile(t, filepath.Join(dir, "src", "App.tsx"), "export default function App() { return <div>Hello</div>; }\n")

	return dir
}

// scaffoldNodeBackend creates a minimal Node/Express/npm project in a temp dir.
func scaffoldNodeBackend(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name": "express-backend",
		"dependencies": map[string]string{
			"express": "^4.18.0",
		},
		"scripts": map[string]string{
			"test":  "jest",
			"build": "tsc",
		},
	}
	writePkgJSON(t, filepath.Join(dir, "package.json"), pkg)
	writeFile(t, filepath.Join(dir, "package-lock.json"), `{"lockfileVersion":3,"name":"express-backend"}`)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	writeFile(t, filepath.Join(dir, "src", "index.js"), "const express = require('express');\nconst app = express();\n")

	return dir
}

// scaffoldJavaSpring creates a minimal Java/Maven/Spring Boot project in a temp dir.
func scaffoldJavaSpring(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	pom := `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>3.1.0</version>
  </parent>
  <groupId>com.example</groupId>
  <artifactId>demo</artifactId>
  <version>0.0.1-SNAPSHOT</version>
</project>
`
	writeFile(t, filepath.Join(dir, "pom.xml"), pom)
	javaDir := filepath.Join(dir, "src", "main", "java")
	if err := os.MkdirAll(javaDir, 0755); err != nil {
		t.Fatalf("mkdir java src: %v", err)
	}
	writeFile(t, filepath.Join(javaDir, "App.java"), "public class App { public static void main(String[] args) {} }\n")

	return dir
}

// ─── Scaffold utilities ───────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writePkgJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal package.json: %v", err)
	}
	writeFile(t, path, string(data))
}

// ─── Config builder for multi-stack tests ────────────────────────────────────

// buildConventionalConfigWithMeta builds a merged config using a pre-populated
// ProjectMeta (from DetectProjectMeta) rather than re-detecting inside the config helper.
func buildConventionalConfigWithMeta(t *testing.T, home, project string, meta domain.ProjectMeta) *domain.MergedConfig {
	t.Helper()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
			string(domain.ComponentSkills): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

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

// ─── Test 1: Python API ───────────────────────────────────────────────────────

// TestMultiStack_PythonAPI_StackDetection validates Python/FastAPI project detection.
func TestMultiStack_PythonAPI_StackDetection(t *testing.T) {
	t.Parallel()

	dir := scaffoldPythonAPI(t)
	meta := cli.DetectProjectMeta(dir)

	if meta.Language != "Python" {
		t.Errorf("Language = %q, want Python", meta.Language)
	}
	if meta.PackageManager != "" {
		t.Errorf("PackageManager = %q, want empty (no Node PM confusion for Python)", meta.PackageManager)
	}
	if meta.TestCommand != "pytest" {
		t.Errorf("TestCommand = %q, want pytest (tests/ dir exists)", meta.TestCommand)
	}
	// Must not detect Node.
	for _, lang := range meta.Languages {
		if strings.Contains(lang, "JavaScript") || strings.Contains(lang, "TypeScript") {
			t.Errorf("unexpected Node language detected: %q", lang)
		}
	}
}

// TestMultiStack_PythonAPI_ConfigFormat validates OpenCode MCP config uses "mcp" key.
func TestMultiStack_PythonAPI_ConfigFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldPythonAPI(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed for Python API project")
	}

	// OpenCode uses MergeIntoSettings strategy: "mcp" key in opencode.json.
	ocJSON := filepath.Join(dir, "opencode.json")
	assertFileExists(t, ocJSON, "Python/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "Python/OpenCode: has 'mcp' key")

	data, err := os.ReadFile(ocJSON)
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	if strings.Contains(string(data), `"mcpServers"`) {
		t.Error("Python/OpenCode: opencode.json must not have 'mcpServers' key (wrong MCP format)")
	}
}

// TestMultiStack_PythonAPI_TemplateRendering validates agent prompts mention Python.
func TestMultiStack_PythonAPI_TemplateRendering(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldPythonAPI(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// Conventional methodology + OpenCode → orchestrator.md under .opencode/agents/.
	// The template uses {{.Language}} which should resolve to "Python".
	orchestratorPath := filepath.Join(dir, ".opencode", "agents", "orchestrator.md")
	assertFileExists(t, orchestratorPath, "Python: orchestrator.md")
	assertFileContains(t, orchestratorPath, "Python", "Python: orchestrator.md mentions Python")

	// Verify it does NOT mention Node/TypeScript language names.
	data, err := os.ReadFile(orchestratorPath)
	if err != nil {
		t.Fatalf("read orchestrator.md: %v", err)
	}
	content := string(data)
	// The language reference should be "Python", not "TypeScript" or "Node".
	if strings.Contains(content, "- Language: TypeScript") || strings.Contains(content, "- Language: Node") {
		t.Error("Python: orchestrator.md incorrectly references TypeScript/Node language")
	}
}

// TestMultiStack_PythonAPI_Reversibility validates Remove cleans up all created files.
func TestMultiStack_PythonAPI_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldPythonAPI(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("apply should succeed before reversibility test")
	}

	// Verify key files exist before removal.
	assertFileExists(t, filepath.Join(dir, "AGENTS.md"), "Python: AGENTS.md exists before remove")
	assertFileExists(t, filepath.Join(dir, "opencode.json"), "Python: opencode.json exists before remove")

	// Remove all managed files.
	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	// After removal, managed agent files should be gone.
	agentsDir := filepath.Join(dir, ".opencode", "agents")
	if _, statErr := os.Stat(agentsDir); statErr == nil {
		// The dir may still exist but orchestrator.md should be gone.
		orchestratorPath := filepath.Join(agentsDir, "orchestrator.md")
		if _, statErr2 := os.Stat(orchestratorPath); statErr2 == nil {
			t.Error("Python: orchestrator.md should be removed after Remove")
		}
	}

	// Original project files (requirements.txt, main.py) must NOT be touched.
	assertFileExists(t, filepath.Join(dir, "requirements.txt"), "Python: requirements.txt untouched after remove")
	assertFileExists(t, filepath.Join(dir, "main.py"), "Python: main.py untouched after remove")
}

// ─── Test 2: Node React (pnpm) ────────────────────────────────────────────────

// TestMultiStack_NodeReact_StackDetection validates TypeScript/pnpm project detection.
func TestMultiStack_NodeReact_StackDetection(t *testing.T) {
	t.Parallel()

	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)

	// tsconfig.json triggers TypeScript detection.
	if meta.Language != "TypeScript" {
		t.Errorf("Language = %q, want TypeScript (tsconfig.json present)", meta.Language)
	}
	if meta.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q, want pnpm (pnpm-lock.yaml present)", meta.PackageManager)
	}
	if meta.TestCommand != "pnpm test" {
		t.Errorf("TestCommand = %q, want 'pnpm test'", meta.TestCommand)
	}
	if meta.BuildCommand != "pnpm run build" {
		t.Errorf("BuildCommand = %q, want 'pnpm run build'", meta.BuildCommand)
	}
	// Must not detect Python.
	for _, lang := range meta.Languages {
		if lang == "Python" {
			t.Errorf("unexpected Python language detected in Node/React project")
		}
	}
}

// TestMultiStack_NodeReact_MCPFormat validates MCP config format for OpenCode.
func TestMultiStack_NodeReact_MCPFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed for Node/React project")
	}

	ocJSON := filepath.Join(dir, "opencode.json")
	assertFileExists(t, ocJSON, "NodeReact/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "NodeReact/OpenCode: has 'mcp' key (MergeIntoSettings strategy)")

	// Orchestrator should mention TypeScript, not Python.
	orchestratorPath := filepath.Join(dir, ".opencode", "agents", "orchestrator.md")
	assertFileExists(t, orchestratorPath, "NodeReact: orchestrator.md")
	assertFileContains(t, orchestratorPath, "TypeScript", "NodeReact: orchestrator.md mentions TypeScript")
}

// TestMultiStack_NodeReact_Verify validates that verify passes after apply.
func TestMultiStack_NodeReact_Verify(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeReact(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, dir)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed && r.Severity != domain.SeverityWarning {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

// ─── Test 3: Node Backend (npm) ───────────────────────────────────────────────

// TestMultiStack_NodeBackend_StackDetection validates JS/npm project detection.
func TestMultiStack_NodeBackend_StackDetection(t *testing.T) {
	t.Parallel()

	dir := scaffoldNodeBackend(t)
	meta := cli.DetectProjectMeta(dir)

	// No tsconfig.json → TypeScript/JavaScript.
	if meta.Language != "TypeScript/JavaScript" {
		t.Errorf("Language = %q, want TypeScript/JavaScript (no tsconfig.json)", meta.Language)
	}
	if meta.PackageManager != "npm" {
		t.Errorf("PackageManager = %q, want npm (package-lock.json present)", meta.PackageManager)
	}
	if meta.TestCommand != "npm test" {
		t.Errorf("TestCommand = %q, want 'npm test'", meta.TestCommand)
	}
	// Must not detect Python or Java.
	for _, lang := range meta.Languages {
		if lang == "Python" || lang == "Java" {
			t.Errorf("unexpected %q language detected in Node Backend project", lang)
		}
	}
}

// TestMultiStack_NodeBackend_ConfigFormat validates config generation and cleanup.
func TestMultiStack_NodeBackend_ConfigFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeBackend(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed for Node Backend project")
	}

	ocJSON := filepath.Join(dir, "opencode.json")
	assertFileExists(t, ocJSON, "NodeBackend/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "NodeBackend/OpenCode: has 'mcp' key")

	// AGENTS.md must have squadai memory marker.
	assertFileExists(t, filepath.Join(dir, "AGENTS.md"), "NodeBackend: AGENTS.md")
	assertFileContains(t, filepath.Join(dir, "AGENTS.md"), "squadai", "NodeBackend: AGENTS.md has marker")
}

// TestMultiStack_NodeBackend_Reversibility validates Remove restores pre-init state.
func TestMultiStack_NodeBackend_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldNodeBackend(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("apply should succeed before reversibility test")
	}

	// Verify key files exist.
	assertFileExists(t, filepath.Join(dir, "AGENTS.md"), "NodeBackend: AGENTS.md exists before remove")

	// Remove.
	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	// Files removed or cleaned.
	totalRemoved := len(removeReport.RemovedFiles) + len(removeReport.CleanedFiles)
	if totalRemoved == 0 {
		t.Error("NodeBackend: Remove should have removed or cleaned at least one file")
	}

	// Original project files must survive.
	assertFileExists(t, filepath.Join(dir, "package.json"), "NodeBackend: package.json untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "index.js"), "NodeBackend: src/index.js untouched after remove")
}

// ─── Test 4: Java Spring (Maven) ─────────────────────────────────────────────

// TestMultiStack_JavaSpring_StackDetection validates Java/Maven project detection.
func TestMultiStack_JavaSpring_StackDetection(t *testing.T) {
	t.Parallel()

	dir := scaffoldJavaSpring(t)
	meta := cli.DetectProjectMeta(dir)

	if meta.Language != "Java" {
		t.Errorf("Language = %q, want Java", meta.Language)
	}
	if meta.TestCommand != "mvn test" {
		t.Errorf("TestCommand = %q, want 'mvn test'", meta.TestCommand)
	}
	if meta.BuildCommand != "mvn package" {
		t.Errorf("BuildCommand = %q, want 'mvn package'", meta.BuildCommand)
	}
	// Must not have Node package manager.
	if meta.PackageManager != "" {
		t.Errorf("PackageManager = %q, want empty (no Node in Java project)", meta.PackageManager)
	}
	// Must not detect Node ecosystem.
	for _, lang := range meta.Languages {
		if strings.Contains(lang, "JavaScript") || strings.Contains(lang, "TypeScript") {
			t.Errorf("unexpected Node language %q detected in Java/Spring project", lang)
		}
	}
}

// TestMultiStack_JavaSpring_ConfigFormat validates Java project config and no Node leakage.
func TestMultiStack_JavaSpring_ConfigFormat(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldJavaSpring(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed for Java/Spring project")
	}

	// opencode.json must exist with "mcp" key, no "mcpServers".
	ocJSON := filepath.Join(dir, "opencode.json")
	assertFileExists(t, ocJSON, "Java/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "Java/OpenCode: has 'mcp' key")

	data, err := os.ReadFile(ocJSON)
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	jsonStr := string(data)
	if strings.Contains(jsonStr, `"mcpServers"`) {
		t.Error("Java/OpenCode: opencode.json must not have 'mcpServers' (wrong MCP format)")
	}
	// No npm/pnpm/yarn leakage in config.
	if strings.Contains(jsonStr, `"packageManager"`) {
		t.Error("Java/OpenCode: opencode.json must not reference packageManager")
	}
}

// TestMultiStack_JavaSpring_TemplateRendering validates orchestrator.md mentions Java.
func TestMultiStack_JavaSpring_TemplateRendering(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldJavaSpring(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	orchestratorPath := filepath.Join(dir, ".opencode", "agents", "orchestrator.md")
	assertFileExists(t, orchestratorPath, "Java: orchestrator.md")
	assertFileContains(t, orchestratorPath, "Java", "Java: orchestrator.md mentions Java")

	// Must not reference Node/Python languages.
	data, err := os.ReadFile(orchestratorPath)
	if err != nil {
		t.Fatalf("read orchestrator.md: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "- Language: Python") || strings.Contains(content, "- Language: TypeScript") {
		t.Error("Java: orchestrator.md incorrectly references Python/TypeScript language")
	}
}

// TestMultiStack_JavaSpring_Reversibility validates Remove cleans up all created files.
func TestMultiStack_JavaSpring_Reversibility(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := scaffoldJavaSpring(t)
	meta := cli.DetectProjectMeta(dir)
	merged := buildConventionalConfigWithMeta(t, home, dir, meta)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, dir)
	if !report.Success {
		t.Fatal("apply should succeed before reversibility test")
	}

	// Confirm squadai-managed files exist.
	assertFileExists(t, filepath.Join(dir, "AGENTS.md"), "Java: AGENTS.md before remove")
	assertFileExists(t, filepath.Join(dir, "opencode.json"), "Java: opencode.json before remove")

	// Remove.
	removeReport, err := cli.Remove(cli.RemoveOptions{ProjectDir: dir})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(removeReport.Errors) > 0 {
		t.Errorf("Remove reported errors: %v", removeReport.Errors)
	}

	// Managed agent files should be gone.
	agentsDir := filepath.Join(dir, ".opencode", "agents")
	if _, statErr := os.Stat(agentsDir); statErr == nil {
		orchestratorPath := filepath.Join(agentsDir, "orchestrator.md")
		if _, statErr2 := os.Stat(orchestratorPath); statErr2 == nil {
			t.Error("Java: orchestrator.md should be removed after Remove")
		}
	}

	// Original project files untouched.
	assertFileExists(t, filepath.Join(dir, "pom.xml"), "Java: pom.xml untouched after remove")
	assertFileExists(t, filepath.Join(dir, "src", "main", "java", "App.java"), "Java: App.java untouched after remove")
}

// ─── Test 5: Global scope ─────────────────────────────────────────────────────

// TestMultiStack_Global_PythonAPI validates --global scope: config written to homeDir.
// Simulates --global by using homeDir as both home and project (what RunInit does with --global).
func TestMultiStack_Global_PythonAPI(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	// In --global mode, projectDir == homeDir.
	globalDir := home

	// Scaffold Python files in a separate location for detection.
	projectScaffold := scaffoldPythonAPI(t)
	meta := cli.DetectProjectMeta(projectScaffold)

	// Build config pointing at the globalDir as project (simulating --global).
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot: domain.CopilotConfig{InstructionsTemplate: "standard"},
		MCP:     cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(globalDir), projCfg); err != nil {
		t.Fatalf("write project config to globalDir: %v", err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(globalDir)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := opencode.New()
	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, globalDir)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("expected at least one action for global scope")
	}

	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), globalDir, merged.Copilot, nil)
	execReport, execErr := exec.Execute(actions)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if !execReport.Success {
		for _, s := range execReport.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("global apply should succeed")
	}

	// AGENTS.md and opencode.json should be in globalDir (== home).
	assertFileExists(t, filepath.Join(globalDir, "AGENTS.md"), "Global: AGENTS.md in homeDir")
	assertFileExists(t, filepath.Join(globalDir, "opencode.json"), "Global: opencode.json in homeDir")

	// MCP format must still be correct.
	assertJSONKey(t, filepath.Join(globalDir, "opencode.json"), "mcp", "Global: opencode.json has 'mcp' key")

	// Verify passes for global scope.
	v := verify.New()
	vReport, verifyErr := v.Verify(merged, []domain.Adapter{adapter}, home, globalDir)
	if verifyErr != nil {
		t.Fatalf("verify: %v", verifyErr)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed && r.Severity != domain.SeverityWarning {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

// ─── Test 6: Cross-stack language isolation ───────────────────────────────────

// TestMultiStack_LanguageIsolation verifies each stack produces the correct
// language in detection with no cross-contamination between stack types.
func TestMultiStack_LanguageIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scaffold func(*testing.T) string
		wantLang string
		wantNoPM bool   // true = PackageManager should be empty
		wantPM   string // expected PackageManager when not empty
	}{
		{
			name:     "PythonAPI",
			scaffold: scaffoldPythonAPI,
			wantLang: "Python",
			wantNoPM: true,
		},
		{
			name:     "NodeReact",
			scaffold: scaffoldNodeReact,
			wantLang: "TypeScript",
			wantPM:   "pnpm",
		},
		{
			name:     "NodeBackend",
			scaffold: scaffoldNodeBackend,
			wantLang: "TypeScript/JavaScript",
			wantPM:   "npm",
		},
		{
			name:     "JavaSpring",
			scaffold: scaffoldJavaSpring,
			wantLang: "Java",
			wantNoPM: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := tc.scaffold(t)
			meta := cli.DetectProjectMeta(dir)

			if meta.Language != tc.wantLang {
				t.Errorf("Language = %q, want %q", meta.Language, tc.wantLang)
			}
			if tc.wantNoPM && meta.PackageManager != "" {
				t.Errorf("PackageManager = %q, want empty", meta.PackageManager)
			}
			if tc.wantPM != "" && meta.PackageManager != tc.wantPM {
				t.Errorf("PackageManager = %q, want %q", meta.PackageManager, tc.wantPM)
			}
		})
	}
}
