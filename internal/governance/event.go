package governance

import "time"

// EventKind identifies the type of governance event.
type EventKind string

const (
	KindDriftDeleted  EventKind = "drift:deleted"          // managed/created file was removed
	KindDriftMarkers  EventKind = "drift:markers-stripped"  // HTML marker blocks removed from text file
	KindDriftJSONKeys EventKind = "drift:json-keys-missing" // managed JSON keys removed from config file
	KindApplyStart    EventKind = "apply:start"
	KindApplyComplete EventKind = "apply:complete"
	KindVerifyPass    EventKind = "verify:pass"
	KindVerifyFail    EventKind = "verify:fail"
	KindWatchStart    EventKind = "watch:start"
	KindWatchStop     EventKind = "watch:stop"
)

// Event is a single governance event written to the audit log.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Kind      EventKind `json:"kind"`
	Path      string    `json:"path,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}
