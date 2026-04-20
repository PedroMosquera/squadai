package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func sampleEntries() []domain.PreviewEntry {
	return []domain.PreviewEntry{
		{
			Component:  domain.ComponentMCP,
			Action:     domain.ActionCreate,
			TargetPath: "/tmp/a.json",
			Diff:       "--- a/a.json\n+++ b/a.json\n@@ -0,0 +1,1 @@\n+hello\n",
		},
		{
			Component:  domain.ComponentMCP,
			Action:     domain.ActionUpdate,
			TargetPath: "/tmp/b.json",
			Diff:       "--- a/b.json\n+++ b/b.json\n@@ -1,1 +1,1 @@\n-old\n+new\n",
			Conflicts: []domain.Conflict{
				{Key: "mcp", UserValue: `{"user":"x"}`, IncomingValue: `{"us":"y"}`},
			},
		},
	}
}

func send(m tea.Model, key string) tea.Model {
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return out
}

func sendSpecial(m tea.Model, t tea.KeyType) tea.Model {
	out, _ := m.Update(tea.KeyMsg{Type: t})
	return out
}

func TestReviewModel_Confirm_ReturnsConfirmed(t *testing.T) {
	m := newReviewModel(sampleEntries())
	updated := send(m, "y")
	got, ok := updated.(reviewModel)
	if !ok {
		t.Fatalf("expected reviewModel, got %T", updated)
	}
	if got.decision != reviewConfirmed {
		t.Errorf("decision = %v, want reviewConfirmed", got.decision)
	}
}

func TestReviewModel_Cancel_ReturnsCanceled(t *testing.T) {
	m := newReviewModel(sampleEntries())
	updated := send(m, "n")
	got, ok := updated.(reviewModel)
	if !ok {
		t.Fatalf("expected reviewModel, got %T", updated)
	}
	if got.decision != reviewCanceled {
		t.Errorf("decision = %v, want reviewCanceled", got.decision)
	}
}

func TestReviewModel_EnterOpensDetail_EscReturnsToList(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	m = sendSpecial(m, tea.KeyEnter)
	if m.(reviewModel).pane != reviewPaneDetail {
		t.Fatalf("enter should move to detail pane, got %v", m.(reviewModel).pane)
	}
	m = sendSpecial(m, tea.KeyEsc)
	if m.(reviewModel).pane != reviewPaneList {
		t.Errorf("esc from detail should return to list, got %v", m.(reviewModel).pane)
	}
	if m.(reviewModel).decision != reviewPending {
		t.Errorf("navigation must not set a decision, got %v", m.(reviewModel).decision)
	}
}

func TestReviewModel_CursorMovement(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	m = send(m, "j")
	if m.(reviewModel).cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.(reviewModel).cursor)
	}
	// Can't go past the end.
	m = send(m, "j")
	if m.(reviewModel).cursor != 1 {
		t.Errorf("cursor clamped at end should stay at 1, got %d", m.(reviewModel).cursor)
	}
	m = send(m, "k")
	if m.(reviewModel).cursor != 0 {
		t.Errorf("cursor = %d, want 0 after k", m.(reviewModel).cursor)
	}
}

func TestReviewModel_RenderList_IncludesConflictCount(t *testing.T) {
	m := newReviewModel(sampleEntries())
	out := m.View()
	if !strings.Contains(out, "Review planned changes") {
		t.Errorf("expected list heading, got:\n%s", out)
	}
	if !strings.Contains(out, "1 conflict") {
		t.Errorf("expected conflict count for entry 2, got:\n%s", out)
	}
}

func TestReviewModel_RenderDetail_ShowsConflictKeysAndDiff(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	m = send(m, "j")                 // move cursor to entry with a conflict
	m = sendSpecial(m, tea.KeyEnter) // open detail
	out := m.(reviewModel).View()

	if !strings.Contains(out, "mcp") {
		t.Errorf("expected conflict key 'mcp' in detail, got:\n%s", out)
	}
	if !strings.Contains(out, "@@ -1,1 +1,1 @@") {
		t.Errorf("expected diff hunk header in detail, got:\n%s", out)
	}
}

func TestReviewModel_EmptyEntries_CanStillCancel(t *testing.T) {
	m := newReviewModel(nil)
	out := m.View()
	if !strings.Contains(out, "nothing to apply") {
		t.Errorf("empty view should mention nothing to apply, got:\n%s", out)
	}
	updated := send(m, "n")
	if updated.(reviewModel).decision != reviewCanceled {
		t.Errorf("n should cancel even when empty")
	}
}

func TestReviewModel_ToggleOverwrite_UpdatesPolicy(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	// Navigate to the second entry (the one with a conflict) and open detail.
	m = send(m, "j")
	m = sendSpecial(m, tea.KeyEnter)
	// Press [o] to toggle the conflict's overwrite flag.
	m = send(m, "o")
	got := m.(reviewModel)

	policy := got.Policy()
	overrides, ok := policy.Overrides["/tmp/b.json"]
	if !ok {
		t.Fatalf("Policy.Overrides missing target /tmp/b.json: %+v", policy.Overrides)
	}
	if !overrides["mcp"] {
		t.Errorf("expected overwrite decision on mcp, got %+v", overrides)
	}
}

func TestReviewModel_ToggleOverwriteTwice_ClearsPolicy(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	m = send(m, "j")
	m = sendSpecial(m, tea.KeyEnter)
	m = send(m, "o")
	m = send(m, "o") // toggle back to keep
	got := m.(reviewModel)

	policy := got.Policy()
	if len(policy.Overrides) != 0 {
		t.Errorf("toggling overwrite twice should clear overrides, got %+v", policy.Overrides)
	}
}

func TestReviewModel_ApplyReturnsConfirmedAndPolicy(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	m = send(m, "j")
	m = sendSpecial(m, tea.KeyEnter)
	m = send(m, "o")
	// Apply all from detail pane.
	m = send(m, "a")
	got := m.(reviewModel)

	if got.decision != reviewConfirmed {
		t.Errorf("a should confirm, got decision = %v", got.decision)
	}
	policy := got.Policy()
	if !policy.Overrides["/tmp/b.json"]["mcp"] {
		t.Errorf("confirmed policy should carry mcp overwrite, got %+v", policy.Overrides)
	}
}

func TestReviewModel_RenderDetail_ShowsKeepBadgeByDefault(t *testing.T) {
	m := tea.Model(newReviewModel(sampleEntries()))
	m = send(m, "j")
	m = sendSpecial(m, tea.KeyEnter)
	out := m.(reviewModel).View()
	if !strings.Contains(out, "KEEP") {
		t.Errorf("detail should show [KEEP] badge by default, got:\n%s", out)
	}
	// Toggle and re-render.
	m = send(m, "o")
	out = m.(reviewModel).View()
	if !strings.Contains(out, "OVERWRITE") {
		t.Errorf("detail should show [OVERWRITE] badge after toggle, got:\n%s", out)
	}
}
