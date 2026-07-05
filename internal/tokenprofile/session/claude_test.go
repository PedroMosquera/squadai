package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeClaudeTranscript writes a .jsonl transcript under
// ~/.claude/projects/<slug>/ rooted at home.
func writeClaudeTranscript(t *testing.T, home, slug, name, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func assistantLine(model string, in, out, cacheRead, cacheWrite int) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`,
		model, in, out, cacheRead, cacheWrite)
}

func TestClaudeProjectSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/Users/x/workspace/personal/squadai", "-Users-x-workspace-personal-squadai"},
		{"/Users/x/web.app", "-Users-x-web-app"},
		{"/Users/x/my_repo", "-Users-x-my-repo"},
		{"/Users/x/al-ready-dashed", "-Users-x-al-ready-dashed"},
	}
	for _, c := range cases {
		if got := ClaudeProjectSlug(c.in); got != c.want {
			t.Errorf("ClaudeProjectSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAggregate_ClaudeTranscripts(t *testing.T) {
	home := t.TempDir()
	transcript := strings.Join([]string{
		`{"type":"user","message":{"content":"hi"}}`,
		assistantLine("claude-sonnet-4-6", 100, 50, 1000, 200),
		assistantLine("claude-sonnet-4-6", 200, 100, 2000, 300),
		`{"type":"system","subtype":"init"}`,
	}, "\n")
	writeClaudeTranscript(t, home, "-Users-x-proj", "s1.jsonl", transcript)
	writeClaudeTranscript(t, home, "-Users-x-proj", "s2.jsonl",
		assistantLine("claude-sonnet-4-6", 10, 5, 0, 0))

	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	u := agg.ByModel["claude-sonnet-4-6"]
	if u.InputTokens != 310 || u.OutputTokens != 155 {
		t.Errorf("in/out = %d/%d, want 310/155", u.InputTokens, u.OutputTokens)
	}
	if u.CacheReadTokens != 3000 || u.CacheCreationTokens != 500 {
		t.Errorf("cache read/write = %d/%d, want 3000/500", u.CacheReadTokens, u.CacheCreationTokens)
	}
	if u.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2 (one per .jsonl file)", u.SessionCount)
	}
	if u.EstimatedCost <= 0 {
		t.Errorf("EstimatedCost = %v, want > 0", u.EstimatedCost)
	}
	if agg.Total.CacheReadTokens != 3000 || agg.Total.CacheCreationTokens != 500 {
		t.Errorf("total cache = %d/%d, want 3000/500",
			agg.Total.CacheReadTokens, agg.Total.CacheCreationTokens)
	}
}

func TestAggregate_ClaudeMalformedLinesSkipped(t *testing.T) {
	home := t.TempDir()
	transcript := strings.Join([]string{
		`{not valid json`,
		``,
		`{"type":"assistant"}`, // no usage — skipped
		`{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":"oops"}}}`,
		assistantLine("claude-sonnet-4-6", 42, 7, 0, 0),
	}, "\n")
	writeClaudeTranscript(t, home, "-Users-x-proj", "s1.jsonl", transcript)

	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	u := agg.ByModel["claude-sonnet-4-6"]
	if u.InputTokens != 42 || u.OutputTokens != 7 || u.SessionCount != 1 {
		t.Errorf("usage = %+v, want in=42 out=7 sessions=1", u)
	}
}

func TestAggregate_ClaudeCutoffHonored(t *testing.T) {
	home := t.TempDir()
	writeClaudeTranscript(t, home, "-Users-x-proj", "recent.jsonl",
		assistantLine("claude-sonnet-4-6", 100, 50, 0, 0))
	old := writeClaudeTranscript(t, home, "-Users-x-proj", "old.jsonl",
		assistantLine("claude-sonnet-4-6", 999, 999, 0, 0))
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	agg, err := Aggregate(home, AggregateOptions{Since: time.Hour})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	u := agg.ByModel["claude-sonnet-4-6"]
	if u.InputTokens != 100 || u.SessionCount != 1 {
		t.Errorf("usage = %+v, want only the recent transcript counted", u)
	}
}

func TestAggregate_ClaudeSlugFilter(t *testing.T) {
	home := t.TempDir()
	projectDir := "/Users/x/proj.web"
	writeClaudeTranscript(t, home, "-Users-x-proj-web", "match.jsonl",
		assistantLine("claude-sonnet-4-6", 100, 50, 0, 0))
	writeClaudeTranscript(t, home, "-Users-x-other", "other.jsonl",
		assistantLine("claude-sonnet-4-6", 999, 999, 0, 0))

	agg, err := Aggregate(home, AggregateOptions{ProjectDir: projectDir})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	u := agg.ByModel["claude-sonnet-4-6"]
	if u.InputTokens != 100 || u.SessionCount != 1 {
		t.Errorf("usage = %+v, want only the matching project slug counted", u)
	}

	// Without a project filter, both projects count.
	all, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if got := all.ByModel["claude-sonnet-4-6"].SessionCount; got != 2 {
		t.Errorf("unfiltered SessionCount = %d, want 2", got)
	}
}

func TestAggregate_ClaudeMultipleModelsInOneTranscript(t *testing.T) {
	home := t.TempDir()
	transcript := strings.Join([]string{
		assistantLine("claude-sonnet-4-6", 100, 50, 0, 0),
		assistantLine("claude-haiku-4-5", 20, 10, 0, 0),
		assistantLine("claude-sonnet-4-6", 100, 50, 0, 0),
	}, "\n")
	writeClaudeTranscript(t, home, "-Users-x-proj", "s1.jsonl", transcript)

	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if got := agg.ByModel["claude-sonnet-4-6"]; got.InputTokens != 200 || got.SessionCount != 1 {
		t.Errorf("sonnet = %+v, want in=200 sessions=1", got)
	}
	if got := agg.ByModel["claude-haiku-4-5"]; got.InputTokens != 20 || got.SessionCount != 1 {
		t.Errorf("haiku = %+v, want in=20 sessions=1", got)
	}
}
