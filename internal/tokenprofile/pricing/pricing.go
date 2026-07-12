// Package pricing maps model names to USD prices and chars-per-token
// fallback divisors. Both live in the embedded pricing.json asset so they
// update together. The asset is compiled into the binary, so `squadai
// update` (internal/update replaces the whole binary) refreshes this data
// alongside the code; bump "generated_at" whenever the numbers change.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

//go:embed pricing.json
var pricingJSON []byte

// staleAfter is how old the embedded price table may be before Stale
// reports a warning.
const staleAfter = 90 * 24 * time.Hour

// ModelPricing holds per-model USD pricing per million tokens.
type ModelPricing struct {
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
}

// Divisors holds chars-per-token estimates for the character-based token
// heuristic. Code tokenizes denser than prose, so its divisor is lower.
type Divisors struct {
	Prose float64 `json:"prose_chars_per_token"`
	Code  float64 `json:"code_chars_per_token"`
}

// priceTable mirrors pricing.json. Model and divisor entries are ordered
// so that more specific prefixes are matched before shorter ones.
type priceTable struct {
	GeneratedAt string `json:"generated_at"`
	Models      []struct {
		Prefix string `json:"prefix"`
		ModelPricing
	} `json:"models"`
	FallbackDivisors []struct {
		Prefix string `json:"prefix"`
		Divisors
	} `json:"fallback_divisors"`
	DefaultDivisor Divisors `json:"default_divisor"`
}

var (
	loadOnce sync.Once
	table    priceTable
)

// load parses the embedded pricing asset once. A parse failure is a build
// defect (the asset ships with the binary), so it panics rather than
// degrading silently; TestEmbeddedTableLoads guards against it.
func load() *priceTable {
	loadOnce.Do(func() {
		if err := json.Unmarshal(pricingJSON, &table); err != nil {
			panic(fmt.Sprintf("pricing: parse embedded pricing.json: %v", err))
		}
	})
	return &table
}

// Lookup returns the pricing for model using a prefix-based match against
// the embedded table. ok is false for unknown models, so callers can
// render "unknown" instead of a misleading $0.00.
func Lookup(model string) (ModelPricing, bool) {
	for _, m := range load().Models {
		if strings.HasPrefix(model, m.Prefix) {
			return m.ModelPricing, true
		}
	}
	return ModelPricing{}, false
}

// EstimateCost returns the estimated USD cost for a request against model
// with the given input/output token counts. ok is false when no pricing
// is known for the model, in which case cost is 0.
func EstimateCost(model string, inputTokens, outputTokens int) (cost float64, ok bool) {
	p, ok := Lookup(model)
	if !ok {
		return 0, false
	}
	return float64(inputTokens)*p.InputPerMillion/1e6 + float64(outputTokens)*p.OutputPerMillion/1e6, true
}

// FallbackDivisors returns the chars-per-token divisors for a model
// family, falling back to the generic default for unknown families.
func FallbackDivisors(model string) Divisors {
	for _, d := range load().FallbackDivisors {
		if strings.HasPrefix(model, d.Prefix) {
			return d.Divisors
		}
	}
	return load().DefaultDivisor
}

// GeneratedAt returns the date the embedded price table was generated.
// The zero time is returned if the date is missing or malformed.
func GeneratedAt() time.Time {
	t, err := time.Parse("2006-01-02", load().GeneratedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// StaleWarning returns a human-readable warning and true when the embedded
// price table is older than 90 days as of now (or its date is unreadable).
// Callers should surface it whenever they print estimated costs.
func StaleWarning(now time.Time) (string, bool) {
	gen := GeneratedAt()
	if gen.IsZero() {
		return "pricing data has no generation date; estimated costs may be stale", true
	}
	if now.Sub(gen) <= staleAfter {
		return "", false
	}
	return fmt.Sprintf("pricing data was generated on %s (more than 90 days ago); run `squadai update` to refresh it with the binary",
		gen.Format("2006-01-02")), true
}
