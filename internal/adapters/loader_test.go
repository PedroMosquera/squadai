package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// writeOverride writes a JSON override file for agentID under .squadai/adapters.
func writeOverride(t *testing.T, projectDir string, agentID domain.AgentID, spec OverrideSpec) {
	t.Helper()
	dir := filepath.Join(projectDir, ".squadai", "adapters")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, string(agentID)+".json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

// ─── LoadOverride ────────────────────────────────────────────────────────────

func TestLoadOverride_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	spec, err := LoadOverride(dir, domain.AgentOpenCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec != nil {
		t.Errorf("expected nil spec, got %+v", spec)
	}
}

func TestLoadOverride_ValidFile(t *testing.T) {
	dir := t.TempDir()
	want := OverrideSpec{
		ConfigDir:    "~/.pi/agent",
		AgentsSubdir: "agents",
		Delegation:   "native",
		Supports:     []string{"workflows"},
	}
	writeOverride(t, dir, domain.AgentOpenCode, want)

	spec, err := LoadOverride(dir, domain.AgentOpenCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if spec.ConfigDir != want.ConfigDir {
		t.Errorf("ConfigDir = %q, want %q", spec.ConfigDir, want.ConfigDir)
	}
	if spec.AgentsSubdir != want.AgentsSubdir {
		t.Errorf("AgentsSubdir = %q, want %q", spec.AgentsSubdir, want.AgentsSubdir)
	}
	if spec.Delegation != want.Delegation {
		t.Errorf("Delegation = %q, want %q", spec.Delegation, want.Delegation)
	}
	if len(spec.Supports) != 1 || spec.Supports[0] != "workflows" {
		t.Errorf("Supports = %v, want [workflows]", spec.Supports)
	}
}

func TestLoadOverride_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	adapterDir := filepath.Join(dir, ".squadai", "adapters")
	if err := os.MkdirAll(adapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adapterDir, "opencode.json"), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadOverride(dir, domain.AgentOpenCode); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ─── OverrideAdapter delegation ──────────────────────────────────────────────

func TestOverrideAdapter_DelegatesToBase(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{}}
	home := "/Users/test"

	if o.ID() != base.ID() {
		t.Errorf("ID() = %q, want %q", o.ID(), base.ID())
	}
	if o.Lane() != base.Lane() {
		t.Errorf("Lane() mismatch")
	}
	if o.GlobalConfigDir(home) != base.GlobalConfigDir(home) {
		t.Errorf("GlobalConfigDir mismatch")
	}
	if o.SystemPromptFile(home) != base.SystemPromptFile(home) {
		t.Errorf("SystemPromptFile mismatch")
	}
	if o.SkillsDir(home) != base.SkillsDir(home) {
		t.Errorf("SkillsDir mismatch")
	}
	if o.AgentsDir(home) != base.AgentsDir(home) {
		t.Errorf("AgentsDir mismatch")
	}
	if o.SubAgentsDir(home) != base.SubAgentsDir(home) {
		t.Errorf("SubAgentsDir mismatch")
	}
	if o.SettingsPath(home) != base.SettingsPath(home) {
		t.Errorf("SettingsPath mismatch")
	}
	if o.DelegationStrategy() != base.DelegationStrategy() {
		t.Errorf("DelegationStrategy mismatch")
	}
	if o.SupportsComponent(domain.ComponentMemory) != base.SupportsComponent(domain.ComponentMemory) {
		t.Errorf("SupportsComponent mismatch")
	}
	if o.SupportsSubAgents() != base.SupportsSubAgents() {
		t.Errorf("SupportsSubAgents mismatch")
	}
	if o.MCPRootKey() != base.MCPRootKey() {
		t.Errorf("MCPRootKey mismatch")
	}
	if o.MCPEnvKey() != base.MCPEnvKey() {
		t.Errorf("MCPEnvKey mismatch")
	}
	if o.RulesFileSizeCap() != base.RulesFileSizeCap() {
		t.Errorf("RulesFileSizeCap mismatch")
	}
}

// ─── OverrideAdapter path overrides ──────────────────────────────────────────

func TestOverrideAdapter_ConfigDirOverride(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{ConfigDir: "/custom/config"}}
	home := "/Users/test"
	wantConfig := "/custom/config"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", o.GlobalConfigDir(home), wantConfig},
		{"SystemPromptFile", o.SystemPromptFile(home), filepath.Join(wantConfig, "AGENTS.md")},
		{"SkillsDir", o.SkillsDir(home), filepath.Join(wantConfig, "skills")},
		{"AgentsDir", o.AgentsDir(home), filepath.Join(wantConfig, "agents")},
		{"SubAgentsDir", o.SubAgentsDir(home), filepath.Join(wantConfig, "agents")},
		{"SettingsPath", o.SettingsPath(home), filepath.Join(wantConfig, "settings.json")},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestOverrideAdapter_SkillsSubdirOverride(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{ConfigDir: "/c", SkillsSubdir: "myskills"}}
	home := "/Users/test"
	want := filepath.Join("/c", "myskills")
	if got := o.SkillsDir(home); got != want {
		t.Errorf("SkillsDir = %q, want %q", got, want)
	}
}

func TestOverrideAdapter_AgentsSubdirOverride(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{ConfigDir: "/c", AgentsSubdir: "myagents"}}
	home := "/Users/test"
	want := filepath.Join("/c", "myagents")
	if got := o.AgentsDir(home); got != want {
		t.Errorf("AgentsDir = %q, want %q", got, want)
	}
	if got := o.SubAgentsDir(home); got != want {
		t.Errorf("SubAgentsDir = %q, want %q", got, want)
	}
}

func TestOverrideAdapter_SettingsPathOverride(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{ConfigDir: "/c", SettingsPath: "mysettings.json"}}
	home := "/Users/test"
	want := filepath.Join("/c", "mysettings.json")
	if got := o.SettingsPath(home); got != want {
		t.Errorf("SettingsPath = %q, want %q", got, want)
	}
}

// ─── OverrideAdapter behavior overrides ──────────────────────────────────────

func TestOverrideAdapter_DelegationOverride(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{Delegation: "solo"}}
	if got := o.DelegationStrategy(); got != domain.DelegationSoloAgent {
		t.Errorf("DelegationStrategy = %q, want %q", got, domain.DelegationSoloAgent)
	}
}

func TestOverrideAdapter_SupportsComponent_Add(t *testing.T) {
	base := opencode.New()
	if base.SupportsComponent(domain.ComponentWorkflows) {
		t.Fatal("test precondition: base should not support workflows")
	}
	o := &OverrideAdapter{base: base, spec: OverrideSpec{Supports: []string{"workflows"}}}
	if !o.SupportsComponent(domain.ComponentWorkflows) {
		t.Error("expected workflows to be supported after override")
	}
}

func TestOverrideAdapter_SupportsComponent_Remove(t *testing.T) {
	base := opencode.New()
	if !base.SupportsComponent(domain.ComponentMemory) {
		t.Fatal("test precondition: base should support memory")
	}
	o := &OverrideAdapter{base: base, spec: OverrideSpec{Unsupported: []string{"memory"}}}
	if o.SupportsComponent(domain.ComponentMemory) {
		t.Error("expected memory to be unsupported after override")
	}
}

// ─── Tilde expansion ─────────────────────────────────────────────────────────

func TestOverrideAdapter_TildeExpansion(t *testing.T) {
	base := opencode.New()
	o := &OverrideAdapter{base: base, spec: OverrideSpec{ConfigDir: "~/agentdir"}}
	home := "/Users/test"
	want := filepath.Join(home, "agentdir")
	if got := o.GlobalConfigDir(home); got != want {
		t.Errorf("GlobalConfigDir = %q, want %q", got, want)
	}
}

// ─── ApplyOverride / ApplyOverrides ──────────────────────────────────────────

func TestApplyOverride_NoFile(t *testing.T) {
	dir := t.TempDir()
	base := opencode.New()
	got, err := ApplyOverride(base, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != base {
		t.Error("expected base adapter returned unchanged when no override file")
	}
}

func TestApplyOverride_WithFile(t *testing.T) {
	dir := t.TempDir()
	writeOverride(t, dir, domain.AgentOpenCode, OverrideSpec{ConfigDir: "/custom"})

	base := opencode.New()
	got, err := ApplyOverride(base, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oa, ok := got.(*OverrideAdapter)
	if !ok {
		t.Fatalf("expected *OverrideAdapter, got %T", got)
	}
	if oa.spec.ConfigDir != "/custom" {
		t.Errorf("spec.ConfigDir = %q, want %q", oa.spec.ConfigDir, "/custom")
	}
	if oa.ID() != domain.AgentOpenCode {
		t.Errorf("ID() = %q, want %q", oa.ID(), domain.AgentOpenCode)
	}
}

func TestApplyOverrides_MixedSlice(t *testing.T) {
	dir := t.TempDir()
	writeOverride(t, dir, domain.AgentOpenCode, OverrideSpec{ConfigDir: "/custom"})

	input := []domain.Adapter{opencode.New(), claude.New()}
	got, err := ApplyOverrides(input, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 adapters, got %d", len(got))
	}
	if _, ok := got[0].(*OverrideAdapter); !ok {
		t.Errorf("expected adapter 0 to be *OverrideAdapter, got %T", got[0])
	}
	if _, ok := got[1].(*OverrideAdapter); ok {
		t.Errorf("expected adapter 1 to be unchanged (no override file), got *OverrideAdapter")
	}
}

// ─── EffectivePaths ──────────────────────────────────────────────────────────

func TestEffectivePaths(t *testing.T) {
	home := "/Users/test"
	paths := EffectivePaths(opencode.New(), home, "/project")

	wantKeys := []string{
		"config_dir",
		"system_prompt",
		"skills_dir",
		"settings_path",
		"agents_dir",
		"sub_agents_dir",
		"delegation",
	}
	for _, k := range wantKeys {
		if _, ok := paths[k]; !ok {
			t.Errorf("missing key %q in EffectivePaths", k)
		}
	}

	wantConfig := filepath.Join(home, ".config", "opencode")
	if paths["config_dir"] != wantConfig {
		t.Errorf("config_dir = %q, want %q", paths["config_dir"], wantConfig)
	}
	if paths["delegation"] != string(domain.DelegationNativeAgents) {
		t.Errorf("delegation = %q, want %q", paths["delegation"], domain.DelegationNativeAgents)
	}
}

// ─── Interface compliance ────────────────────────────────────────────────────

func TestOverrideAdapter_ImplementsInterface(t *testing.T) {
	var _ domain.Adapter = (*OverrideAdapter)(nil)
}
