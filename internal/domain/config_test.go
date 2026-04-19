package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// ─── DefaultTeam ────────────────────────────────────────────────────────────

func TestDefaultTeam_TDD_HasSixRoles(t *testing.T) {
	team := DefaultTeam(MethodologyTDD)
	if len(team) != 6 {
		t.Errorf("TDD team len = %d, want 6", len(team))
	}
}

func TestDefaultTeam_TDD_RoleNames(t *testing.T) {
	team := DefaultTeam(MethodologyTDD)
	expectedRoles := []string{"orchestrator", "brainstormer", "planner", "implementer", "reviewer", "debugger"}
	for _, role := range expectedRoles {
		if _, ok := team[role]; !ok {
			t.Errorf("TDD team missing role %q", role)
		}
	}
}

func TestDefaultTeam_SDD_HasEightRoles(t *testing.T) {
	team := DefaultTeam(MethodologySDD)
	if len(team) != 8 {
		t.Errorf("SDD team len = %d, want 8", len(team))
	}
}

func TestDefaultTeam_SDD_RoleNames(t *testing.T) {
	team := DefaultTeam(MethodologySDD)
	expectedRoles := []string{"orchestrator", "explorer", "proposer", "spec-writer", "designer", "task-planner", "implementer", "verifier"}
	for _, role := range expectedRoles {
		if _, ok := team[role]; !ok {
			t.Errorf("SDD team missing role %q", role)
		}
	}
}

func TestDefaultTeam_Conventional_HasFourRoles(t *testing.T) {
	team := DefaultTeam(MethodologyConventional)
	if len(team) != 4 {
		t.Errorf("Conventional team len = %d, want 4", len(team))
	}
}

func TestDefaultTeam_Conventional_RoleNames(t *testing.T) {
	team := DefaultTeam(MethodologyConventional)
	expectedRoles := []string{"orchestrator", "implementer", "reviewer", "tester"}
	for _, role := range expectedRoles {
		if _, ok := team[role]; !ok {
			t.Errorf("Conventional team missing role %q", role)
		}
	}
}

func TestDefaultTeam_Empty_ReturnsNil(t *testing.T) {
	team := DefaultTeam("")
	if team != nil {
		t.Errorf("empty methodology should return nil, got %v", team)
	}
}

// ─── ProjectMeta JSON serialization ─────────────────────────────────────────

func TestProjectMeta_PackageManager_OmitemptyWhenEmpty(t *testing.T) {
	meta := ProjectMeta{
		Name:     "my-service",
		Language: "Go",
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(data), "package_manager") {
		t.Errorf("JSON output should not contain 'package_manager' when it is empty, got: %s", data)
	}
}

func TestProjectMeta_PackageManager_PresentWhenSet(t *testing.T) {
	meta := ProjectMeta{
		Name:           "my-app",
		Language:       "TypeScript",
		PackageManager: "pnpm",
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"package_manager":"pnpm"`) {
		t.Errorf("JSON output should contain 'package_manager', got: %s", data)
	}
}

func TestDefaultTeam_OrchestratorAlwaysPresent(t *testing.T) {
	methodologies := []Methodology{MethodologyTDD, MethodologySDD, MethodologyConventional}
	for _, m := range methodologies {
		team := DefaultTeam(m)
		if _, ok := team["orchestrator"]; !ok {
			t.Errorf("methodology %q missing orchestrator role", m)
		}
	}
}

func TestDefaultTeam_AllRolesHaveSubagentMode(t *testing.T) {
	methodologies := []Methodology{MethodologyTDD, MethodologySDD, MethodologyConventional}
	for _, m := range methodologies {
		team := DefaultTeam(m)
		for name, role := range team {
			if role.Mode != "subagent" {
				t.Errorf("methodology %q role %q: Mode = %q, want %q", m, name, role.Mode, "subagent")
			}
		}
	}
}

// ─── DefaultMCPCatalog ───────────────────────────────────────────────────────

func TestDefaultMCPCatalog_ReturnsFiveServers(t *testing.T) {
	catalog := DefaultMCPCatalog()
	if len(catalog) != 5 {
		t.Errorf("DefaultMCPCatalog() len = %d, want 5", len(catalog))
	}
}

func TestDefaultMCPCatalog_ExactlyTwoPreChecked(t *testing.T) {
	catalog := DefaultMCPCatalog()
	var preChecked []string
	for _, s := range catalog {
		if s.PreChecked {
			preChecked = append(preChecked, s.Name)
		}
	}
	if len(preChecked) != 2 {
		t.Errorf("expected 2 pre-checked servers, got %d: %v", len(preChecked), preChecked)
	}
}

func TestDefaultMCPCatalog_PreCheckedAreContext7AndGitHub(t *testing.T) {
	catalog := DefaultMCPCatalog()
	preChecked := make(map[string]bool)
	for _, s := range catalog {
		if s.PreChecked {
			preChecked[s.Name] = true
		}
	}
	for _, expected := range []string{"context7", "github"} {
		if !preChecked[expected] {
			t.Errorf("expected %q to be pre-checked", expected)
		}
	}
}

func TestDefaultMCPCatalog_AllHaveNonEmptyNameAndDescription(t *testing.T) {
	for _, s := range DefaultMCPCatalog() {
		if s.Name == "" {
			t.Error("found server with empty Name")
		}
		if s.Description == "" {
			t.Errorf("server %q has empty Description", s.Name)
		}
	}
}

func TestDefaultMCPCatalog_AllHaveValidType(t *testing.T) {
	for _, s := range DefaultMCPCatalog() {
		if s.Type != "remote" && s.Type != "local" {
			t.Errorf("server %q has invalid Type %q", s.Name, s.Type)
		}
	}
}

func TestDefaultMCPCatalog_AllRolesHaveSkillRef(t *testing.T) {
	// Orchestrators don't have SkillRef — they coordinate. All others should.
	methodologies := []Methodology{MethodologyTDD, MethodologySDD, MethodologyConventional}
	for _, m := range methodologies {
		team := DefaultTeam(m)
		for name, role := range team {
			if name == "orchestrator" {
				continue // orchestrators are allowed to have no SkillRef
			}
			// conventional implementer also has no SkillRef (general purpose)
			if m == MethodologyConventional && name == "implementer" {
				continue
			}
			if role.SkillRef == "" {
				t.Errorf("methodology %q role %q: missing SkillRef", m, name)
			}
		}
	}
}
