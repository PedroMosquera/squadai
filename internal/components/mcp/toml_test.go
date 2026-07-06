package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// ─── TOML rendering ─────────────────────────────────────────────────────────

func TestRenderTOMLServers_StdioServer(t *testing.T) {
	got := renderTOMLServers("mcp_servers", map[string]domain.MCPServerDef{
		"context7": {
			Type:    "local",
			Command: []string{"npx", "-y", "@upstash/context7-mcp"},
			Enabled: true,
		},
	})

	want := "[mcp_servers.context7]\n" +
		"command = \"npx\"\n" +
		"args = [\"-y\", \"@upstash/context7-mcp\"]"
	if got != want {
		t.Errorf("renderTOMLServers =\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTOMLServers_StdioServerWithEnv(t *testing.T) {
	got := renderTOMLServers("mcp_servers", map[string]domain.MCPServerDef{
		"github": {
			Type:        "local",
			Command:     []string{"gh-mcp"},
			Environment: map[string]string{"GITHUB_TOKEN": "tok", "API_URL": "https://api.github.com"},
			Enabled:     true,
		},
	})

	want := "[mcp_servers.github]\n" +
		"command = \"gh-mcp\"\n" +
		"env = { API_URL = \"https://api.github.com\", GITHUB_TOKEN = \"tok\" }"
	if got != want {
		t.Errorf("renderTOMLServers =\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTOMLServers_RemoteServer(t *testing.T) {
	got := renderTOMLServers("mcp_servers", map[string]domain.MCPServerDef{
		"context7": {
			Type:    "remote",
			URL:     "https://mcp.context7.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer x"},
			Enabled: true,
		},
	})

	want := "[mcp_servers.context7]\n" +
		"url = \"https://mcp.context7.com/mcp\"\n" +
		"http_headers = { Authorization = \"Bearer x\" }"
	if got != want {
		t.Errorf("renderTOMLServers =\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTOMLServers_SortedServerNames(t *testing.T) {
	got := renderTOMLServers("mcp_servers", map[string]domain.MCPServerDef{
		"zeta":  {Command: []string{"z"}, Enabled: true},
		"alpha": {Command: []string{"a"}, Enabled: true},
		"mid":   {Command: []string{"m"}, Enabled: true},
	})

	alphaIdx := strings.Index(got, "[mcp_servers.alpha]")
	midIdx := strings.Index(got, "[mcp_servers.mid]")
	zetaIdx := strings.Index(got, "[mcp_servers.zeta]")
	if alphaIdx < 0 || midIdx < 0 || zetaIdx < 0 {
		t.Fatalf("missing server tables in output:\n%s", got)
	}
	if !(alphaIdx < midIdx && midIdx < zetaIdx) {
		t.Errorf("server tables not sorted:\n%s", got)
	}
}

func TestRenderTOMLServers_Deterministic(t *testing.T) {
	servers := map[string]domain.MCPServerDef{
		"a": {Command: []string{"cmd"}, Environment: map[string]string{"X": "1", "Y": "2", "Z": "3"}, Enabled: true},
		"b": {URL: "https://b.example", Enabled: true},
	}
	first := renderTOMLServers("mcp_servers", servers)
	for i := 0; i < 20; i++ {
		if got := renderTOMLServers("mcp_servers", servers); got != first {
			t.Fatalf("rendering is not deterministic:\n%s\nvs\n%s", first, got)
		}
	}
}

func TestTOMLString_Escaping(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`plain`, `"plain"`},
		{`has "quotes"`, `"has \"quotes\""`},
		{`back\slash`, `"back\\slash"`},
		{"new\nline", `"new\nline"`},
		{"tab\there", `"tab\there"`},
	}
	for _, tt := range tests {
		if got := tomlString(tt.in); got != tt.want {
			t.Errorf("tomlString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestTOMLKey_QuotesNonBareKeys(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"context7", "context7"},
		{"my-server_2", "my-server_2"},
		{"has space", `"has space"`},
		{"dot.ted", `"dot.ted"`},
		{"", `""`},
	}
	for _, tt := range tests {
		if got := tomlKey(tt.in); got != tt.want {
			t.Errorf("tomlKey(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

// ─── TOML strategy: Plan / Apply / Verify ───────────────────────────────────

func tomlTestServers() map[string]domain.MCPServerDef {
	return map[string]domain.MCPServerDef{
		"context7": {
			Type:    "local",
			Command: []string{"npx", "-y", "@upstash/context7-mcp"},
			Enabled: true,
		},
	}
}

func TestPlanTOML_FreshHome_Create(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want create", actions[0].Action)
	}
	wantPath := filepath.Join(home, ".codex", "config.toml")
	if actions[0].TargetPath != wantPath {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, wantPath)
	}
	if !strings.HasPrefix(actions[0].Description, "mcp:toml:") {
		t.Errorf("Description = %q, want mcp:toml: prefix", actions[0].Description)
	}
}

func TestApplyTOML_WritesManagedBlock(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# squadai:mcp:start") {
		t.Error("config.toml missing opening hash marker")
	}
	if !strings.Contains(content, "# squadai:mcp:end") {
		t.Error("config.toml missing closing hash marker")
	}
	if !strings.Contains(content, "[mcp_servers.context7]") {
		t.Error("config.toml missing [mcp_servers.context7] table")
	}
	if !strings.Contains(content, `command = "npx"`) {
		t.Error("config.toml missing command key")
	}
	if !strings.Contains(content, `args = ["-y", "@upstash/context7-mcp"]`) {
		t.Error("config.toml missing args array")
	}
}

func TestApplyTOML_PreservesUserContent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	userContent := "# my config\nmodel = \"gpt-5.2\"\napproval_policy = \"on-request\"\n"
	configPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	inst := New(tomlTestServers())
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionUpdate {
		t.Fatalf("expected 1 update action, got %+v", actions)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, userContent) {
		t.Errorf("user content should be preserved at top of file, got:\n%s", content)
	}
	if !strings.Contains(content, "[mcp_servers.context7]") {
		t.Error("managed block missing after apply")
	}
}

func TestPlanTOML_Idempotent_SecondPlanSkips(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	before, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}

	// Fresh installer (as a new run would construct).
	inst2 := New(tomlTestServers())
	actions2, err := inst2.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("second Plan: %v", err)
	}
	if len(actions2) != 1 || actions2[0].Action != domain.ActionSkip {
		t.Fatalf("expected skip on second plan, got %+v", actions2)
	}
	if err := inst2.Apply(actions2[0]); err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("file changed on re-apply:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestPlanTOML_ChangedServers_Update(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()

	inst := New(tomlTestServers())
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	changed := map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
	}
	inst2 := New(changed)
	actions2, err := inst2.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("second Plan: %v", err)
	}
	if len(actions2) != 1 || actions2[0].Action != domain.ActionUpdate {
		t.Fatalf("expected update action for changed servers, got %+v", actions2)
	}
	if err := inst2.Apply(actions2[0]); err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `url = "https://mcp.context7.com/mcp"`) {
		t.Error("updated block missing url key")
	}
	if strings.Contains(content, `command = "npx"`) {
		t.Error("stale stdio config should have been replaced")
	}
	if strings.Count(content, "# squadai:mcp:start") != 1 {
		t.Errorf("expected exactly one managed block, got:\n%s", content)
	}
}

func TestVerifyTOML_PassesAfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerifyTOML_FailsOnDrift(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Tamper with the managed block.
	configPath := filepath.Join(home, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `command = "npx"`, `command = "evil"`, 1)
	if err := os.WriteFile(configPath, []byte(tampered), 0644); err != nil {
		t.Fatal(err)
	}

	inst2 := New(tomlTestServers())
	results, err := inst2.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	var foundFailure bool
	for _, r := range results {
		if r.Check == "mcp-toml-servers-current" && !r.Passed {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Errorf("expected mcp-toml-servers-current failure after tamper, got %+v", results)
	}
}

func TestVerifyTOML_MissingFile_Fails(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	var foundFailure bool
	for _, r := range results {
		if r.Check == "mcp-toml-exists" && !r.Passed {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Errorf("expected mcp-toml-exists failure, got %+v", results)
	}
}

func TestPlanTOML_PruneWhenEmpty_RemovesBlock(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()

	inst := New(tomlTestServers())
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Add user content so the file survives block removal.
	configPath := filepath.Join(home, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5.2\"\n\n"+string(data)), 0644); err != nil {
		t.Fatal(err)
	}

	pruner := New(nil, Options{PruneWhenEmpty: true})
	actions2, err := pruner.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("prune Plan: %v", err)
	}
	if len(actions2) != 1 || actions2[0].Action != domain.ActionUpdate {
		t.Fatalf("expected update action for prune, got %+v", actions2)
	}
	if err := pruner.Apply(actions2[0]); err != nil {
		t.Fatalf("prune Apply: %v", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(after)
	if strings.Contains(content, "squadai:mcp") {
		t.Errorf("managed block should be removed, got:\n%s", content)
	}
	if !strings.Contains(content, `model = "gpt-5.2"`) {
		t.Errorf("user content should survive prune, got:\n%s", content)
	}
}

func TestPlanTOML_EmptyServersNoBlock_NoActions(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()

	pruner := New(nil, Options{PruneWhenEmpty: true})
	actions, err := pruner.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions for empty servers and no block, got %+v", actions)
	}
}

func TestRenderContentTOML_MatchesApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	inst := New(tomlTestServers())

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	rendered, err := inst.RenderContent(actions[0])
	if err != nil {
		t.Fatalf("RenderContent: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(rendered) != string(written) {
		t.Errorf("RenderContent differs from Apply output:\nrendered:\n%s\nwritten:\n%s", rendered, written)
	}
}

// TestApplyTOML_StripAll_RoundTrip verifies the managed block written by the
// TOML strategy is recognized and removed by marker.StripAll — the primitive
// squadai remove relies on for cleanup.
func TestApplyTOML_StripAll_RoundTrip(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5.2\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	inst := New(tomlTestServers())
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	stripped, found := marker.StripAll(string(data))
	if !found {
		t.Fatal("StripAll should find the hash-marker block")
	}
	if strings.Contains(stripped, "mcp_servers") {
		t.Errorf("StripAll should remove the managed block, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, `model = "gpt-5.2"`) {
		t.Errorf("StripAll should preserve user content, got:\n%s", stripped)
	}
}
