package pricing

import "github.com/PedroMosquera/squadai/internal/modelcatalog"

// ModelPricing holds per-model USD pricing per million tokens.
type ModelPricing struct {
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
}

// Lookup returns the pricing for model from the unified model catalog
// (exact-then-longest-prefix matching, provider prefixes normalized).
// Unknown models return zero pricing.
func Lookup(model string) ModelPricing {
	p, ok := modelcatalog.Default().Pricing(model)
	if !ok {
		return ModelPricing{}
	}
	return ModelPricing{InputPerMillion: p.InputPerMTok, OutputPerMillion: p.OutputPerMTok}
}

// EstimateCost returns the estimated USD cost for a request against
// model with the given input/output token counts.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	p := Lookup(model)
	return float64(inputTokens)*p.InputPerMillion/1e6 + float64(outputTokens)*p.OutputPerMillion/1e6
}
