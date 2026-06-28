package pricing

import "strings"

// ModelPricing holds per-model USD pricing per million tokens.
type ModelPricing struct {
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
}

// modelPricing is the known 2025 price table. Entries are ordered so
// that more specific prefixes are matched before shorter ones.
var modelPricing = []struct {
	prefix  string
	pricing ModelPricing
}{
	{"claude-opus-4", ModelPricing{15, 75}},
	{"claude-sonnet-4", ModelPricing{3, 15}},
	{"claude-haiku-3.5", ModelPricing{0.80, 4}},
	{"gpt-4.1-mini", ModelPricing{0.40, 1.60}},
	{"gpt-4.1", ModelPricing{2, 8}},
	{"gpt-4o", ModelPricing{2.50, 10}},
	{"gpt-4-turbo", ModelPricing{10, 30}},
	{"gpt-3.5-turbo", ModelPricing{0.50, 1.50}},
}

// Lookup returns the pricing for model using a prefix-based match
// against the known table. Unknown models return zero pricing.
func Lookup(model string) ModelPricing {
	for _, m := range modelPricing {
		if strings.HasPrefix(model, m.prefix) {
			return m.pricing
		}
	}
	return ModelPricing{}
}

// EstimateCost returns the estimated USD cost for a request against
// model with the given input/output token counts.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	p := Lookup(model)
	return float64(inputTokens)*p.InputPerMillion/1e6 + float64(outputTokens)*p.OutputPerMillion/1e6
}
