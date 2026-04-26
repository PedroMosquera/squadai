package governance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	auditLogFile = "audit.log"
	maxLogSize   = 10 * 1024 * 1024 // 10 MB
	squadaiDir   = ".squadai"
)

// AuditLog is a thread-safe, append-only JSON-lines log at .squadai/audit.log.
// It rotates to audit.log.1 when the file exceeds maxLogSize.
type AuditLog struct {
	path string
	mu   sync.Mutex
}

// OpenAuditLog returns an AuditLog bound to projectDir.
func OpenAuditLog(projectDir string) *AuditLog {
	return &AuditLog{path: filepath.Join(projectDir, squadaiDir, auditLogFile)}
}

// Path returns the absolute path to the audit log file.
func (l *AuditLog) Path() string { return l.path }

// Append writes e as a JSON line. Sets Timestamp to now if zero.
func (l *AuditLog) Append(e Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return fmt.Errorf("audit log: create dir: %w", err)
	}
	if err := l.maybeRotate(); err != nil {
		return fmt.Errorf("audit log: rotate: %w", err)
	}

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("audit log: marshal: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("audit log: open: %w", err)
	}
	defer f.Close()

	_, err = f.Write(line)
	return err
}

// Read returns events from the log. sinceAge (if > 0) excludes events older
// than that duration. filterPrefix (if non-empty) includes only events whose
// Kind starts with the given prefix (e.g. "drift").
func (l *AuditLog) Read(sinceAge time.Duration, filterPrefix string) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("audit log: open: %w", err)
	}
	defer f.Close()

	var cutoff time.Time
	if sinceAge > 0 {
		cutoff = time.Now().UTC().Add(-sinceAge)
	}

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(raw, &e); err != nil {
			continue // skip malformed lines
		}
		if !cutoff.IsZero() && e.Timestamp.Before(cutoff) {
			continue
		}
		if filterPrefix != "" && !strings.HasPrefix(string(e.Kind), filterPrefix) {
			continue
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}

// HasDriftSince returns true if the log contains any drift:* events newer than age.
// age == 0 checks the entire log.
func (l *AuditLog) HasDriftSince(age time.Duration) (bool, error) {
	events, err := l.Read(age, "drift")
	if err != nil {
		return false, err
	}
	return len(events) > 0, nil
}

func (l *AuditLog) maybeRotate() error {
	info, err := os.Stat(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < maxLogSize {
		return nil
	}
	return os.Rename(l.path, l.path+".1")
}
