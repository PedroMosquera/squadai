package cli

import (
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ReviewPromptHook, when non-nil, renders the pre-apply review TUI and
// returns whether the user confirmed the apply. The cli package declares the
// variable but cannot import tui (that would create a tui→cli→tui cycle), so
// the app package wires this in its init().
//
// If this is nil, apply falls through to the non-interactive path (same as
// when --no-review is passed).
var ReviewPromptHook func([]domain.PreviewEntry) (domain.ReviewDecision, error)

// IsTTYHook reports whether stdout is an interactive terminal. Wired by
// the app package to tui.IsTTY. When nil, treat as non-interactive so
// automated invocations don't hang waiting for a terminal prompt.
var IsTTYHook func() bool

// ReviewScreenWired reports whether app.init installed the review hooks.
// The doctor `config.review-screen` check reads this to tell the user
// if their binary was built without TUI integration.
var ReviewScreenWired bool
