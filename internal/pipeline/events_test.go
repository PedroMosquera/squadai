package pipeline

import (
	"errors"
	"strings"
	"testing"
)

// ─── Event.String ─────────────────────────────────────────────────────────────

func TestEvent_String_PipelineStart(t *testing.T) {
	ev := Event{Type: EventPipelineStart, Total: 10}
	got := ev.String()
	if !strings.Contains(got, "10") {
		t.Errorf("PipelineStart string should mention total, got: %q", got)
	}
}

func TestEvent_String_PipelineDone(t *testing.T) {
	ev := Event{Type: EventPipelineDone, Total: 5}
	got := ev.String()
	if !strings.Contains(got, "done") {
		t.Errorf("PipelineDone string should contain 'done', got: %q", got)
	}
}

func TestEvent_String_StepStart_WithAdapter(t *testing.T) {
	ev := Event{
		Type:      EventStepStart,
		Component: "memory",
		Adapter:   "claude-code",
		Action:    "create",
		Index:     2,
		Total:     10,
	}
	got := ev.String()
	if !strings.Contains(got, "[3/10]") {
		t.Errorf("counter wrong, got: %q", got)
	}
	if !strings.Contains(got, "memory") || !strings.Contains(got, "claude-code") {
		t.Errorf("should contain component and adapter, got: %q", got)
	}
}

func TestEvent_String_StepStart_NoAdapter(t *testing.T) {
	ev := Event{
		Type:      EventStepStart,
		Component: "rules",
		Action:    "update",
		Index:     0,
		Total:     3,
	}
	got := ev.String()
	if !strings.Contains(got, "[1/3]") {
		t.Errorf("counter wrong, got: %q", got)
	}
	if strings.Contains(got, "·  ·") {
		t.Errorf("should not emit empty adapter field, got: %q", got)
	}
}

func TestEvent_String_StepDone_WithPath(t *testing.T) {
	ev := Event{
		Type:      EventStepDone,
		Component: "settings",
		Adapter:   "opencode",
		Action:    "create",
		Path:      "/home/user/.opencode/config.json",
		Index:     4,
		Total:     27,
	}
	got := ev.String()
	if !strings.Contains(got, "ok") {
		t.Errorf("StepDone should contain 'ok', got: %q", got)
	}
	if !strings.Contains(got, "/home/user/.opencode/config.json") {
		t.Errorf("StepDone should contain path, got: %q", got)
	}
}

func TestEvent_String_StepSkipped(t *testing.T) {
	ev := Event{
		Type:      EventStepSkipped,
		Component: "memory",
		Adapter:   "opencode",
		Index:     3,
		Total:     27,
	}
	got := ev.String()
	if !strings.Contains(got, "skip") {
		t.Errorf("StepSkipped should contain 'skip', got: %q", got)
	}
	if !strings.Contains(got, "[4/27]") {
		t.Errorf("counter wrong, got: %q", got)
	}
}

func TestEvent_String_StepFailed_WithError(t *testing.T) {
	ev := Event{
		Type:      EventStepFailed,
		Component: "settings",
		Adapter:   "vscode-copilot",
		Action:    "update",
		Path:      "/some/path",
		Err:       errors.New("permission denied"),
		Index:     5,
		Total:     10,
	}
	got := ev.String()
	if !strings.Contains(got, "FAIL") {
		t.Errorf("StepFailed should contain 'FAIL', got: %q", got)
	}
	if !strings.Contains(got, "permission denied") {
		t.Errorf("StepFailed should contain error message, got: %q", got)
	}
}

func TestEvent_String_StepFailed_NoErr(t *testing.T) {
	ev := Event{Type: EventStepFailed, Component: "mcp", Index: 0, Total: 1}
	got := ev.String()
	if !strings.Contains(got, "FAIL") {
		t.Errorf("StepFailed without error should still contain 'FAIL', got: %q", got)
	}
}

// ─── NopSink ─────────────────────────────────────────────────────────────────

func TestNopSink_SwallowsAllEvents(t *testing.T) {
	var s NopSink
	for i := 0; i < 1000; i++ {
		s.Send(Event{Type: EventStepDone, Index: i, Total: 1000})
	}
	// Reaching here without panic or deadlock is the assertion.
}

// ─── ChannelSink ─────────────────────────────────────────────────────────────

func TestChannelSink_Blocking_DeliversInOrder(t *testing.T) {
	ch := make(chan Event, 10)
	sink := NewChannelSink(ch, true)

	for i := 0; i < 5; i++ {
		sink.Send(Event{Type: EventStepDone, Index: i, Total: 5})
	}

	for i := 0; i < 5; i++ {
		ev := <-ch
		if ev.Index != i {
			t.Errorf("expected index %d, got %d", i, ev.Index)
		}
	}
}

func TestChannelSink_NonBlocking_DropsWhenFull(t *testing.T) {
	ch := make(chan Event, 2)
	sink := NewChannelSink(ch, false)

	// Send 10 events into a capacity-2 channel; should not panic or block.
	for i := 0; i < 10; i++ {
		sink.Send(Event{Type: EventStepDone, Index: i, Total: 10})
	}

	if len(ch) != 2 {
		t.Errorf("expected channel to hold exactly 2 events (capacity), got %d", len(ch))
	}

	// First 2 events should be in order (they fit).
	first := <-ch
	if first.Index != 0 {
		t.Errorf("first event index = %d, want 0", first.Index)
	}
}
