package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/state"
	"github.com/PedroMosquera/squadai/internal/tui"
	"github.com/PedroMosquera/squadai/internal/update"
)

// Version is set from main via ldflags at build time.
var Version = "dev"

// Run is the top-level entry point. It parses args and dispatches to subcommands.
// When no args are given, it launches the interactive TUI.
func Run(args []string, stdout, stderr io.Writer) error {
	// Phase 1: apply any pending update before doing anything else.
	// (Non-fatal — apply writes its own warnings to stderr.)
	if err := update.Apply(stderr); err != nil {
		fmt.Fprintf(stderr, "squadai: warning: %v\n", err)
	}

	if len(args) == 0 {
		// Start background update check, then launch TUI.
		cancelBg := maybeStartBackgroundCheck(stderr)
		defer cancelBg()
		return tui.Run(Version)
	}

	// Propagate version to cli package so RunDoctor can include it in JSON output.
	cli.Version = Version

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "SquadAI %s\n", Version)
		return nil

	case "help", "--help", "-h":
		printUsage(stdout)
		return nil

	case "update":
		return cli.RunUpdate(args[1:], stdout, stderr)

	case "init":
		return cli.RunInit(args[1:], stdout)

	case "validate-policy":
		return cli.RunValidatePolicy(args[1:], stdout)

	case "plan":
		return cli.RunPlan(args[1:], stdout)

	case "diff":
		return cli.RunDiff(args[1:], stdout)

	case "apply":
		cancelBg := maybeStartBackgroundCheck(stderr)
		defer cancelBg()
		return cli.RunApply(args[1:], stdout)

	case "verify":
		return cli.RunVerify(args[1:], stdout)

	case "status":
		return cli.RunStatus(args[1:], stdout)

	case "backup":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" || args[1] == "help" {
			printBackupUsage(stdout)
			return nil
		}
		switch args[1] {
		case "create":
			return cli.RunBackupCreate(args[2:], stdout)
		case "list":
			return cli.RunBackupList(args[2:], stdout)
		case "delete":
			return cli.RunBackupDelete(args[2:], stdout)
		case "prune":
			return cli.RunBackupPrune(args[2:], stdout)
		default:
			return fmt.Errorf("unknown backup subcommand %q", args[1])
		}

	case "restore":
		return cli.RunRestore(args[1:], stdout)

	case "remove":
		return cli.RunRemove(args[1:], stdout)

	case "doctor":
		return cli.RunDoctor(args[1:], stdout)

	default:
		return fmt.Errorf("unknown command %q — run 'squadai help' for available commands", args[0])
	}
}

// maybeStartBackgroundCheck spawns a goroutine to check for updates if
// update checks are enabled and 24 hours have elapsed since the last check.
// It returns a cancel function that should be deferred by the caller.
func maybeStartBackgroundCheck(stderr io.Writer) context.CancelFunc {
	if update.IsDevBuild(Version) {
		return func() {}
	}

	statePath, err := state.DefaultPath()
	if err != nil {
		return func() {}
	}

	s, err := state.Load(statePath)
	if err != nil {
		return func() {}
	}

	if !s.UpdateChecksEnabled {
		return func() {}
	}

	if time.Since(s.LastUpdateCheck) < 24*time.Hour {
		return func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := update.Run(ctx, Version, stderr); err != nil {
			if errors.Is(err, update.ErrDevBuild) ||
				errors.Is(err, update.ErrUpToDate) ||
				errors.Is(err, update.ErrNoRelease) {
				// Expected non-error conditions — silent.
			}
			// All other errors are silently dropped in background mode.
		}

		// Persist the check timestamp regardless of result.
		s2, loadErr := state.Load(statePath)
		if loadErr != nil {
			return
		}
		s2.LastUpdateCheck = time.Now()
		_ = state.Save(statePath, s2) // best effort
	}()

	return cancel
}

func printBackupUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: squadai backup <subcommand> [flags]

Subcommands:
  create        Create a backup of all managed files
  list          List all backup snapshots
  delete <id>   Delete a specific backup snapshot
  prune         Remove old backups, keep N most recent (default 10)

Flags:
  --json        Output results in JSON format
`)
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `SquadAI %s — Team-consistent AI setup with safe local customization.

Usage:
  squadai <command> [flags]

Commands:
  init               Initialize project config and optional policy template
  validate-policy    Validate policy schema and lock/required consistency
  plan               Compute action plan (use --dry-run to preview)
  diff               Show what apply would change as unified diffs
  apply              Execute plan with backup and rollback safety
  verify             Print compliance and health report
  status             Show project configuration summary
  doctor             Run pre-flight diagnostics (environment, agents, config, MCP, filesystem, drift)
  backup create      Snapshot managed files
  backup list        List available backups
  backup delete <id> Delete a backup snapshot
  backup prune       Remove old backups (keep N most recent)
  restore <id>       Restore from a backup
  remove             Remove all managed files (use --force to confirm)
  update             Check for updates and download (see 'squadai update --help')
  version            Print version

Flags:
  --dry-run          Preview changes without applying (plan, apply)
  --json             Machine-readable JSON output (all commands)
  -h, --help         Show this help

`, Version)
}
