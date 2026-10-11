package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
)

// skillReadDirs lists the project skill directories each harness documents
// that it scans, as of 2026-10-11:
//
//	claude-code    https://code.claude.com/docs/en/skills
//	codex          https://learn.chatgpt.com/docs/build-skills
//	pi             https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md
//	opencode       https://opencode.ai/docs/skills/
//	cursor         https://cursor.com/docs/context/skills
//	vscode-copilot https://code.visualstudio.com/docs/copilot/customization/agent-skills
//	windsurf       https://docs.devin.ai/desktop/cascade/skills
var skillReadDirs = map[domain.AgentID][]string{
	domain.AgentClaudeCode:    {".claude/skills"},
	domain.AgentCodex:         {".agents/skills"},
	domain.AgentPi:            {".agents/skills", ".pi/skills"},
	domain.AgentOpenCode:      {".agents/skills", ".opencode/skills", ".claude/skills"},
	domain.AgentCursor:        {".agents/skills", ".cursor/skills", ".claude/skills"},
	domain.AgentVSCodeCopilot: {".agents/skills", ".github/skills", ".claude/skills"},
	domain.AgentWindsurf:      {".agents/skills", ".devin/skills", ".windsurf/skills"},
}

const houseStyleSkill = "---\ndescription: House style\n---\n\nPrefer small functions.\n"

// applySkillsProject writes a tdd project with every adapter and the skills
// and agents components enabled, lets seed place files first, applies it, and
// leaves the test on a bare machine in the project directory.
func applySkillsProject(t *testing.T, seed func(project string)) (string, []domain.PlannedAction, *planner.Planner) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	applyHome := filepath.Join(root, "apply-home")
	adapters := map[string]domain.AdapterConfig{}
	for id := range skillReadDirs {
		adapters[string(id)] = domain.AdapterConfig{Enabled: true}
	}
	proj := &domain.ProjectConfig{
		Version:     1,
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
		Adapters:    adapters,
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentSkills): {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
		},
		Skills: map[string]domain.SkillDef{
			"house-style": {Description: "House style", Content: "Prefer small functions."},
		},
	}
	if err := os.MkdirAll(filepath.Join(project, config.ProjectConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), proj); err != nil {
		t.Fatal(err)
	}
	if seed != nil {
		seed(project)
	}

	merged, err := loadAndMerge(applyHome, project)
	if err != nil {
		t.Fatal(err)
	}
	applyDefaultProfile(merged)
	p := planner.New()
	actions, err := p.Plan(merged, allAdapters(), applyHome, project)
	if err != nil {
		t.Fatal(err)
	}
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil).Execute(actions)
	if err != nil || !report.Success {
		t.Fatalf("apply failed: %v", err)
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_ACTIONS", "")
	t.Chdir(project)
	return project, actions, p
}

// skillFilesByRoot maps each "<dir>/skills" root to the skill paths found
// under it, relative to that root.
func skillFilesByRoot(t *testing.T, project string) map[string][]string {
	t.Helper()
	byRoot := map[string][]string{}
	err := filepath.WalkDir(project, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, project+string(filepath.Separator)))
		parts := strings.SplitN(rel, "/", 3)
		if len(parts) < 3 || parts[1] != "skills" {
			t.Fatalf("SKILL.md outside a skills directory: %s", rel)
		}
		root := parts[0] + "/skills"
		byRoot[root] = append(byRoot[root], strings.TrimSuffix(parts[2], "/SKILL.md"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for root := range byRoot {
		sort.Strings(byRoot[root])
	}
	return byRoot
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func embeddedSkill(t *testing.T, rel string) string {
	t.Helper()
	content, err := assets.Read("skills/" + rel + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestApplySkills_WritesEachSkillOnceToAgentsSkills(t *testing.T) {
	project, _, _ := applySkillsProject(t, nil)

	byRoot := skillFilesByRoot(t, project)
	var roots []string
	for root := range byRoot {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	// Claude Code is the only enabled harness that does not read .agents/skills.
	if got, want := strings.Join(roots, ","), ".agents/skills,.claude/skills"; got != want {
		t.Fatalf("skill roots = %s, want %s", got, want)
	}

	shared := byRoot[".agents/skills"]
	for _, want := range []string{"house-style", "context-discipline", "brainstorming"} {
		if !slices.Contains(shared, want) {
			t.Errorf(".agents/skills is missing %s; has %v", want, shared)
		}
	}
	if strings.Join(byRoot[".claude/skills"], ",") != strings.Join(shared, ",") {
		t.Errorf(".claude/skills = %v, want the same skills as .agents/skills %v", byRoot[".claude/skills"], shared)
	}

	if out, err := runVerifyCIForTest(t); err != nil {
		t.Fatalf("verify --ci after a fresh apply: %v\n%s", err, out)
	}
}

func TestApplySkills_MigrationRemovesOnlyUnchangedRedundantCopies(t *testing.T) {
	edited := embeddedSkill(t, "shared/testing") + "\nLocal note kept by the user.\n"
	project, _, _ := applySkillsProject(t, func(project string) {
		writeSkillFile(t, filepath.Join(project, ".opencode/skills/shared/context-discipline/SKILL.md"), embeddedSkill(t, "shared/context-discipline"))
		writeSkillFile(t, filepath.Join(project, ".pi/skills/house-style/SKILL.md"), houseStyleSkill)
		writeSkillFile(t, filepath.Join(project, ".cursor/skills/shared/testing/SKILL.md"), edited)
	})

	for _, rel := range []string{".opencode/skills/shared/context-discipline/SKILL.md", ".pi/skills/house-style/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(project, rel)); !os.IsNotExist(err) {
			t.Errorf("%s matches what squadai writes and the harness reads .agents/skills, so it should be removed (stat err %v)", rel, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(project, ".cursor/skills/shared/testing/SKILL.md"))
	if err != nil || string(got) != edited {
		t.Errorf("user-edited .cursor/skills copy must survive unchanged (err %v)", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude/skills/context-discipline/SKILL.md")); err != nil {
		t.Errorf("Claude Code does not read .agents/skills, so its copy must stay: %v", err)
	}

	if out, err := runVerifyCIForTest(t); err != nil {
		t.Fatalf("verify --ci after migration: %v\n%s", err, out)
	}
}

var skillRef = regexp.MustCompile("`([^`\\s]*)/[a-z0-9-]+/SKILL\\.md`")

func TestApplySkills_OrchestratorPointsAtADirTheHarnessReads(t *testing.T) {
	project, actions, p := applySkillsProject(t, nil)
	installer, ok := p.ComponentInstallers()[domain.ComponentAgents].(*agents.Installer)
	if !ok {
		t.Fatal("agents installer not configured")
	}

	checked := map[domain.AgentID]bool{}
	for _, a := range actions {
		if a.Component != domain.ComponentAgents || !strings.Contains(a.Description, "orchestrator") {
			continue
		}
		content, err := installer.RenderContent(a)
		if err != nil {
			t.Fatalf("render %s for %s: %v", a.Description, a.Agent, err)
		}
		refs := skillRef.FindAllStringSubmatch(content, -1)
		if len(refs) == 0 {
			continue
		}
		checked[a.Agent] = true
		for _, ref := range refs {
			if !slices.Contains(skillReadDirs[a.Agent], ref[1]) {
				t.Errorf("%s orchestrator points at %q, which %s does not read (reads %v)", a.Agent, ref[1], a.Agent, skillReadDirs[a.Agent])
				continue
			}
			path := strings.Trim(ref[0], "`")
			if _, err := os.Stat(filepath.Join(project, path)); err != nil {
				t.Errorf("%s orchestrator references %s, which apply did not write", a.Agent, path)
			}
		}
	}
	for id := range skillReadDirs {
		if !checked[id] {
			t.Errorf("no rendered orchestrator with skill references for %s", id)
		}
	}
}

var frontmatterName = regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`)

// Claude Code, VS Code Copilot and Windsurf document only <skills-dir>/<name>/SKILL.md,
// and VS Code and Cursor drop a skill whose frontmatter name differs from its folder.
func TestApplySkills_WritesOneLevelDirsNamedLikeTheSkill(t *testing.T) {
	project, _, _ := applySkillsProject(t, nil)

	for root, rels := range skillFilesByRoot(t, project) {
		for _, rel := range rels {
			if strings.Contains(rel, "/") {
				t.Errorf("%s/%s/SKILL.md is nested; harnesses read <skills-dir>/<name>/SKILL.md", root, rel)
				continue
			}
			data, err := os.ReadFile(filepath.Join(project, root, rel, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if m := frontmatterName.FindStringSubmatch(string(data)); m != nil && m[1] != rel {
				t.Errorf("%s/%s/SKILL.md declares name %q; it must match its directory", root, rel, m[1])
			}
		}
	}
}

var anySkillPath = regexp.MustCompile("`([^`\\s]*/SKILL\\.md)`")

func TestApplySkills_EverySkillPathInGeneratedFilesExists(t *testing.T) {
	project, _, _ := applySkillsProject(t, nil)

	checked := 0
	err := filepath.WalkDir(project, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "SKILL.md" || filepath.Ext(path) != ".md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(project, path)
		for _, m := range anySkillPath.FindAllStringSubmatch(string(data), -1) {
			checked++
			if _, err := os.Stat(filepath.Join(project, m[1])); err != nil {
				t.Errorf("%s references %s, which apply did not write", rel, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no SKILL.md references found in generated files")
	}
}

func TestApplySkills_MigrationFlattensNestedCopies(t *testing.T) {
	edited := embeddedSkill(t, "shared/code-review") + "\nLocal note kept by the user.\n"
	project, _, _ := applySkillsProject(t, func(project string) {
		writeSkillFile(t, filepath.Join(project, ".agents/skills/shared/testing/SKILL.md"), embeddedSkill(t, "shared/testing"))
		writeSkillFile(t, filepath.Join(project, ".claude/skills/tdd/brainstorming/SKILL.md"), embeddedSkill(t, "tdd/brainstorming"))
		writeSkillFile(t, filepath.Join(project, ".agents/skills/shared/code-review/SKILL.md"), edited)
		// An earlier migration deleted files but left their folders behind.
		if err := os.MkdirAll(filepath.Join(project, ".opencode/skills/shared/find-skills"), 0o755); err != nil {
			t.Fatal(err)
		}
	})

	for _, rel := range []string{".agents/skills/shared/testing", ".claude/skills/tdd", ".opencode/skills"} {
		if _, err := os.Stat(filepath.Join(project, rel)); !os.IsNotExist(err) {
			t.Errorf("%s held only unchanged nested copies or nothing, so it should be gone (stat err %v)", rel, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(project, ".agents/skills/shared/code-review/SKILL.md"))
	if err != nil || string(got) != edited {
		t.Errorf("user-edited nested copy must survive unchanged (err %v)", err)
	}
	for _, rel := range []string{".agents/skills/testing/SKILL.md", ".agents/skills/code-review/SKILL.md", ".claude/skills/brainstorming/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(project, rel)); err != nil {
			t.Errorf("flat copy %s missing: %v", rel, err)
		}
	}

	if out, err := runVerifyCIForTest(t); err != nil {
		t.Fatalf("verify --ci after migration: %v\n%s", err, out)
	}
}
