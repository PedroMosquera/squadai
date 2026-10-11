package legacyinstructions

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/pipeline"
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
	return planAndApplyFor(t, inst, vscode.New(), home, project)
}

func planAndApplyFor(t *testing.T, inst *Installer, adapter domain.Adapter, home, project string) []domain.PlannedAction {
	t.Helper()
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	installers := map[domain.ComponentID]domain.ComponentInstaller{domain.ComponentInstructions: inst}
	report, err := pipeline.New(installers, nil, project, domain.CopilotConfig{}, nil).Execute(actions)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !report.Success {
		t.Fatalf("Execute failed: %+v", report.Steps)
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

func TestPlan_AdapterWithoutThatLegacyFile_NoAction(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	legacy := filepath.Join(project, ".instructions.md")
	writeFile(t, legacy, squadaiBlocks())

	actions, err := New(project).Plan(claude.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if a.TargetPath == legacy {
			t.Errorf("action %+v targets .instructions.md, which claude never wrote", a)
		}
	}
}

const claudeUser = "# House rules\n\nUse tabs in Makefiles.\n"

func TestApply_ClaudeMD_SharedBlocksReplacedByImport(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	path := filepath.Join(project, "CLAUDE.md")
	writeFile(t, path, claudeUser+"\n"+squadaiBlocks())

	planAndApplyFor(t, New(project), claude.New(), home, project)

	got := readFile(t, path)
	if ids := marker.SectionIDs(got); len(ids) != 1 || ids[0] != ImportSectionID {
		t.Errorf("CLAUDE.md sections = %v, want only %s:\n%s", ids, ImportSectionID, got)
	}
	if rest := strings.TrimSpace(marker.InjectSection(got, ImportSectionID, "")); rest != strings.TrimSpace(claudeUser) {
		t.Errorf("CLAUDE.md user content = %q, want %q", rest, claudeUser)
	}
	again, err := New(project).Plan(claude.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range again {
		if a.Action != domain.ActionSkip {
			t.Errorf("second plan should be all skip, got %+v", a)
		}
	}
}

func TestApply_ClaudeMD_CreatedSquadaiOnlyFileKeepsImport(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	path := filepath.Join(project, "CLAUDE.md")
	writeFile(t, path, squadaiBlocks())
	if err := managed.TrackCreatedFile(project, "CLAUDE.md"); err != nil {
		t.Fatal(err)
	}

	planAndApplyFor(t, New(project), claude.New(), home, project)

	if got := strings.TrimSpace(readFile(t, path)); got != marker.OpenTag(ImportSectionID)+"\n@AGENTS.md\n"+marker.CloseTag(ImportSectionID) {
		t.Errorf("CLAUDE.md = %q, want only the import block", got)
	}
	created, err := managed.ListCreatedFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(created, "CLAUDE.md") {
		t.Errorf("CLAUDE.md still holds squadai's import and must stay tracked as created, got %v", created)
	}
}

func TestApply_CopilotInstructions_KeepsTemplateSection(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	path := filepath.Join(project, ".github", "copilot-instructions.md")
	template := marker.InjectSection("", "copilot-instructions", "template text")
	writeFile(t, path, userContent+"\n"+squadaiBlocks()+"\n"+template)

	planAndApply(t, New(project), home, project)

	got := readFile(t, path)
	if ids := marker.SectionIDs(got); len(ids) != 1 || ids[0] != "copilot-instructions" {
		t.Errorf("sections = %v, want only the copilot template section:\n%s", ids, got)
	}
	if !strings.Contains(got, "Prefer table-driven tests.") {
		t.Errorf("user content lost: %q", got)
	}
}

const cursorFM = "---\ndescription: Project coding standards and architecture\nalwaysApply: true\n---\n\n"

func TestApply_StructuredRuleFile_SquadaiOnlyIsDeletedEvenUntracked(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	path := filepath.Join(project, ".cursor", "rules", "squadai.mdc")
	writeFile(t, path, cursorFM+"Team standards.\n\n"+squadaiBlocks())

	planAndApplyFor(t, New(project, Options{TeamStandards: "Team standards."}), cursor.New(), home, project)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("squadai-only rule file should be deleted, stat err = %v", err)
	}
}

func TestApply_StructuredRuleFile_UserEditedBodyIsKept(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	path := filepath.Join(project, ".cursor", "rules", "squadai.mdc")
	writeFile(t, path, cursorFM+"Team standards.\nMy own extra rule.\n\n"+squadaiBlocks())
	if err := managed.TrackCreatedFile(project, ".cursor/rules/squadai.mdc"); err != nil {
		t.Fatal(err)
	}

	planAndApplyFor(t, New(project, Options{TeamStandards: "Team standards."}), cursor.New(), home, project)

	got := readFile(t, path)
	if !strings.Contains(got, "My own extra rule.") {
		t.Errorf("user edit lost: %q", got)
	}
	if len(marker.SectionIDs(got)) != 0 {
		t.Errorf("squadai blocks remain: %q", got)
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

func TestApply_PopulatedRefinementBlockIsKept(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	path := filepath.Join(project, ".github", "copilot-instructions.md")
	refined := marker.InjectSection("", "refinement", "Project-specific notes from /squadai-init.")
	team := marker.InjectSection("", "team", "orchestrator")
	writeFile(t, path, squadaiBlocks()+"\n"+team+"\n"+refined)

	planAndApply(t, New(project), home, project)

	got := readFile(t, path)
	if ids := marker.SectionIDs(got); len(ids) != 1 || ids[0] != "refinement" {
		t.Errorf("sections = %v, want only the populated refinement block:\n%s", ids, got)
	}
}
