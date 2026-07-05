package pricing

import (
	"math"
	"testing"

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
		{"claude-opus-4", ModelPricing{InputPerMillion: 15, OutputPerMillion: 75}},
		{"claude-sonnet-4", ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}},
		{"claude-haiku-3.5", ModelPricing{InputPerMillion: 0.80, OutputPerMillion: 4}},
		{"gpt-4o", ModelPricing{InputPerMillion: 2.50, OutputPerMillion: 10}},
		{"gpt-4.1", ModelPricing{InputPerMillion: 2, OutputPerMillion: 8}},
		{"gpt-4.1-mini", ModelPricing{InputPerMillion: 0.40, OutputPerMillion: 1.60}},
		{"gpt-4-turbo", ModelPricing{InputPerMillion: 10, OutputPerMillion: 30}},
		{"gpt-3.5-turbo", ModelPricing{InputPerMillion: 0.50, OutputPerMillion: 1.50}},
	}
	for _, c := range cases {
		got := Lookup(c.model)
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
		if got := Lookup(c.model); got != withDefaults(c.want) {
			t.Errorf("Lookup(%q) = %+v, want %+v", c.model, got, withDefaults(c.want))
		}
	}
}

func TestLookup_ExactBeatsLegacyPrefix(t *testing.T) {
	// Regression for the HasPrefix trap: claude-haiku-4-5 must never be
	// captured by a shorter legacy prefix row, and claude-sonnet-4-6 must
	// resolve to its own (non-legacy) pricing even though claude-sonnet-4
	// exists as a legacy row with identical numbers.
	if got := Lookup("claude-haiku-4-5"); got.InputPerMillion != 1 {
		t.Errorf("Lookup(claude-haiku-4-5).Input = %v, want 1", got.InputPerMillion)
	}
	if got := Lookup("claude-sonnet-4-6"); got != withDefaults(ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}) {
		t.Errorf("Lookup(claude-sonnet-4-6) = %+v, want 3/15", got)
	}
}

func TestLookup_UnknownModel(t *testing.T) {
	got := Lookup("some-unknown-model")
	if got != (ModelPricing{}) {
		t.Errorf("Lookup(unknown) = %+v, want zeros", got)
	}
}

func TestLookup_PrefixMatch(t *testing.T) {
	got := Lookup("claude-sonnet-4-20250514")
	want := withDefaults(ModelPricing{InputPerMillion: 3, OutputPerMillion: 15})
	if got != want {
		t.Errorf("Lookup(claude-sonnet-4-20250514) = %+v, want %+v", got, want)
	}
}

func TestEstimateCost(t *testing.T) {
	// claude-sonnet-4: $3 input / $15 output per million tokens.
	// 1_000_000 input -> $3; 500_000 output -> $7.5; total $10.5.
	got := EstimateCost("claude-sonnet-4", 1_000_000, 500_000)
	want := 10.5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("EstimateCost = %v, want %v", got, want)
	}
}

func TestEstimateCostWithCache_DefaultMultipliers(t *testing.T) {
	// claude-sonnet-4: $3 input / $15 output per million tokens.
	// Cache read at 10% of input rate, cache write at 125%.
	// 1M in -> $3; 500k out -> $7.5; 2M cache-read -> 2*3*0.1 = $0.6;
	// 1M cache-write -> 3*1.25 = $3.75. Total $14.85.
	got := EstimateCostWithCache("claude-sonnet-4", 1_000_000, 500_000, 2_000_000, 1_000_000)
	want := 14.85
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("EstimateCostWithCache = %v, want %v", got, want)
	}
}

func TestEstimateCostWithCache_ZeroCacheMatchesEstimateCost(t *testing.T) {
	plain := EstimateCost("claude-sonnet-4-6", 12_345, 6_789)
	cached := EstimateCostWithCache("claude-sonnet-4-6", 12_345, 6_789, 0, 0)
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

	p := Lookup("custom-model")
	if p.CacheReadMultiplier != 0.5 || p.CacheWriteMultiplier != 2 {
		t.Fatalf("Lookup multipliers = %v/%v, want 0.5/2", p.CacheReadMultiplier, p.CacheWriteMultiplier)
	}
	// 1M in -> $10; 1M out -> $20; 1M cache-read -> 10*0.5 = $5;
	// 1M cache-write -> 10*2 = $20. Total $55.
	got := EstimateCostWithCache("custom-model", 1_000_000, 1_000_000, 1_000_000, 1_000_000)
	if math.Abs(got-55) > 1e-9 {
		t.Errorf("EstimateCostWithCache = %v, want 55", got)
	}
}

func TestEstimateCostWithCache_UnknownModelIsFree(t *testing.T) {
	if got := EstimateCostWithCache("mystery-model", 1000, 1000, 1000, 1000); got != 0 {
		t.Errorf("EstimateCostWithCache(unknown) = %v, want 0", got)
	}
}
