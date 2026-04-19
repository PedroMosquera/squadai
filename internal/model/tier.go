package model

import (
	"fmt"
	"strings"
)

// Tier represents an abstract model quality/cost tier used for per-role assignment.
// Each adapter resolves a Tier to its concrete model name at install time.
type Tier string

const (
	TierPremium  Tier = "premium"
	TierStandard Tier = "standard"
	TierCheap    Tier = "cheap"
)

// ParseTier parses a tier string case-insensitively.
// Returns an error for unknown values.
func ParseTier(s string) (Tier, error) {
	switch strings.ToLower(s) {
	case "premium":
		return TierPremium, nil
	case "standard":
		return TierStandard, nil
	case "cheap":
		return TierCheap, nil
	default:
		return "", fmt.Errorf("unknown model tier %q (expected: premium, standard, cheap)", s)
	}
}

// String returns the string representation of the tier.
func (t Tier) String() string { return string(t) }

// DefaultTier returns the default tier for roles that do not specify one.
func DefaultTier() Tier { return TierStandard }
