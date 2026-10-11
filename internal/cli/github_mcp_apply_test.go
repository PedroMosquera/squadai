package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunApply_GithubMCPAllHarnessesScansClean(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Chdir(project)
	for _, d := range []string{".claude", ".codex", ".cursor", filepath.Join(".pi", "agent"), filepath.Join(".codeium", "windsurf"), filepath.Join(".config", "opencode")} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var initOut bytes.Buffer
	agents := "--agents=claude-code,opencode,cursor,windsurf,vscode-copilot,codex,pi"
	if err := RunInit([]string{agents, "--methodology=tdd", "--mcp=context7,github", "--json"}, &initOut); err != nil {
		t.Fatalf("RunInit: %v\n%s", err, initOut.String())
	}
	applyNoReview(t)

	const image = "ghcr.io/github/github-mcp-server:v2.0.2"
	for _, f := range []string{
		filepath.Join(project, ".mcp.json"),
		filepath.Join(project, ".cursor", "mcp.json"),
		filepath.Join(project, ".windsurf", "mcp_config.json"),
		filepath.Join(project, ".pi", "mcp.json"),
		filepath.Join(project, "opencode.json"),
		filepath.Join(home, ".codex", "config.toml"),
	} {
		data := readFile(t, f)
		if !strings.Contains(data, image) || strings.Contains(data, "@modelcontextprotocol/server-github") {
			t.Errorf("%s does not launch %s:\n%s", f, image, data)
		}
	}

	for _, args := range [][]string{{"--json"}, {"--json", "--user"}} {
		var scanOut bytes.Buffer
		_ = RunScan(args, &scanOut)
		var report struct {
			Findings []map[string]any `json:"findings"`
		}
		if err := json.Unmarshal(scanOut.Bytes(), &report); err != nil {
			t.Fatalf("scan %v output is not JSON: %v\n%s", args, err, scanOut.String())
		}
		for _, f := range report.Findings {
			t.Errorf("scan %v flags the generated config: %v", args, f)
		}
	}
}
