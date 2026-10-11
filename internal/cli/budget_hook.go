package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// budgetHookMaxInput bounds the stdin read; a hook payload is a few KB.
const budgetHookMaxInput = 1 << 20

// budgetHookInput is the subset of the Claude Code hook payload budget-hook
// reads (common input fields, https://code.claude.com/docs/en/hooks).
type budgetHookInput struct {
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
}

// RunBudgetHook implements the hidden `squadai budget-hook --event <name>`
// that the hooks component installs into .claude/settings.json. It reads the
// hook payload on stdin and answers per the Claude Code hook contract:
//
//   - within budget, or enforcement unset or off: exit 0, no output (allow)
//   - over budget under warn, and on Stop under any enforcement: exit 0 with a
//     JSON systemMessage; PreToolUse stays silent under warn
//   - over budget under ask, on PreToolUse: exit 0 with permissionDecision "ask"
//   - over budget under block, on PreToolUse or UserPromptSubmit: an
//     exitcode.HookBlock error, which main.go turns into exit 2 with the
//     reason on stderr
//
// Every other failure exits 1, which Claude Code treats as non-blocking
// unless the entry sets onFailure "block". Reads only the project config and
// the last 24h of local transcripts; no network.
func RunBudgetHook(args []string, stdin io.Reader, stdout io.Writer) error {
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
	switch event {
	case "PreToolUse", "UserPromptSubmit", "Stop":
	default:
		return fmt.Errorf("budget-hook: unsupported --event %q (want PreToolUse, UserPromptSubmit or Stop)", event)
	}

	data, err := io.ReadAll(io.LimitReader(stdin, budgetHookMaxInput))
	if err != nil {
		return fmt.Errorf("budget-hook: read hook input: %w", err)
	}
	var in budgetHookInput
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &in); err != nil {
			return fmt.Errorf("budget-hook: parse hook input: %w", err)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("budget-hook: resolve home directory: %w", err)
	}
	projectDir := hookProjectDir(in)
	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("budget-hook: %w", err)
	}
	usage := merged.Usage
	// evaluateBudgets reads an unset enforcement as warn; here unset means
	// "not configured", matching the install rule, so stale entries stay inert.
	if usage.Enforcement == "" || usage.Enforcement == "off" {
		return nil
	}
	if usage.DailyTokenBudget <= 0 && usage.SessionTokenBudget <= 0 {
		return nil
	}

	daily, err := session.Aggregate(homeDir, session.AggregateOptions{Since: 24 * time.Hour, ProjectDir: projectDir})
	if err != nil {
		return fmt.Errorf("budget-hook: aggregate usage: %w", err)
	}
	// Inside a session the session budget means this session, not the largest
	// one of the day, which would lock every new session after one big one.
	if in.TranscriptPath != "" {
		daily.MaxSessionTokens = session.ClaudeTranscriptTokens(in.TranscriptPath)
	}

	reason := overBudgetReason(evaluateBudgets(usage, daily))
	if reason == "" {
		return nil
	}
	switch {
	case event == "Stop":
		// A Stop "block" keeps Claude working, the opposite of a budget stop.
		return writeHookJSON(stdout, map[string]any{"systemMessage": "squadai: " + reason})
	case usage.Enforcement == "block":
		return exitcode.ErrBudgetHookBlock(reason)
	case usage.Enforcement == "ask" && event == "PreToolUse":
		return writeHookJSON(stdout, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "ask",
			"permissionDecisionReason": reason,
		}})
	case event == "PreToolUse":
		return nil
	default:
		return writeHookJSON(stdout, map[string]any{"systemMessage": "squadai: " + reason})
	}
}

// hookProjectDir prefers CLAUDE_PROJECT_DIR, the project root Claude Code
// exports to hooks, over the payload cwd, which follows cd inside the session.
func hookProjectDir(in budgetHookInput) string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	if in.Cwd != "" {
		return in.Cwd
	}
	d, _ := os.Getwd()
	return d
}

func overBudgetReason(r *budgetReport) string {
	if r == nil {
		return ""
	}
	var parts []string
	for _, f := range r.Findings {
		if !f.Over {
			continue
		}
		window := "in the last 24h"
		if f.Kind == "session" {
			window = "in this session"
		}
		parts = append(parts, fmt.Sprintf("%s token budget exceeded: %d / %d tokens %s", f.Kind, f.Used, f.Limit, window))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + " (enforcement: " + r.Enforcement + ")"
}

func writeHookJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("budget-hook: marshal response: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
