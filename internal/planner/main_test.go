package planner

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates HOME because applying to a home dir outside the project
// records state in the per-user sidecar under HOME, which must never be the
// real one.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "squadai-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
