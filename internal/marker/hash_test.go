package marker

import (
	"strings"
	"testing"
)

// ─── Hash tags ──────────────────────────────────────────────────────────────

func TestHashOpenTag(t *testing.T) {
	got := HashOpenTag("mcp")
	want := "# squadai:mcp:start"
	if got != want {
		t.Errorf("HashOpenTag = %q, want %q", got, want)
	}
}

func TestHashCloseTag(t *testing.T) {
	got := HashCloseTag("mcp")
	want := "# squadai:mcp:end"
	if got != want {
		t.Errorf("HashCloseTag = %q, want %q", got, want)
	}
}

// ─── InjectHashSection ──────────────────────────────────────────────────────

func TestInjectHashSection_EmptyDocument(t *testing.T) {
	got := InjectHashSection("", "mcp", "[mcp_servers.x]\ncommand = \"npx\"")
	assertContains(t, got, "# squadai:mcp:start")
	assertContains(t, got, "[mcp_servers.x]")
	assertContains(t, got, "# squadai:mcp:end")
}

func TestInjectHashSection_AppendsToExistingContent(t *testing.T) {
	doc := "model = \"gpt-5.2\"\n"
	got := InjectHashSection(doc, "mcp", "[mcp_servers.x]\ncommand = \"npx\"")
	if !strings.HasPrefix(got, "model = \"gpt-5.2\"\n") {
		t.Errorf("user content should be preserved at top, got:\n%s", got)
	}
	assertContains(t, got, "# squadai:mcp:start")
	assertContains(t, got, "# squadai:mcp:end")
}

func TestInjectHashSection_ReplacesExistingSection(t *testing.T) {
	doc := InjectHashSection("user = true\n", "mcp", "old content")
	got := InjectHashSection(doc, "mcp", "new content")
	assertContains(t, got, "new content")
	assertNotContains(t, got, "old content")
	assertContains(t, got, "user = true")
	if strings.Count(got, "# squadai:mcp:start") != 1 {
		t.Errorf("expected exactly one open tag, got:\n%s", got)
	}
}

func TestInjectHashSection_EmptyContentRemovesSection(t *testing.T) {
	doc := InjectHashSection("user = true\n", "mcp", "managed")
	got := InjectHashSection(doc, "mcp", "")
	assertNotContains(t, got, "# squadai:mcp:start")
	assertNotContains(t, got, "managed")
	assertContains(t, got, "user = true")
}

func TestInjectHashSection_Idempotent(t *testing.T) {
	once := InjectHashSection("", "mcp", "content")
	twice := InjectHashSection(once, "mcp", "content")
	if once != twice {
		t.Errorf("injection should be idempotent:\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

// ─── ExtractHashSection / HasHashSection ────────────────────────────────────

func TestExtractHashSection_RoundTrip(t *testing.T) {
	content := "[mcp_servers.ctx]\ncommand = \"npx\"\nargs = [\"-y\", \"pkg\"]"
	doc := InjectHashSection("# user comment\n", "mcp", content)
	got := ExtractHashSection(doc, "mcp")
	if got != content {
		t.Errorf("ExtractHashSection = %q, want %q", got, content)
	}
}

func TestExtractHashSection_NotFound(t *testing.T) {
	if got := ExtractHashSection("no markers here", "mcp"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestHasHashSection(t *testing.T) {
	doc := InjectHashSection("", "mcp", "x")
	if !HasHashSection(doc, "mcp") {
		t.Error("expected HasHashSection=true after injection")
	}
	if HasHashSection(doc, "other") {
		t.Error("expected HasHashSection=false for other section")
	}
}

// ─── StripAll with hash sections ────────────────────────────────────────────

func TestStripAll_HashSection_PreservesUserContent(t *testing.T) {
	doc := InjectHashSection("model = \"gpt-5.2\"\n", "mcp", "[mcp_servers.x]\ncommand = \"npx\"")
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	assertContains(t, got, "model = \"gpt-5.2\"")
	assertNotContains(t, got, "squadai:mcp")
	assertNotContains(t, got, "[mcp_servers.x]")
}

func TestStripAll_HashSection_OnlyManaged(t *testing.T) {
	doc := InjectHashSection("", "mcp", "[mcp_servers.x]\ncommand = \"npx\"")
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty/whitespace-only result, got %q", got)
	}
}

func TestStripAll_MixedHTMLAndHashSections(t *testing.T) {
	doc := "user text\n\n<!-- squadai:memory -->\nmem\n<!-- /squadai:memory -->\n"
	doc = InjectHashSection(doc, "mcp", "[mcp_servers.x]")
	got, found := StripAll(doc)
	if !found {
		t.Error("expected found=true")
	}
	assertContains(t, got, "user text")
	assertNotContains(t, got, "squadai:memory")
	assertNotContains(t, got, "squadai:mcp")
}

func TestStripAll_NoHashMarkers_ReturnsFalse(t *testing.T) {
	doc := "# just a comment\nmodel = \"gpt-5.2\"\n"
	got, found := StripAll(doc)
	if found {
		t.Error("expected found=false")
	}
	if got != doc {
		t.Errorf("document should be unchanged, got %q", got)
	}
}
