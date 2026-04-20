package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// RunReviewPrompt launches a bubbletea program that shows the user each
// PreviewEntry's diff + conflicts and collects per-conflict overwrite
// decisions. The returned ReviewDecision carries Confirmed + the ApplyPolicy
// the caller should hand to the pipeline.
//
// It is self-contained: caller supplies the entries and receives a decision.
// The caller is responsible for deciding whether to call this at all (e.g.,
// skipping when --no-review is set or the process isn't attached to a TTY).
func RunReviewPrompt(entries []domain.PreviewEntry) (domain.ReviewDecision, error) {
	if len(entries) == 0 {
		return domain.ReviewDecision{Confirmed: true}, nil
	}
	model := newReviewModel(entries)
	p := tea.NewProgram(model, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return domain.ReviewDecision{}, fmt.Errorf("run review prompt: %w", err)
	}
	m, ok := final.(reviewModel)
	if !ok {
		return domain.ReviewDecision{}, nil
	}
	return domain.ReviewDecision{
		Confirmed: m.decision == reviewConfirmed,
		Policy:    m.Policy(),
	}, nil
}

// IsTTY reports whether stdout is attached to a terminal. The review prompt
// is only useful in interactive sessions — callers should fall through to
// the non-interactive path (e.g., `--json`, CI) when this returns false.
func IsTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
