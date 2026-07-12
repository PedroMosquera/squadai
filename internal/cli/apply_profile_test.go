package cli

import (
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

func profileTestConfig() *domain.MergedConfig {
	return &domain.MergedConfig{
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"context7": {Type: "local", Enabled: true},
			"github":   {Type: "local", Enabled: true},
		},
		Context: domain.ContextConfig{
			DefaultProfile: "default",
			Profiles: map[string]domain.ContextProfile{
				"default": {MemoryScope: "project", MCPServers: []string{"context7"}, MaxApproxTokens: 12000},
				"cheap":   {MemoryScope: "summary", MCPServers: []string{}, SkillScopes: []string{"shared"}, MaxApproxTokens: 6000},
				"open":    {MemoryScope: "none"},
			},
		},
		Usage: domain.DefaultUsageConfig(),
	}
}

// ─── resolveActiveProfile ────────────────────────────────────────────────────

func TestResolveActiveProfile(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		def      string
		wantName string
		wantErr  string
	}{
		{name: "flag wins over default", flag: "cheap", def: "default", wantName: "cheap"},
		{name: "default used when no flag", flag: "", def: "default", wantName: "default"},
		{name: "no flag no default means none", flag: "", def: "", wantName: ""},
		{name: "unknown flag errors with available list", flag: "nope", def: "default", wantErr: "unknown context profile"},
		{name: "stale default is ignored", flag: "", def: "gone", wantName: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := profileTestConfig()
			merged.Context.DefaultProfile = tt.def
			name, prof, err := resolveActiveProfile(merged, tt.flag)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				if !strings.Contains(err.Error(), "cheap") || !strings.Contains(err.Error(), "default") {
					t.Errorf("error should list available profiles, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if (prof == nil) != (tt.wantName == "") {
				t.Errorf("prof nil-ness mismatch for %q", tt.wantName)
			}
		})
	}
}

// ─── applyProfileToConfig ────────────────────────────────────────────────────

func TestApplyProfileToConfig_MCPFilter(t *testing.T) {
	t.Run("nil MCPServers keeps all", func(t *testing.T) {
		merged := profileTestConfig()
		prof := domain.ContextProfile{MemoryScope: "project"} // MCPServers nil
		applyProfileToConfig(merged, "p", &prof)
		if len(merged.MCP) != 2 {
			t.Errorf("nil filter should keep all servers, got %d", len(merged.MCP))
		}
	})
	t.Run("present list is a strict filter", func(t *testing.T) {
		merged := profileTestConfig()
		prof := merged.Context.Profiles["default"]
		applyProfileToConfig(merged, "default", &prof)
		if len(merged.MCP) != 1 {
			t.Fatalf("expected only context7 to survive, got %v", merged.MCP)
		}
		if _, ok := merged.MCP["context7"]; !ok {
			t.Error("context7 should survive the filter")
		}
	})
	t.Run("empty list filters everything", func(t *testing.T) {
		merged := profileTestConfig()
		prof := merged.Context.Profiles["cheap"]
		applyProfileToConfig(merged, "cheap", &prof)
		if len(merged.MCP) != 0 {
			t.Errorf("empty filter should remove all servers, got %v", merged.MCP)
		}
	})
}

// TestApplyProfileToConfig_BuiltinProfilesKeepSquadai: switching to any
// built-in profile with an explicit MCP filter must keep the SquadAI
// control-plane server so agents never lose console access; review and cheap
// intentionally drop all MCP, squadai included.
func TestApplyProfileToConfig_BuiltinProfilesKeepSquadai(t *testing.T) {
	tests := []struct {
		profile     string
		wantSquadai bool
	}{
		{"debug", true},
		{"feature", true},
		{"docs", true},
		{"incident", true},
		{"review", false},
		{"cheap", false},
	}
	for _, tc := range tests {
		t.Run(tc.profile, func(t *testing.T) {
			merged := profileTestConfig()
			merged.MCP = DefaultMCPServers()
			merged.Context = domain.DefaultContextConfig()

			prof, ok := merged.Context.Profiles[tc.profile]
			if !ok {
				t.Fatalf("built-in profile %q missing", tc.profile)
			}
			applyProfileToConfig(merged, tc.profile, &prof)

			_, got := merged.MCP["squadai"]
			if got != tc.wantSquadai {
				t.Errorf("profile %q: squadai present = %v, want %v (MCP after filter: %v)",
					tc.profile, got, tc.wantSquadai, merged.MCP)
			}
		})
	}
}

func TestApplyProfileToConfig_MemoryScope(t *testing.T) {
	t.Run("none disables the memory component", func(t *testing.T) {
		merged := profileTestConfig()
		prof := merged.Context.Profiles["open"]
		applyProfileToConfig(merged, "open", &prof)
		if c := merged.Components[string(domain.ComponentMemory)]; c.Enabled {
			t.Error("memory scope none should disable the memory component")
		}
	})
	t.Run("other scopes keep the component and set the runtime profile", func(t *testing.T) {
		merged := profileTestConfig()
		prof := merged.Context.Profiles["cheap"]
		applyProfileToConfig(merged, "cheap", &prof)
		if c := merged.Components[string(domain.ComponentMemory)]; !c.Enabled {
			t.Error("summary scope must not disable the memory component")
		}
		if merged.ActiveContextProfile == nil || merged.ActiveContextProfile.MemoryScope != "summary" {
			t.Error("ActiveContextProfile should carry the scope for the installers")
		}
		if merged.ActiveProfileName != "cheap" {
			t.Errorf("ActiveProfileName = %q, want cheap", merged.ActiveProfileName)
		}
	})
}

// ─── effectiveTokenCap ───────────────────────────────────────────────────────

func TestEffectiveTokenCap(t *testing.T) {
	prof := &domain.ContextProfile{MaxApproxTokens: 6000}
	tests := []struct {
		name string
		flag int
		prof *domain.ContextProfile
		want int
	}{
		{"flag wins over profile", 9000, prof, 9000},
		{"profile cap when no flag", 0, prof, 6000},
		{"zero when neither", 0, nil, 0},
		{"zero when profile has no cap", 0, &domain.ContextProfile{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveTokenCap(tt.flag, tt.prof); got != tt.want {
				t.Errorf("effectiveTokenCap(%d) = %d, want %d", tt.flag, got, tt.want)
			}
		})
	}
}

// ─── resolveFitModel ─────────────────────────────────────────────────────────

func TestResolveFitModel_Chain(t *testing.T) {
	merged := profileTestConfig()
	cat := modelcatalog.Default()

	t.Run("flag wins", func(t *testing.T) {
		if got := resolveFitModel(merged, "cheap", "my-model"); got != "my-model" {
			t.Errorf("got %q, want flag value", got)
		}
	})
	t.Run("profile tier via tier bridge and catalog", func(t *testing.T) {
		// Usage.ProfileTiers["cheap"] = "cheap" → catalog cheap tier for opencode.
		want := cat.TierModel("opencode", "cheap")
		if got := resolveFitModel(merged, "cheap", ""); got != want {
			t.Errorf("got %q, want catalog cheap-tier model %q", got, want)
		}
	})
	t.Run("standard tier default when no profile", func(t *testing.T) {
		want := cat.TierModel("opencode", "standard")
		if got := resolveFitModel(merged, "", ""); got != want {
			t.Errorf("got %q, want standard-tier default %q", got, want)
		}
		if got := resolveFitModel(merged, "", ""); got == "" {
			t.Error("fit model must never be empty — fitting must use a real tokenizer")
		}
	})
	t.Run("unmapped profile falls back to standard tier", func(t *testing.T) {
		want := cat.TierModel("opencode", "standard")
		if got := resolveFitModel(merged, "open", ""); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// ─── Lazy-load cost model ────────────────────────────────────────────────────

func TestFrontmatterOnly(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "frontmatter extracted",
			content: "---\ndescription: a skill\n---\n\nA very long body that costs nothing until invoked.\n",
			want:    "---\ndescription: a skill\n---\n",
		},
		{
			name:    "no frontmatter returns full content",
			content: "just a body\n",
			want:    "just a body\n",
		},
		{
			name:    "unterminated frontmatter returns full content",
			content: "---\ndescription: broken\n",
			want:    "---\ndescription: broken\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(frontmatterOnly([]byte(tt.content))); got != tt.want {
				t.Errorf("frontmatterOnly = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLazyLoadTables(t *testing.T) {
	if !lazyLoadingAdapters[domain.AgentClaudeCode] || !lazyLoadingAdapters[domain.AgentOpenCode] {
		t.Error("claude-code and opencode lazy-load skills/commands")
	}
	if lazyLoadingAdapters[domain.AgentWindsurf] {
		t.Error("windsurf does not lazy-load")
	}
	if !lazyLoadedComponents[domain.ComponentSkills] || !lazyLoadedComponents[domain.ComponentCommands] {
		t.Error("skills and commands are the lazy-loaded components")
	}
	if lazyLoadedComponents[domain.ComponentMemory] {
		t.Error("memory is injected inline, never lazy-loaded")
	}
}

// ─── summaryComponentTokens ──────────────────────────────────────────────────

func TestSummaryComponentTokens(t *testing.T) {
	actions := []domain.PlannedAction{
		{Component: domain.ComponentMemory, Action: domain.ActionCreate, TargetPath: "/p/AGENTS.md"},
		{Component: domain.ComponentMemory, Action: domain.ActionCreate, TargetPath: "/p/AGENTS.md"}, // same path deduped
		{Component: domain.ComponentRules, Action: domain.ActionCreate, TargetPath: "/p/AGENTS.md"},
		{Component: domain.ComponentSkills, Action: domain.ActionCreate, TargetPath: "/p/skill.md"},
	}
	got := summaryComponentTokens(actions, "", 0)
	if got[domain.ComponentMemory] <= 0 {
		t.Error("memory summary tokens should be positive")
	}
	if got[domain.ComponentRules] <= 0 {
		t.Error("rules summary tokens should be positive")
	}
	if _, ok := got[domain.ComponentSkills]; ok {
		t.Error("non-summarizable components have no summary count")
	}
	// The stub is tiny — far below a typical full protocol (~230 tokens).
	if got[domain.ComponentMemory] > 120 {
		t.Errorf("memory stub count suspiciously large: %d", got[domain.ComponentMemory])
	}
}

// ─── legacy v0.6.0 default profile migration ─────────────────────────────────

func legacyV060DefaultProfile() domain.ContextProfile {
	return domain.ContextProfile{
		MemoryScope:     "project",
		MCPServers:      []string{"context7"},
		SkillScopes:     []string{"shared"},
		MaxApproxTokens: 12000,
		Include:         []string{"**/*"},
		Exclude:         []string{".git/**", "node_modules/**", "dist/**"},
	}
}

// squadai <= v0.6.0 persisted a restrictive-but-inert "default" profile in
// every project.json. Now that profiles are enforced, that exact profile must
// resolve to "no profile" — otherwise every upgrading project gets a silent
// 12k cap that apply enforces but diff/verify never converge with.
func TestResolveActiveProfile_LegacyInertDefaultIgnored(t *testing.T) {
	merged := &domain.MergedConfig{
		Context: domain.ContextConfig{
			DefaultProfile: "default",
			Profiles: map[string]domain.ContextProfile{
				"default": legacyV060DefaultProfile(),
			},
		},
	}
	name, prof, err := resolveActiveProfile(merged, "")
	if err != nil {
		t.Fatalf("resolveActiveProfile: %v", err)
	}
	if name != "" || prof != nil {
		t.Errorf("legacy inert default profile should resolve to no profile, got name=%q prof=%+v", name, prof)
	}

	// Explicit --profile=default with the legacy shape is treated the same:
	// the profile carries no user intent.
	name, prof, err = resolveActiveProfile(merged, "default")
	if err != nil {
		t.Fatalf("resolveActiveProfile(flag): %v", err)
	}
	if name != "" || prof != nil {
		t.Errorf("explicit legacy default should also resolve to no profile, got name=%q", name)
	}
}

// A default profile the user has customized in any way is NOT the inert
// legacy scaffolding and must be enforced.
func TestResolveActiveProfile_CustomizedDefaultStillEnforced(t *testing.T) {
	custom := legacyV060DefaultProfile()
	custom.MaxApproxTokens = 9000 // user picked their own cap
	merged := &domain.MergedConfig{
		Context: domain.ContextConfig{
			DefaultProfile: "default",
			Profiles:       map[string]domain.ContextProfile{"default": custom},
		},
	}
	name, prof, err := resolveActiveProfile(merged, "")
	if err != nil {
		t.Fatalf("resolveActiveProfile: %v", err)
	}
	if name != "default" || prof == nil || prof.MaxApproxTokens != 9000 {
		t.Errorf("customized default profile must be enforced, got name=%q prof=%+v", name, prof)
	}
}
