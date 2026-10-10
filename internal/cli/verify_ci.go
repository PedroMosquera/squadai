package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/verify"
)

// runVerifyCI checks that the committed project files match what apply would
// write for every adapter the project config enables, whether or not that
// agent is installed. It plans against an empty scratch home, so user config,
// user-level files and the agents on PATH cannot change the result: the same
// checkout gives the same answer on a laptop and on a bare runner. Targets
// outside projectDir and targets the project gitignores are not compared,
// because a checkout can never contain them.
func runVerifyCI(stdout io.Writer, projectDir string, jsonOut bool) error {
	bareHome, err := os.MkdirTemp("", "squadai-ci-home-")
	if err != nil {
		return fmt.Errorf("create scratch home: %w", err)
	}
	defer os.RemoveAll(bareHome)

	merged, err := loadAndMerge(bareHome, projectDir)
	if err != nil {
		return err
	}
	applyDefaultProfile(merged)

	// Every adapter goes to the planner: enabled ones are planned, and disabled
	// ones still get their stale squadai content flagged for cleanup.
	p := planner.New()
	actions, err := p.Plan(merged, allAdapters(), bareHome, projectDir)
	if err != nil {
		return exitcode.ErrPlanFailed(err)
	}

	pending, inSync := projectPending(actions, projectDir)
	ignored := gitIgnored(projectDir, pending)

	report := &domain.VerifyReport{AllPass: true}
	var drifted []domain.PlannedAction
	var skipped []string
	for _, a := range pending {
		rel := filepath.ToSlash(relPath(projectDir, a.TargetPath))
		if ignored[rel] {
			skipped = append(skipped, rel)
			continue
		}
		drifted = append(drifted, a)
		report.AllPass = false
		report.Results = append(report.Results, domain.VerifyResult{
			Check:     ciCheckName(a.Action),
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: string(a.Component),
			Path:      rel,
			Message:   ciMessage(a, rel),
		})
	}
	if len(drifted) == 0 {
		report.Results = append(report.Results, domain.VerifyResult{
			Check:     "project-content",
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "ci",
			Message:   fmt.Sprintf("%d managed project file(s) match what apply would write", inSync),
		})
	}
	if len(skipped) > 0 {
		report.Results = append(report.Results, domain.VerifyResult{
			Check:     "project-content-gitignored",
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "ci",
			Message:   fmt.Sprintf("%d file(s) apply would write are gitignored and were not checked, e.g. %s", len(skipped), skipped[0]),
		})
	}
	report.Results = append(report.Results, verify.PolicyResults(merged)...)

	if jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal verify report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		if len(report.Results) > 5 {
			printGroupedResults(stdout, report.Results)
		} else {
			for _, r := range report.Results {
				printVerifyResult(stdout, r)
			}
		}
		printVerifySummary(stdout, report.Results)
		printCIDiffs(stdout, p, drifted, bareHome, projectDir)
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			printGitHubAnnotations(stdout, report.Results)
		}
	}

	if !report.AllPass {
		return exitcode.New(exitcode.Drift, "E-401",
			fmt.Sprintf("%d project file(s) differ from what apply would write", len(drifted)),
			"Run 'squadai apply' on a machine with every configured agent and commit the result.")
	}
	return nil
}

// projectPending returns the non-skip actions that target files inside
// projectDir, one per file, and how many project files are already in sync.
// Several components and adapters can plan the same file (AGENTS.md is shared
// by OpenCode, Pi and Codex), so the file is reported once.
func projectPending(actions []domain.PlannedAction, projectDir string) ([]domain.PlannedAction, int) {
	seen := make(map[string]bool)
	pendingPaths := make(map[string]bool)
	var pending []domain.PlannedAction
	for _, a := range actions {
		if a.TargetPath == "" || !insideDir(projectDir, a.TargetPath) || a.Action == domain.ActionSkip {
			continue
		}
		pendingPaths[a.TargetPath] = true
		if seen[a.TargetPath] {
			continue
		}
		seen[a.TargetPath] = true
		pending = append(pending, a)
	}
	inSync := make(map[string]bool)
	for _, a := range actions {
		if a.Action == domain.ActionSkip && a.TargetPath != "" && insideDir(projectDir, a.TargetPath) && !pendingPaths[a.TargetPath] {
			inSync[a.TargetPath] = true
		}
	}
	return pending, len(inSync)
}

func insideDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// gitIgnored returns the project-relative paths among actions that git would
// ignore. Outside a git work tree, or without git, nothing is treated as
// ignored, so the check stays strict rather than silently skipping files.
func gitIgnored(projectDir string, actions []domain.PlannedAction) map[string]bool {
	ignored := make(map[string]bool)
	if len(actions) == 0 {
		return ignored
	}
	var in bytes.Buffer
	for _, a := range actions {
		in.WriteString(filepath.ToSlash(relPath(projectDir, a.TargetPath)))
		in.WriteByte(0)
	}
	cmd := exec.Command("git", "check-ignore", "-z", "--stdin")
	cmd.Dir = projectDir
	cmd.Stdin = &in
	// check-ignore exits 1 when nothing matches and 128 on errors; either way
	// the output is empty, which is the answer we want.
	out, _ := cmd.Output()
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			ignored[p] = true
		}
	}
	return ignored
}

func ciCheckName(action domain.ActionType) string {
	switch action {
	case domain.ActionCreate:
		return "project-file-missing"
	case domain.ActionDelete:
		return "project-file-stale"
	default:
		return "project-file-drift"
	}
}

func ciMessage(a domain.PlannedAction, rel string) string {
	owner := string(a.Component)
	if a.Agent != "" {
		owner += "/" + string(a.Agent)
	}
	switch a.Action {
	case domain.ActionCreate:
		return fmt.Sprintf("%s is missing; apply would create it (%s)", rel, owner)
	case domain.ActionDelete:
		return fmt.Sprintf("%s is stale; apply would remove it (%s)", rel, owner)
	default:
		return fmt.Sprintf("%s differs from what apply would write (%s)", rel, owner)
	}
}

// printCIDiffs prints the change apply would make to each drifted file that
// already exists, so the CI log shows the hand edit without a second command.
func printCIDiffs(stdout io.Writer, p *planner.Planner, drifted []domain.PlannedAction, homeDir, projectDir string) {
	for _, a := range drifted {
		if a.Action != domain.ActionUpdate {
			continue
		}
		old, newContent, err := p.RenderAction(a, homeDir, projectDir)
		if err != nil {
			continue
		}
		if diff := fileutil.UnifiedDiff(filepath.ToSlash(relPath(projectDir, a.TargetPath)), string(old), string(newContent)); diff != "" {
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, diff)
		}
	}
}

// printGitHubAnnotations emits workflow commands so failures show inline on
// the pull request. Paths are made relative to GITHUB_WORKSPACE when the
// project is a subdirectory of the checkout.
func printGitHubAnnotations(stdout io.Writer, results []domain.VerifyResult) {
	workspace := os.Getenv("GITHUB_WORKSPACE")
	cwd, _ := os.Getwd()
	for _, r := range results {
		if r.Passed || r.Severity != domain.SeverityError {
			continue
		}
		path := r.Path
		if workspace != "" && cwd != "" && path != "" {
			if rel, err := filepath.Rel(workspace, filepath.Join(cwd, path)); err == nil && !strings.HasPrefix(rel, "..") {
				path = filepath.ToSlash(rel)
			}
		}
		if path == "" {
			fmt.Fprintf(stdout, "::error title=squadai verify::%s\n", escapeAnnotationData(r.Message))
			continue
		}
		fmt.Fprintf(stdout, "::error file=%s,title=squadai verify::%s\n", escapeAnnotationProperty(path), escapeAnnotationData(r.Message))
	}
}

func escapeAnnotationData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func escapeAnnotationProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}
