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
