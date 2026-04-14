package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	// Add package-lock.json so the npm expectations are satisfied.
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
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

// ─── C/C++ project detection ───────────────────────────────────────────────

func TestDetectProjectMeta_Cpp_Detected(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.20)\nproject(myapp)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "C/C++" {
		t.Errorf("Language = %q, want %q", meta.Language, "C/C++")
	}
}

func TestDetectProjectMeta_Cpp_TestCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.20)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if !strings.Contains(meta.TestCommand, "ctest") {
		t.Errorf("TestCommand = %q, want it to contain %q", meta.TestCommand, "ctest")
	}
}

func TestDetectProjectMeta_Cpp_BuildCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.20)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if !strings.Contains(meta.BuildCommand, "cmake") {
		t.Errorf("BuildCommand = %q, want it to contain %q", meta.BuildCommand, "cmake")
	}
}

func TestDetectProjectMeta_Cpp_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// No CMakeLists.txt — C/C++ should not be detected.

	meta := DetectProjectMeta(dir)

	for _, l := range meta.Languages {
		if l == "C/C++" {
			t.Errorf("Languages %v should not contain 'C/C++' when CMakeLists.txt is absent", meta.Languages)
		}
	}
}

// ─── Dart project detection ────────────────────────────────────────────────

func TestDetectProjectMeta_Dart_Detected(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: my_dart_app\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Dart" {
		t.Errorf("Language = %q, want %q", meta.Language, "Dart")
	}
}

func TestDetectProjectMeta_Dart_TestCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: my_dart_app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// No lib/main.dart — pure Dart, not Flutter.

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "dart test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "dart test")
	}
}

func TestDetectProjectMeta_Dart_FlutterTest(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: my_flutter_app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create lib/main.dart to signal a Flutter project.
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "main.dart"), []byte("void main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "flutter test" {
		t.Errorf("TestCommand = %q, want %q (Flutter project)", meta.TestCommand, "flutter test")
	}
}

func TestDetectProjectMeta_Dart_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// No pubspec.yaml — Dart should not be detected.

	meta := DetectProjectMeta(dir)

	for _, l := range meta.Languages {
		if l == "Dart" {
			t.Errorf("Languages %v should not contain 'Dart' when pubspec.yaml is absent", meta.Languages)
		}
	}
}

// ─── Elixir project detection ──────────────────────────────────────────────

func TestDetectProjectMeta_Elixir_Detected(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "mix.exs"), []byte("defmodule MyApp.MixProject do\n  use Mix.Project\nend\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Elixir" {
		t.Errorf("Language = %q, want %q", meta.Language, "Elixir")
	}
}

func TestDetectProjectMeta_Elixir_TestCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "mix.exs"), []byte("defmodule MyApp.MixProject do\n  use Mix.Project\nend\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "mix test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "mix test")
	}
}

func TestDetectProjectMeta_Elixir_BuildCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "mix.exs"), []byte("defmodule MyApp.MixProject do\n  use Mix.Project\nend\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.BuildCommand != "mix compile" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "mix compile")
	}
}

func TestDetectProjectMeta_Elixir_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// No mix.exs — Elixir should not be detected.

	meta := DetectProjectMeta(dir)

	for _, l := range meta.Languages {
		if l == "Elixir" {
			t.Errorf("Languages %v should not contain 'Elixir' when mix.exs is absent", meta.Languages)
		}
	}
}

// ─── Scala project detection ───────────────────────────────────────────────

func TestDetectProjectMeta_Scala_Detected(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "build.sbt"), []byte("name := \"my-scala-app\"\nscalaVersion := \"3.3.1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.Language != "Scala" {
		t.Errorf("Language = %q, want %q", meta.Language, "Scala")
	}
}

func TestDetectProjectMeta_Scala_TestCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "build.sbt"), []byte("name := \"myapp\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "sbt test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "sbt test")
	}
}

func TestDetectProjectMeta_Scala_BuildCommand(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "build.sbt"), []byte("name := \"myapp\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.BuildCommand != "sbt compile" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "sbt compile")
	}
}

func TestDetectProjectMeta_Scala_NotDetected(t *testing.T) {
	dir := t.TempDir()
	// No build.sbt — Scala should not be detected.

	meta := DetectProjectMeta(dir)

	for _, l := range meta.Languages {
		if l == "Scala" {
			t.Errorf("Languages %v should not contain 'Scala' when build.sbt is absent", meta.Languages)
		}
	}
}

// ─── Package manager detection ─────────────────────────────────────────────

func TestDetectPackageManager_PnpmLock(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfileVersion: '6.0'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "pnpm" {
		t.Errorf("detectPackageManager = %q, want %q", got, "pnpm")
	}
}

func TestDetectPackageManager_BunLockb(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "bun" {
		t.Errorf("detectPackageManager = %q, want %q", got, "bun")
	}
}

func TestDetectPackageManager_YarnLock(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte("# yarn lockfile v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "yarn" {
		t.Errorf("detectPackageManager = %q, want %q", got, "yarn")
	}
}

func TestDetectPackageManager_PackageLockJson(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "npm" {
		t.Errorf("detectPackageManager = %q, want %q", got, "npm")
	}
}

func TestDetectPackageManager_NoLockFile_DefaultsPnpm(t *testing.T) {
	dir := t.TempDir()
	if got := detectPackageManager(dir); got != "pnpm" {
		t.Errorf("detectPackageManager = %q, want %q (default)", got, "pnpm")
	}
}

func TestDetectPackageManager_PriorityPnpmOverBun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "pnpm" {
		t.Errorf("detectPackageManager = %q, want %q (pnpm > bun)", got, "pnpm")
	}
}

func TestDetectPackageManager_PriorityPnpmOverYarn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "pnpm" {
		t.Errorf("detectPackageManager = %q, want %q (pnpm > yarn)", got, "pnpm")
	}
}

func TestDetectPackageManager_PriorityPnpmOverNpm(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "pnpm" {
		t.Errorf("detectPackageManager = %q, want %q (pnpm > npm)", got, "pnpm")
	}
}

func TestDetectPackageManager_PriorityBunOverYarn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "bun" {
		t.Errorf("detectPackageManager = %q, want %q (bun > yarn)", got, "bun")
	}
}

func TestDetectPackageManager_PriorityYarnOverNpm(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectPackageManager(dir); got != "yarn" {
		t.Errorf("detectPackageManager = %q, want %q (yarn > npm)", got, "yarn")
	}
}

// ─── Node integration tests with package manager ───────────────────────────

// writePackageJSON is a helper that writes a package.json with the given scripts.
func writePackageJSON(t *testing.T, dir string, scripts map[string]string) {
	t.Helper()
	pkg := map[string]interface{}{
		"name":    "test-project",
		"scripts": scripts,
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectNode_PnpmProject_Commands(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "vitest", "build": "tsc", "lint": "eslint ."})
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "pnpm test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "pnpm test")
	}
	if meta.BuildCommand != "pnpm run build" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "pnpm run build")
	}
	if meta.LintCommand != "pnpm run lint" {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "pnpm run lint")
	}
}

func TestDetectNode_BunProject_TestCmd(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "bun test"})
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "bun test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "bun test")
	}
}

func TestDetectNode_BunProject_BuildLintCmd(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "bun test", "build": "bun build", "lint": "eslint ."})
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.BuildCommand != "bun run build" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "bun run build")
	}
	if meta.LintCommand != "bun run lint" {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "bun run lint")
	}
}

func TestDetectNode_YarnProject_Commands(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "jest", "build": "webpack", "lint": "eslint ."})
	if err := os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "yarn test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "yarn test")
	}
	if meta.BuildCommand != "yarn run build" {
		t.Errorf("BuildCommand = %q, want %q", meta.BuildCommand, "yarn run build")
	}
	if meta.LintCommand != "yarn run lint" {
		t.Errorf("LintCommand = %q, want %q", meta.LintCommand, "yarn run lint")
	}
}

func TestDetectNode_NpmProject_Commands(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "jest", "build": "tsc", "lint": "eslint ."})
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

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

func TestDetectNode_NoLockFile_DefaultsPnpm(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "jest"})
	// No lock file — should default to pnpm.

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "pnpm test" {
		t.Errorf("TestCommand = %q, want %q (default pnpm)", meta.TestCommand, "pnpm test")
	}
}

func TestDetectNode_PackageManagerStoredInMeta(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "vitest"})
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q, want %q", meta.PackageManager, "pnpm")
	}
}

func TestDetectNode_PackageManager_EmptyForGo(t *testing.T) {
	dir := t.TempDir()
	goMod := "module github.com/example/go-only\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.PackageManager != "" {
		t.Errorf("PackageManager = %q, want empty for Go project", meta.PackageManager)
	}
}

func TestDetectNode_NoScripts_CommandsEmpty(t *testing.T) {
	dir := t.TempDir()
	// package.json with no scripts section.
	pkg := map[string]interface{}{"name": "no-scripts"}
	data, _ := json.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.TestCommand != "" {
		t.Errorf("TestCommand = %q, want empty (no scripts)", meta.TestCommand)
	}
	if meta.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty (no scripts)", meta.BuildCommand)
	}
	if meta.LintCommand != "" {
		t.Errorf("LintCommand = %q, want empty (no scripts)", meta.LintCommand)
	}
	// PackageManager should still be detected.
	if meta.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q, want %q even without scripts", meta.PackageManager, "pnpm")
	}
}

func TestDetectNode_Monorepo_RootLockFileWins(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, map[string]string{"test": "jest", "build": "tsc"})
	// Root has pnpm-lock.yaml — this is what wins.
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	// A nested package might have a yarn.lock, but we only check root.
	nested := filepath.Join(dir, "packages", "ui")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	meta := DetectProjectMeta(dir)

	if meta.PackageManager != "pnpm" {
		t.Errorf("PackageManager = %q, want %q (root lock file wins)", meta.PackageManager, "pnpm")
	}
	if meta.TestCommand != "pnpm test" {
		t.Errorf("TestCommand = %q, want %q", meta.TestCommand, "pnpm test")
	}
}
