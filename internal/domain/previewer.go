package domain

// Previewer is implemented by ComponentInstallers that can preview the file
// changes they would perform during apply. The review screen calls Preview for
// every wired installer before the user confirms the apply, so that the user
// sees a unified diff per change and a list of per-key conflicts before
// anything is written to disk.
type Previewer interface {
	// Preview returns the set of proposed file changes for this installer and
	// the given adapter. It must be pure: no file mutation, no persistent state
	// change. Returning an error aborts the pre-apply review for this installer
	// and surfaces the error in the TUI.
	Preview(adapter Adapter, homeDir, projectDir string) ([]PreviewEntry, error)
}

// PreviewEntry describes a single proposed file change surfaced by a
// Previewer. One installer may produce several entries (e.g., one per adapter
// config file it touches). Entries with ActionSkip carry an empty Diff.
type PreviewEntry struct {
	Component  ComponentID `json:"component"`
	Action     ActionType  `json:"action"`
	TargetPath string      `json:"target_path"`
	Diff       string      `json:"diff"`
	Conflicts  []Conflict  `json:"conflicts,omitempty"`
}

// Conflict represents a single key that SquadAI would overwrite but cannot,
// because the user has hand-edited the key to a value SquadAI does not own.
// The review screen renders these so the user can reconcile them manually.
// UserValue and IncomingValue are already-stringified and truncated for
// compact TUI display.
type Conflict struct {
	Key           string `json:"key"`
	UserValue     string `json:"user_value"`
	IncomingValue string `json:"incoming_value"`
}

// truncate returns s shortened to at most n runes, appending "…" when it had
// to shorten. Guards n <= 0 by returning "". Intended for TUI-safe rendering
// of arbitrary user/incoming JSON values in Conflict.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
