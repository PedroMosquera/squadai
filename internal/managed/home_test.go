package managed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const squadaiOnlyBlock = "<!-- squadai:memory -->\nmanaged\n<!-- /squadai:memory -->\n"

// separateHomeAndProject returns a project dir and a home file path that lies
// outside it, with HOME pointing at the home dir.
func separateHomeAndProject(t *testing.T) (project, homeFile string) {
	t.Helper()
	root := t.TempDir()
	project = filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	homeFile = filepath.Join(home, ".pi", "agent", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(homeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homeFile, []byte(squadaiOnlyBlock), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	return project, homeFile
}

func TestHomeFileCreatedBySquadAIStaysDeletableOnTheSameMachine(t *testing.T) {
	project, homeFile := separateHomeAndProject(t)

	// The executor records creations as filepath.Rel from the project.
	rel, err := filepath.Rel(project, homeFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := TrackCreatedFile(project, rel); err != nil {
		t.Fatal(err)
	}

	stale, exists, err := InspectStale(project, homeFile)
	if err != nil || !exists {
		t.Fatalf("InspectStale: exists=%v err=%v", exists, err)
	}
	if !stale.Deletable() {
		t.Fatalf("a home file SquadAI created holding only its content must be deletable, got %+v", stale)
	}

	if err := ForgetFile(project, homeFile); err != nil {
		t.Fatal(err)
	}
	stale, _, err = InspectStale(project, homeFile)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Created {
		t.Error("ForgetFile left the home file recorded as created")
	}
}

func TestLegacyOutsideEntryInProjectSidecarNeverDeletesAndIsDropped(t *testing.T) {
	project, homeFile := separateHomeAndProject(t)
	rel, err := filepath.Rel(project, homeFile)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"managed_files": {"` + rel + `": {"managed_keys": ["mcpServers"]}}, "created_files": ["` + rel + `"]}`
	if err := os.MkdirAll(filepath.Dir(SidecarPath(project)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SidecarPath(project), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	stale, _, err := InspectStale(project, homeFile)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Deletable() {
		t.Fatalf("a legacy ../ created entry must not make the file deletable, got %+v", stale)
	}

	if err := TrackCreatedFile(project, "CLAUDE.md"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(SidecarPath(project))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "..") {
		t.Errorf("the next write kept the legacy entry:\n%s", data)
	}
}
