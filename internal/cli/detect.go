package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// DetectProjectMeta inspects the project directory for common files and returns
// populated ProjectMeta. Detection is best-effort; missing files are silently
// skipped and the corresponding fields are left empty.
func DetectProjectMeta(projectDir string) domain.ProjectMeta {
	var meta domain.ProjectMeta

	// Try Go project first, then Node.
	if detectGo(projectDir, &meta) {
		// Go project detected.
	} else {
		detectNode(projectDir, &meta)
	}

	// Detect build/test/lint commands from Makefile or Taskfile.
	detectBuildSystem(projectDir, &meta)

	return meta
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

	// Extract commands from scripts.
	if cmd, ok := pkg.Scripts["test"]; ok && meta.TestCommand == "" {
		meta.TestCommand = "npm test"
		// If the test script is just "jest" or "vitest", keep npm test as wrapper.
		_ = cmd
	}
	if _, ok := pkg.Scripts["build"]; ok && meta.BuildCommand == "" {
		meta.BuildCommand = "npm run build"
	}
	if _, ok := pkg.Scripts["lint"]; ok && meta.LintCommand == "" {
		meta.LintCommand = "npm run lint"
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
