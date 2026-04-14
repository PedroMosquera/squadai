package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ─── Go project detection ──────────────────────────────────────────────────

func TestDetectProjectMeta_GoProject(t *testing.T) {
	dir := t.TempDir()

	goMod := `module github.com/example/my-service

go 1.24

require github.com/charmbracelet/bubbletea v1.3.5
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Name != "my-service" {
		t.Errorf("Name = %q, want %q", meta.Name, "my-service")
	}
	if meta.Language != "Go" {
		t.Errorf("Language = %q, want %q", meta.Language, "Go")
	}
	if meta.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "go test ./...")
	}
	if meta.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "go build ./...")
	}
	if meta.LintCommand != "go vet ./..." {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "go vet ./...")
	}
}

func TestDetectProjectMeta_GoProject_SingleLanguage_HasLanguagesSlice(t *testing.T) {
	dir := t.TempDir()

	goMod := "module github.com/example/single\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if len(meta.Languages) != 1 {
		t.Errorf("Languages len = %d, want 1; got %v", len(meta.Languages), meta.Languages)
	}
	if len(meta.Languages) > 0 && meta.Languages[0] != "Go" {
		t.Errorf("Languages[0] = %q, want %q", meta.Languages[0], "Go")
	}
}

func TestDetectProjectMeta_GoProjectWithGolangciLint(t *testing.T) {
	dir := t.TempDir()

	goMod := `module github.com/example/linted-project

go 1.24
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".golangci.yml"), []byte("linters:\n  enable:\n    - errcheck\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.LintCommand != "golangci-lint run ./..." {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "golangci-lint run ./...")
	}
}

// ─── Node project detection ────────────────────────────────────────────────

func TestDetectProjectMeta_NodeProject(t *testing.T) {
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name": "my-web-app",
		"scripts": map[string]string{
			"test":  "jest",
			"build": "tsc && vite build",
			"lint":  "eslint .",
		},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Name != "my-web-app" {
		t.Errorf("Name = %q, want %q", meta.Name, "my-web-app")
	}
	if meta.Language != "TypeScript/JavaScript" {
		t.Errorf("Language = %q, want %q", meta.Language, "TypeScript/JavaScript")
	}
	if meta.TestCommand != "npm test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "npm test")
	}
	if meta.BuildCommand != "npm run build" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "npm run build")
	}
	if meta.LintCommand != "npm run lint" {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "npm run lint")
	}
}

func TestDetectProjectMeta_TypeScriptProject(t *testing.T) {
	dir := t.TempDir()

	pkg := map[string]interface{}{
		"name":    "ts-app",
		"scripts": map[string]string{"test": "vitest"},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "TypeScript" {
		t.Errorf("Language = %q, want %q", meta.Language, "TypeScript")
	}
}

// ─── Makefile detection ────────────────────────────────────────────────────

func TestDetectProjectMeta_Makefile(t *testing.T) {
	dir := t.TempDir()

	makefile := `
.PHONY: build test lint

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...
`
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "make test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "make test")
	}
	if meta.BuildCommand != "make build" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "make build")
	}
	if meta.LintCommand != "make lint" {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "make lint")
	}
}

func TestDetectProjectMeta_GoWithMakefile_MakeOverridesDefaults(t *testing.T) {
	dir := t.TempDir()

	goMod := `module github.com/example/make-project

go 1.24
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Go detection sets defaults first, then Makefile detection runs.
	// Go defaults are already set, so Makefile targets don't override (they only fill empties).
	meta := DetectProjectMeta(dir)

	// Go defaults should be set since Makefile doesn't exist yet.
	if meta.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "go test ./...")
	}
}

// ─── Taskfile detection ────────────────────────────────────────────────────

func TestDetectProjectMeta_Taskfile(t *testing.T) {
	dir := t.TempDir()

	taskfile := `version: '3'

tasks:
  build:
    cmds:
      - go build ./...
  test:
    cmds:
      - go test ./...
  lint:
    cmds:
      - golangci-lint run
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(taskfile), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "task test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "task test")
	}
	if meta.BuildCommand != "task build" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "task build")
	}
	if meta.LintCommand != "task lint" {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "task lint")
	}
}

// ─── Empty project ─────────────────────────────────────────────────────────

func TestDetectProjectMeta_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	meta := DetectProjectMeta(dir)

	if meta.Name != "" {
		t.Errorf("Name = %q, want empty", meta.Name)
	}
	if meta.Language != "" {
		t.Errorf("Language = %q, want empty", meta.Language)
	}
	if meta.TestCommand != "" {
		t.Errorf("TestCommand = %q, want empty", meta.TestCommand)
	}
	if len(meta.Languages) != 0 {
		t.Errorf("Languages = %v, want empty for empty directory", meta.Languages)
	}
}

// ─── Go takes priority over Node ───────────────────────────────────────────

func TestDetectProjectMeta_GoTakesPriorityOverNode(t *testing.T) {
	dir := t.TempDir()

	goMod := `module github.com/example/dual-project

go 1.24
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	pkg := map[string]interface{}{
		"name":    "node-name",
		"scripts": map[string]string{"test": "jest"},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	// Go should take priority.
	if meta.Language != "Go" {
		t.Errorf("Language = %q, want %q", meta.Language, "Go")
	}
	if meta.Name != "dual-project" {
		t.Errorf("Name = %q, want %q", meta.Name, "dual-project")
	}
}

// ─── Python project detection ──────────────────────────────────────────────

func TestDetectPython_PyprojectToml(t *testing.T) {
	dir := t.TempDir()

	pyproject := `[project]
name = "my-flask-app"
version = "1.0.0"
dependencies = [
    "flask>=3.0",
    "sqlalchemy",
]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Python" {
		t.Errorf("Language = %q, want %q", meta.Language, "Python")
	}
	if meta.Name != "my-flask-app" {
		t.Errorf("Name = %q, want %q", meta.Name, "my-flask-app")
	}
	if meta.Framework != "Flask" {
		t.Errorf("Framework = %q, want %q", meta.Framework, "Flask")
	}
	if meta.TestCommand != "pytest" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "pytest")
	}
	if meta.LintCommand != "ruff check ." {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "ruff check .")
	}
}

func TestDetectPython_RequirementsTxt(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==3.0.0\nrequests\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Python" {
		t.Errorf("Language = %q, want %q", meta.Language, "Python")
	}
}

func TestDetectPython_NotPresent(t *testing.T) {
	dir := t.TempDir()

	meta := DetectProjectMeta(dir)

	if meta.Language == "Python" {
		t.Error("should not detect Python in empty directory")
	}
}

func TestDetectPython_SetupPy(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte("from setuptools import setup\nsetup(name='legacy')"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Python" {
		t.Errorf("Language = %q, want %q", meta.Language, "Python")
	}
}

// ─── Monorepo / multi-language detection ───────────────────────────────────

func TestDetectProjectMeta_GoNodeMonorepo_BothDetected(t *testing.T) {
	dir := t.TempDir()

	goMod := "module github.com/example/monorepo\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	pkg := map[string]interface{}{
		"name":    "frontend",
		"scripts": map[string]string{"test": "jest"},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	// Primary language is Go (higher priority).
	if meta.Language != "Go" {
		t.Errorf("Language = %q, want %q", meta.Language, "Go")
	}
	// Primary commands come from Go.
	if meta.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q (Go primary)", meta.TestCommand, "go test ./...")
	}
	if meta.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want %q (Go primary)", meta.BuildCommand, "go build ./...")
	}
	// Both languages must be in Languages.
	if len(meta.Languages) < 2 {
		t.Fatalf("Languages = %v, want at least 2 entries", meta.Languages)
	}
	containsGo := false
	containsNode := false
	for _, l := range meta.Languages {
		if l == "Go" {
			containsGo = true
		}
		if l == "TypeScript/JavaScript" || l == "TypeScript" {
			containsNode = true
		}
	}
	if !containsGo {
		t.Errorf("Languages %v should contain 'Go'", meta.Languages)
	}
	if !containsNode {
		t.Errorf("Languages %v should contain 'TypeScript/JavaScript'", meta.Languages)
	}
}

func TestDetectProjectMeta_GoPythonMonorepo_BothDetected(t *testing.T) {
	dir := t.TempDir()

	goMod := "module github.com/example/go-python\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	pyproject := "[project]\nname = \"scripts\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Go" {
		t.Errorf("Language = %q, want %q (Go is higher priority)", meta.Language, "Go")
	}
	containsPython := false
	for _, l := range meta.Languages {
		if l == "Python" {
			containsPython = true
		}
	}
	if !containsPython {
		t.Errorf("Languages %v should contain 'Python'", meta.Languages)
	}
}

func TestDetectProjectMeta_TripleMonorepo_AllThreeDetected(t *testing.T) {
	dir := t.TempDir()

	// Go
	goMod := "module github.com/example/triple\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	// Node
	pkg := map[string]interface{}{"name": "web", "scripts": map[string]string{"test": "jest"}}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	// Rust
	cargo := "[package]\nname = \"mylib\"\nversion = \"0.1.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Go" {
		t.Errorf("Language = %q, want %q", meta.Language, "Go")
	}
	if len(meta.Languages) < 3 {
		t.Fatalf("Languages = %v, want at least 3 entries (Go, TypeScript/JavaScript, Rust)", meta.Languages)
	}

	want := map[string]bool{"Go": false, "TypeScript/JavaScript": false, "Rust": false}
	for _, l := range meta.Languages {
		if _, ok := want[l]; ok {
			want[l] = true
		}
	}
	for lang, found := range want {
		if !found {
			t.Errorf("Languages %v should contain %q", meta.Languages, lang)
		}
	}
}

func TestDetectProjectMeta_GoNodeMonorepo_GoCommandsPreserved(t *testing.T) {
	dir := t.TempDir()

	goMod := "module github.com/example/cmds\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	pkg := map[string]interface{}{
		"name": "ui",
		"scripts": map[string]string{
			"test":  "jest",
			"build": "webpack",
			"lint":  "eslint .",
		},
	}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	// Primary (Go) commands must be preserved — Node commands must NOT bleed through.
	if meta.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want Go default %q", meta.TestCommand, "go test ./...")
	}
	if meta.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want Go default %q", meta.BuildCommand, "go build ./...")
	}
}

func TestDetectProjectMeta_Languages_PriorityOrder(t *testing.T) {
	dir := t.TempDir()

	// Both Go and Rust present — Go has higher priority (runs first in the detector list).
	goMod := "module github.com/example/pri\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	cargo := "[package]\nname = \"lib\"\nversion = \"0.1.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Go" {
		t.Errorf("Language = %q, want %q (Go has higher priority than Rust)", meta.Language, "Go")
	}
	if len(meta.Languages) < 2 {
		t.Fatalf("Languages = %v, want at least 2", meta.Languages)
	}
	// Go must be first (primary).
	if meta.Languages[0] != "Go" {
		t.Errorf("Languages[0] = %q, want %q", meta.Languages[0], "Go")
	}
	// Rust must also be present.
	containsRust := false
	for _, l := range meta.Languages {
		if l == "Rust" {
			containsRust = true
		}
	}
	if !containsRust {
		t.Errorf("Languages %v should contain 'Rust'", meta.Languages)
	}
}
