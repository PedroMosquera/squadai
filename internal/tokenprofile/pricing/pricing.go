package pricing

import "github.com/PedroMosquera/squadai/internal/modelcatalog"

// ModelPricing holds per-model USD pricing per million tokens. The cache
// multipliers express prompt-cache pricing as a fraction of the input rate;
// zero values fall back to the catalog defaults (0.1 read, 1.25 write).
type ModelPricing struct {
	InputPerMillion      float64 `json:"input_per_million"`
	OutputPerMillion     float64 `json:"output_per_million"`
	CacheReadMultiplier  float64 `json:"cache_read_multiplier,omitempty"`
	CacheWriteMultiplier float64 `json:"cache_write_multiplier,omitempty"`
}

// Lookup returns the pricing for model from the unified model catalog
// (exact-then-longest-prefix matching, provider prefixes normalized).
// Unknown models return zero pricing.
func Lookup(model string) ModelPricing {
	p, ok := modelcatalog.Default().Pricing(model)
	if !ok {
		return ModelPricing{}
	}
	return ModelPricing{
		InputPerMillion:      p.InputPerMTok,
		OutputPerMillion:     p.OutputPerMTok,
		CacheReadMultiplier:  p.CacheReadMultiplier,
		CacheWriteMultiplier: p.CacheWriteMultiplier,
	}
}

// EstimateCost returns the estimated USD cost for a request against
// model with the given input/output token counts.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	p := Lookup(model)
	return float64(inputTokens)*p.InputPerMillion/1e6 + float64(outputTokens)*p.OutputPerMillion/1e6
}

// EstimateCostWithCache extends EstimateCost with prompt-cache accounting:
// cache reads and writes are billed as multiples of the input rate. Missing
// per-model multipliers fall back to 0.1 (read) and 1.25 (write).
func EstimateCostWithCache(model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) float64 {
	p := Lookup(model)
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
		float64(cacheWriteTokens)*p.InputPerMillion*writeMult/1e6
}
