package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/verify"
)

// Every harness squadai supports reads a project-root AGENTS.md, checked
// against these docs on 2026-10-11:
//   - Claude Code: https://code.claude.com/docs/en/memory (reads AGENTS.md
//     only when no CLAUDE.md exists; CLAUDE.md supports @path imports)
//   - VS Code Copilot: https://code.visualstudio.com/docs/copilot/customization/custom-instructions
//     (chat.useAgentsMdFile, default true)
//   - Cursor: https://cursor.com/docs/context/rules
//   - Windsurf: https://docs.devin.ai/desktop/cascade/agents-md
//   - OpenCode: https://opencode.ai/docs/rules/
//   - Pi: github.com/badlogic/pi-mono packages/coding-agent/docs/configuration.md
//   - Codex: https://learn.chatgpt.com/docs/agent-configuration/agents-md
// So shared blocks land once in AGENTS.md and CLAUDE.md only imports it.

const (
	teamStandardsText = "Keep functions small and named for what they return."
	claudeUserText    = "# House rules\n\nUse tabs in Makefiles.\n"
	copilotUserText   = "# Copilot notes\n\nPrefer table-driven tests.\n"
)

// sharedBases are the section IDs of blocks every harness should read from
// AGENTS.md. Adapter-scoped variants ("memory:opencode") count as copies.
var sharedBases = []string{"memory", "efficiency", "brand", "team-standards", "team"}

var openTagRE = regexp.MustCompile(`<!-- squadai:([^ /][^ ]*) -->`)

func allSevenProject(t *testing.T) (project, home string) {
	t.Helper()
	root := t.TempDir()
	project = filepath.Join(root, "project")
	home = filepath.Join(root, "home")
	adapters := map[string]domain.AdapterConfig{}
	for _, a := range allAdapters() {
		adapters[string(a.ID())] = domain.AdapterConfig{Enabled: true}
	}
	writeProjectConfig(t, project, &domain.ProjectConfig{
		Version:  1,
		Adapters: adapters,
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentRules):  {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
			string(domain.ComponentBrand):  {Enabled: true},
		},
		Rules:       domain.RulesConfig{TeamStandards: teamStandardsText},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
	})
	return project, home
}

func writeProjectConfig(t *testing.T, project string, cfg *domain.ProjectConfig) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(project, config.ProjectConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), cfg); err != nil {
		t.Fatal(err)
	}
}

func applyForTest(t *testing.T, project, home string) *domain.ApplyReport {
	t.Helper()
	merged, err := loadAndMerge(home, project)
	if err != nil {
		t.Fatal(err)
	}
	applyDefaultProfile(merged)
	p := planner.New()
	actions, err := p.Plan(merged, allAdapters(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil).Execute(actions)
	if err != nil || !report.Success {
		t.Fatalf("apply failed: %v %+v", err, report)
	}
	// Per-adapter verify must agree that the one shared copy is current.
	vr, err := verify.New().Verify(merged, allAdapters(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range vr.Results {
		if !r.Passed && r.Component != "health" {
			t.Errorf("verify after apply: %s/%s failed: %s", r.Component, r.Check, r.Message)
		}
	}
	return report
}

func verifyCIAfterApply(t *testing.T, project string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_ACTIONS", "")
	t.Chdir(project)
	out, err := runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("verify --ci should pass, got %v\n%s", err, out)
	}
}

// sharedBlockFiles maps each shared base section ID to the project files
// (relative) holding it, one entry per copy. An adapter-scoped copy is
// recorded as "<file> as <id>", so it never matches a bare "AGENTS.md".
func sharedBlockFiles(t *testing.T, project string) map[string][]string {
	t.Helper()
	got := map[string][]string{}
	err := filepath.WalkDir(project, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == config.ProjectConfigDir {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(project, path)
		for _, m := range openTagRE.FindAllStringSubmatch(string(data), -1) {
			for _, base := range sharedBases {
				switch {
				case m[1] == base:
					got[base] = append(got[base], filepath.ToSlash(rel))
				case strings.HasPrefix(m[1], base+":"):
					got[base] = append(got[base], filepath.ToSlash(rel)+" as "+m[1])
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func sectionIDs(doc string) []string {
	var ids []string
	for _, m := range openTagRE.FindAllStringSubmatch(doc, -1) {
		ids = append(ids, m[1])
	}
	sort.Strings(ids)
	return ids
}

func readProjectFile(t *testing.T, project, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func writeProjectRelFile(t *testing.T, project, rel, content string) {
	t.Helper()
	path := filepath.Join(project, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireClaudeImportOnly(t *testing.T, doc, userText string) {
	t.Helper()
	if ids := sectionIDs(doc); len(ids) != 1 || ids[0] != "agents-md" {
		t.Errorf("CLAUDE.md squadai sections = %v, want only [agents-md]\n%s", ids, doc)
	}
	if got := marker.ExtractSection(doc, "agents-md"); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md import block = %q, want %q", got, "@AGENTS.md")
	}
	if userText != "" {
		rest := strings.TrimSpace(marker.InjectSection(doc, "agents-md", ""))
		if rest != strings.TrimSpace(userText) {
			t.Errorf("CLAUDE.md user content = %q, want %q", rest, strings.TrimSpace(userText))
		}
	}
}

func TestAgentsMD_FreshApplyAllAdapters_SharedBlocksOnceInAgentsMD(t *testing.T) {
	project, home := allSevenProject(t)
	applyForTest(t, project, home)

	got := sharedBlockFiles(t, project)
	for _, base := range sharedBases {
		files := got[base]
		if len(files) != 1 || files[0] != "AGENTS.md" {
			t.Errorf("shared block %q found in %v, want exactly once in AGENTS.md", base, files)
		}
	}
	requireClaudeImportOnly(t, readProjectFile(t, project, "CLAUDE.md"), "")
	copilot := readProjectFile(t, project, ".github/copilot-instructions.md")
	if ids := sectionIDs(copilot); len(ids) != 1 || ids[0] != "copilot-instructions" {
		t.Errorf("copilot-instructions.md sections = %v, want only the copilot template section", ids)
	}

	verifyCIAfterApply(t, project)
}

func oldInstructionBlocks(agent string) string {
	doc := marker.InjectSection("", "memory:"+agent, "## Project Memory Protocol\n\nold "+agent+" memory")
	doc = marker.InjectSection(doc, "team-standards", teamStandardsText)
	doc = marker.InjectSection(doc, "efficiency:"+agent, "## Session Efficiency Protocol\n\nold")
	return marker.InjectSection(doc, "brand:"+agent, "```text\nold banner\n```")
}

func TestAgentsMD_MigrationStripsSharedBlocksFromOldFiles(t *testing.T) {
	project, home := allSevenProject(t)
	writeProjectRelFile(t, project, "CLAUDE.md", claudeUserText+"\n"+oldInstructionBlocks("claude-code"))
	writeProjectRelFile(t, project, ".github/copilot-instructions.md",
		copilotUserText+"\n"+marker.InjectSection(oldInstructionBlocks("vscode-copilot"), "team", "# old solo orchestrator"))
	agentsMD := oldInstructionBlocks("opencode")
	agentsMD = marker.InjectSection(agentsMD, "memory:pi", "old pi memory")
	agentsMD = marker.InjectSection(agentsMD, "brand:codex", "old codex banner")
	writeProjectRelFile(t, project, "AGENTS.md", agentsMD)
	writeProjectRelFile(t, project, ".cursor/rules/squadai.mdc",
		"---\ndescription: Project coding standards and architecture\nalwaysApply: true\n---\n\n"+teamStandardsText+"\n\n"+
			marker.InjectSection("", "memory:cursor", "old cursor memory"))
	if err := managed.TrackCreatedFile(project, ".cursor/rules/squadai.mdc"); err != nil {
		t.Fatal(err)
	}
	writeProjectRelFile(t, project, ".windsurf/rules/squadai.md",
		"---\ntrigger: always_on\n---\n\n"+teamStandardsText+"\n\n"+
			marker.InjectSection("", "efficiency:windsurf", "old windsurf efficiency"))

	applyForTest(t, project, home)

	requireClaudeImportOnly(t, readProjectFile(t, project, "CLAUDE.md"), claudeUserText)

	copilot := readProjectFile(t, project, ".github/copilot-instructions.md")
	if ids := sectionIDs(copilot); len(ids) != 1 || ids[0] != "copilot-instructions" {
		t.Errorf("copilot-instructions.md sections = %v, want only the copilot template section\n%s", ids, copilot)
	}
	if rest := strings.TrimSpace(marker.InjectSection(copilot, "copilot-instructions", "")); rest != strings.TrimSpace(copilotUserText) {
		t.Errorf("copilot-instructions.md user content = %q, want %q", rest, strings.TrimSpace(copilotUserText))
	}

	for _, rel := range []string{".cursor/rules/squadai.mdc", ".windsurf/rules/squadai.md"} {
		if _, err := os.Stat(filepath.Join(project, rel)); !os.IsNotExist(err) {
			t.Errorf("%s held only squadai content and should be deleted, stat err = %v", rel, err)
		}
	}

	got := sharedBlockFiles(t, project)
	for _, base := range sharedBases {
		files := got[base]
		if len(files) != 1 || files[0] != "AGENTS.md" {
			t.Errorf("after migration shared block %q found in %v, want exactly once in AGENTS.md", base, files)
		}
	}

	verifyCIAfterApply(t, project)
}

func TestAgentsMD_DisablingClaudeKeepsAgentsMDAndStripsOnlyTheImport(t *testing.T) {
	project, home := allSevenProject(t)
	writeProjectRelFile(t, project, "CLAUDE.md", claudeUserText)
	applyForTest(t, project, home)
	agentsBefore := readProjectFile(t, project, "AGENTS.md")
	requireClaudeImportOnly(t, readProjectFile(t, project, "CLAUDE.md"), claudeUserText)

	cfg, err := config.LoadProject(project)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: false}
	writeProjectConfig(t, project, cfg)
	applyForTest(t, project, home)

	if got := readProjectFile(t, project, "CLAUDE.md"); strings.TrimSpace(got) != strings.TrimSpace(claudeUserText) {
		t.Errorf("CLAUDE.md after disabling Claude = %q, want only the user content", got)
	}
	if got := readProjectFile(t, project, "AGENTS.md"); got != agentsBefore {
		t.Errorf("AGENTS.md changed when Claude was disabled:\nbefore:\n%s\nafter:\n%s", agentsBefore, got)
	}
}
func TestAgentsMD_MigrationNotePrintedOnlyWhenMigrating(t *testing.T) {
	migrated := &domain.ApplyReport{Steps: []domain.StepResult{{
		Action: domain.PlannedAction{ID: "claude-code-legacy-instructions-0", Component: domain.ComponentInstructions, Action: domain.ActionUpdate},
		Status: domain.StepSuccess,
	}}}
	if !strings.Contains(agentsMDMigrationNote(migrated), "AGENTS.md") {
		t.Errorf("a migration step should produce the AGENTS.md note")
	}
	importOnly := &domain.ApplyReport{Steps: []domain.StepResult{{
		Action: domain.PlannedAction{ID: "claude-code-agents-md-import", Component: domain.ComponentInstructions, Action: domain.ActionCreate},
		Status: domain.StepSuccess,
	}}}
	if note := agentsMDMigrationNote(importOnly); note != "" {
		t.Errorf("a fresh import is not a migration, got note %q", note)
	}
}
