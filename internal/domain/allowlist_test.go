package domain

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"claude-sonnet-*", "claude-sonnet-5-5", true},
		{"claude-sonnet-*", "claude-opus-5-5", false},
		{"*claude-*", "anthropic/claude-opus-4-7", true},
		{"npx -y @scope/*@*", "npx -y @scope/server@1.2.3", true},
		{"claude-?", "claude-5", true},
		{"claude-?", "claude-55", false},
		{"exact", "exact", true},
		{"exact", "exactly", false},
		{"*", "", true},
		{"", "", true},
		{"", "x", false},
	}
	for _, c := range cases {
		if got := MatchGlob(c.pattern, c.s); got != c.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestMCPServerViolation_EveryListThatIsSetMustPass(t *testing.T) {
	a := AllowLists{MCPServers: &MCPServerAllowList{Names: []string{"docs"}, URLs: []string{"https://docs.invalid/*"}}}
	if v := a.MCPServerViolation("docs", "", "https://docs.invalid/mcp"); v != "" {
		t.Errorf("allowed server flagged: %s", v)
	}
	if v := a.MCPServerViolation("docs", "", "https://elsewhere.invalid/mcp"); v == "" {
		t.Error("an allowed name must not excuse a disallowed URL")
	}
	if v := a.MCPServerViolation("docs", "npx some-server", ""); v != "" {
		t.Errorf("unset commands list must not restrict stdio servers: %s", v)
	}
}
