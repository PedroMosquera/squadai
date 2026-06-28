package pricing

import (
	"math"
	"testing"
)

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
		if got != c.want {
			t.Errorf("Lookup(%q) = %+v, want %+v", c.model, got, c.want)
		}
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
	want := ModelPricing{InputPerMillion: 3, OutputPerMillion: 15}
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
