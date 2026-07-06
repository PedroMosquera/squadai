package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/planner"
)

// writeUserFile writes a pre-existing user-owned file, creating parents.
func writeUserFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// The pre-apply review preview (the default `squadai apply` path on a TTY)
// must never fail, whatever pre-existing user files it meets. This guards the
// whole previewer surface across every adapter at once — the Codex TOML
// config parsed as JSON ("invalid character 'o' in literal null") escaped
// because nothing exercised collectPreviewEntries.
func TestCollectPreviewEntries_AllAdapters_WithExistingUserFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)

	// Make every detection-gated adapter detectable.
	for _, dir := range []string{".pi/agent", ".codex", ".claude", ".cursor", ".codeium/windsurf", ".config/opencode"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Realistic pre-existing user files — valid for their own format, hostile
	// to wrong-format parsers.
	writeUserFile(t, filepath.Join(home, ".codex", "config.toml"),
		"notify = true\nmodel = \"gpt-5.2\"\n")
	writeUserFile(t, filepath.Join(project, "opencode.json"),
		`{"$schema": "https://opencode.ai/config.json", "theme": "dark"}`)
	writeUserFile(t, filepath.Join(project, ".mcp.json"),
		`{"mcpServers": {"my-custom": {"command": "my-server", "args": []}}}`)
	writeUserFile(t, filepath.Join(project, "AGENTS.md"),
		"# My project\n\nUser-authored instructions that must survive.\n")
	writeUserFile(t, filepath.Join(project, "CLAUDE.md"),
		"# Notes\n\nHand-written Claude guidance.\n")
	writeUserFile(t, filepath.Join(project, ".vscode", "mcp.json"),
		`{"servers": {}}`)

	var out bytes.Buffer
	if err := RunInit([]string{"--preset=solo-power", "--agents=opencode,pi,codex,claude-code,cursor,vscode-copilot,windsurf", "--json"}, &out); err != nil {
		t.Fatalf("RunInit: %v\n%s", err, out.String())
	}

	buildEntries := func(stage string) {
		t.Helper()
		merged, err := loadAndMerge(home, project)
		if err != nil {
			t.Fatalf("[%s] loadAndMerge: %v", stage, err)
		}
		adapters := DetectAdapters(home)
		p := planner.New()
		if _, err := p.Plan(merged, adapters, home, project); err != nil {
			t.Fatalf("[%s] Plan: %v", stage, err)
		}
		entries, err := collectPreviewEntries(p.ComponentInstallers(), adapters, home, project)
		if err != nil {
			t.Fatalf("[%s] collectPreviewEntries must not fail: %v", stage, err)
		}
		if len(entries) == 0 {
			t.Fatalf("[%s] expected preview entries for a planned install", stage)
		}
	}

	// Stage 1: first run — creates/updates against pre-existing user files.
	buildEntries("pre-apply")

	// Stage 2: after apply plus fresh user edits — update/conflict paths.
	var applyOut bytes.Buffer
	// --overwrite-unmanaged mirrors the review prompt's "overwrite" decision
	// for the user-owned .mcp.json root key planted above.
	if err := RunApply([]string{"--no-review", "--overwrite-unmanaged", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}
	writeUserFile(t, filepath.Join(home, ".codex", "config.toml"),
		"notify = false\nmodel = \"gpt-5.2\"\n# user comment\n"+readFileString(t, filepath.Join(home, ".codex", "config.toml"), "# squadai:mcp:start"))
	buildEntries("post-apply-with-user-edits")
}

// readFileString returns the file content from the first occurrence of marker
// onward, so tests can keep a managed block while replacing user content.
func readFileString(t *testing.T, path, marker string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if idx := bytes.Index(data, []byte(marker)); idx >= 0 {
		return s[idx:]
	}
	return ""
}
