package modelcatalog

import "strings"

// Price holds per-model USD pricing per million tokens.
type Price struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

// Normalize strips a provider prefix from a model reference so that
// provider-qualified names (anthropic/claude-sonnet-4-6, openai/gpt-5-mini)
// resolve against bare catalog IDs.
func Normalize(model string) string {
	if i := strings.LastIndex(model, "/"); i >= 0 {
		return model[i+1:]
	}
	return model
}

// Resolve finds the catalog row for model using exact-then-longest-prefix
// matching over canonical IDs and aliases. Exact matches always win, so a
// current-generation ID (claude-sonnet-4-6) can never be captured by a
// shorter legacy row (claude-sonnet-4). Prefix matches pick the longest
// matching ID/alias.
func (c *Catalog) Resolve(model string) (id string, m Model, ok bool) {
	name := Normalize(model)
	if name == "" {
		return "", Model{}, false
	}

	// Exact match: canonical ID first, then alias.
	if m, ok := c.models[name]; ok {
		return name, m, true
	}
	if canonical, ok := c.aliases[name]; ok {
		return canonical, c.models[canonical], true
	}

	// Longest-prefix match over IDs and aliases.
	best := ""
	bestID := ""
	for id := range c.models {
		if strings.HasPrefix(name, id) && len(id) > len(best) {
			best, bestID = id, id
		}
	}
	for alias, canonical := range c.aliases {
		if strings.HasPrefix(name, alias) && len(alias) > len(best) {
			best, bestID = alias, canonical
		}
	}
	if bestID == "" {
		return "", Model{}, false
	}
	return bestID, c.models[bestID], true
}

// Known reports whether model resolves to a catalog row.
func (c *Catalog) Known(model string) bool {
	_, _, ok := c.Resolve(model)
	return ok
}

// Pricing returns the pricing for model. Unknown models return ok=false.
func (c *Catalog) Pricing(model string) (Price, bool) {
	_, m, ok := c.Resolve(model)
	if !ok {
		return Price{}, false
	}
	return Price{InputPerMTok: m.InputPerMTok, OutputPerMTok: m.OutputPerMTok}, true
}

// ContextWindow returns the context window size for model (0 when unknown).
func (c *Catalog) ContextWindow(model string) int {
	_, m, ok := c.Resolve(model)
	if !ok {
		return 0
	}
	return m.ContextWindow
}

// Encoding returns the tokenizer encoding name for model. It first resolves
// the model to a catalog row, then falls back to the longest matching
// encoding prefix. Returns "" when no encoding is known.
func (c *Catalog) Encoding(model string) string {
	if _, m, ok := c.Resolve(model); ok && m.Encoding != "" {
		return m.Encoding
	}
	name := Normalize(model)
	best := ""
	bestEnc := ""
	for _, ep := range c.encodingPrefixes {
		if strings.HasPrefix(name, ep.Prefix) && len(ep.Prefix) > len(best) {
			best, bestEnc = ep.Prefix, ep.Encoding
		}
	}
	return bestEnc
}

// fallbackAdapter is used for adapter names without a catalog entry,
// mirroring the historical behavior of falling back to the OpenCode resolver.
const fallbackAdapter = "opencode"

// TierModel returns the concrete model ID for an adapter and tier
// (premium, standard, cheap). Unknown adapters fall back to the opencode
// entry; unknown tiers fall back to the adapter's standard tier.
func (c *Catalog) TierModel(adapterID, tier string) string {
	entry, ok := c.adapters[adapterID]
	if !ok {
		entry, ok = c.adapters[fallbackAdapter]
		if !ok {
			return ""
		}
	}
	if m, ok := entry.Tiers[tier]; ok && m != "" {
		return m
	}
	return entry.Tiers["standard"]
}

// Hint returns the prompt-hint string for an adapter and tier. Unknown
// adapters or tiers return "".
func (c *Catalog) Hint(adapterID, tier string) string {
	entry, ok := c.adapters[adapterID]
	if !ok {
		return ""
	}
	return entry.Hints[tier]
}
