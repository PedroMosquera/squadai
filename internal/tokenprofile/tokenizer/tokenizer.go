package tokenizer

import (
	"strings"
	"sync"

	"github.com/PedroMosquera/squadai/internal/modelcatalog"
	"github.com/pkoukk/tiktoken-go"
)

// Counter counts tokens for a given text.
type Counter interface {
	Count(text string) int
}

// CounterFunc is a function adapter implementing Counter.
type CounterFunc func(text string) int

// Count calls f(text).
func (f CounterFunc) Count(text string) int { return f(text) }

// FallbackCounter estimates tokens using a 4 chars/token heuristic.
type FallbackCounter struct{}

// Count returns ApproxCount(text).
func (FallbackCounter) Count(text string) int { return ApproxCount(text) }

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

// resolveEncodingName maps a model name to a tiktoken encoding name. It
// consults tiktoken's own model and prefix maps first, then the unified
// model catalog (per-model encodings and encoding_prefixes, which cover
// claude-, gpt-5, o4, gemini-, ...). It returns "" when no encoding is
// known, in which case callers should use FallbackCounter.
func resolveEncodingName(model string) string {
	if name, ok := tiktoken.MODEL_TO_ENCODING[model]; ok {
		return name
	}
	for prefix, name := range tiktoken.MODEL_PREFIX_TO_ENCODING {
		if strings.HasPrefix(model, prefix) {
			return name
		}
	}
	return modelcatalog.Default().Encoding(model)
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
// loaded, it falls back to the chars/4 heuristic.
type tiktokenCounter struct {
	encodingName string
	once         sync.Once
	enc          *tiktoken.Tiktoken
}

// Count returns the number of tokens in text. If the BPE ranks could
// not be loaded, it falls back to ApproxCount.
func (c *tiktokenCounter) Count(text string) int {
	c.once.Do(func() { c.enc = getEncoder(c.encodingName) })
	if c.enc == nil {
		return ApproxCount(text)
	}
	return len(c.enc.EncodeOrdinary(text))
}

// ForModel returns a Counter for the given model. Known models (and
// known prefixes) are backed by tiktoken; unknown models use
// FallbackCounter. The tiktoken encoder is loaded lazily, so ForModel
// itself never performs network I/O.
func ForModel(model string) Counter {
	name := resolveEncodingName(model)
	if name == "" {
		return FallbackCounter{}
	}
	return &tiktokenCounter{encodingName: name}
}

// CountBytes returns the token count for data using the counter for
// the given model.
func CountBytes(model string, data []byte) int {
	return ForModel(model).Count(string(data))
}
