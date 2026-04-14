package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// langDetector pairs a canonical language name with its detector function.
type langDetector struct {
	name string
	fn   func(string, *domain.ProjectMeta) bool
}

// DetectProjectMeta inspects the project directory for common files and returns
// populated ProjectMeta. All detectors run so that monorepos with multiple
// language ecosystems are fully captured.
//
// meta.Language is the primary language (highest-priority match). Its
// associated TestCommand/BuildCommand/LintCommand values are preserved.
// meta.Languages lists every detected language; it has at least one entry
// when any language is found.
func DetectProjectMeta(projectDir string) domain.ProjectMeta {
	// Priority order: Go, Node, Python, Rust, Java/Kotlin, Ruby, C#, PHP, Swift,
	// then C/C++, Dart, Elixir, Scala (lowest priority to preserve existing behaviour).
	detectors := []langDetector{
		{"Go", detectGo},
		{"TypeScript/JavaScript", detectNode},
		{"Python", detectPython},
		{"Rust", detectRust},
		{"Java", detectJava},
		{"Ruby", detectRuby},
		{"C#", detectCSharp},
		{"PHP", detectPHP},
		{"Swift", detectSwift},
		{"C/C++", detectCpp},
		{"Dart", detectDart},
		{"Elixir", detectElixir},
		{"Scala", detectScala},
	}

	var primaryMeta domain.ProjectMeta
	var languages []string
	primarySet := false

	for _, d := range detectors {
		// Each detector writes into a scratch copy so we can inspect what it
		// set for Language without permanently overwriting primary values.
		scratch := domain.ProjectMeta{}
		if d.fn(projectDir, &scratch) {
			// Use the language name the detector actually set (e.g. "TypeScript"
			// vs "TypeScript/JavaScript", "Kotlin" vs "Java").
			lang := scratch.Language
			if lang == "" {
				lang = d.name
			}
			languages = append(languages, lang)

			if !primarySet {
				// The first match wins: keep all its fields (Name, Language,
				// TestCommand, BuildCommand, LintCommand, Framework, etc.).
				primaryMeta = scratch
				primarySet = true
			} else {
				// Subsequent matches: only promote Name if primary didn't set one.
				if primaryMeta.Name == "" && scratch.Name != "" {
					primaryMeta.Name = scratch.Name
				}
			}
		}
	}

	// Attach the full language list.
	if len(languages) > 0 {
		primaryMeta.Languages = languages
	}

	// Detect build/test/lint commands from Makefile or Taskfile.
	// These override empty fields only, so the primary language's defaults win.
	detectBuildSystem(projectDir, &primaryMeta)

	return primaryMeta
}

// detectGo reads go.mod to extract module name and set language.
func detectGo(projectDir string, meta *domain.ProjectMeta) bool {
	goModPath := filepath.Join(projectDir, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return false
	}
	defer f.Close()

	meta.Language = "Go"

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimPrefix(line, "module ")
			modulePath = strings.TrimSpace(modulePath)
			// Use the last path segment as the project name.
			parts := strings.Split(modulePath, "/")
			if len(parts) > 0 {
				meta.Name = parts[len(parts)-1]
			}
			break
		}
	}

	// Default commands for Go projects if not already set.
	if meta.TestCommand == "" {
		meta.TestCommand = "go test ./..."
	}
	if meta.BuildCommand == "" {
		meta.BuildCommand = "go build ./..."
	}
	if meta.LintCommand == "" {
		// Check if golangci-lint config exists.
		for _, name := range []string{".golangci.yml", ".golangci.yaml", ".golangci.toml"} {
			if _, err := os.Stat(filepath.Join(projectDir, name)); err == nil {
				meta.LintCommand = "golangci-lint run ./..."
				break
			}
		}
		if meta.LintCommand == "" {
			meta.LintCommand = "go vet ./..."
		}
	}

	return true
}

// packageJSON is a minimal struct for reading package.json fields.
type packageJSON struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

// detectPackageManager inspects the project directory for lock files.
// Priority: pnpm-lock.yaml > bun.lockb > yarn.lock > package-lock.json
// Default: "pnpm" when no lock file found.
// Only the project root is checked; monorepos with mixed package managers are not supported.
func detectPackageManager(projectDir string) string {
	checks := []struct {
		lockFile string
		pm       string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"bun.lockb", "bun"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(projectDir, c.lockFile)); err == nil {
			return c.pm
		}
	}
	return "pnpm"
}

func testCmd(pm string) string {
	switch pm {
	case "bun":
		return "bun test"
	case "pnpm":
		return "pnpm test"
	case "yarn":
		return "yarn test"
	default:
		return "npm test"
	}
}

func buildCmd(pm string) string {
	switch pm {
	case "bun":
		return "bun run build"
	case "pnpm":
		return "pnpm run build"
	case "yarn":
		return "yarn run build"
	default:
		return "npm run build"
	}
}

func lintCmd(pm string) string {
	switch pm {
	case "bun":
		return "bun run lint"
	case "pnpm":
		return "pnpm run lint"
	case "yarn":
		return "yarn run lint"
	default:
		return "npm run lint"
	}
}

// detectNode reads package.json to extract name, language, and scripts.
func detectNode(projectDir string, meta *domain.ProjectMeta) bool {
	pkgPath := filepath.Join(projectDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	meta.Language = "TypeScript/JavaScript"
	if pkg.Name != "" {
		meta.Name = pkg.Name
	}

	// Check for TypeScript indicators.
	if _, err := os.Stat(filepath.Join(projectDir, "tsconfig.json")); err == nil {
		meta.Language = "TypeScript"
	}

	pm := detectPackageManager(projectDir)
	meta.PackageManager = pm

	// Extract commands from scripts using the detected package manager.
	if cmd, ok := pkg.Scripts["test"]; ok && meta.TestCommand == "" {
		meta.TestCommand = testCmd(pm)
		// If the test script is just "jest" or "vitest", keep pm test as wrapper.
		_ = cmd
	}
	if _, ok := pkg.Scripts["build"]; ok && meta.BuildCommand == "" {
		meta.BuildCommand = buildCmd(pm)
	}
	if _, ok := pkg.Scripts["lint"]; ok && meta.LintCommand == "" {
		meta.LintCommand = lintCmd(pm)
	}

	return true
}

// detectBuildSystem checks for Makefile or Taskfile.yml and extracts
// build/test/lint targets as commands. These override empty fields only.
func detectBuildSystem(projectDir string, meta *domain.ProjectMeta) {
	// Check Makefile first.
	if detectMakefileTargets(projectDir, meta) {
		return
	}
	// Then Taskfile.
	detectTaskfileTargets(projectDir, meta)
}

// detectMakefileTargets reads a Makefile to find test/build/lint targets.
func detectMakefileTargets(projectDir string, meta *domain.ProjectMeta) bool {
	makePath := filepath.Join(projectDir, "Makefile")
	f, err := os.Open(makePath)
	if err != nil {
		return false
	}
	defer f.Close()

	targets := scanMakeTargets(f)

	if _, ok := targets["test"]; ok && meta.TestCommand == "" {
		meta.TestCommand = "make test"
	}
	if _, ok := targets["build"]; ok && meta.BuildCommand == "" {
		meta.BuildCommand = "make build"
	}
	if _, ok := targets["lint"]; ok && meta.LintCommand == "" {
		meta.LintCommand = "make lint"
	}

	return len(targets) > 0
}

// scanMakeTargets extracts target names from a Makefile-style reader.
func scanMakeTargets(f *os.File) map[string]bool {
	targets := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Makefile targets are lines like "target:" at the start of a line.
		if len(line) == 0 || line[0] == '\t' || line[0] == '#' || line[0] == ' ' {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			target := strings.TrimSpace(line[:idx])
			// Skip variable assignments and multi-word targets.
			if !strings.ContainsAny(target, " \t=") && target != "" {
				targets[target] = true
			}
		}
	}
	return targets
}

// detectTaskfileTargets reads Taskfile.yml to find test/build/lint tasks.
func detectTaskfileTargets(projectDir string, meta *domain.ProjectMeta) bool {
	for _, name := range []string{"Taskfile.yml", "Taskfile.yaml"} {
		taskPath := filepath.Join(projectDir, name)
		f, err := os.Open(taskPath)
		if err != nil {
			continue
		}
		defer f.Close()

		tasks := scanTaskfileTasks(f)

		if _, ok := tasks["test"]; ok && meta.TestCommand == "" {
			meta.TestCommand = "task test"
		}
		if _, ok := tasks["build"]; ok && meta.BuildCommand == "" {
			meta.BuildCommand = "task build"
		}
		if _, ok := tasks["lint"]; ok && meta.LintCommand == "" {
			meta.LintCommand = "task lint"
		}

		return len(tasks) > 0
	}
	return false
}

// scanTaskfileTasks does a simple scan for task names in a Taskfile.
// It looks for lines matching "  taskname:" under a "tasks:" section.
func scanTaskfileTasks(f *os.File) map[string]bool {
	tasks := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	inTasks := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "tasks:" {
			inTasks = true
			continue
		}

		if inTasks {
			// A top-level key (no leading space) ends the tasks section.
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				inTasks = false
				continue
			}
			// Task entries are indented with 2 spaces: "  taskname:"
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
				if idx := strings.Index(trimmed, ":"); idx > 0 {
					taskName := strings.TrimSpace(trimmed[:idx])
					if taskName != "" {
						tasks[taskName] = true
					}
				}
			}
		}
	}
	return tasks
}

// detectPython checks for Python project indicators and populates meta.
func detectPython(projectDir string, meta *domain.ProjectMeta) bool {
	// Try pyproject.toml first.
	if detectPyprojectToml(projectDir, meta) {
		setPythonDefaults(projectDir, meta)
		return true
	}

	// Then setup.py.
	if _, err := os.Stat(filepath.Join(projectDir, "setup.py")); err == nil {
		meta.Language = "Python"
		setPythonDefaults(projectDir, meta)
		return true
	}

	// Then requirements.txt.
	if _, err := os.Stat(filepath.Join(projectDir, "requirements.txt")); err == nil {
		meta.Language = "Python"
		setPythonDefaults(projectDir, meta)
		return true
	}

	return false
}

// detectPyprojectToml reads pyproject.toml for project name and framework hints.
// Uses simple line scanning instead of a full TOML parser.
func detectPyprojectToml(projectDir string, meta *domain.ProjectMeta) bool {
	pyprojectPath := filepath.Join(projectDir, "pyproject.toml")
	f, err := os.Open(pyprojectPath)
	if err != nil {
		return false
	}
	defer f.Close()

	meta.Language = "Python"

	scanner := bufio.NewScanner(f)
	inProject := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Track [project] section.
		if line == "[project]" {
			inProject = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inProject = false
			continue
		}

		// Extract name from [project] section.
		if inProject && strings.HasPrefix(line, "name") {
			if idx := strings.Index(line, "="); idx >= 0 {
				val := strings.TrimSpace(line[idx+1:])
				val = strings.Trim(val, "\"'")
				if val != "" {
					meta.Name = val
				}
			}
		}

		// Detect frameworks from dependencies.
		lower := strings.ToLower(line)
		if strings.Contains(lower, "django") {
			meta.Framework = "Django"
		} else if strings.Contains(lower, "fastapi") {
			meta.Framework = "FastAPI"
		} else if strings.Contains(lower, "flask") {
			meta.Framework = "Flask"
		}
	}

	return true
}

// setPythonDefaults sets default commands for Python projects.
func setPythonDefaults(projectDir string, meta *domain.ProjectMeta) {
	if meta.TestCommand == "" {
		// Use pytest if tests/ dir exists or pytest is likely configured.
		if _, err := os.Stat(filepath.Join(projectDir, "tests")); err == nil {
			meta.TestCommand = "pytest"
		} else if _, err := os.Stat(filepath.Join(projectDir, "test")); err == nil {
			meta.TestCommand = "pytest"
		} else {
			meta.TestCommand = "python -m unittest discover"
		}
	}

	// Python typically doesn't have a build command.

	if meta.LintCommand == "" {
		// Check for ruff configuration.
		if _, err := os.Stat(filepath.Join(projectDir, "ruff.toml")); err == nil {
			meta.LintCommand = "ruff check ."
			return
		}
		// Check pyproject.toml for [tool.ruff] section.
		if pyprojectHasSection(projectDir, "[tool.ruff]") {
			meta.LintCommand = "ruff check ."
			return
		}
		// Default to ruff (most common modern Python linter).
		meta.LintCommand = "ruff check ."
	}
}

// pyprojectHasSection checks if pyproject.toml contains a specific section header.
func pyprojectHasSection(projectDir, section string) bool {
	f, err := os.Open(filepath.Join(projectDir, "pyproject.toml"))
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == section {
			return true
		}
	}
	return false
}

// detectRust checks for Cargo.toml and extracts project name.
func detectRust(projectDir string, meta *domain.ProjectMeta) bool {
	cargoPath := filepath.Join(projectDir, "Cargo.toml")
	f, err := os.Open(cargoPath)
	if err != nil {
		return false
	}
	defer f.Close()

	meta.Language = "Rust"

	scanner := bufio.NewScanner(f)
	inPackage := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[package]" {
			inPackage = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPackage = false
			continue
		}

		if inPackage && strings.HasPrefix(line, "name") {
			if idx := strings.Index(line, "="); idx >= 0 {
				val := strings.TrimSpace(line[idx+1:])
				val = strings.Trim(val, "\"'")
				if val != "" {
					meta.Name = val
				}
			}
		}
	}

	if meta.TestCommand == "" {
		meta.TestCommand = "cargo test"
	}
	if meta.BuildCommand == "" {
		meta.BuildCommand = "cargo build"
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "cargo clippy -- -D warnings"
	}

	return true
}

// detectJava checks for pom.xml, build.gradle, or build.gradle.kts.
func detectJava(projectDir string, meta *domain.ProjectMeta) bool {
	// Check Maven.
	if _, err := os.Stat(filepath.Join(projectDir, "pom.xml")); err == nil {
		meta.Language = "Java"
		if meta.TestCommand == "" {
			meta.TestCommand = "mvn test"
		}
		if meta.BuildCommand == "" {
			meta.BuildCommand = "mvn package"
		}
		if meta.LintCommand == "" {
			meta.LintCommand = "mvn checkstyle:check"
		}
		return true
	}

	// Check Gradle (Kotlin DSL first, then Groovy).
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		if _, err := os.Stat(filepath.Join(projectDir, name)); err == nil {
			meta.Language = "Java"
			if name == "build.gradle.kts" {
				// Kotlin DSL often indicates a Kotlin project.
				meta.Language = "Kotlin"
			}
			if meta.TestCommand == "" {
				meta.TestCommand = "./gradlew test"
			}
			if meta.BuildCommand == "" {
				meta.BuildCommand = "./gradlew build"
			}
			if meta.LintCommand == "" {
				meta.LintCommand = "./gradlew check"
			}
			return true
		}
	}

	return false
}

// detectRuby checks for Gemfile.
func detectRuby(projectDir string, meta *domain.ProjectMeta) bool {
	if _, err := os.Stat(filepath.Join(projectDir, "Gemfile")); err != nil {
		return false
	}

	meta.Language = "Ruby"

	if meta.TestCommand == "" {
		// Check for RSpec.
		if _, err := os.Stat(filepath.Join(projectDir, "spec")); err == nil {
			meta.TestCommand = "bundle exec rspec"
		} else {
			meta.TestCommand = "bundle exec rake test"
		}
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "bundle exec rubocop"
	}

	return true
}

// detectCSharp checks for *.csproj or *.sln files.
func detectCSharp(projectDir string, meta *domain.ProjectMeta) bool {
	// Check for .sln first (solution file).
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return false
	}

	found := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sln") || strings.HasSuffix(name, ".csproj") {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	meta.Language = "C#"

	if meta.TestCommand == "" {
		meta.TestCommand = "dotnet test"
	}
	if meta.BuildCommand == "" {
		meta.BuildCommand = "dotnet build"
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "dotnet format --verify-no-changes"
	}

	return true
}

// detectPHP checks for composer.json.
func detectPHP(projectDir string, meta *domain.ProjectMeta) bool {
	composerPath := filepath.Join(projectDir, "composer.json")
	data, err := os.ReadFile(composerPath)
	if err != nil {
		return false
	}

	meta.Language = "PHP"

	// Try to extract project name from composer.json.
	var composer struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &composer); err == nil && composer.Name != "" {
		// Composer names are vendor/package; use the package part.
		parts := strings.Split(composer.Name, "/")
		if len(parts) > 1 {
			meta.Name = parts[len(parts)-1]
		} else {
			meta.Name = composer.Name
		}
	}

	if meta.TestCommand == "" {
		meta.TestCommand = "./vendor/bin/phpunit"
	}
	if meta.BuildCommand == "" {
		// PHP typically doesn't have a build step.
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "./vendor/bin/phpcs"
	}

	return true
}

// detectCpp checks for CMakeLists.txt and populates meta for C/C++ projects.
func detectCpp(projectDir string, meta *domain.ProjectMeta) bool {
	if _, err := os.Stat(filepath.Join(projectDir, "CMakeLists.txt")); err != nil {
		return false
	}
	meta.Language = "C/C++"
	if meta.TestCommand == "" {
		meta.TestCommand = "cmake --build build && ctest --test-dir build"
	}
	if meta.BuildCommand == "" {
		meta.BuildCommand = "cmake -B build && cmake --build build"
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "clang-tidy"
	}
	return true
}

// detectDart checks for pubspec.yaml and distinguishes Flutter from pure Dart projects.
func detectDart(projectDir string, meta *domain.ProjectMeta) bool {
	if _, err := os.Stat(filepath.Join(projectDir, "pubspec.yaml")); err != nil {
		return false
	}
	meta.Language = "Dart"

	// Detect Flutter vs pure Dart via the presence of lib/main.dart.
	isFlutter := false
	if _, err := os.Stat(filepath.Join(projectDir, "lib", "main.dart")); err == nil {
		isFlutter = true
	}

	if meta.TestCommand == "" {
		if isFlutter {
			meta.TestCommand = "flutter test"
		} else {
			meta.TestCommand = "dart test"
		}
	}
	if meta.BuildCommand == "" {
		if isFlutter {
			meta.BuildCommand = "flutter build"
		} else {
			meta.BuildCommand = "dart compile exe"
		}
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "dart analyze"
	}
	return true
}

// detectElixir checks for mix.exs and populates meta for Elixir/OTP projects.
func detectElixir(projectDir string, meta *domain.ProjectMeta) bool {
	if _, err := os.Stat(filepath.Join(projectDir, "mix.exs")); err != nil {
		return false
	}
	meta.Language = "Elixir"
	if meta.TestCommand == "" {
		meta.TestCommand = "mix test"
	}
	if meta.BuildCommand == "" {
		meta.BuildCommand = "mix compile"
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "mix credo"
	}
	return true
}

// detectScala checks for build.sbt and populates meta for Scala projects.
func detectScala(projectDir string, meta *domain.ProjectMeta) bool {
	if _, err := os.Stat(filepath.Join(projectDir, "build.sbt")); err != nil {
		return false
	}
	meta.Language = "Scala"
	if meta.TestCommand == "" {
		meta.TestCommand = "sbt test"
	}
	if meta.BuildCommand == "" {
		meta.BuildCommand = "sbt compile"
	}
	if meta.LintCommand == "" {
		meta.LintCommand = "sbt scalafmtCheck"
	}
	return true
}

// detectSwift checks for Package.swift or *.xcodeproj.
func detectSwift(projectDir string, meta *domain.ProjectMeta) bool {
	// Check for Swift Package Manager.
	if _, err := os.Stat(filepath.Join(projectDir, "Package.swift")); err == nil {
		meta.Language = "Swift"
		if meta.TestCommand == "" {
			meta.TestCommand = "swift test"
		}
		if meta.BuildCommand == "" {
			meta.BuildCommand = "swift build"
		}
		if meta.LintCommand == "" {
			meta.LintCommand = "swiftlint"
		}
		return true
	}

	// Check for Xcode project.
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".xcodeproj") {
			meta.Language = "Swift"
			if meta.TestCommand == "" {
				meta.TestCommand = "xcodebuild test"
			}
			if meta.BuildCommand == "" {
				meta.BuildCommand = "xcodebuild build"
			}
			if meta.LintCommand == "" {
				meta.LintCommand = "swiftlint"
			}
			return true
		}
	}

	return false
}
