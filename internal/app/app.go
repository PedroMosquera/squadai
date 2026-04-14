package app

import (
	"fmt"
	"io"

	"github.com/PedroMosquera/agent-manager-pro/internal/cli"
	"github.com/PedroMosquera/agent-manager-pro/internal/tui"
)

// Version is set from main via ldflags at build time.
var Version = "dev"

// Run is the top-level entry point. It parses args and dispatches to subcommands.
// When no args are given, it launches the interactive TUI.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return tui.Run(Version)
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "agent-manager %s\n", Version)
		return nil

	case "help", "--help", "-h":
		printUsage(stdout)
		return nil

	case "init":
		return cli.RunInit(args[1:], stdout)

	case "validate-policy":
		return cli.RunValidatePolicy(args[1:], stdout)

	case "plan":
		return cli.RunPlan(args[1:], stdout)

	case "diff":
		return cli.RunDiff(args[1:], stdout)

	case "apply":
		return cli.RunApply(args[1:], stdout)

	case "sync":
		return cli.RunSync(args[1:], stdout)

	case "verify":
		return cli.RunVerify(args[1:], stdout)

	case "status":
		return cli.RunStatus(args[1:], stdout)

	case "backup":
		if len(args) < 2 {
			return fmt.Errorf("backup requires a subcommand: create, list, delete, prune")
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

	default:
		return fmt.Errorf("unknown command %q — run 'agent-manager help' for available commands", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `agent-manager %s — Team-consistent AI setup with safe local customization.

Usage:
  agent-manager <command> [flags]

Commands:
  init               Initialize project config and optional policy template
  validate-policy    Validate policy schema and lock/required consistency
  plan               Compute action plan (use --dry-run to preview)
  diff               Show what apply would change as unified diffs
  apply              Execute plan with backup and rollback safety
  sync               Idempotent reconciliation to desired state
  verify             Print compliance and health report
  status             Show project configuration summary
  backup create      Snapshot managed files
  backup list        List available backups
  backup delete <id> Delete a backup snapshot
  backup prune       Remove old backups (keep N most recent)
  restore <id>       Restore from a backup
  remove             Remove all managed files (use --force to confirm)
  version            Print version

Flags:
  --dry-run          Preview changes without applying (plan, apply, sync)
  --json             Machine-readable JSON output (all commands)
  -h, --help         Show this help

`, Version)
}
