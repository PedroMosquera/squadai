package tokenizer

import (
	"math"
	"strings"
	"sync"

	"github.com/PedroMosquera/squadai/internal/modelcatalog"
	"github.com/pkoukk/tiktoken-go"

	"github.com/PedroMosquera/squadai/internal/tokenprofile/pricing"
)

// Counter counts tokens for a given text.
type Counter interface {
	Count(text string) int
	// Approximate reports whether Count returns heuristic estimates
	// rather than output from the model's own tokenizer. Callers must
	// mark approximate counts as such (e.g. a "~" prefix) in any output.
	Approximate() bool
}

// CounterFunc is a function adapter implementing Counter.
type CounterFunc func(text string) int

// Count calls f(text).
func (f CounterFunc) Count(text string) int { return f(text) }

// Approximate always reports true: an arbitrary function gives no
// guarantee of being a real tokenizer.
func (CounterFunc) Approximate() bool { return true }

// FallbackCounter estimates tokens using a chars-per-token heuristic.
// The zero value uses the legacy flat 4 chars/token; set Divisors (from
// pricing.FallbackDivisors) for a per-model-family calibration.
type FallbackCounter struct {
	Divisors pricing.Divisors
}

// Count estimates tokens for text, picking the code divisor when the
// text is symbol-dense and the prose divisor otherwise.
func (f FallbackCounter) Count(text string) int {
	d := f.Divisors.Prose
	if looksLikeCode(text) && f.Divisors.Code > 0 {
		d = f.Divisors.Code
	}
	if d <= 0 {
		return ApproxCount(text)
	}
	if len(text) == 0 {
		return 0
	}
	return int(math.Ceil(float64(len(text)) / d))
}

// Approximate always reports true for the heuristic counter.
func (FallbackCounter) Approximate() bool { return true }

// looksLikeCode reports whether text is symbol-dense enough to tokenize
// like code rather than prose. BPE tokenizers emit more tokens per char
// for code, so code-like content gets the lower chars/token divisor.
func looksLikeCode(text string) bool {
	if len(text) == 0 {
		return false
	}
	symbols := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{', '}', '[', ']', '(', ')', ';', '=', '<', '>', '|', '\\':
			symbols++
		}
	}
	return float64(symbols)/float64(len(text)) > 0.02
}

// ApproxCount estimates tokens for text using a 4 chars/token heuristic
// (ceiling division). Returns 0 for empty text.
func ApproxCount(text string) int {
	n := len(text)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// encoderCache caches loaded tiktoken encoders keyed by encoding name.
var encoderCache sync.Map // map[string]*tiktoken.Tiktoken

// resolveEncoding maps a model name to a tiktoken encoding name. It
// consults tiktoken's own model and prefix maps first, then the unified
// model catalog (per-model encodings and encoding_prefixes, which cover
// claude-, gpt-5, o4, gemini-, ...). It returns "" when no encoding is
// known, in which case callers should use FallbackCounter. The approx
// result reports that the encoding is a proxy rather than the model's
// real tokenizer (e.g. o200k_base for Claude): such counts must be
// presented as estimates.
func resolveEncoding(model string) (name string, approx bool) {
	if name, ok := tiktoken.MODEL_TO_ENCODING[model]; ok {
		return name, false
	}
	for prefix, name := range tiktoken.MODEL_PREFIX_TO_ENCODING {
		if strings.HasPrefix(model, prefix) {
			return name, false
		}
	}
	return modelcatalog.Default().EncodingApprox(model)
}

// getEncoder returns a cached tiktoken encoder for the given encoding
// name, loading it on first use. It returns nil if the BPE ranks cannot
// be loaded (e.g. no network on first use).
func getEncoder(encodingName string) *tiktoken.Tiktoken {
	if v, ok := encoderCache.Load(encodingName); ok {
		if t, ok := v.(*tiktoken.Tiktoken); ok {
			return t
		}
	}
	enc, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil
	}
	actual, _ := encoderCache.LoadOrStore(encodingName, enc)
	if t, ok := actual.(*tiktoken.Tiktoken); ok {
		return t
	}
	return enc
}

// tiktokenCounter is a Counter backed by a tiktoken encoder. The BPE
// ranks are loaded lazily on the first Count call; if they cannot be
// loaded, it falls back to the calibrated chars/token heuristic. proxy
// marks encodings that stand in for a model's real tokenizer (per the
// model catalog), so their counts are reported as approximate.
type tiktokenCounter struct {
	encodingName string
	proxy        bool
	fallback     FallbackCounter
	once         sync.Once
	enc          *tiktoken.Tiktoken
}

// Count returns the number of tokens in text. If the BPE ranks could
// not be loaded, it falls back to the heuristic counter.
func (c *tiktokenCounter) Count(text string) int {
	c.once.Do(func() { c.enc = getEncoder(c.encodingName) })
	if c.enc == nil {
		return c.fallback.Count(text)
	}
	return len(c.enc.EncodeOrdinary(text))
}

// Approximate reports whether counts are estimates: either the encoding
// is a proxy for the model's real tokenizer, or the heuristic fallback
// is in use. It forces the lazy encoder load so the answer matches what
// Count does.
func (c *tiktokenCounter) Approximate() bool {
	c.once.Do(func() { c.enc = getEncoder(c.encodingName) })
	return c.proxy || c.enc == nil
}

// ForModel returns a Counter for the given model. Models with a known
// encoding (via tiktoken's maps or the model catalog) are backed by
// tiktoken; encodings the catalog marks as a proxy (e.g. o200k_base for
// Claude) report Approximate() = true. Models with no known encoding use
// a FallbackCounter calibrated per model family. The tiktoken encoder is
// loaded lazily, so ForModel itself never performs network I/O.
func ForModel(model string) Counter {
	fb := FallbackCounter{Divisors: pricing.FallbackDivisors(model)}
	name, approx := resolveEncoding(model)
	if name == "" {
		return fb
	}
	return &tiktokenCounter{encodingName: name, proxy: approx, fallback: fb}
}

// CountBytes returns the token count for data using the counter for
// the given model.
func CountBytes(model string, data []byte) int {
	return ForModel(model).Count(string(data))
}
