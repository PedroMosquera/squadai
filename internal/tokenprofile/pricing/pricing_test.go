package pricing

import (
	"math"
	"testing"
	"time"
)

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
		if got != c.want {
			t.Errorf("Lookup(%q) = %+v, want %+v", c.model, got, c.want)
		}
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
	got, ok := Lookup("claude-sonnet-4-5-20250929")
	want := ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}
	if !ok || got != want {
		t.Errorf("Lookup(claude-sonnet-4-5-20250929) = %+v, %v; want %+v, true", got, ok, want)
	}
	// A dated Opus 4.5 id must hit the specific 4-5 entry, not the older
	// "claude-opus-4" family price.
	got, ok = Lookup("claude-opus-4-5-20251101")
	want = ModelPricing{InputPerMillion: 5, OutputPerMillion: 25}
	if !ok || got != want {
		t.Errorf("Lookup(claude-opus-4-5-20251101) = %+v, %v; want %+v, true", got, ok, want)
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
	if len(tab.Models) == 0 {
		t.Error("embedded pricing.json has no model entries")
	}
	if len(tab.FallbackDivisors) == 0 {
		t.Error("embedded pricing.json has no fallback divisor entries")
	}
}
