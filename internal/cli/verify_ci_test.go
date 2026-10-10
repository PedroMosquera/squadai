package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
)

// setupCIProject writes a project that enables Claude Code and OpenCode, applies
// it for every adapter, then leaves the test on a bare machine: an empty HOME,
// a PATH with no agent binaries, and the project as working directory.
func setupCIProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	applyHome := filepath.Join(root, "apply-home")
	proj := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
			string(domain.AgentOpenCode):   {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"docs": {Type: "remote", URL: "https://example.invalid/mcp", Enabled: true},
		},
	}
	if err := os.MkdirAll(filepath.Join(project, config.ProjectConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), proj); err != nil {
		t.Fatal(err)
	}

	merged, err := loadAndMerge(applyHome, project)
	if err != nil {
		t.Fatal(err)
	}
	applyDefaultProfile(merged)
	p := planner.New()
	actions, err := p.Plan(merged, allAdapters(), applyHome, project)
	if err != nil {
		t.Fatal(err)
	}
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil).Execute(actions)
	if err != nil || !report.Success {
		t.Fatalf("apply failed: %v", err)
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_ACTIONS", "")
	t.Chdir(project)
	return project
}

func runVerifyCIForTest(t *testing.T, extra ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := RunVerify(append([]string{"--ci"}, extra...), &buf)
	return buf.String(), err
}

func requireDrift(t *testing.T, err error, out string) {
	t.Helper()
	var appErr *exitcode.AppError
	if !errors.As(err, &appErr) || appErr.Code != exitcode.Drift {
		t.Fatalf("want exit code %d (drift), got %v\n%s", exitcode.Drift, err, out)
	}
}

func handEditClaudeMD(t *testing.T, project string) {
	t.Helper()
	path := filepath.Join(project, "CLAUDE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	open := strings.Index(content, "<!-- squadai:")
	if open < 0 {
		t.Fatalf("CLAUDE.md has no managed block:\n%s", content)
	}
	eol := open + strings.Index(content[open:], "\n") + 1
	edited := content[:eol] + "Hand edit inside the managed block.\n" + content[eol:]
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCI_CleanApplyPassesWithNoAgentsInstalled(t *testing.T) {
	setupCIProject(t)

	out, err := runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("clean apply should verify on a bare machine, got %v\n%s", err, out)
	}
}

func TestVerifyCI_HandEditInManagedBlockIsDriftNamingTheFile(t *testing.T) {
	project := setupCIProject(t)
	handEditClaudeMD(t, project)

	for _, args := range [][]string{nil, {"--json"}} {
		out, err := runVerifyCIForTest(t, args...)
		requireDrift(t, err, out)
		if !strings.Contains(out, "CLAUDE.md") {
			t.Errorf("args %v: output should name CLAUDE.md, got:\n%s", args, out)
		}
	}

	var buf bytes.Buffer
	_ = RunVerify([]string{"--ci", "--json"}, &buf)
	var report domain.VerifyReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("--json output is not a verify report: %v\n%s", err, buf.String())
	}
	var paths []string
	for _, r := range report.Results {
		if !r.Passed {
			paths = append(paths, r.Path)
		}
	}
	if len(paths) != 1 || paths[0] != "CLAUDE.md" {
		t.Errorf("want exactly one failing result for CLAUDE.md, got paths %v", paths)
	}
}

func TestVerifyCI_UserAddedMCPServerIsNotDrift(t *testing.T) {
	project := setupCIProject(t)
	path := filepath.Join(project, ".mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if servers["docs"] == nil {
		t.Fatalf("apply should have written the docs server to .mcp.json:\n%s", data)
	}
	servers["mine"] = map[string]any{"command": "my-local-server"}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("a user-added MCP server is not drift, got %v\n%s", err, got)
	}
}

func TestVerifyCI_PrintsGitHubAnnotationsUnderActions(t *testing.T) {
	project := setupCIProject(t)
	handEditClaudeMD(t, project)
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_WORKSPACE", "")

	out, err := runVerifyCIForTest(t)
	requireDrift(t, err, out)
	if !strings.Contains(out, "::error file=CLAUDE.md,title=squadai verify::CLAUDE.md differs") {
		t.Errorf("want a GitHub error annotation for CLAUDE.md, got:\n%s", out)
	}

	jsonOut, _ := runVerifyCIForTest(t, "--json")
	if strings.Contains(jsonOut, "::error") {
		t.Errorf("annotations would corrupt --json output:\n%s", jsonOut)
	}
}

func TestVerifyCI_SkipsFilesTheProjectGitignores(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	project := setupCIProject(t)
	bin := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := os.Remove(filepath.Join(project, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	init := exec.Command("git", "init", "-q")
	init.Dir = project
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	out, err := runVerifyCIForTest(t)
	requireDrift(t, err, out)
	if !strings.Contains(out, "AGENTS.md is missing") {
		t.Errorf("a missing tracked file should be reported, got:\n%s", out)
	}

	if err := os.WriteFile(filepath.Join(project, ".gitignore"), []byte("AGENTS.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("a gitignored file cannot be in a checkout and must not fail CI, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "gitignored") {
		t.Errorf("skipped files should be reported, got:\n%s", out)
	}
}
