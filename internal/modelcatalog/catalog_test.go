package modelcatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOverride writes a models.json override under dir/.squadai.
func writeOverride(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squadai", "models.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestParse_EmbeddedCatalogIsValid(t *testing.T) {
	f := loadEmbedded() // panics on error
	if len(f.Models) == 0 {
		t.Fatal("embedded catalog has no models")
	}
	if f.Updated == "" {
		t.Fatal("embedded catalog has no updated date")
	}
	// Every adapter tier must reference a model row that exists (after
	// provider-prefix normalization).
	cat := FromFile(f, SourceEmbedded)
	for name, a := range f.Adapters {
		for tier, id := range a.Tiers {
			if !cat.Known(id) {
				t.Errorf("adapter %s tier %s references unknown model %q", name, tier, id)
			}
		}
		for _, tier := range []string{"premium", "standard", "cheap"} {
			if a.Tiers[tier] == "" {
				t.Errorf("adapter %s missing tier %s", name, tier)
			}
		}
	}
}

func TestParse_Errors(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"bad json", `{`, "parse models catalog"},
		{"wrong schema version", `{"schema_version": 2}`, "unsupported schema_version"},
		{"bad updated date", `{"schema_version": 1, "updated": "July 2026"}`, "invalid updated date"},
		{"negative pricing", `{"schema_version": 1, "models": {"m": {"provider": "x", "input_per_mtok": -1}}}`, "negative pricing"},
		{"empty encoding prefix", `{"schema_version": 1, "encoding_prefixes": [{"prefix": "", "encoding": "x"}]}`, "encoding_prefixes"},
		{"empty adapter tier model", `{"schema_version": 1, "adapters": {"a": {"tiers": {"standard": ""}}}}`, "empty model id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.input))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Parse(%s) error = %v, want containing %q", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestLoad_LayeringAndSourceAttribution(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	writeOverride(t, home, `{
		"schema_version": 1,
		"updated": "2027-02-01",
		"models": {
			"my-user-model": {"provider": "acme", "input_per_mtok": 1, "output_per_mtok": 2},
			"claude-sonnet-4-6": {"provider": "anthropic", "input_per_mtok": 9, "output_per_mtok": 99}
		}
	}`)
	writeOverride(t, project, `{
		"schema_version": 1,
		"models": {
			"my-project-model": {"provider": "acme", "input_per_mtok": 3, "output_per_mtok": 4},
			"my-user-model": {"provider": "acme", "input_per_mtok": 5, "output_per_mtok": 6}
		}
	}`)

	cat, err := Load(home, project)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Source attribution.
	cases := []struct {
		id         string
		wantSource string
	}{
		{"claude-fable-5", SourceEmbedded},
		{"claude-sonnet-4-6", SourceUser},   // replaced by user layer
		{"my-user-model", SourceProject},    // project replaces user
		{"my-project-model", SourceProject}, // project-only addition
	}
	for _, tc := range cases {
		if got := cat.Source(tc.id); got != tc.wantSource {
			t.Errorf("Source(%q) = %q, want %q", tc.id, got, tc.wantSource)
		}
	}

	// Layered pricing values win.
	if p, _ := cat.Pricing("claude-sonnet-4-6"); p.InputPerMTok != 9 || p.OutputPerMTok != 99 {
		t.Errorf("user-overridden pricing = %+v, want 9/99", p)
	}
	if p, _ := cat.Pricing("my-user-model"); p.InputPerMTok != 5 || p.OutputPerMTok != 6 {
		t.Errorf("project-overridden pricing = %+v, want 5/6", p)
	}

	// Updated date: last non-empty layer wins (user set it, project did not).
	if got := cat.UpdatedString(); got != "2027-02-01" {
		t.Errorf("UpdatedString() = %q, want 2027-02-01", got)
	}
}

func TestLoad_MissingOverridesUseEmbedded(t *testing.T) {
	cat, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.Source("claude-fable-5"); got != SourceEmbedded {
		t.Errorf("Source(claude-fable-5) = %q, want embedded", got)
	}
	if p, ok := cat.Pricing("claude-fable-5"); !ok || p.InputPerMTok != 10 || p.OutputPerMTok != 50 {
		t.Errorf("Pricing(claude-fable-5) = %+v ok=%v, want 10/50", p, ok)
	}
}

func TestLoad_InvalidOverrideFails(t *testing.T) {
	home := t.TempDir()
	writeOverride(t, home, `{"schema_version": 99}`)
	if _, err := Load(home, t.TempDir()); err == nil {
		t.Fatal("Load with invalid override should error")
	}
}

func TestResolve_ExactBeatsPrefix_TrapRegression(t *testing.T) {
	cat, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The trap: claude-sonnet-4-6 must resolve to its own row, never to the
	// legacy claude-sonnet-4 prefix.
	id, m, ok := cat.Resolve("claude-sonnet-4-6")
	if !ok || id != "claude-sonnet-4-6" {
		t.Fatalf("Resolve(claude-sonnet-4-6) = %q ok=%v, want exact claude-sonnet-4-6 row", id, ok)
	}
	if m.Legacy {
		t.Error("claude-sonnet-4-6 resolved to a legacy row")
	}
	if m.ContextWindow != 1000000 {
		t.Errorf("claude-sonnet-4-6 context window = %d, want 1000000", m.ContextWindow)
	}

	// Longest-prefix: a dated 4-6 snapshot matches the 4-6 row, not 4.
	if id, _, _ := cat.Resolve("claude-sonnet-4-6-20260101"); id != "claude-sonnet-4-6" {
		t.Errorf("Resolve(claude-sonnet-4-6-20260101) = %q, want claude-sonnet-4-6", id)
	}
	// The legacy dated ID has its own exact row.
	if id, _, _ := cat.Resolve("claude-sonnet-4-20250514"); id != "claude-sonnet-4-20250514" {
		t.Errorf("Resolve(claude-sonnet-4-20250514) = %q, want its exact legacy row", id)
	}
}

func TestResolve_AliasesAndNormalization(t *testing.T) {
	cat, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := []struct {
		input  string
		wantID string
	}{
		{"anthropic/claude-fable-5", "claude-fable-5"},
		{"openai/gpt-5-mini", "gpt-5-mini"},
		{"claude-haiku-4-5-20251001", "claude-haiku-4-5"}, // alias
		{"claude-haiku-3.5", "claude-haiku-3-5"},          // legacy dot alias
		{"anthropic/claude-haiku-4-5-20251001", "claude-haiku-4-5"},
	}
	for _, tc := range cases {
		id, _, ok := cat.Resolve(tc.input)
		if !ok || id != tc.wantID {
			t.Errorf("Resolve(%q) = %q ok=%v, want %q", tc.input, id, ok, tc.wantID)
		}
	}
	if _, _, ok := cat.Resolve("totally-unknown-model"); ok {
		t.Error("Resolve(totally-unknown-model) should not match")
	}
	if _, _, ok := cat.Resolve(""); ok {
		t.Error("Resolve(\"\") should not match")
	}
}

func TestEncoding_ModelsAndPrefixes(t *testing.T) {
	cat, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := []struct {
		model string
		want  string
	}{
		{"claude-fable-5", "o200k_base"},
		{"gpt-5.2", "o200k_base"},
		{"gpt-5-nano-preview", "o200k_base"},   // prefix gpt-5
		{"gemini-3-ultra", "o200k_base"},       // prefix gemini-
		{"claude-anything-new", "o200k_base"},  // prefix claude-
		{"gpt-4-32k-legacy", "cl100k_base"},    // prefix gpt-4 (not gpt-4.1/gpt-4o)
		{"anthropic/claude-fable-5", "o200k_base"},
		{"total-junk-model", ""},
	}
	for _, tc := range cases {
		if got := cat.Encoding(tc.model); got != tc.want {
			t.Errorf("Encoding(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestTierModel_FallbacksAndHints(t *testing.T) {
	cat, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.TierModel("claude-code", "standard"); got != "claude-sonnet-4-6" {
		t.Errorf("TierModel(claude-code, standard) = %q", got)
	}
	// Unknown tier falls back to standard.
	if got := cat.TierModel("claude-code", "mystery"); got != "claude-sonnet-4-6" {
		t.Errorf("TierModel(claude-code, mystery) = %q, want standard fallback", got)
	}
	// Unknown adapter falls back to opencode.
	if got, want := cat.TierModel("mystery-agent", "premium"), cat.TierModel("opencode", "premium"); got != want {
		t.Errorf("TierModel(mystery-agent, premium) = %q, want %q", got, want)
	}
	if cat.Hint("cursor", "standard") == "" {
		t.Error("Hint(cursor, standard) should be non-empty")
	}
	if cat.Hint("mystery-agent", "standard") != "" {
		t.Error("Hint(unknown adapter) should be empty")
	}
}

func TestSetDefaultForTest_RoundTrip(t *testing.T) {
	custom := FromFile(&File{
		SchemaVersion: 1,
		Models:        map[string]Model{"only-model": {Provider: "acme", InputPerMTok: 1, OutputPerMTok: 2}},
	}, SourceUser)
	restore := SetDefaultForTest(custom)
	if !Default().Known("only-model") {
		t.Error("Default() should be the injected catalog")
	}
	restore()
	if Default().Known("only-model") {
		t.Error("restore() should reinstate the previous default")
	}
}

func TestContextWindow(t *testing.T) {
	cat, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cat.ContextWindow("claude-haiku-4-5"); got != 200000 {
		t.Errorf("ContextWindow(claude-haiku-4-5) = %d, want 200000", got)
	}
	if got := cat.ContextWindow("junk"); got != 0 {
		t.Errorf("ContextWindow(junk) = %d, want 0", got)
	}
}
