package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/components/bundle"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// memoryHookMinToolCalls is the transcript size below which a session is too
// small to have produced a decision worth recording, so the hook stays quiet.
const memoryHookMinToolCalls = 10

// memoryHookMarkerTTL bounds how long per-session markers are kept; sessions
// are rarely resumed after this long, and markers are a few bytes each.
const memoryHookMarkerTTL = 30 * 24 * time.Hour

// memoryHookPrompt is shown to Claude, not the user, as Stop hook feedback.
const memoryHookPrompt = "squadai: before you finish, record anything from this session a future session should know: " +
	"decisions and why they were made, non-obvious fixes, dead ends worth not repeating. " +
	"Call the memory_add tool of the squadai MCP server once per item, or run `squadai memory add \"<note>\"` if the tool is unavailable. " +
	"Keep each note to a few sentences and skip anything already in project memory. " +
	"If nothing durable came out of this session, stop without writing anything. This reminder is shown once per session."

var memoryHookSessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// memoryHookInput is the subset of the Claude Code Stop payload memory-hook
// reads (https://code.claude.com/docs/en/hooks).
type memoryHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// RunMemoryHook implements the hidden `squadai memory-hook --event Stop` that
// the hooks component installs when memory.capture_on_stop is true. A hook
// cannot summarize a session itself, so it asks Claude to: on the first Stop
// of a session with at least memoryHookMinToolCalls tool calls it exits 0
// with hookSpecificOutput.additionalContext, which keeps Claude going for one
// more turn to call memory_add. Every other case exits 0 with no output and
// Claude stops. It never re-asks: stop_hook_active short-circuits the
// continuation turn, and a marker under ~/.squadai/memory-hook records each
// session already asked. Failures exit 1, which Claude Code treats as
// non-blocking.
func RunMemoryHook(args []string, stdin io.Reader, stdout io.Writer) error {
	event := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--event" && i+1 < len(args):
			i++
			event = args[i]
		case strings.HasPrefix(args[i], "--event="):
			event = strings.TrimPrefix(args[i], "--event=")
		}
	}
	if event != "Stop" {
		return fmt.Errorf("memory-hook: unsupported --event %q (want Stop)", event)
	}

	data, err := io.ReadAll(io.LimitReader(stdin, budgetHookMaxInput))
	if err != nil {
		return fmt.Errorf("memory-hook: read hook input: %w", err)
	}
	var in memoryHookInput
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &in); err != nil {
			return fmt.Errorf("memory-hook: parse hook input: %w", err)
		}
	}
	if in.StopHookActive || !memoryHookSessionID.MatchString(in.SessionID) {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("memory-hook: resolve home directory: %w", err)
	}
	merged, err := loadAndMerge(homeDir, hookProjectDir(budgetHookInput{Cwd: in.Cwd}, "claude"))
	if err != nil {
		return fmt.Errorf("memory-hook: %w", err)
	}
	// Re-checked here, not only at install, so an entry left behind by a
	// failed apply stays inert once the flag is off.
	if !bundle.MemoryEnabled(merged) || !merged.Memory.CaptureOnStop {
		return nil
	}
	if session.ClaudeTranscriptToolCalls(in.TranscriptPath) < memoryHookMinToolCalls {
		return nil
	}

	first, err := markSessionAsked(filepath.Join(homeDir, ".squadai", "memory-hook"), in.SessionID)
	if err != nil {
		return fmt.Errorf("memory-hook: %w", err)
	}
	if !first {
		return nil
	}
	return writeHookJSON(stdout, map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":     "Stop",
		"additionalContext": memoryHookPrompt,
	}})
}

// markSessionAsked creates dir/<sessionID> and reports whether this call
// created it. O_EXCL makes two concurrent Stop hooks agree on one winner.
func markSessionAsked(dir, sessionID string) (bool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create marker dir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, sessionID), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("write session marker: %w", err)
	}
	if err := f.Close(); err != nil {
		return false, fmt.Errorf("write session marker: %w", err)
	}
	pruneSessionMarkers(dir, time.Now().Add(-memoryHookMarkerTTL))
	return true, nil
}

func pruneSessionMarkers(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
