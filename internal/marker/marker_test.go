package marker

import (
	"strings"
	"testing"
)

// ─── OpenTag / CloseTag ─────────────────────────────────────────────────────

func TestOpenTag(t *testing.T) {
	got := OpenTag("memory")
	want := "<!-- squadai:memory -->"
	if got != want {
		t.Errorf("OpenTag = %q, want %q", got, want)
	}
}

func TestCloseTag(t *testing.T) {
	got := CloseTag("memory")
	want := "<!-- /squadai:memory -->"
	if got != want {
		t.Errorf("CloseTag = %q, want %q", got, want)
	}
}

// ─── InjectSection ──────────────────────────────────────────────────────────

func TestInjectSection_EmptyDocument_AppendsSection(t *testing.T) {
	result := InjectSection("", "memory", "memory content")
	assertContains(t, result, "<!-- squadai:memory -->")
	assertContains(t, result, "memory content")
	assertContains(t, result, "<!-- /squadai:memory -->")
}

func TestInjectSection_ExistingDocument_AppendsAtEnd(t *testing.T) {
	doc := "# My Config\n\nSome user content.\n"
	result := InjectSection(doc, "memory", "injected")

	// User content preserved.
	assertContains(t, result, "# My Config")
	assertContains(t, result, "Some user content.")
	// Section appended.
	assertContains(t, result, "<!-- squadai:memory -->")
	assertContains(t, result, "injected")
}

func TestInjectSection_ReplacesExisting(t *testing.T) {
	doc := "before\n<!-- squadai:memory -->\nold content\n<!-- /squadai:memory -->\nafter\n"
	result := InjectSection(doc, "memory", "new content")

	assertContains(t, result, "new content")
	assertNotContains(t, result, "old content")
	assertContains(t, result, "before")
	assertContains(t, result, "after")
}

func TestInjectSection_RemovesWhenContentEmpty(t *testing.T) {
	doc := "before\n<!-- squadai:memory -->\nold\n<!-- /squadai:memory -->\nafter\n"
	result := InjectSection(doc, "memory", "")

	assertNotContains(t, result, "<!-- squadai:memory -->")
	assertNotContains(t, result, "old")
	assertContains(t, result, "before")
	assertContains(t, result, "after")
}

func TestInjectSection_EmptyContent_NoSection_NoChange(t *testing.T) {
	doc := "unchanged content"
	result := InjectSection(doc, "memory", "")
	if result != doc {
		t.Errorf("expected no change, got %q", result)
	}
}

func TestInjectSection_Idempotent(t *testing.T) {
	doc := ""
	first := InjectSection(doc, "memory", "stable content")
	second := InjectSection(first, "memory", "stable content")
	if first != second {
		t.Errorf("expected idempotent result:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestInjectSection_MultipleSections(t *testing.T) {
	doc := ""
	doc = InjectSection(doc, "memory", "mem content")
	doc = InjectSection(doc, "copilot", "copilot content")

	assertContains(t, doc, "mem content")
	assertContains(t, doc, "copilot content")
	assertContains(t, doc, "<!-- squadai:memory -->")
	assertContains(t, doc, "<!-- squadai:copilot -->")
}

func TestInjectSection_UpdatesOnlyTargetSection(t *testing.T) {
	doc := "<!-- squadai:memory -->\nmem\n<!-- /squadai:memory -->\n\n<!-- squadai:copilot -->\ncop\n<!-- /squadai:copilot -->\n"

	result := InjectSection(doc, "memory", "new-mem")

	assertContains(t, result, "new-mem")
	assertContains(t, result, "cop") // copilot untouched
	assertNotContains(t, result, "\nmem\n")
}

// ─── ExtractSection ─────────────────────────────────────────────────────────

func TestExtractSection_Found(t *testing.T) {
	doc := "<!-- squadai:memory -->\nmy content\n<!-- /squadai:memory -->"
	got := ExtractSection(doc, "memory")
	if got != "my content" {
		t.Errorf("ExtractSection = %q, want %q", got, "my content")
	}
}

func TestExtractSection_NotFound(t *testing.T) {
	got := ExtractSection("no markers here", "memory")
	if got != "" {
		t.Errorf("ExtractSection = %q, want empty", got)
	}
}

// ─── HasSection ─────────────────────────────────────────────────────────────

func TestHasSection_True(t *testing.T) {
	doc := "<!-- squadai:memory -->\nstuff\n<!-- /squadai:memory -->"
	if !HasSection(doc, "memory") {
		t.Error("expected HasSection=true")
	}
}

func TestHasSection_False(t *testing.T) {
	if HasSection("no markers", "memory") {
		t.Error("expected HasSection=false")
	}
}

// ─── StripAll ───────────────────────────────────────────────────────────────

func TestStripAll_NoMarkers(t *testing.T) {
	doc := "# My Config\n\nSome user content.\n"
	got, found := StripAll(doc)
	if found {
		t.Error("expected found=false for document with no markers")
	}
	if got != doc {
		t.Errorf("expected original document returned unchanged\ngot:  %q\nwant: %q", got, doc)
	}
}

func TestStripAll_SingleSection(t *testing.T) {
	doc := "<!-- squadai:memory -->\nmanaged content\n<!-- /squadai:memory -->\n"
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	assertNotContains(t, got, "<!-- squadai:memory -->")
	assertNotContains(t, got, "<!-- /squadai:memory -->")
	assertNotContains(t, got, "managed content")
}

func TestStripAll_MultipleSections(t *testing.T) {
	doc := "<!-- squadai:memory -->\nmem\n<!-- /squadai:memory -->\n\n<!-- squadai:rules -->\nrules\n<!-- /squadai:rules -->\n"
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	assertNotContains(t, got, "<!-- squadai:memory -->")
	assertNotContains(t, got, "<!-- /squadai:memory -->")
	assertNotContains(t, got, "<!-- squadai:rules -->")
	assertNotContains(t, got, "<!-- /squadai:rules -->")
	assertNotContains(t, got, "mem")
	assertNotContains(t, got, "rules")
}

func TestStripAll_PreservesUserContent(t *testing.T) {
	doc := "# Title\n\nUser paragraph.\n\n<!-- squadai:memory -->\nmanaged\n<!-- /squadai:memory -->\n\nTrailing user text.\n"
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	assertContains(t, got, "# Title")
	assertContains(t, got, "User paragraph.")
	assertContains(t, got, "Trailing user text.")
	assertNotContains(t, got, "<!-- squadai:memory -->")
	assertNotContains(t, got, "managed")
}

func TestStripAll_EmptyDocument(t *testing.T) {
	got, found := StripAll("")
	if found {
		t.Error("expected found=false for empty document")
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestStripAll_OnlyMarkers(t *testing.T) {
	doc := "<!-- squadai:memory -->\nmanaged\n<!-- /squadai:memory -->"
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty/whitespace-only result, got %q", got)
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strContains(s, substr) {
		t.Errorf("expected string to contain %q, got:\n%s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strContains(s, substr) {
		t.Errorf("expected string NOT to contain %q, got:\n%s", substr, s)
	}
}

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
