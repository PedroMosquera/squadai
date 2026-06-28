package methodology

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── Load ─────────────────────────────────────────────────────────────────────

func TestLoad_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	spec, err := Load(dir, "nope")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if spec != nil {
		t.Errorf("expected nil spec, got %+v", spec)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	methDir := filepath.Join(dir, ".squadai", "methodologies")
	if err := os.MkdirAll(methDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	content := `{
		"name": "my-method",
		"description": "a custom methodology",
		"roles": {
			"orchestrator": {
				"description": "leads the team",
				"mode": "subagent",
				"delegates_to": ["implementer"]
			},
			"implementer": {
				"description": "writes code",
				"mode": "inline",
				"skill_ref": "shared/testing",
				"model": "cheap"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(methDir, "my-method.json"), []byte(content), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	spec, err := Load(dir, "my-method")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if spec.Name != "my-method" {
		t.Errorf("Name = %q, want %q", spec.Name, "my-method")
	}
	if spec.Description != "a custom methodology" {
		t.Errorf("Description = %q, want %q", spec.Description, "a custom methodology")
	}
	if len(spec.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(spec.Roles))
	}

	orch, ok := spec.Roles["orchestrator"]
	if !ok {
		t.Fatal("orchestrator role missing")
	}
	if orch.Description != "leads the team" {
		t.Errorf("orchestrator Description = %q", orch.Description)
	}
	if orch.Mode != "subagent" {
		t.Errorf("orchestrator Mode = %q, want subagent", orch.Mode)
	}
	if len(orch.DelegatesTo) != 1 || orch.DelegatesTo[0] != "implementer" {
		t.Errorf("orchestrator DelegatesTo = %v, want [implementer]", orch.DelegatesTo)
	}

	impl, ok := spec.Roles["implementer"]
	if !ok {
		t.Fatal("implementer role missing")
	}
	if impl.Mode != "inline" {
		t.Errorf("implementer Mode = %q, want inline", impl.Mode)
	}
	if impl.SkillRef != "shared/testing" {
		t.Errorf("implementer SkillRef = %q, want shared/testing", impl.SkillRef)
	}
	if impl.Model != "cheap" {
		t.Errorf("implementer Model = %q, want cheap", impl.Model)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	methDir := filepath.Join(dir, ".squadai", "methodologies")
	if err := os.MkdirAll(methDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(methDir, "bad.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := Load(dir, "bad"); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ─── LoadAll ──────────────────────────────────────────────────────────────────

func TestLoadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	methDir := filepath.Join(dir, ".squadai", "methodologies")
	if err := os.MkdirAll(methDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	specs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected empty slice, got %d specs", len(specs))
	}
}

func TestLoadAll_WithSpecs(t *testing.T) {
	dir := t.TempDir()
	methDir := filepath.Join(dir, ".squadai", "methodologies")
	if err := os.MkdirAll(methDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	specA := `{"name":"alpha","description":"a","roles":{}}`
	specB := `{"name":"beta","description":"b","roles":{}}`
	if err := os.WriteFile(filepath.Join(methDir, "alpha.json"), []byte(specA), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(methDir, "beta.json"), []byte(specB), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	specs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	// LoadAll returns specs sorted by name.
	if specs[0].Name != "alpha" {
		t.Errorf("specs[0].Name = %q, want alpha", specs[0].Name)
	}
	if specs[1].Name != "beta" {
		t.Errorf("specs[1].Name = %q, want beta", specs[1].Name)
	}
}

func TestLoadAll_NoDir(t *testing.T) {
	dir := t.TempDir()
	specs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected empty slice, got %d specs", len(specs))
	}
}

// ─── ToTeamRoles ──────────────────────────────────────────────────────────────

func TestToTeamRoles(t *testing.T) {
	spec := &Spec{
		Name:        "custom",
		Description: "d",
		Roles: map[string]RoleSpec{
			"orchestrator": {
				Description: "leads",
				Mode:        "subagent",
				DelegatesTo: []string{"implementer"},
			},
			"implementer": {
				Description: "writes",
				Mode:        "inline",
				SkillRef:    "shared/testing",
				Model:       "cheap",
			},
		},
	}
	roles := ToTeamRoles(spec)
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}

	orch, ok := roles["orchestrator"]
	if !ok {
		t.Fatal("orchestrator missing")
	}
	if orch.Description != "leads" || orch.Mode != "subagent" {
		t.Errorf("orchestrator = %+v", orch)
	}
	if len(orch.DelegatesTo) != 1 || orch.DelegatesTo[0] != "implementer" {
		t.Errorf("orchestrator DelegatesTo = %v, want [implementer]", orch.DelegatesTo)
	}

	impl, ok := roles["implementer"]
	if !ok {
		t.Fatal("implementer missing")
	}
	if impl.Description != "writes" || impl.Mode != "inline" {
		t.Errorf("implementer = %+v", impl)
	}
	if impl.SkillRef != "shared/testing" || impl.Model != "cheap" {
		t.Errorf("implementer SkillRef=%q Model=%q", impl.SkillRef, impl.Model)
	}
}

func TestToTeamRoles_NilSpec(t *testing.T) {
	if roles := ToTeamRoles(nil); roles != nil {
		t.Errorf("expected nil for nil spec, got %v", roles)
	}
}

// ─── IsBuiltIn ────────────────────────────────────────────────────────────────

func TestIsBuiltIn(t *testing.T) {
	builtIns := []string{"tdd", "sdd", "conventional"}
	for _, name := range builtIns {
		if !IsBuiltIn(name) {
			t.Errorf("IsBuiltIn(%q) = false, want true", name)
		}
	}
	others := []string{"custom", "tdd-v2", "", "TDD", "SDD"}
	for _, name := range others {
		if IsBuiltIn(name) {
			t.Errorf("IsBuiltIn(%q) = true, want false", name)
		}
	}
}

// ─── ListAll ──────────────────────────────────────────────────────────────────

func TestListAll_BuiltInsOnly(t *testing.T) {
	dir := t.TempDir()
	got := ListAll(dir)
	want := []string{"conventional", "sdd", "tdd"}
	if len(got) != len(want) {
		t.Fatalf("expected %d names, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q (all: %v)", i, got[i], w, got)
		}
	}
}

func TestListAll_Mixed(t *testing.T) {
	dir := t.TempDir()
	methDir := filepath.Join(dir, ".squadai", "methodologies")
	if err := os.MkdirAll(methDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// user-defined methodologies; "tdd" intentionally collides with a built-in
	// to verify dedup.
	for _, name := range []string{"custom-a", "custom-b", "tdd"} {
		if err := os.WriteFile(filepath.Join(methDir, name+".json"), []byte(`{"name":"`+name+`","roles":{}}`), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	got := ListAll(dir)
	want := []string{"conventional", "custom-a", "custom-b", "sdd", "tdd"}
	if len(got) != len(want) {
		t.Fatalf("expected %d names, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q (all: %v)", i, got[i], w, got)
		}
	}

	seen := map[string]bool{}
	for _, name := range got {
		if seen[name] {
			t.Errorf("duplicate name %q in list", name)
		}
		seen[name] = true
	}
}
