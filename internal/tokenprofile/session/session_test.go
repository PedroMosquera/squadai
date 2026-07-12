package session

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeSession writes a session file under the OpenCode sessions dir
// rooted at home, creating directories as needed.
func writeSession(t *testing.T, home, name, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".local/share/opencode/sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAggregate_NoSessionDir(t *testing.T) {
	home := t.TempDir()
	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if agg == nil || agg.Total.SessionCount != 0 {
		t.Errorf("expected empty aggregation, got %+v", agg)
	}
	if agg.Period != "all" {
		t.Errorf("Period = %q, want %q", agg.Period, "all")
	}
}

func TestAggregate_EmptySessionDir(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".local/share/opencode/sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if agg.Total.SessionCount != 0 {
		t.Errorf("expected 0 sessions, got %d", agg.Total.SessionCount)
	}
}

func TestAggregate_WithSessions(t *testing.T) {
	home := t.TempDir()
	writeSession(t, home, "s1.json", `{"model":"claude-sonnet-4","usage":{"input_tokens":1000,"output_tokens":500}}`)
	writeSession(t, home, "s2.json", `{"model":"claude-sonnet-4","usage":{"input_tokens":2000,"output_tokens":1000}}`)
	writeSession(t, home, "s3.json", `{"model":"gpt-4o","input_tokens":500,"output_tokens":250}`)

	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}

	cs := agg.ByModel["claude-sonnet-4"]
	if cs.InputTokens != 3000 || cs.OutputTokens != 1500 || cs.SessionCount != 2 {
		t.Errorf("claude-sonnet-4 = %+v, want in=3000 out=1500 count=2", cs)
	}
	if cs.TotalTokens != 4500 {
		t.Errorf("claude-sonnet-4 TotalTokens = %d, want 4500", cs.TotalTokens)
	}
	wantCost := 3000*3.0/1e6 + 1500*15.0/1e6
	if math.Abs(cs.EstimatedCost-wantCost) > 1e-9 {
		t.Errorf("claude-sonnet-4 cost = %v, want %v", cs.EstimatedCost, wantCost)
	}
	if !cs.CostKnown {
		t.Error("claude-sonnet-4 CostKnown = false, want true")
	}

	gp := agg.ByModel["gpt-4o"]
	if gp.InputTokens != 500 || gp.OutputTokens != 250 || gp.SessionCount != 1 {
		t.Errorf("gpt-4o = %+v, want in=500 out=250 count=1", gp)
	}

	if agg.Total.SessionCount != 3 {
		t.Errorf("total sessions = %d, want 3", agg.Total.SessionCount)
	}
	if agg.Total.InputTokens != 3500 || agg.Total.OutputTokens != 1750 {
		t.Errorf("total tokens = in=%d out=%d, want 3500/1750", agg.Total.InputTokens, agg.Total.OutputTokens)
	}
	if !agg.Total.CostKnown {
		t.Error("total CostKnown = false, want true when all models have pricing")
	}
}

func TestAggregate_UnknownModelCost(t *testing.T) {
	home := t.TempDir()
	writeSession(t, home, "s1.json", `{"model":"mystery-model-9000","usage":{"input_tokens":1000,"output_tokens":500}}`)
	writeSession(t, home, "s2.json", `{"model":"gpt-4o","usage":{"input_tokens":1000,"output_tokens":0}}`)

	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}

	u := agg.ByModel["mystery-model-9000"]
	if u.CostKnown {
		t.Error("unknown model CostKnown = true, want false")
	}
	if u.EstimatedCost != 0 {
		t.Errorf("unknown model EstimatedCost = %v, want 0", u.EstimatedCost)
	}
	if agg.Total.CostKnown {
		t.Error("total CostKnown = true, want false when any model lacks pricing")
	}
	// The total still sums the known models' cost.
	wantKnown := 1000 * 2.50 / 1e6
	if math.Abs(agg.Total.EstimatedCost-wantKnown) > 1e-9 {
		t.Errorf("total EstimatedCost = %v, want %v (known models only)", agg.Total.EstimatedCost, wantKnown)
	}
}

func TestAggregate_ProjectDirDoesNotFilterSessionPath(t *testing.T) {
	home := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "repo")
	writeSession(t, home, "s1.json", `{"model":"gpt-4o","usage":{"input_tokens":100,"output_tokens":50}}`)

	agg, err := Aggregate(home, AggregateOptions{ProjectDir: projectDir})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if agg.Total.SessionCount != 1 {
		t.Fatalf("SessionCount = %d, want 1", agg.Total.SessionCount)
	}
	if agg.Total.InputTokens != 100 || agg.Total.OutputTokens != 50 {
		t.Errorf("tokens = %d/%d, want 100/50", agg.Total.InputTokens, agg.Total.OutputTokens)
	}
}

func TestAggregate_SinceFilter(t *testing.T) {
	home := t.TempDir()
	recent := writeSession(t, home, "recent.json", `{"model":"gpt-4o","usage":{"input_tokens":100,"output_tokens":50}}`)
	old := writeSession(t, home, "old.json", `{"model":"gpt-4o","usage":{"input_tokens":999,"output_tokens":999}}`)

	// Make "old" look two hours in the past.
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	agg, err := Aggregate(home, AggregateOptions{Since: time.Hour})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	gp := agg.ByModel["gpt-4o"]
	if gp.InputTokens != 100 || gp.OutputTokens != 50 || gp.SessionCount != 1 {
		t.Errorf("gpt-4o = %+v, want only the recent session counted", gp)
	}
	if agg.Total.SessionCount != 1 {
		t.Errorf("total sessions = %d, want 1 (old excluded)", agg.Total.SessionCount)
	}
	_ = recent
}

func TestAggregate_DefensiveUnparseable(t *testing.T) {
	home := t.TempDir()
	// Invalid JSON.
	writeSession(t, home, "bad.json", `{not valid json`)
	// Valid JSON without token fields.
	writeSession(t, home, "notokens.json", `{"model":"gpt-4o","messages":[]}`)
	// Valid JSON with tokens (should be counted).
	writeSession(t, home, "good.json", `{"model":"gpt-4o","usage":{"input_tokens":10,"output_tokens":5}}`)

	agg, err := Aggregate(home, AggregateOptions{})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if agg.Total.SessionCount != 1 {
		t.Errorf("total sessions = %d, want 1 (only good.json)", agg.Total.SessionCount)
	}
	gp := agg.ByModel["gpt-4o"]
	if gp.InputTokens != 10 || gp.OutputTokens != 5 {
		t.Errorf("gpt-4o = %+v, want in=10 out=5", gp)
	}
}
