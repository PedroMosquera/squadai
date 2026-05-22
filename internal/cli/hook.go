// Package cli — hook.go implements the internal `_hook` subcommand
// family. Hooks are invoked by the agent runtime (Claude Code,
// OpenCode, …) at lifecycle points such as session start. They are
// NOT for interactive use, hence the leading underscore and the
// deliberate omission from `--help`.
//
// Hook contract: every hook MUST exit 0, MUST be silent on its own
// errors (so a corrupt state file does not break a user's session),
// and MUST complete in well under 200 ms. Hook output is shown
// in-band to the user via the agent's stdout passthrough, so any
// emitted line should be a concise, actionable nudge.
//
// Currently implemented hooks:
//
//	squadai _hook squad-nudge   — session-start drift nudge
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/squadrefine"
)

var hookStdout io.Writer = os.Stdout
var hookGetwd = os.Getwd

func RunHookCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "squad-nudge":
		runSquadNudge()
		return nil
	default:
		return nil
	}
}

func RunHookCommandIn(args []string, projectDir string, w io.Writer) error {
	if len(args) == 0 {
		return nil
	}
	prevOut, prevWd := hookStdout, hookGetwd
	hookStdout = w
	hookGetwd = func() (string, error) { return projectDir, nil }
	defer func() {
		hookStdout = prevOut
		hookGetwd = prevWd
	}()
	return RunHookCommand(args)
}

func runSquadNudge() {
	if v := os.Getenv("SQUADAI_NO_NUDGE"); v != "" {
		return
	}

	projectDir, err := hookGetwd()
	if err != nil {
		return
	}

	state, existed, err := squadrefine.Load(projectDir)
	if err != nil {
		return
	}

	signals := sampleDriftSignals(projectDir)

	if existed && squadrefine.IsFresh(state, signals) {
		return
	}

	reasons := squadrefine.DriftReasons(state, signals)
	if len(reasons) == 0 {
		return
	}
	signature := strings.Join(reasons, ",")

	if !existed {
		printNudge(reasons)
		return
	}

	squadrefine.NoteNudgeFired(state, signature)
	if !state.Nudges.Throttled || state.Nudges.UnactionedCount == squadrefine.NudgeThrottleAt {
		printNudge(reasons)
	}

	_ = squadrefine.Save(projectDir, state)
}

func printNudge(reasons []string) {
	if len(reasons) == 1 && reasons[0] == "never-refined" {
		fmt.Fprintln(hookStdout,
			"💡 squadai: this squad hasn't been tuned for this repo yet. "+
				"Running /squadai-init will scan your codebase (~500–2000 tokens) "+
				"and propose tailored agent refinements. Skip with: ignore.")
		return
	}
	fmt.Fprintf(hookStdout,
		"💡 squadai: codebase has changed since /squadai-init was last run "+
			"(drift: %s). Re-run /squadai-init to refresh agent context "+
			"(~500–2000 tokens). Skip with: ignore.\n",
		strings.Join(reasons, ","))
}

func sampleDriftSignals(projectDir string) map[string]string {
	out := map[string]string{}

	if h, ok := hashTopLevelDirs(projectDir); ok {
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
		if h, ok := hashFileIfPresent(filepath.Join(projectDir, m.rel)); ok {
			out[m.name] = h
		}
	}
	return out
}

var driftIgnoredPrefixes = []string{
	".squadai",
	".git",
	".idea",
	".vscode",
	"node_modules",
	"vendor",
	"dist",
	"build",
	"target",
}

func hashTopLevelDirs(projectDir string) (string, bool) {
	var names []string
	root := filepath.Clean(projectDir)
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
		for _, ignored := range driftIgnoredPrefixes {
			if first == ignored {
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
	if err != nil {
		return "", false
	}
	if len(names) == 0 {
		return "", false
	}
	sort.Strings(names)
	return squadrefine.HashContent([]byte(strings.Join(names, "\n"))), true
}

func hashFileIfPresent(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return squadrefine.HashContent(data), true
}
