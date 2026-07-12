package pricing

import (
	"math"
	"testing"
	"time"

	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// withDefaults fills the default cache multipliers so pre-cache expectations
// stay readable.
func withDefaults(p ModelPricing) ModelPricing {
	if p.CacheReadMultiplier == 0 {
		p.CacheReadMultiplier = modelcatalog.DefaultCacheReadMultiplier
	}
	if p.CacheWriteMultiplier == 0 {
		p.CacheWriteMultiplier = modelcatalog.DefaultCacheWriteMultiplier
	}
	return p
}

func TestLookup_KnownModels(t *testing.T) {
	cases := []struct {
		model string
		want  ModelPricing
	}{
		{"claude-fable-5", ModelPricing{InputPerMillion: 10, OutputPerMillion: 50}},
		{"claude-opus-4-8", ModelPricing{InputPerMillion: 5, OutputPerMillion: 25}},
		{"claude-opus-4-1", ModelPricing{InputPerMillion: 15, OutputPerMillion: 75}},
		{"claude-sonnet-5", ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}},
		{"claude-sonnet-4-6", ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}},
		{"claude-haiku-4-5", ModelPricing{InputPerMillion: 1, OutputPerMillion: 5}},
		{"claude-haiku-3.5", ModelPricing{InputPerMillion: 0.80, OutputPerMillion: 4}},
		{"gpt-4o", ModelPricing{InputPerMillion: 2.50, OutputPerMillion: 10}},
		{"gpt-4.1", ModelPricing{InputPerMillion: 2, OutputPerMillion: 8}},
		{"gpt-4.1-mini", ModelPricing{InputPerMillion: 0.40, OutputPerMillion: 1.60}},
		{"gpt-4-turbo", ModelPricing{InputPerMillion: 10, OutputPerMillion: 30}},
		{"gpt-3.5-turbo", ModelPricing{InputPerMillion: 0.50, OutputPerMillion: 1.50}},
	}
	for _, c := range cases {
		got, ok := Lookup(c.model)
		if !ok {
			t.Errorf("Lookup(%q) ok = false, want true", c.model)
			continue
		}
		if got != withDefaults(c.want) {
			t.Errorf("Lookup(%q) = %+v, want %+v", c.model, got, withDefaults(c.want))
		}
	}
}

func TestLookup_CurrentGenerationModels(t *testing.T) {
	cases := []struct {
		model string
		want  ModelPricing
	}{
		{"claude-fable-5", ModelPricing{InputPerMillion: 10, OutputPerMillion: 50}},
		{"claude-opus-4-8", ModelPricing{InputPerMillion: 5, OutputPerMillion: 25}},
		{"claude-sonnet-4-6", ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}},
		{"claude-haiku-4-5", ModelPricing{InputPerMillion: 1, OutputPerMillion: 5}},
		{"gpt-5-mini", ModelPricing{InputPerMillion: 0.25, OutputPerMillion: 2}},
		// Provider-qualified names normalize to bare catalog IDs.
		{"anthropic/claude-fable-5", ModelPricing{InputPerMillion: 10, OutputPerMillion: 50}},
		// Alias resolution.
		{"claude-haiku-4-5-20251001", ModelPricing{InputPerMillion: 1, OutputPerMillion: 5}},
	}
	for _, c := range cases {
		got, ok := Lookup(c.model)
		if !ok || got != withDefaults(c.want) {
			t.Errorf("Lookup(%q) = %+v (ok=%v), want %+v", c.model, got, ok, withDefaults(c.want))
		}
	}
}

func TestLookup_ExactBeatsLegacyPrefix(t *testing.T) {
	// Regression for the HasPrefix trap: claude-haiku-4-5 must never be
	// captured by a shorter legacy prefix row, and claude-sonnet-4-6 must
	// resolve to its own (non-legacy) pricing even though claude-sonnet-4
	// exists as a legacy row with identical numbers.
	if got, _ := Lookup("claude-haiku-4-5"); got.InputPerMillion != 1 {
		t.Errorf("Lookup(claude-haiku-4-5).Input = %v, want 1", got.InputPerMillion)
	}
	if got, _ := Lookup("claude-sonnet-4-6"); got != withDefaults(ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}) {
		t.Errorf("Lookup(claude-sonnet-4-6) = %+v, want 3/15", got)
	}
}

func TestLookup_UnknownModel(t *testing.T) {
	got, ok := Lookup("some-unknown-model")
	if ok {
		t.Errorf("Lookup(unknown) ok = true, want false")
	}
	if got != (ModelPricing{}) {
		t.Errorf("Lookup(unknown) = %+v, want zeros", got)
	}
}

func TestLookup_PrefixMatch(t *testing.T) {
	got, ok := Lookup("claude-sonnet-4-20250514")
	want := withDefaults(ModelPricing{InputPerMillion: 3, OutputPerMillion: 15})
	if !ok || got != want {
		t.Errorf("Lookup(claude-sonnet-4-20250514) = %+v, %v; want %+v, true", got, ok, want)
	}
	// A dated Sonnet 4.5 id must hit the claude-sonnet-4-5 entry, not the
	// older "claude-sonnet-4" family price.
	got, ok = Lookup("claude-sonnet-4-5-20250929")
	want = withDefaults(ModelPricing{InputPerMillion: 3, OutputPerMillion: 15})
	if !ok || got != want {
		t.Errorf("Lookup(claude-sonnet-4-5-20250929) = %+v, %v; want %+v, true", got, ok, want)
	}
}

func TestEstimateCost(t *testing.T) {
	// claude-sonnet-4: $3 input / $15 output per million tokens.
	// 1_000_000 input -> $3; 500_000 output -> $7.5; total $10.5.
	got, ok := EstimateCost("claude-sonnet-4", 1_000_000, 500_000)
	want := 10.5
	if !ok {
		t.Fatal("EstimateCost(known model) ok = false, want true")
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("EstimateCost = %v, want %v", got, want)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	got, ok := EstimateCost("some-unknown-model", 1_000_000, 500_000)
	if ok {
		t.Error("EstimateCost(unknown) ok = true, want false")
	}
	if got != 0 {
		t.Errorf("EstimateCost(unknown) = %v, want 0", got)
	}
}

func TestFallbackDivisors(t *testing.T) {
	claude := FallbackDivisors("claude-sonnet-4-6")
	if claude.Prose != 3.7 || claude.Code != 3.0 {
		t.Errorf("FallbackDivisors(claude) = %+v, want {3.7 3}", claude)
	}
	def := FallbackDivisors("some-unknown-model")
	if def.Prose <= 0 || def.Code <= 0 {
		t.Errorf("FallbackDivisors(unknown) = %+v, want positive defaults", def)
	}
}

func TestGeneratedAt_Parses(t *testing.T) {
	if GeneratedAt().IsZero() {
		t.Fatal("GeneratedAt() is zero; embedded pricing.json generated_at missing or malformed")
	}
}

func TestStaleWarning(t *testing.T) {
	gen := GeneratedAt()
	if _, stale := StaleWarning(gen.Add(24 * time.Hour)); stale {
		t.Error("StaleWarning fired 1 day after generation")
	}
	if _, stale := StaleWarning(gen.Add(89 * 24 * time.Hour)); stale {
		t.Error("StaleWarning fired 89 days after generation")
	}
	msg, stale := StaleWarning(gen.Add(91 * 24 * time.Hour))
	if !stale {
		t.Fatal("StaleWarning did not fire 91 days after generation")
	}
	if msg == "" {
		t.Error("StaleWarning returned an empty message")
	}
}

// TestEmbeddedTableLoads guards the load() panic path: the embedded asset
// must always parse and contain entries.
func TestEmbeddedTableLoads(t *testing.T) {
	tab := load()
	if len(tab.FallbackDivisors) == 0 {
		t.Error("embedded pricing.json has no fallback divisor entries")
	}
}

func TestEstimateCostWithCache_DefaultMultipliers(t *testing.T) {
	// claude-sonnet-4: $3 input / $15 output per million tokens.
	// Cache read at 10% of input rate, cache write at 125%.
	// 1M in -> $3; 500k out -> $7.5; 2M cache-read -> 2*3*0.1 = $0.6;
	// 1M cache-write -> 3*1.25 = $3.75. Total $14.85.
	got, ok := EstimateCostWithCache("claude-sonnet-4", 1_000_000, 500_000, 2_000_000, 1_000_000)
	want := 14.85
	if !ok {
		t.Fatal("EstimateCostWithCache(known model) ok = false, want true")
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("EstimateCostWithCache = %v, want %v", got, want)
	}
}

func TestEstimateCostWithCache_ZeroCacheMatchesEstimateCost(t *testing.T) {
	plain, _ := EstimateCost("claude-sonnet-4-6", 12_345, 6_789)
	cached, _ := EstimateCostWithCache("claude-sonnet-4-6", 12_345, 6_789, 0, 0)
	if math.Abs(plain-cached) > 1e-12 {
		t.Errorf("EstimateCostWithCache(0,0) = %v, want %v", cached, plain)
	}
}

func TestEstimateCostWithCache_CatalogSuppliedMultipliers(t *testing.T) {
	f := &modelcatalog.File{
		SchemaVersion: 1,
		Models: map[string]modelcatalog.Model{
			"custom-model": {
				Provider:             "test",
				InputPerMTok:         10,
				OutputPerMTok:        20,
				CacheReadMultiplier:  0.5,
				CacheWriteMultiplier: 2,
			},
		},
	}
	restore := modelcatalog.SetDefaultForTest(modelcatalog.FromFile(f, modelcatalog.SourceProject))
	defer restore()

	p, ok := Lookup("custom-model")
	if !ok || p.CacheReadMultiplier != 0.5 || p.CacheWriteMultiplier != 2 {
		t.Fatalf("Lookup multipliers = %v/%v (ok=%v), want 0.5/2", p.CacheReadMultiplier, p.CacheWriteMultiplier, ok)
	}
	// 1M in -> $10; 1M out -> $20; 1M cache-read -> 10*0.5 = $5;
	// 1M cache-write -> 10*2 = $20. Total $55.
	got, _ := EstimateCostWithCache("custom-model", 1_000_000, 1_000_000, 1_000_000, 1_000_000)
	if math.Abs(got-55) > 1e-9 {
		t.Errorf("EstimateCostWithCache = %v, want 55", got)
	}
}

func TestEstimateCostWithCache_UnknownModelIsFree(t *testing.T) {
	got, ok := EstimateCostWithCache("mystery-model", 1000, 1000, 1000, 1000)
	if ok {
		t.Error("EstimateCostWithCache(unknown) ok = true, want false")
	}
	if got != 0 {
		t.Errorf("EstimateCostWithCache(unknown) = %v, want 0", got)
	}
}
