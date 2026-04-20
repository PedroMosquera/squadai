package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── backup help ─────────────────────────────────────────────────────────────

func TestRun_BackupHelp_NoSubcommand(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"backup"}, &buf, &buf)
	if err != nil {
		t.Fatalf("backup with no subcommand should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: squadai backup",
		"create",
		"list",
		"delete",
		"prune",
		"--json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("backup help missing %q, got:\n%s", want, out)
		}
	}
}

func TestRun_BackupHelp_HelpFlag(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "help"} {
		t.Run(flag, func(t *testing.T) {
			var buf bytes.Buffer
			err := Run([]string{"backup", flag}, &buf, &buf)
			if err != nil {
				t.Fatalf("backup %s should not error: %v", flag, err)
			}
			out := buf.String()
			for _, want := range []string{
				"Usage: squadai backup",
				"create",
				"list",
				"delete",
				"prune",
				"--json",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("backup %s help missing %q, got:\n%s", flag, want, out)
				}
			}
		})
	}
}

func TestRun_BackupHelp_ShowsSubcommandDescriptions(t *testing.T) {
	var buf bytes.Buffer
	if err := Run([]string{"backup"}, &buf, &buf); err != nil {
		t.Fatalf("backup should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Create a backup",
		"List all backup snapshots",
		"Delete a specific backup",
		"Remove old backups",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("backup help missing description %q, got:\n%s", want, out)
		}
	}
}

func TestRun_BackupUnknownSubcommand_StillErrors(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"backup", "frobnicate"}, &buf, &buf)
	if err == nil {
		t.Fatal("backup with unknown subcommand should return an error")
	}
	if !strings.Contains(err.Error(), "unknown backup subcommand") {
		t.Errorf("error should mention unknown backup subcommand, got: %v", err)
	}
}

// ─── printBackupUsage ────────────────────────────────────────────────────────

func TestPrintBackupUsage_ContainsAllSubcommands(t *testing.T) {
	var buf bytes.Buffer
	printBackupUsage(&buf)
	out := buf.String()
	for _, want := range []string{"create", "list", "delete", "prune"} {
		if !strings.Contains(out, want) {
			t.Errorf("printBackupUsage missing %q, got:\n%s", want, out)
		}
	}
}

func TestPrintBackupUsage_ContainsJSONFlag(t *testing.T) {
	var buf bytes.Buffer
	printBackupUsage(&buf)
	if !strings.Contains(buf.String(), "--json") {
		t.Errorf("printBackupUsage should mention --json flag")
	}
}

func TestPrintBackupUsage_ContainsUsageLine(t *testing.T) {
	var buf bytes.Buffer
	printBackupUsage(&buf)
	if !strings.Contains(buf.String(), "Usage: squadai backup") {
		t.Errorf("printBackupUsage should contain usage line")
	}
}

// ─── version / help dispatch ────────────────────────────────────────────────

func TestRun_Version_PrintsVersion(t *testing.T) {
	Version = "test-version-1.2.3"
	t.Cleanup(func() { Version = "dev" })

	for _, arg := range []string{"version", "--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Run([]string{arg}, &buf, &buf); err != nil {
				t.Fatalf("Run(%q) error: %v", arg, err)
			}
			out := buf.String()
			if !strings.Contains(out, "SquadAI") {
				t.Errorf("version output missing 'SquadAI', got: %s", out)
			}
			if !strings.Contains(out, "test-version-1.2.3") {
				t.Errorf("version output missing injected version, got: %s", out)
			}
		})
	}
}

func TestRun_Help_PrintsTopLevelUsage(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Run([]string{arg}, &buf, &buf); err != nil {
				t.Fatalf("Run(%q) error: %v", arg, err)
			}
			out := buf.String()
			for _, want := range []string{
				"SquadAI",
				"Usage:",
				"init",
				"plan",
				"apply",
				"verify",
				"doctor",
				"backup",
				"restore",
				"remove",
				"update",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("Run(%q) output missing %q, got:\n%s", arg, want, out)
				}
			}
		})
	}
}

func TestRun_UnknownCommand_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"frobnicate"}, &buf, &buf)
	if err == nil {
		t.Fatal("unknown command should return an error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error should mention unknown command, got: %v", err)
	}
	if !strings.Contains(err.Error(), "frobnicate") {
		t.Errorf("error should name the offending command, got: %v", err)
	}
}

// ─── subcommand --help dispatch ─────────────────────────────────────────────

// Every subcommand exposes an early-return --help handler that writes usage
// to stdout without touching the filesystem. This exercises the dispatch
// switch in Run() cheaply.
func TestRun_SubcommandHelp_Dispatches(t *testing.T) {
	cases := []struct {
		cmd      string
		contains string
	}{
		{"init", "Usage: squadai init"},
		{"validate-policy", "Usage: squadai validate-policy"},
		{"plan", "Usage: squadai plan"},
		{"diff", "Usage: squadai diff"},
		{"apply", "Usage: squadai apply"},
		{"verify", "Usage: squadai verify"},
		{"status", "Usage: squadai status"},
		{"restore", "Usage: squadai restore"},
		{"remove", "Usage: squadai remove"},
		{"doctor", "Usage: squadai doctor"},
		{"update", "Usage: squadai update"},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run([]string{tc.cmd, "--help"}, &stdout, &stderr)
			if err != nil {
				t.Fatalf("Run(%q --help) error: %v", tc.cmd, err)
			}
			if !strings.Contains(stdout.String(), tc.contains) {
				t.Errorf("Run(%q --help) stdout missing %q, got:\n%s",
					tc.cmd, tc.contains, stdout.String())
			}
		})
	}
}

// ─── backup subcommand dispatch ─────────────────────────────────────────────

func TestRun_BackupSubcommands_Dispatch(t *testing.T) {
	cases := []struct {
		sub      string
		contains string
	}{
		{"create", "Usage: squadai backup create"},
		{"list", "Usage: squadai backup list"},
		{"delete", "Usage: squadai backup delete"},
		{"prune", "Usage: squadai backup prune"},
	}
	for _, tc := range cases {
		t.Run(tc.sub, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run([]string{"backup", tc.sub, "--help"}, &stdout, &stderr)
			if err != nil {
				t.Fatalf("Run(backup %s --help) error: %v", tc.sub, err)
			}
			if !strings.Contains(stdout.String(), tc.contains) {
				t.Errorf("Run(backup %s --help) stdout missing %q, got:\n%s",
					tc.sub, tc.contains, stdout.String())
			}
		})
	}
}

// ─── printUsage ─────────────────────────────────────────────────────────────

func TestPrintUsage_IncludesAllCommands(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()
	for _, want := range []string{
		"init", "validate-policy", "plan", "diff", "apply",
		"verify", "status", "doctor", "backup", "restore",
		"remove", "update", "version",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printUsage missing command %q, got:\n%s", want, out)
		}
	}
}

func TestPrintUsage_IncludesVersionInHeader(t *testing.T) {
	Version = "audit-version-9.9.9"
	t.Cleanup(func() { Version = "dev" })

	var buf bytes.Buffer
	printUsage(&buf)
	if !strings.Contains(buf.String(), "audit-version-9.9.9") {
		t.Errorf("printUsage should embed Version in its header, got:\n%s", buf.String())
	}
}

// ─── maybeStartBackgroundCheck ───────────────────────────────────────────────

func TestMaybeStartBackgroundCheck_DevBuild_NoOp(t *testing.T) {
	// Version defaults to "dev" — update.IsDevBuild returns true, so the
	// function must bail out before touching state or the network.
	cancel := maybeStartBackgroundCheck(&bytes.Buffer{})
	if cancel == nil {
		t.Fatal("expected non-nil cancel function")
	}
	cancel() // must not panic
}

func TestMaybeStartBackgroundCheck_ChecksDisabled_NoOp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Write state with UpdateChecksEnabled = false.
	writeState(t, home, map[string]any{"update_checks_enabled": false})

	Version = "1.2.3"
	t.Cleanup(func() { Version = "dev" })

	cancel := maybeStartBackgroundCheck(&bytes.Buffer{})
	if cancel == nil {
		t.Fatal("expected non-nil cancel function")
	}
	cancel()
}

func TestMaybeStartBackgroundCheck_RecentCheck_NoOp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeState(t, home, map[string]any{
		"update_checks_enabled": true,
		"last_update_check":     time.Now().Format(time.RFC3339),
	})

	Version = "1.2.3"
	t.Cleanup(func() { Version = "dev" })

	cancel := maybeStartBackgroundCheck(&bytes.Buffer{})
	if cancel == nil {
		t.Fatal("expected non-nil cancel function")
	}
	cancel()
}

func writeState(t *testing.T, home string, payload map[string]any) {
	t.Helper()
	dir := filepath.Join(home, ".squadai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), data, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
}
