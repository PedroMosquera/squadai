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

// memoryHookPrompt goes to the agent, not the user: Stop hook feedback in
// Claude Code, the continuation prompt in Codex.
const memoryHookPrompt = "squadai: before you finish, record anything from this session a future session should know: " +
	"decisions and why they were made, non-obvious fixes, dead ends worth not repeating. " +
	"Call the memory_add tool of the squadai MCP server once per item, or run `squadai memory add \"<note>\"` if the tool is unavailable. " +
	"Keep each note to a few sentences and skip anything already in project memory. " +
	"If nothing durable came out of this session, stop without writing anything. This reminder is shown once per session."

var memoryHookSessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// memoryHookInput is the subset of the Stop payload memory-hook reads. Claude
// Code: https://code.claude.com/docs/en/hooks. Codex: StopCommandInput in
// codex-rs/hooks/src/schema.rs (github.com/openai/codex), where
// transcript_path may be null and last_assistant_message is Codex's own.
type memoryHookInput struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

// RunMemoryHook implements the hidden `squadai memory-hook --event Stop
// [--harness claude|codex]` that the hooks component installs when
// memory.capture_on_stop is true. A hook cannot summarize a session itself,
// so it asks the agent to, once per session, then exits 0 with no output on
// every other Stop so the agent stops. It never re-asks: stop_hook_active
// short-circuits the continuation turn, and a marker under
// ~/.squadai/memory-hook records each session already asked. Failures exit 1,
// which both harnesses treat as non-blocking.
//
// Claude Code: asked once the transcript has memoryHookMinToolCalls tool
// calls, through hookSpecificOutput.additionalContext.
//
// Codex: asked on the first Stop with a non-empty last_assistant_message,
// through {"decision":"block","reason":...}, which Codex records as the next
// prompt (codex-rs/hooks/src/events/stop.rs). The Codex rollout format is not
// a documented contract, so no tool-call threshold is applied there.
func RunMemoryHook(args []string, stdin io.Reader, stdout io.Writer) error {
	event, harness := "", "claude"
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--event" && i+1 < len(args):
			i++
			event = args[i]
		case strings.HasPrefix(args[i], "--event="):
			event = strings.TrimPrefix(args[i], "--event=")
		case args[i] == "--harness" && i+1 < len(args):
			i++
			harness = args[i]
		case strings.HasPrefix(args[i], "--harness="):
			harness = strings.TrimPrefix(args[i], "--harness=")
		}
	}
	if event != "Stop" {
		return fmt.Errorf("memory-hook: unsupported --event %q (want Stop)", event)
	}
	if harness != "claude" && harness != "codex" {
		return fmt.Errorf("memory-hook: unsupported --harness %q (want claude or codex)", harness)
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
	merged, err := loadAndMerge(homeDir, hookProjectDir(budgetHookInput{Cwd: in.Cwd}, harness))
	if err != nil {
		return fmt.Errorf("memory-hook: %w", err)
	}
	// Re-checked here, not only at install, so an entry left behind by a
	// failed apply stays inert once the flag is off.
	if !bundle.MemoryEnabled(merged) || !merged.Memory.CaptureOnStop {
		return nil
	}
	if harness == "codex" {
		if strings.TrimSpace(in.LastAssistantMessage) == "" {
			return nil
		}
	} else if session.ClaudeTranscriptToolCalls(in.TranscriptPath) < memoryHookMinToolCalls {
		return nil
	}

	first, err := markSessionAsked(filepath.Join(homeDir, ".squadai", "memory-hook"), in.SessionID)
	if err != nil {
		return fmt.Errorf("memory-hook: %w", err)
	}
	if !first {
		return nil
	}
	if harness == "codex" {
		return writeHookJSON(stdout, map[string]any{"decision": "block", "reason": memoryHookPrompt})
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
