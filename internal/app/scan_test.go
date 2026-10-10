package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_ScanIsDispatchedAndRegistered(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"scan", "--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run(scan --help): %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage: squadai scan") {
		t.Fatalf("scan --help output:\n%s", stdout.String())
	}

	found := false
	for _, c := range buildCommandRegistry().Commands {
		if c.Name == "scan" {
			found = true
		}
	}
	if !found {
		t.Fatal("scan missing from the command registry")
	}
}
