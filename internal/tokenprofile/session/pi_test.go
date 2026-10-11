package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Expected names follow getDefaultSessionDirPath in Pi's session-manager.js:
// drop one leading separator, replace / \ and : with dashes, wrap in "--".
func TestPiSessionDir_MatchesPiEncoding(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	got := PiSessionDir("/home/u", "/Users/x/web.app")
	want := filepath.Join("/home/u", ".pi", "agent", "sessions", "--Users-x-web.app--")
	if got != want {
		t.Errorf("PiSessionDir = %q, want %q", got, want)
	}
	t.Setenv("PI_CODING_AGENT_DIR", "/custom/agent")
	if got := PiSessionDir("/home/u", "/p"); got != filepath.Join("/custom/agent", "sessions", "--p--") {
		t.Errorf("PiSessionDir with PI_CODING_AGENT_DIR = %q", got)
	}
}

func TestPiUsage_CountsAllUsageEntriesAndSkipsStaleFiles(t *testing.T) {
	dir := t.TempDir()
	current := `{"type":"session","version":3,"id":"s1","cwd":"/p"}
{"type":"message","message":{"role":"assistant","model":"m1","usage":{"input":100,"output":50}}}
{"type":"message","message":{"role":"toolResult","usage":{"input":10,"output":5}}}
{"type":"usage","kind":"cache_warm","model":"m1","usage":{"input":1,"output":2}}
{"type":"compaction","summary":"s","firstKeptEntryId":"x","usage":{"input":20,"output":10}}
not json
`
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "b.jsonl")
	if err := os.WriteFile(stale, []byte(`{"type":"message","message":{"role":"assistant","model":"m1","usage":{"input":9000,"output":9000}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	agg := PiUsage(dir, 24*time.Hour)
	if agg.Total.TotalTokens != 198 {
		t.Errorf("daily total = %d, want 198 (stale file excluded)", agg.Total.TotalTokens)
	}
	if got := PiSessionTokens(filepath.Join(dir, "a.jsonl")); got != 198 {
		t.Errorf("PiSessionTokens = %d, want 198", got)
	}
}
