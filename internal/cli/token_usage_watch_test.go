package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func makeWatchAgg(totalTokens, cacheRead, cacheWrite int) *session.Aggregation {
	u := session.Usage{
		InputTokens:         totalTokens,
		TotalTokens:         totalTokens,
		CacheReadTokens:     cacheRead,
		CacheCreationTokens: cacheWrite,
		SessionCount:        1,
		CostKnown:           true,
	}
	byModel := u
	byModel.Model = "test-model"
	return &session.Aggregation{
		ByModel: map[string]session.Usage{"test-model": byModel},
		Total:   u,
		Period:  "7d",
	}
}

// sequenceAggregate returns aggs in order, then repeats the last one forever.
func sequenceAggregate(results ...func() (*session.Aggregation, error)) func() (*session.Aggregation, error) {
	var calls atomic.Int32
	return func() (*session.Aggregation, error) {
		i := int(calls.Add(1)) - 1
		if i >= len(results) {
			i = len(results) - 1
		}
		return results[i]()
	}
}

func fixed(total, cacheRead, cacheWrite int) func() (*session.Aggregation, error) {
	return func() (*session.Aggregation, error) { return makeWatchAgg(total, cacheRead, cacheWrite), nil }
}

func runWatchFor(t *testing.T, d, interval time.Duration, isTTY bool, aggregate func() (*session.Aggregation, error)) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	var buf bytes.Buffer
	if err := runTokenUsageWatch(ctx, &buf, aggregate, interval, isTTY); err != nil {
		t.Fatalf("runTokenUsageWatch: %v", err)
	}
	return buf.String()
}

// renderLines returns the non-TTY compact lines, which start with HH:MM:SS.
func renderLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if len(line) >= 8 && line[2] == ':' && line[5] == ':' {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestRunTokenUsageWatch_InitialRenderIncludesCacheFields(t *testing.T) {
	out := runWatchFor(t, 100*time.Millisecond, time.Hour, false, fixed(1234, 50, 7))
	lines := renderLines(out)
	if len(lines) != 1 {
		t.Fatalf("want 1 initial render line before any tick, got %d:\n%s", len(lines), out)
	}
	for _, want := range []string{"total=1234", "cache_read=50", "cache_write=7", "delta=+0"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("render line %q missing %q", lines[0], want)
		}
	}
}

func TestRunTokenUsageWatch_ChangedTotalRerendersWithDelta(t *testing.T) {
	out := runWatchFor(t, 300*time.Millisecond, 10*time.Millisecond, false, sequenceAggregate(fixed(1000, 0, 0), fixed(2500, 0, 0)))
	lines := renderLines(out)
	if len(lines) != 2 {
		t.Fatalf("want exactly 2 render lines (initial + change), got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[1], "delta=+1500") {
		t.Errorf("second line %q should carry delta=+1500", lines[1])
	}
}

// TotalTokens excludes cache tokens, so a cache-only change must still
// count as a change.
func TestRunTokenUsageWatch_CacheOnlyChangeRerenders(t *testing.T) {
	out := runWatchFor(t, 300*time.Millisecond, 10*time.Millisecond, false, sequenceAggregate(fixed(1000, 0, 0), fixed(1000, 400, 0)))
	if n := len(renderLines(out)); n != 2 {
		t.Fatalf("want 2 render lines after a cache-only change, got %d:\n%s", n, out)
	}
}

func TestRunTokenUsageWatch_UnchangedNoRerender(t *testing.T) {
	out := runWatchFor(t, 150*time.Millisecond, 10*time.Millisecond, false, fixed(1000, 5, 5))
	if n := len(renderLines(out)); n != 1 {
		t.Errorf("want exactly 1 render line for stable usage, got %d:\n%s", n, out)
	}
}

func TestRunTokenUsageWatch_CancelReturnsNil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runTokenUsageWatch(ctx, &bytes.Buffer{}, fixed(100, 0, 0), 10*time.Millisecond, false)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("want nil on cancel, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runTokenUsageWatch did not return after cancel")
	}
}

func TestRunTokenUsageWatch_TickErrorWarnsAndContinues(t *testing.T) {
	failing := func() (*session.Aggregation, error) { return nil, errors.New("transient") }
	out := runWatchFor(t, 300*time.Millisecond, 10*time.Millisecond, false, sequenceAggregate(fixed(1000, 0, 0), failing, fixed(2000, 0, 0)))
	if !strings.Contains(out, "warning: aggregate failed: transient") {
		t.Errorf("want warning line for tick error:\n%s", out)
	}
	if n := len(renderLines(out)); n != 2 {
		t.Errorf("want render to resume after the error (2 lines), got %d:\n%s", n, out)
	}
}

func TestRunTokenUsageWatch_TTY(t *testing.T) {
	out := runWatchFor(t, 100*time.Millisecond, time.Hour, true, fixed(500, 0, 0))
	for _, want := range []string{"\x1b[2J\x1b[H", "Token Usage (last 7d)", "Ctrl+C to stop", "Delta since watch start: +0 tokens"} {
		if !strings.Contains(out, want) {
			t.Errorf("TTY output missing %q:\n%s", want, out)
		}
	}
}

func TestRunTokenUsageWatch_NonTTYNoEscapeCodes(t *testing.T) {
	out := runWatchFor(t, 100*time.Millisecond, 10*time.Millisecond, false, fixed(500, 0, 0))
	if out == "" {
		t.Fatal("want output, got none")
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-TTY output must not contain ANSI escapes:\n%q", out)
	}
}

func TestRunTokenUsage_WatchFlagErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantMsg  string
	}{
		{name: "watch with json", args: []string{"--watch", "--json"}, wantCode: exitcode.Config, wantMsg: "--json"},
		{name: "watch with against-budget", args: []string{"--against-budget", "--watch"}, wantCode: exitcode.Config, wantMsg: "--against-budget"},
		{name: "unparseable interval", args: []string{"--watch", "--interval=abc"}, wantMsg: "--interval"},
		{name: "zero interval", args: []string{"--watch", "--interval", "0s"}, wantMsg: "--interval"},
		{name: "negative interval", args: []string{"--watch", "--interval=-1s"}, wantMsg: "--interval"},
		{name: "interval missing value", args: []string{"--watch", "--interval"}, wantMsg: "--interval"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RunTokenUsage(tc.args, &bytes.Buffer{})
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q should mention %q", err, tc.wantMsg)
			}
			if tc.wantCode != 0 {
				var ae *exitcode.AppError
				if !errors.As(err, &ae) || ae.Code != tc.wantCode {
					t.Errorf("want AppError code %d, got %v", tc.wantCode, err)
				}
			}
		})
	}
}
