package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/scan"
)

// Split so the source holds no token-shaped literal for push protection.
const scanTestToken = "ghp_" + "aB3dE5fG7hJ9kL1mN3pQ5rS7tU9vW1xY3zA5"

func seedScanProject(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	t.Chdir(project)
	for rel, content := range files {
		p := filepath.Join(project, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return project
}

func appErrorCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return exitcode.OK
	}
	var ae *exitcode.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want *exitcode.AppError, got %T: %v", err, err)
	}
	return ae.Code
}

func TestRunScan_SeededBadConfig_FailsWithJSON(t *testing.T) {
	seedScanProject(t, map[string]string{
		".mcp.json": `{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "` + scanTestToken + `"}
    }
  }
}`,
		".claude/settings.json": `{"permissions": {"defaultMode": "bypassPermissions"}}`,
	})

	var out bytes.Buffer
	err := RunScan([]string{"--json"}, &out)
	if code := appErrorCode(t, err); code != exitcode.Policy {
		t.Fatalf("want exit code %d (policy), got %d: %v", exitcode.Policy, code, err)
	}
	if strings.Contains(out.String(), scanTestToken) {
		t.Fatalf("JSON output leaks the token:\n%s", out.String())
	}

	var raw struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	got := map[string]map[string]any{}
	for _, f := range raw.Findings {
		got[f["id"].(string)] = f
	}
	for id, sev := range map[string]string{"SCAN-001": "high", "SCAN-003": "medium", "SCAN-005": "low"} {
		f, ok := got[id]
		if !ok {
			t.Fatalf("missing %s in %s", id, out.String())
		}
		if f["severity"] != sev {
			t.Errorf("%s severity = %v, want %s", id, f["severity"], sev)
		}
		for _, key := range []string{"file", "line", "subject", "message"} {
			if _, ok := f[key]; !ok {
				t.Errorf("%s missing %q field: %v", id, key, f)
			}
		}
	}
	if got["SCAN-001"]["file"] != ".mcp.json" || got["SCAN-001"]["line"] != float64(6) {
		t.Errorf("SCAN-001 location = %v:%v, want .mcp.json:6", got["SCAN-001"]["file"], got["SCAN-001"]["line"])
	}
}

func TestRunScan_FailOnThreshold(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{nil, exitcode.OK},
		{[]string{"--fail-on", "high"}, exitcode.OK},
		{[]string{"--fail-on=medium"}, exitcode.OK},
		{[]string{"--fail-on", "low"}, exitcode.Policy},
		{[]string{"--fail-on", "info"}, exitcode.Policy},
		{[]string{"--fail-on", "none"}, exitcode.OK},
		{[]string{"--fail-on", "critical"}, exitcode.Config},
		{[]string{"--fail-on"}, exitcode.Config},
		{[]string{"--strict"}, exitcode.Config},
	}
	seedScanProject(t, map[string]string{
		".claude/settings.json": `{"permissions": {"defaultMode": "bypassPermissions"}}`,
	})
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out bytes.Buffer
			err := RunScan(tc.args, &out)
			if code := appErrorCode(t, err); code != tc.want {
				t.Fatalf("exit code = %d, want %d (%v)\n%s", code, tc.want, err, out.String())
			}
		})
	}
}

func TestRunScan_HumanTable(t *testing.T) {
	seedScanProject(t, map[string]string{
		".claude/settings.json": `{"permissions": {"defaultMode": "bypassPermissions"}}`,
	})
	var out bytes.Buffer
	if err := RunScan(nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SEVERITY", "SCAN-005", ".claude/settings.json:1", "1 finding(s): 0 high, 0 medium, 1 low"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("table missing %q:\n%s", want, out.String())
		}
	}
}

// Apply output must pass its own gate: every preset, every harness, no high
// findings. Medium findings are allowed and logged so a regression in what the
// defaults emit is visible in -v output.
func TestRunScan_DefaultApplyHasNoHighOrUnpinnedFindings(t *testing.T) {
	presets := []domain.SetupPreset{
		domain.PresetSoloMinimal, domain.PresetSoloPower, domain.PresetTeamStandard,
		domain.PresetEnterpriseLock, domain.PresetFullSquad, domain.PresetLean,
	}
	for _, preset := range presets {
		t.Run(string(preset), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Chdir(t.TempDir())
			for _, a := range scan.AllAdapters() {
				if err := os.MkdirAll(a.GlobalConfigDir(home), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			var initOut bytes.Buffer
			agents := "opencode,claude-code,vscode-copilot,cursor,windsurf,pi,codex"
			if err := RunInit([]string{"--preset=" + string(preset), "--agents=" + agents, "--json"}, &initOut); err != nil {
				t.Fatalf("RunInit: %v\n%s", err, initOut.String())
			}
			var applyOut bytes.Buffer
			if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
				t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
			}

			var out bytes.Buffer
			err := RunScan([]string{"--json", "--fail-on", "high"}, &out)
			var report scan.Report
			if jerr := json.Unmarshal(out.Bytes(), &report); jerr != nil {
				t.Fatalf("scan output is not JSON: %v\n%s", jerr, out.String())
			}
			if err != nil {
				t.Fatalf("scan after default apply failed: %v\n%s", err, out.String())
			}
			if len(report.Scanned) == 0 {
				t.Fatal("scan read no files after apply; the test would pass vacuously")
			}
			if len(report.Skipped) != 0 {
				t.Errorf("generated config could not be parsed: %+v", report.Skipped)
			}
			byRule := map[string]int{}
			for _, f := range report.Findings {
				if f.Severity >= scan.SeverityHigh {
					t.Errorf("high finding on generated config: %+v", f)
				}
				if f.ID == "SCAN-003" {
					t.Errorf("unpinned package launcher in generated config: %+v", f)
				}
				byRule[f.ID+"/"+f.Severity.String()]++
			}
			t.Logf("scanned %v; findings by rule: %v", report.Scanned, byRule)
		})
	}
}
