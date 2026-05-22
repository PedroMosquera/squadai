package tokenprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApproxTokens_Empty(t *testing.T) {
	if got := ApproxTokens(nil); got != 0 {
		t.Errorf("ApproxTokens(nil) = %d, want 0", got)
	}
	if got := ApproxTokens([]byte{}); got != 0 {
		t.Errorf("ApproxTokens([]) = %d, want 0", got)
	}
}

func TestApproxTokens_Rounding(t *testing.T) {
	cases := []struct {
		bytes int
		want  int
	}{
		{1, 1},
		{4, 1},
		{5, 2},
		{8, 2},
		{9, 3},
		{400, 100},
	}
	for _, c := range cases {
		content := make([]byte, c.bytes)
		got := ApproxTokens(content)
		if got != c.want {
			t.Errorf("ApproxTokens(%d bytes) = %d, want %d", c.bytes, got, c.want)
		}
	}
}

func TestScanPaths_CountsTokens(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.md")
	p2 := filepath.Join(dir, "b.md")
	// Write 400 bytes to a.md (→ 100 tokens), 800 bytes to b.md (→ 200 tokens)
	if err := os.WriteFile(p1, make([]byte, 400), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, make([]byte, 800), 0644); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		p1: "agents",
		p2: "agents",
	}
	report, err := ScanPaths(paths)
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}

	if report.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", report.TotalTokens)
	}
	if report.TotalBytes != 1200 {
		t.Errorf("TotalBytes = %d, want 1200", report.TotalBytes)
	}
	cat := report.ByCategory["agents"]
	if cat.Tokens != 300 {
		t.Errorf("agents tokens = %d, want 300", cat.Tokens)
	}
	if cat.Files != 2 {
		t.Errorf("agents files = %d, want 2", cat.Files)
	}
}

func TestScanPaths_MissingFileIsSkipped(t *testing.T) {
	paths := map[string]string{
		"/nonexistent/path/file.md": "agents",
	}
	report, err := ScanPaths(paths)
	if err != nil {
		t.Fatalf("ScanPaths with missing file should not error: %v", err)
	}
	if report.TotalTokens != 0 {
		t.Errorf("expected 0 tokens for missing file, got %d", report.TotalTokens)
	}
	if report.Missing != 1 {
		t.Errorf("expected Missing = 1, got %d", report.Missing)
	}
}

func TestScanPaths_MultipleCategories(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "orchestrator.md")
	skill := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(agent, make([]byte, 800), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, make([]byte, 400), 0644); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		agent: "agents",
		skill: "skills",
	}
	report, err := ScanPaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if report.ByCategory["agents"].Files != 1 {
		t.Errorf("agents files = %d, want 1", report.ByCategory["agents"].Files)
	}
	if report.ByCategory["skills"].Files != 1 {
		t.Errorf("skills files = %d, want 1", report.ByCategory["skills"].Files)
	}
	if report.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", report.TotalTokens)
	}
}
