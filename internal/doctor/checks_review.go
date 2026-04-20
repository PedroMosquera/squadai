package doctor

// ReviewScreenWiredHook, when non-nil, reports whether the pre-apply review
// hooks were wired at startup. The app package assigns it — keeping doctor
// free of a cli import that would cause a cli→doctor→cli cycle.
var ReviewScreenWiredHook func() bool

// checkReviewScreen verifies the app package wired the TUI review hooks into
// the cli package at startup. When this is false, `squadai apply` silently
// skips the pre-apply review screen even on an interactive terminal — which
// defeats the user-wins safety story — so surface it clearly as a warning.
func (d *Doctor) checkReviewScreen() CheckResult {
	if ReviewScreenWiredHook != nil && ReviewScreenWiredHook() {
		return pass(catEnv, "Review screen", "pre-apply review screen is wired", "")
	}
	return warn(
		catEnv,
		"Review screen",
		"pre-apply review screen is NOT wired — apply will proceed without showing diffs",
		"",
		"Rebuild from the canonical entry point (cmd/squadai) so app.init wires tui hooks into cli.",
	)
}
