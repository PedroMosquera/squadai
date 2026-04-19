package pipeline

import "fmt"

// EventType classifies a pipeline progress event.
type EventType int

const (
	EventStepStart EventType = iota
	EventStepDone
	EventStepFailed
	EventStepSkipped
	EventPipelineStart
	EventPipelineDone
)

// Event carries progress information about a single pipeline step or the
// pipeline as a whole.
type Event struct {
	Type      EventType
	Component string // e.g. "memory", "settings", "permissions"
	Adapter   string // e.g. "claude-code", "opencode"
	Action    string // e.g. "create", "update", "skip"
	Path      string // optional file path being operated on
	Err       error  // populated for EventStepFailed
	Index     int    // 0-based step index
	Total     int    // total steps in pipeline
}

// String returns a terse, human-readable one-line representation.
func (e Event) String() string {
	counter := fmt.Sprintf("[%d/%d]", e.Index+1, e.Total)
	switch e.Type {
	case EventPipelineStart:
		return fmt.Sprintf("Pipeline starting — %d step(s)", e.Total)
	case EventPipelineDone:
		return fmt.Sprintf("Pipeline done — %d step(s)", e.Total)
	case EventStepStart:
		if e.Adapter != "" {
			return fmt.Sprintf("%s %s · %s · %s ...", counter, e.Component, e.Adapter, e.Action)
		}
		return fmt.Sprintf("%s %s · %s ...", counter, e.Component, e.Action)
	case EventStepDone:
		base := counter
		if e.Adapter != "" {
			base += fmt.Sprintf(" %s · %s · %s", e.Component, e.Adapter, e.Action)
		} else {
			base += fmt.Sprintf(" %s · %s", e.Component, e.Action)
		}
		if e.Path != "" {
			base += " · " + e.Path
		}
		return base + " ... ok"
	case EventStepSkipped:
		base := counter
		if e.Adapter != "" {
			base += fmt.Sprintf(" %s · %s", e.Component, e.Adapter)
		} else {
			base += fmt.Sprintf(" %s", e.Component)
		}
		return base + " · skip (already up to date)"
	case EventStepFailed:
		base := counter
		if e.Adapter != "" {
			base += fmt.Sprintf(" %s · %s · %s", e.Component, e.Adapter, e.Action)
		} else {
			base += fmt.Sprintf(" %s · %s", e.Component, e.Action)
		}
		if e.Path != "" {
			base += " · " + e.Path
		}
		errMsg := ""
		if e.Err != nil {
			errMsg = e.Err.Error()
		}
		return base + " ... FAIL: " + errMsg
	default:
		return fmt.Sprintf("event(%d)", e.Type)
	}
}

// EventSink receives pipeline progress events.
type EventSink interface {
	Send(Event)
}

// NopSink discards all events. It is the default when no sink is configured.
type NopSink struct{}

// Send discards ev.
func (NopSink) Send(Event) {}

// ChannelSink forwards events to a channel.
type ChannelSink struct {
	ch       chan<- Event
	blocking bool
}

// NewChannelSink creates a ChannelSink that writes to ch.
// When blocking is true, Send blocks until the channel accepts the event —
// suitable for CLI verbose mode where output ordering matters.
// When blocking is false, Send drops the event silently if the channel is
// full — suitable for TUI use where a stale drop is preferable to blocking
// the pipeline goroutine.
func NewChannelSink(ch chan<- Event, blocking bool) *ChannelSink {
	return &ChannelSink{ch: ch, blocking: blocking}
}

// Send forwards ev to the underlying channel. Behaviour depends on the
// blocking flag set at construction time.
func (s *ChannelSink) Send(ev Event) {
	if s.blocking {
		s.ch <- ev
	} else {
		select {
		case s.ch <- ev:
		default:
		}
	}
}
