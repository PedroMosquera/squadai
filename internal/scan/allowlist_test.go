package scan

import "testing"

const twoServersMCP = `{
  "mcpServers": {
    "approved": {"command": "npx", "args": ["-y", "approved-mcp@1.2.3"]},
    "handadded": {"command": "npx", "args": ["-y", "unvetted-mcp@0.0.1"]}
  }
}`

func scanAllowListHits(t *testing.T, dir string) map[string]Finding {
	t.Helper()
	r, err := Scan(dir, AllAdapters())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Finding{}
	for _, f := range r.Findings {
		if f.ID == "SCAN-007" {
			out[f.Subject] = f
		}
	}
	return out
}

func TestScan007_FlagsHandAddedServerOutsideAllowList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".mcp.json", twoServersMCP)
	writeFile(t, dir, ".squadai/policy.json", `{"version": 1, "mode": "team", "allowed": {"mcp_servers": {"names": ["approved"]}}}`)

	got := scanAllowListHits(t, dir)
	f, ok := got[`mcp server "handadded"`]
	if !ok {
		t.Fatalf("want SCAN-007 for the hand-added server, got %v", got)
	}
	if f.Severity != SeverityHigh || f.File != ".mcp.json" || f.Line == 0 {
		t.Errorf("want high severity at a line of .mcp.json, got %+v", f)
	}
	if _, ok := got[`mcp server "approved"`]; ok {
		t.Errorf("allowed server must not be flagged, got %v", got)
	}
}

func TestScan007_CommandPatternsCheckTheFullCommandLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".mcp.json", twoServersMCP)
	writeFile(t, dir, ".squadai/policy.json", `{"version": 1, "mode": "team", "allowed": {"mcp_servers": {"commands": ["npx -y approved-mcp@*"]}}}`)

	got := scanAllowListHits(t, dir)
	if _, ok := got[`mcp server "handadded"`]; !ok || len(got) != 1 {
		t.Errorf("want exactly the hand-added server flagged, got %v", got)
	}
}

func TestScan007_FlagsHooksAndPluginsOutsideAllowList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".claude/settings.json", `{
  "enabledPlugins": {"good@vetted-market": true, "other@random-market": true},
  "hooks": {"Stop": [{"hooks": [
    {"type": "command", "command": "squadai budget check"},
    {"type": "command", "command": "./scripts/notify.sh"}
  ]}]}
}`)
	writeFile(t, dir, ".squadai/policy.json", `{"version": 1, "mode": "team", "allowed": {
  "hook_commands": ["squadai *"],
  "plugin_sources": ["vetted-market"]
}}`)

	got := scanAllowListHits(t, dir)
	if len(got) != 2 {
		t.Fatalf("want one hook and one plugin flagged, got %v", got)
	}
	if f, ok := got["hook Stop"]; !ok || f.locator != "./scripts/notify.sh" {
		t.Errorf("want the notify hook flagged, got %v", got)
	}
	if _, ok := got[`plugin "other@random-market"`]; !ok {
		t.Errorf("want the plugin from an unlisted marketplace flagged, got %v", got)
	}
}

func TestScan007_NoPolicyOrNoAllowListsFlagsNothing(t *testing.T) {
	for name, policy := range map[string]string{
		"no policy file":   "",
		"no allowed block": `{"version": 1, "mode": "team"}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, ".mcp.json", twoServersMCP)
			if policy != "" {
				writeFile(t, dir, ".squadai/policy.json", policy)
			}
			if got := scanAllowListHits(t, dir); len(got) != 0 {
				t.Errorf("want no SCAN-007 without allow-lists, got %v", got)
			}
		})
	}
}

func TestScan_NonBooleanEnabledPluginsKeepsTheFileScanned(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".claude/settings.json", `{
  "enabledPlugins": {"tool@some-market": {"version": "1.0.0"}},
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "./notify.sh"}]}]}
}`)
	writeFile(t, dir, ".squadai/policy.json", `{"version": 1, "mode": "team", "allowed": {"plugin_sources": ["vetted-market"]}}`)

	got := scanAllowListHits(t, dir)
	if _, ok := got[`plugin "tool@some-market"`]; !ok {
		t.Errorf("a plugin enabled with an object value must still be checked, got %v", got)
	}
}

func TestScan_UnparseablePolicyIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".squadai/policy.json", `{"allowed": `)
	if _, err := Scan(dir, AllAdapters()); err == nil {
		t.Error("a broken policy.json must fail the scan rather than skip the allow-lists")
	}
}
