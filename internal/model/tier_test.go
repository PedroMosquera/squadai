package model

import (
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func TestParseTier_ValidValues(t *testing.T) {
	cases := []struct {
		input string
		want  Tier
	}{
		{"premium", TierPremium},
		{"standard", TierStandard},
		{"cheap", TierCheap},
		{"PREMIUM", TierPremium},
		{"Standard", TierStandard},
		{"CHEAP", TierCheap},
	}
	for _, tc := range cases {
		got, err := ParseTier(tc.input)
		if err != nil {
			t.Errorf("ParseTier(%q) error = %v, want nil", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseTier(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseTier_InvalidValue_ReturnsError(t *testing.T) {
	cases := []string{"", "elite", "free", "balanced", "performance", "starter"}
	for _, input := range cases {
		_, err := ParseTier(input)
		if err == nil {
			t.Errorf("ParseTier(%q) expected error, got nil", input)
		}
	}
}

func TestTierString(t *testing.T) {
	if TierPremium.String() != "premium" {
		t.Errorf("TierPremium.String() = %q, want %q", TierPremium.String(), "premium")
	}
	if TierStandard.String() != "standard" {
		t.Errorf("TierStandard.String() = %q, want %q", TierStandard.String(), "standard")
	}
	if TierCheap.String() != "cheap" {
		t.Errorf("TierCheap.String() = %q, want %q", TierCheap.String(), "cheap")
	}
}

func TestDefaultTier(t *testing.T) {
	if DefaultTier() != TierStandard {
		t.Errorf("DefaultTier() = %q, want %q", DefaultTier(), TierStandard)
	}
}

func TestTierFromModelTier(t *testing.T) {
	cases := []struct {
		input domain.ModelTier
		want  Tier
	}{
		{domain.ModelTierBalanced, TierStandard},
		{domain.ModelTierPerformance, TierPremium},
		{domain.ModelTierStarter, TierCheap},
		{domain.ModelTierManual, TierStandard},
		{domain.ModelTier(""), TierStandard},
		{domain.ModelTier("bogus"), TierStandard},
	}
	for _, tc := range cases {
		if got := TierFromModelTier(tc.input); got != tc.want {
			t.Errorf("TierFromModelTier(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTierFromProfile(t *testing.T) {
	cases := []struct {
		input string
		want  Tier
	}{
		{"cheap", TierCheap},
		{"balanced", TierStandard},
		{"premium", TierPremium},
		{"PREMIUM", TierPremium},
		{"", TierStandard},
		{"bogus", TierStandard},
	}
	for _, tc := range cases {
		if got := TierFromProfile(tc.input); got != tc.want {
			t.Errorf("TierFromProfile(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
