package tokenizer

import (
	"testing"

	"github.com/pkoukk/tiktoken-go"
)

func TestForModel_KnownModels(t *testing.T) {
	models := []string{"claude-sonnet-4", "gpt-4o", "gpt-4", "gpt-3.5-turbo"}
	for _, m := range models {
		c := ForModel(m)
		if _, ok := c.(FallbackCounter); ok {
			t.Errorf("ForModel(%q) = FallbackCounter, want a real tokenizer", m)
		}
	}
}

func TestForModel_UnknownModel(t *testing.T) {
	c := ForModel("some-unknown-model-xyz")
	if _, ok := c.(FallbackCounter); !ok {
		t.Errorf("ForModel(unknown) = %T, want FallbackCounter", c)
	}
}

func TestForModel_CatalogResolvedModels(t *testing.T) {
	// Current-generation models resolve encodings through the model catalog
	// (per-model encodings + encoding_prefixes).
	models := []string{
		"claude-fable-5",
		"claude-sonnet-4-6",
		"gpt-5.2",
		"gpt-5-mini",
		"o4",
		"gemini-3-pro",
		"gemini-3-flash",
		"gemini-3-something-new", // prefix match, no exact row
		"anthropic/claude-fable-5",
	}
	for _, m := range models {
		c := ForModel(m)
		if _, ok := c.(FallbackCounter); ok {
			t.Errorf("ForModel(%q) = FallbackCounter, want a real tokenizer", m)
		}
	}
}

func TestForModel_JunkFallsBack(t *testing.T) {
	for _, m := range []string{"", "totally-made-up", "acme/quantum-brain-7"} {
		c := ForModel(m)
		if _, ok := c.(FallbackCounter); !ok {
			t.Errorf("ForModel(%q) = %T, want FallbackCounter", m, c)
		}
	}
}

func TestFallbackCounter(t *testing.T) {
	c := FallbackCounter{}
	if got := c.Count(""); got != 0 {
		t.Errorf("Count(\"\") = %d, want 0", got)
	}
	text := "hello world"
	if got := c.Count(text); got != (len(text)+3)/4 {
		t.Errorf("Count(%q) = %d, want %d", text, got, (len(text)+3)/4)
	}
}

func TestCount_EmptyString(t *testing.T) {
	for _, m := range []string{"gpt-4o", "claude-sonnet-4", "some-unknown-model"} {
		if got := ForModel(m).Count(""); got != 0 {
			t.Errorf("Count(\"\") for %s = %d, want 0", m, got)
		}
	}
}

func TestCount_NonEmpty(t *testing.T) {
	c := ForModel("gpt-4o")
	if got := c.Count("The quick brown fox jumps over the lazy dog"); got <= 0 {
		t.Errorf("Count(non-empty) = %d, want > 0", got)
	}
}

func TestCountBytes(t *testing.T) {
	text := []byte("The quick brown fox jumps over the lazy dog")
	want := ForModel("gpt-4o").Count(string(text))
	if got := CountBytes("gpt-4o", text); got != want {
		t.Errorf("CountBytes = %d, want %d", got, want)
	}
}

func TestCount_RealTokenizer(t *testing.T) {
	// tiktoken downloads BPE files on first use; skip if unavailable.
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		t.Skipf("tiktoken BPE unavailable: %v", err)
	}
	const text = "The quick brown fox jumps over the lazy dog. Tokenization differs from a naive character heuristic."
	real := len(enc.EncodeOrdinary(text))
	fallback := ApproxCount(text)
	if real == fallback {
		t.Skip("real tokenizer count equals fallback; cannot distinguish")
	}
	c := ForModel("gpt-4")
	if got := c.Count(text); got != real {
		t.Errorf("ForModel(gpt-4).Count = %d, want real tokenizer %d", got, real)
	}
}
