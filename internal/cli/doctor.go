package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// RunDoctor runs pre-flight diagnostics and reports environment, agent, config,
// MCP, filesystem, and config-drift health.
//
// Flags:
//
//	--json              Output structured JSON for CI integration.
//	--verbose / -v      Show detail and fix hints for all checks.
//	--fix               Interactive auto-fix mode: list fixable issues and prompt to resolve them.
//	--category=<slug>   Run only the specified category (environment|agents|config|mcp|filesystem|drift).
//	--check=<cat.name>  Run only a single named check (e.g. mcp.github).
//
// Exit code: 0 when all checks pass/warn/skip; non-zero when any check fails.
func RunDoctor(args []string, stdout io.Writer) error {
	return RunDoctorWithReader(args, stdout, os.Stdin)
}

// RunDoctorWithReader is RunDoctor with an injectable stdin reader (for tests).
func RunDoctorWithReader(args []string, stdout io.Writer, stdin io.Reader) error {
	var opts doctor.Options
	fixMode := false
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--verbose" || arg == "-v":
			opts.Verbose = true
		case arg == "--fix":
			fixMode = true
		case strings.HasPrefix(arg, "--category="):
			opts.Category = strings.TrimPrefix(arg, "--category=")
		case strings.HasPrefix(arg, "--check="):
			opts.Check = strings.TrimPrefix(arg, "--check=")
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai doctor [--json] [--verbose] [--fix] [--category=<slug>] [--check=<cat.name>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Run pre-flight diagnostics across six check categories: Environment, AI Agents,")
			fmt.Fprintln(stdout, "Project Configuration, MCP Servers, Filesystem, and Config Drift.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json                    Output structured JSON for CI integration.")
			fmt.Fprintln(stdout, "  --verbose / -v            Show detail and fix hints for all checks.")
			fmt.Fprintln(stdout, "  --fix                     Interactive: list fixable issues and prompt to resolve them.")
			fmt.Fprintln(stdout, "  --category=<slug>         Run only one category.")
			fmt.Fprintln(stdout, "                            Slugs: environment, agents, config, mcp, filesystem, drift")
			fmt.Fprintln(stdout, "  --check=<category.name>   Run only a single named check (e.g. --check=mcp.github).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Exit code: 0 = all pass/warn/skip, 1 = any fail.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai doctor")
			fmt.Fprintln(stdout, "  squadai doctor --json")
			fmt.Fprintln(stdout, "  squadai doctor --verbose")
			fmt.Fprintln(stdout, "  squadai doctor --fix")
			fmt.Fprintln(stdout, "  squadai doctor --category=mcp")
			fmt.Fprintln(stdout, "  squadai doctor --check=mcp.github")
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

	adapters := DetectAdapters(homeDir)
	catalog := domain.DefaultMCPCatalog()

	d := doctor.New(homeDir, projectDir, adapters, catalog)
	results, err := d.Run(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("doctor: %w", err)
	}

	if opts.JSON {
		return doctor.RenderJSON(stdout, results, Version)
	}

	doctor.RenderHuman(stdout, results, Version, opts.Verbose)

	if fixMode {
		return runDoctorFix(context.Background(), d, results, stdout, stdin)
	}

	// Exit code 1 if any check failed.
	for _, r := range results {
		if r.Status == doctor.CheckFail {
			return fmt.Errorf("doctor found issues that need attention")
		}
	}
	return nil
}

// runDoctorFix implements the interactive --fix flow.
// stdin is injectable for testing.
func runDoctorFix(ctx context.Context, d *doctor.Doctor, results []doctor.CheckResult, stdout io.Writer, stdin io.Reader) error {
	// Collect fixable failures.
	var fixable []doctor.CheckResult
	for _, r := range results {
		if r.AutoFixable && r.Status == doctor.CheckFail {
			fixable = append(fixable, r)
		}
	}

	if len(fixable) == 0 {
		fmt.Fprintln(stdout, "\n  No auto-fixable issues found.")
		return nil
	}

	fmt.Fprintf(stdout, "\n  Found %d auto-fixable issue(s):\n\n", len(fixable))
	for i, r := range fixable {
		fmt.Fprintf(stdout, "  [%d] %s\n", i+1, r.Message)
		if r.FixHint != "" {
			fmt.Fprintf(stdout, "      Fix: %s\n", r.FixHint)
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprint(stdout, "  Fix all? (y/n/select): ")

	scanner := newLineScanner(stdin)
	answer := readLine(scanner)

	var toFix []doctor.CheckResult
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		toFix = fixable
	case "n", "no":
		fmt.Fprintln(stdout, "  Skipped.")
		return nil
	case "select":
		for i, r := range fixable {
			fmt.Fprintf(stdout, "\n  Fix [%d] %s? (y/n): ", i+1, r.Message)
			ans := readLine(scanner)
			if strings.ToLower(strings.TrimSpace(ans)) == "y" {
				toFix = append(toFix, r)
			} else {
				fmt.Fprintln(stdout, "  ── Skipped")
			}
		}
	default:
		fmt.Fprintln(stdout, "  Invalid input. Skipped.")
		return nil
	}

	if len(toFix) == 0 {
		fmt.Fprintln(stdout, "\n  Nothing to fix.")
		return nil
	}

	fmt.Fprintln(stdout)
	fixResults := d.Fix(ctx, toFix)
	anyFailed := false
	for _, fr := range fixResults {
		if fr.Err != nil {
			fmt.Fprintf(stdout, "  ✗ %s — error: %v\n", fr.CheckResult.Message, fr.Err)
			anyFailed = true
		} else {
			fmt.Fprintf(stdout, "  ✓ %s\n", fr.CheckResult.Message)
		}
	}

	// Re-run the originally-failing checks.
	fmt.Fprintln(stdout, "\n  Re-checking...")
	reResults, err := d.Run(ctx, doctor.Options{})
	if err != nil {
		return fmt.Errorf("re-check after fix: %w", err)
	}

	// Build a set of the originally-fixed check names for quick lookup.
	fixedNames := make(map[string]bool, len(toFix))
	for _, r := range toFix {
		fixedNames[r.Category+"."+r.Name] = true
	}

	stillFailing := false
	for _, r := range reResults {
		if !fixedNames[r.Category+"."+r.Name] {
			continue
		}
		icon := "✓"
		if r.Status == doctor.CheckFail {
			icon = "✗"
			stillFailing = true
		} else if r.Status == doctor.CheckWarn {
			icon = "⚠"
		}
		fmt.Fprintf(stdout, "  %s %s — %s\n", icon, r.Name, r.Status.String())
	}

	if anyFailed || stillFailing {
		return fmt.Errorf("some fixes did not resolve all issues")
	}
	return nil
}

// newLineScanner wraps an io.Reader in a bufio.Scanner for line reading.
func newLineScanner(r io.Reader) *bufio.Scanner {
	return bufio.NewScanner(r)
}

// readLine reads one line from the scanner, returning "" on EOF or error.
func readLine(s *bufio.Scanner) string {
	if s.Scan() {
		return s.Text()
	}
	return ""
}
