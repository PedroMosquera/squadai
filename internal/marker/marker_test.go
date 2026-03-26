package marker

import (
	"testing"
)

// ─── OpenTag / CloseTag ─────────────────────────────────────────────────────

func TestOpenTag(t *testing.T) {
	got := OpenTag("memory")
	want := "<!-- agent-manager:memory -->"
	if got != want {
		t.Errorf("OpenTag = %q, want %q", got, want)
	}
}

func TestCloseTag(t *testing.T) {
	got := CloseTag("memory")
	want := "<!-- /agent-manager:memory -->"
	if got != want {
		t.Errorf("CloseTag = %q, want %q", got, want)
	}
}

// ─── InjectSection ──────────────────────────────────────────────────────────

func TestInjectSection_EmptyDocument_AppendsSection(t *testing.T) {
	result := InjectSection("", "memory", "memory content")
	assertContains(t, result, "<!-- agent-manager:memory -->")
	assertContains(t, result, "memory content")
	assertContains(t, result, "<!-- /agent-manager:memory -->")
}

func TestInjectSection_ExistingDocument_AppendsAtEnd(t *testing.T) {
	doc := "# My Config\n\nSome user content.\n"
	result := InjectSection(doc, "memory", "injected")

	// User content preserved.
	assertContains(t, result, "# My Config")
	assertContains(t, result, "Some user content.")
	// Section appended.
	assertContains(t, result, "<!-- agent-manager:memory -->")
	assertContains(t, result, "injected")
}

func TestInjectSection_ReplacesExisting(t *testing.T) {
	doc := "before\n<!-- agent-manager:memory -->\nold content\n<!-- /agent-manager:memory -->\nafter\n"
	result := InjectSection(doc, "memory", "new content")

	assertContains(t, result, "new content")
	assertNotContains(t, result, "old content")
	assertContains(t, result, "before")
	assertContains(t, result, "after")
}

func TestInjectSection_RemovesWhenContentEmpty(t *testing.T) {
	doc := "before\n<!-- agent-manager:memory -->\nold\n<!-- /agent-manager:memory -->\nafter\n"
	result := InjectSection(doc, "memory", "")

	assertNotContains(t, result, "<!-- agent-manager:memory -->")
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
	assertContains(t, doc, "<!-- agent-manager:memory -->")
	assertContains(t, doc, "<!-- agent-manager:copilot -->")
}

func TestInjectSection_UpdatesOnlyTargetSection(t *testing.T) {
	doc := "<!-- agent-manager:memory -->\nmem\n<!-- /agent-manager:memory -->\n\n<!-- agent-manager:copilot -->\ncop\n<!-- /agent-manager:copilot -->\n"

	result := InjectSection(doc, "memory", "new-mem")

	assertContains(t, result, "new-mem")
	assertContains(t, result, "cop") // copilot untouched
	assertNotContains(t, result, "\nmem\n")
}

// ─── ExtractSection ─────────────────────────────────────────────────────────

func TestExtractSection_Found(t *testing.T) {
	doc := "<!-- agent-manager:memory -->\nmy content\n<!-- /agent-manager:memory -->"
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
	doc := "<!-- agent-manager:memory -->\nstuff\n<!-- /agent-manager:memory -->"
	if !HasSection(doc, "memory") {
		t.Error("expected HasSection=true")
	}
}

func TestHasSection_False(t *testing.T) {
	if HasSection("no markers", "memory") {
		t.Error("expected HasSection=false")
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
