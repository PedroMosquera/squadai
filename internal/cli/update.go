package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/PedroMosquera/squadai/internal/state"
	"github.com/PedroMosquera/squadai/internal/update"
)

// RunUpdate implements the `squadai update` command.
//
// Flags:
//
//	--enable-checks    Enable background update checks; saves state and exits.
//	--disable-checks   Disable background update checks; saves state and exits.
//	--check            Check only (no download); prints latest version to stdout.
//	--apply-pending    Apply a pending update immediately (skips waiting for next launch).
//	(no flags)         Full synchronous update: check + download, then prompt to relaunch.
func RunUpdate(args []string, stdout, stderr io.Writer) error {
	var enableChecks, disableChecks, checkOnly, applyPending bool
	for _, a := range args {
		switch a {
		case "--enable-checks":
			enableChecks = true
		case "--disable-checks":
			disableChecks = true
		case "--check":
			checkOnly = true
		case "--apply-pending":
			applyPending = true
		case "--help", "-h":
			printUpdateUsage(stdout)
			return nil
		}
	}

	statePath, err := state.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve state path: %w", err)
	}

	if enableChecks {
		s, err := state.Load(statePath)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		s.UpdateChecksEnabled = true
		if err := state.Save(statePath, s); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		fmt.Fprintln(stdout, "Background update checks enabled.")
		return nil
	}

	if disableChecks {
		s, err := state.Load(statePath)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		s.UpdateChecksEnabled = false
		if err := state.Save(statePath, s); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		fmt.Fprintln(stdout, "Background update checks disabled.")
		return nil
	}

	if applyPending {
		return update.Apply(stderr)
	}

	if Version == "dev" || Version == "" {
		fmt.Fprintln(stderr, "squadai: current build is dev — update checks skipped.")
		return nil
	}

	ctx := context.Background()

	if checkOnly {
		result, err := update.Check(ctx, Version)
		if err != nil {
			if errors.Is(err, update.ErrDevBuild) {
				fmt.Fprintln(stdout, "Current build is dev — update checks skipped.")
				return nil
			}
			if errors.Is(err, update.ErrNoRelease) {
				fmt.Fprintln(stdout, "No stable release found.")
				return nil
			}
			return fmt.Errorf("check update: %w", err)
		}
		if result.UpdateAvailable {
			fmt.Fprintf(stdout, "Update available: %s (current: %s)\n", result.LatestVersion, result.CurrentVersion)
		} else {
			fmt.Fprintf(stdout, "squadai %s is up to date.\n", result.CurrentVersion)
		}
		return nil
	}

	// Full foreground update: check + download.
	if err := update.Run(ctx, Version, stderr); err != nil {
		if errors.Is(err, update.ErrDevBuild) {
			fmt.Fprintln(stderr, "squadai: current build is dev — update checks skipped.")
			return nil
		}
		if errors.Is(err, update.ErrUpToDate) {
			fmt.Fprintf(stdout, "squadai %s is already up to date.\n", Version)
			return nil
		}
		if errors.Is(err, update.ErrNoRelease) {
			fmt.Fprintln(stdout, "No stable release found.")
			return nil
		}
		return fmt.Errorf("update: %w", err)
	}

	fmt.Fprintln(stdout, "Relaunch squadai to start using the new version.")
	return nil
}

func printUpdateUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: squadai update [flags]

Check for a new release, download it, and apply on next launch.

Flags:
  --enable-checks    Enable automatic background update checks (once per 24h)
  --disable-checks   Disable automatic background update checks
  --check            Check for a new version without downloading
  --apply-pending    Apply a downloaded update immediately
  -h, --help         Show this help

With no flags: check + download synchronously, then relaunch to apply.
`)
}
