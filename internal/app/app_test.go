package app

import (
	"bytes"
	"strings"
	"testing"
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
