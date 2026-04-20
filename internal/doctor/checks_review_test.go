package doctor

import (
	"testing"
)

func TestCheckReviewScreen_Wired_Passes(t *testing.T) {
	prev := ReviewScreenWiredHook
	ReviewScreenWiredHook = func() bool { return true }
	t.Cleanup(func() { ReviewScreenWiredHook = prev })

	d := &Doctor{}
	r := d.checkReviewScreen()
	if r.Status != CheckPass {
		t.Errorf("status = %v, want CheckPass (msg=%q)", r.Status, r.Message)
	}
}

func TestCheckReviewScreen_NotWired_Warns(t *testing.T) {
	prev := ReviewScreenWiredHook
	ReviewScreenWiredHook = nil
	t.Cleanup(func() { ReviewScreenWiredHook = prev })

	d := &Doctor{}
	r := d.checkReviewScreen()
	if r.Status != CheckWarn {
		t.Errorf("status = %v, want CheckWarn (msg=%q)", r.Status, r.Message)
	}
	if r.FixHint == "" {
		t.Error("expected a fix hint when review screen is not wired")
	}
}

func TestCheckReviewScreen_HookReturnsFalse_Warns(t *testing.T) {
	prev := ReviewScreenWiredHook
	ReviewScreenWiredHook = func() bool { return false }
	t.Cleanup(func() { ReviewScreenWiredHook = prev })

	d := &Doctor{}
	r := d.checkReviewScreen()
	if r.Status != CheckWarn {
		t.Errorf("status = %v, want CheckWarn", r.Status)
	}
}
