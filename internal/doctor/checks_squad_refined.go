package doctor

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/squadrefine"
)

// checkSquadRefined inspects the .squadai/.squad-refined state file and
// reports whether the squad refinement is fresh, stale, or has never been run.
//
// It is appended to the Project Configuration category checks so it benefits
// from the existing category display logic without requiring a new category.
func (d *Doctor) checkSquadRefined(_ context.Context) CheckResult {
	// Only meaningful when the project has been initialized (project.json exists).
	projectConfigPath := filepath.Join(d.projectDir, config.ProjectConfigDir, "project.json")
	if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
		return skip(catConfig, "squad-refined",
			"skipped — project not initialized (run 'squadai init' first)")
	}

	state, exists, err := squadrefine.Load(d.projectDir)
	if err != nil {
		return warn(catConfig, "squad-refined",
			"could not read .squad-refined: "+err.Error(), "", "")
	}

	if !exists {
		return warn(catConfig, "squad-refined",
			".squadai/.squad-refined absent — /squadai-init has never been run",
			"",
			"Run /squadai-init in your AI agent to tune the squad for this codebase")
	}

	signals := sampleDriftSignalsForDoctor(d.projectDir)
	reasons := squadrefine.DriftReasons(state, signals)

	if len(reasons) == 0 {
		return pass(catConfig, "squad-refined",
			".squad-refined is fresh (no drift since last /squadai-init)",
			"last_run_at="+state.LastRunAt)
	}

	return warn(catConfig, "squad-refined",
		".squad-refined is stale — drift detected: "+strings.Join(reasons, ", "),
		"reasons: "+strings.Join(reasons, ", "),
		"Re-run /squadai-init to refresh agent context")
}

// sampleDriftSignalsForDoctor samples the same signals as sampleDriftSignals
// in internal/cli/hook.go. Duplicated here to avoid a cross-package import
// cycle; the logic is intentionally identical and kept short (~10 lines).
func sampleDriftSignalsForDoctor(projectDir string) map[string]string {
	out := map[string]string{}
	if h, ok := hashTopLevelDirsDoctor(projectDir); ok {
		out["top_level_dirs"] = h
	}
	for _, m := range []struct {
		name string
		rel  string
	}{
		{"go.mod", "go.mod"},
		{"package.json", "package.json"},
		{"pyproject.toml", "pyproject.toml"},
	} {
		if h, ok := hashFileIfPresentDoctor(filepath.Join(projectDir, m.rel)); ok {
			out[m.name] = h
		}
	}
	return out
}

// hashTopLevelDirsDoctor is the doctor-local copy of hashTopLevelDirs from hook.go.
func hashTopLevelDirsDoctor(projectDir string) (string, bool) {
	var names []string
	root := filepath.Clean(projectDir)
	ignored := []string{
		".squadai", ".git", ".idea", ".vscode",
		"node_modules", "vendor", "dist", "build", "target",
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		first := strings.SplitN(rel, string(os.PathSeparator), 2)[0]
		for _, ig := range ignored {
			if first == ig {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		depth := len(strings.Split(rel, string(os.PathSeparator)))
		if depth > 2 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		names = append(names, rel)
		return nil
	})
	if err != nil || len(names) == 0 {
		return "", false
	}
	// Sort for determinism.
	sort.Strings(names)
	return squadrefine.HashContent([]byte(strings.Join(names, "\n"))), true
}

// hashFileIfPresentDoctor is the doctor-local copy of hashFileIfPresent from hook.go.
func hashFileIfPresentDoctor(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return squadrefine.HashContent(data), true
}
