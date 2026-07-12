// Package pricing maps model names to USD prices and chars-per-token
// fallback divisors. Prices come from the unified model catalog (embedded
// models.json, overridable via `squadai models update`); the fallback
// divisors live in the embedded pricing.json asset. Both assets are
// compiled into the binary, so `squadai update` (internal/update replaces
// the whole binary) refreshes this data alongside the code.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

//go:embed pricing.json
var pricingJSON []byte

// staleAfter is how old the catalog price data may be before StaleWarning
// reports a warning.
const staleAfter = 90 * 24 * time.Hour

// ModelPricing holds per-model USD pricing per million tokens. The cache
// multipliers express prompt-cache pricing as a fraction of the input rate;
// zero values fall back to the catalog defaults (0.1 read, 1.25 write).
type ModelPricing struct {
	InputPerMillion      float64 `json:"input_per_million"`
	OutputPerMillion     float64 `json:"output_per_million"`
	CacheReadMultiplier  float64 `json:"cache_read_multiplier,omitempty"`
	CacheWriteMultiplier float64 `json:"cache_write_multiplier,omitempty"`
}

// Divisors holds chars-per-token estimates for the character-based token
// heuristic. Code tokenizes denser than prose, so its divisor is lower.
type Divisors struct {
	Prose float64 `json:"prose_chars_per_token"`
	Code  float64 `json:"code_chars_per_token"`
}

// priceTable mirrors pricing.json. Divisor entries are ordered so that
// more specific prefixes are matched before shorter ones.
type priceTable struct {
	GeneratedAt      string `json:"generated_at"`
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

// Lookup returns the pricing for model from the unified model catalog
// (exact-then-longest-prefix matching, provider prefixes normalized).
// ok is false for unknown models, so callers can render "unknown"
// instead of a misleading $0.00.
func Lookup(model string) (ModelPricing, bool) {
	p, ok := modelcatalog.Default().Pricing(model)
	if !ok {
		return ModelPricing{}, false
	}
	return ModelPricing{
		InputPerMillion:      p.InputPerMTok,
		OutputPerMillion:     p.OutputPerMTok,
		CacheReadMultiplier:  p.CacheReadMultiplier,
		CacheWriteMultiplier: p.CacheWriteMultiplier,
	}, true
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

// EstimateCostWithCache extends EstimateCost with prompt-cache accounting:
// cache reads and writes are billed as multiples of the input rate. Missing
// per-model multipliers fall back to 0.1 (read) and 1.25 (write). ok is
// false when no pricing is known for the model, in which case cost is 0.
func EstimateCostWithCache(model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) (cost float64, ok bool) {
	p, ok := Lookup(model)
	if !ok {
		return 0, false
	}
	readMult := p.CacheReadMultiplier
	if readMult == 0 {
		readMult = modelcatalog.DefaultCacheReadMultiplier
	}
	writeMult := p.CacheWriteMultiplier
	if writeMult == 0 {
		writeMult = modelcatalog.DefaultCacheWriteMultiplier
	}
	return float64(inputTokens)*p.InputPerMillion/1e6 +
		float64(outputTokens)*p.OutputPerMillion/1e6 +
		float64(cacheReadTokens)*p.InputPerMillion*readMult/1e6 +
		float64(cacheWriteTokens)*p.InputPerMillion*writeMult/1e6, true
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

// GeneratedAt returns the date of the effective model catalog price data.
// The zero time is returned if the date is missing or malformed.
func GeneratedAt() time.Time {
	return modelcatalog.Default().Updated()
}

// StaleWarning returns a human-readable warning and true when the catalog
// price data is older than 90 days as of now (or its date is unreadable).
// Callers should surface it whenever they print estimated costs.
func StaleWarning(now time.Time) (string, bool) {
	gen := GeneratedAt()
	if gen.IsZero() {
		return "model catalog has no updated date; estimated costs may be stale", true
	}
	if now.Sub(gen) <= staleAfter {
		return "", false
	}
	return fmt.Sprintf("model catalog price data is dated %s (more than 90 days ago); run `squadai models update` to refresh it",
		gen.Format("2006-01-02")), true
}
