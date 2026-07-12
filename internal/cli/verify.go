package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/verify"
)

// RunVerify runs compliance checks and prints the report.
func RunVerify(args []string, stdout io.Writer) error {
	jsonOut := false
	strict := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--strict":
			strict = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai verify [--json] [--strict]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Run compliance and health checks against the current project configuration.")
			fmt.Fprintln(stdout, "Verifies that all enabled components are correctly installed for each detected")
			fmt.Fprintln(stdout, "agent: expected files exist, marker blocks are present, and settings are valid.")
			fmt.Fprintln(stdout, "Exits non-zero if any check fails.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Each check is reported as PASS, FAIL, or WARN. Warnings do not cause a non-zero")
			fmt.Fprintln(stdout, "exit. Results are grouped by component when there are more than 5 checks.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json    Output the full verification report as JSON.")
			fmt.Fprintln(stdout, "  --strict  Also run a drift check; fail if any managed file has drifted.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai verify")
			fmt.Fprintln(stdout, "  squadai verify --json")
			fmt.Fprintln(stdout, "  squadai verify --strict")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}
	applyDefaultProfile(merged)

	adapters := DetectAdapters(homeDir)
	v := verify.New()
	report, err := v.Verify(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal verify report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		if !report.AllPass {
			return exitcode.New(exitcode.Config, "E-301",
				"verification failed",
				"Run 'squadai diff' to see what's out of sync, then 'squadai apply' to fix.")
		}
		return nil
	}

	if len(report.Results) == 0 {
		fmt.Fprintln(stdout, "No checks to run (no components or adapters enabled).")
		return nil
	}

	// Group results by component if there are enough.
	if len(report.Results) > 5 {
		printGroupedResults(stdout, report.Results)
	} else {
		for _, r := range report.Results {
			printVerifyResult(stdout, r)
		}
	}

	// Print summary line.
	printVerifySummary(stdout, report.Results)

	if !report.AllPass {
		return exitcode.New(exitcode.Config, "E-301",
			"verification failed",
			"Run 'squadai diff' to see what's out of sync, then 'squadai apply' to fix.")
	}

	if strict {
		if err := runStrictDriftCheck(projectDir, stdout, jsonOut); err != nil {
			return err
		}
	}

	return nil
}

// runStrictDriftCheck runs governance.CheckDrift and fails if any files have drifted.
func runStrictDriftCheck(projectDir string, stdout io.Writer, jsonOut bool) error {
	results, err := governance.CheckDrift(projectDir)
	if err != nil {
		return fmt.Errorf("strict drift check: %w", err)
	}

	var drifted []governance.DriftResult
	for _, r := range results {
		if r.Drifted() {
			drifted = append(drifted, r)
		}
	}

	if len(drifted) == 0 {
		if !jsonOut {
			fmt.Fprintln(stdout, "  [PASS] drift check — no managed files have drifted")
		}
		return nil
	}

	if jsonOut {
		type driftEntry struct {
			Path   string `json:"path"`
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		}
		out := make([]driftEntry, len(drifted))
		for i, r := range drifted {
			out[i] = driftEntry{Path: r.Path, Kind: string(r.Kind), Detail: r.Detail}
		}
		data, _ := json.MarshalIndent(map[string]any{"drift": out}, "", "  ")
		fmt.Fprintln(stdout, string(data))
	} else {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Drift detected (--strict):")
		for _, r := range drifted {
			fmt.Fprintf(stdout, "  [FAIL] %s — %s (%s)\n", r.Path, r.Detail, r.Kind)
		}
	}
	return exitcode.New(exitcode.Drift, "E-401",
		fmt.Sprintf("drift check failed: %d file(s) have drifted", len(drifted)),
		"Run 'squadai apply' to restore managed files to their expected state.")
}

// printVerifyResult prints a single verification result line.
func printVerifyResult(stdout io.Writer, r domain.VerifyResult) {
	icon := "PASS"
	if !r.Passed {
		icon = "FAIL"
	}
	if r.Severity == domain.SeverityWarning {
		icon = "WARN"
	}
	line := fmt.Sprintf("  [%s] %s", icon, r.Check)
	if r.Message != "" {
		line += " — " + r.Message
	}
	fmt.Fprintln(stdout, line)
}

// printGroupedResults groups verification results by Component field and prints them.
func printGroupedResults(stdout io.Writer, results []domain.VerifyResult) {
	// Collect groups in order of first appearance.
	type group struct {
		name    string
		results []domain.VerifyResult
	}
	var groups []group
	seen := make(map[string]int)

	for _, r := range results {
		comp := r.Component
		if comp == "" {
			comp = "General"
		}
		if idx, ok := seen[comp]; ok {
			groups[idx].results = append(groups[idx].results, r)
		} else {
			seen[comp] = len(groups)
			groups = append(groups, group{name: comp, results: []domain.VerifyResult{r}})
		}
	}

	for i, g := range groups {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "%s:\n", g.name)
		for _, r := range g.results {
			printVerifyResult(stdout, r)
		}
	}
}

// printApplySummary counts written/skipped/failed steps and prints a one-line summary.
func printApplySummary(stdout io.Writer, steps []domain.StepResult) {
	var written, skipped, failed int
	for _, s := range steps {
		switch {
		case s.Status == domain.StepSuccess:
			if s.Action.Action == domain.ActionSkip {
				skipped++
			} else {
				written++
			}
		case s.Status == domain.StepFailed:
			failed++
		case s.Status == domain.StepRolledBack:
			failed++
		default:
			written++
		}
	}
	fmt.Fprintf(stdout, "\nApplied %d action(s): %d written, %d skipped, %d failed\n", len(steps), written, skipped, failed)
}

// printVerifySummary counts passed/failed/warning results and prints a one-line summary.
func printVerifySummary(stdout io.Writer, results []domain.VerifyResult) {
	var passed, failedCount, warnings int
	for _, r := range results {
		if r.Severity == domain.SeverityWarning {
			warnings++
		} else if r.Passed {
			passed++
		} else {
			failedCount++
		}
	}
	fmt.Fprintf(stdout, "\n%d checks: %d passed, %d failed, %d warnings\n", len(results), passed, failedCount, warnings)
}
