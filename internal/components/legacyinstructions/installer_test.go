package legacyinstructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

const userContent = "# My Copilot notes\n\nPrefer table-driven tests.\n"

func squadaiBlocks() string {
	doc := marker.InjectSection("", "memory", "squadai memory protocol")
	return marker.InjectSection(doc, "team-standards", "squadai team standards")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func planAndApply(t *testing.T, inst *Installer, home, project string) []domain.PlannedAction {
	t.Helper()
	actions, err := inst.Plan(vscode.New(), home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply %s: %v", a.ID, err)
		}
	}
	return actions
}

func TestApply_ProjectLegacyFile_KeepsUserContentDropsSquadaiBlocks(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	legacy := filepath.Join(project, ".instructions.md")
	writeFile(t, legacy, userContent+"\n"+squadaiBlocks())

	actions := planAndApply(t, New(project), home, project)

	if len(actions) != 1 || actions[0].TargetPath != legacy {
		t.Fatalf("actions = %+v, want one action targeting %s", actions, legacy)
	}
	got := readFile(t, legacy)
	if got != userContent {
		t.Errorf("legacy file = %q, want only the user content %q", got, userContent)
	}
}

func TestApply_UserLevelLegacyFile_KeepsUserContentDropsSquadaiBlocks(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	legacy := filepath.Join(vscode.ConfigDir(home), ".instructions.md")
	writeFile(t, legacy, squadaiBlocks()+"\n"+userContent)

	planAndApply(t, New(project), home, project)

	got := readFile(t, legacy)
	if strings.Contains(got, "squadai") {
		t.Errorf("legacy file still holds squadai content: %q", got)
	}
	if !strings.Contains(got, "Prefer table-driven tests.") {
		t.Errorf("legacy file lost user content: %q", got)
	}
}

func TestApply_SquadaiOnlyLegacyFile_IsDeletedAndUntracked(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	legacy := filepath.Join(project, ".instructions.md")
	writeFile(t, legacy, squadaiBlocks())
	if err := managed.TrackCreatedFile(project, ".instructions.md"); err != nil {
		t.Fatal(err)
	}

	planAndApply(t, New(project), home, project)

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("squadai-only legacy file should be deleted, stat err = %v", err)
	}
	created, err := managed.ListCreatedFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range created {
		if f == ".instructions.md" {
			t.Errorf("deleted legacy file is still tracked as created: %v", created)
		}
	}
}

func TestPlan_LegacyFileWithoutSquadaiBlocks_NoActionFileUntouched(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	legacy := filepath.Join(project, ".instructions.md")
	writeFile(t, legacy, userContent)

	actions := planAndApply(t, New(project), home, project)

	if len(actions) != 0 {
		t.Errorf("actions = %+v, want none for a file squadai never wrote to", actions)
	}
	if got := readFile(t, legacy); got != userContent {
		t.Errorf("user-only file changed: %q", got)
	}
}

func TestPlan_AdapterWithoutLegacyFiles_NoAction(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(project, ".instructions.md"), squadaiBlocks())

	actions, err := New(project).Plan(claude.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Errorf("actions = %+v, want none for an adapter that never wrote .instructions.md", actions)
	}
}

func TestVerify_FailsUntilLegacyBlocksAreRemoved(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(project, ".instructions.md"), userContent+"\n"+squadaiBlocks())
	inst := New(project)

	before, err := inst.Verify(vscode.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Passed {
		t.Fatalf("Verify before migration = %+v, want one failing check", before)
	}

	planAndApply(t, inst, home, project)

	after, err := inst.Verify(vscode.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range after {
		if !r.Passed {
			t.Errorf("Verify after migration still fails: %+v", r)
		}
	}
}

func TestRender_ShowsFileWithoutSquadaiBlocks(t *testing.T) {
	got := Render([]byte(userContent + "\n" + squadaiBlocks()))
	if string(got) != userContent {
		t.Errorf("Render = %q, want %q", got, userContent)
	}
}
