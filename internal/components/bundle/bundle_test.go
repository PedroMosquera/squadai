package bundle

import (
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func componentCfg(enabled bool) domain.ComponentConfig {
	return domain.ComponentConfig{Enabled: enabled}
}

// allComponentsEnabled returns a minimal MergedConfig with every component
// enabled, so Build populates every field in the Set.
func allComponentsEnabled(projectDir string) *domain.MergedConfig {
	return &domain.MergedConfig{
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):      componentCfg(true),
			string(domain.ComponentRules):       componentCfg(true),
			string(domain.ComponentSettings):    componentCfg(true),
			string(domain.ComponentPermissions): componentCfg(true),
			string(domain.ComponentMCP):         componentCfg(true),
			string(domain.ComponentAgents):      componentCfg(true),
			string(domain.ComponentSkills):      componentCfg(true),
			string(domain.ComponentCommands):    componentCfg(true),
			string(domain.ComponentPlugins):     componentCfg(true),
			string(domain.ComponentWorkflows):   componentCfg(true),
		},
		Adapters: map[string]domain.AdapterConfig{},
		Rules:    domain.RulesConfig{},
	}
}

func TestBuild_AllEnabled_PopulatesEveryInstaller(t *testing.T) {
	cfg := allComponentsEnabled(t.TempDir())
	set, err := Build(cfg, t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	cases := map[string]bool{
		"Memory":      set.Memory != nil,
		"Rules":       set.Rules != nil,
		"Settings":    set.Settings != nil,
		"Permissions": set.Permissions != nil,
		"MCP":         set.MCP != nil,
		"Agents":      set.Agents != nil,
		"Skills":      set.Skills != nil,
		"Commands":    set.Commands != nil,
		"Plugins":     set.Plugins != nil,
		"Workflows":   set.Workflows != nil,
		"Copilot":     set.Copilot != nil,
	}
	for name, ok := range cases {
		if !ok {
			t.Errorf("Set.%s is nil, want non-nil", name)
		}
	}
}

func TestBuild_AllDisabled_OnlyAlwaysOn(t *testing.T) {
	cfg := &domain.MergedConfig{
		Components: map[string]domain.ComponentConfig{},
		Adapters:   map[string]domain.AdapterConfig{},
	}
	set, err := Build(cfg, t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Memory, Permissions, and Copilot are always present — the planner and
	// verifier treat them as default-on fixtures.
	if set.Memory == nil {
		t.Error("Memory should always be built")
	}
	if set.Permissions == nil {
		t.Error("Permissions should always be built")
	}
	if set.Copilot == nil {
		t.Error("Copilot should always be built")
	}
	// Everything else must be nil when not enabled.
	cases := map[string]bool{
		"Rules":     set.Rules == nil,
		"Settings":  set.Settings == nil,
		"MCP":       set.MCP == nil,
		"Agents":    set.Agents == nil,
		"Skills":    set.Skills == nil,
		"Commands":  set.Commands == nil,
		"Plugins":   set.Plugins == nil,
		"Workflows": set.Workflows == nil,
	}
	for name, isNil := range cases {
		if !isNil {
			t.Errorf("Set.%s should be nil when component is disabled", name)
		}
	}
}

func TestBuild_PartialEnabled_BuildsOnlyEnabled(t *testing.T) {
	cfg := &domain.MergedConfig{
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMCP):      componentCfg(true),
			string(domain.ComponentSettings): componentCfg(false),
			string(domain.ComponentAgents):   componentCfg(true),
		},
		Adapters: map[string]domain.AdapterConfig{},
	}
	set, err := Build(cfg, t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if set.MCP == nil {
		t.Error("MCP should be built when enabled")
	}
	if set.Agents == nil {
		t.Error("Agents should be built when enabled")
	}
	if set.Settings != nil {
		t.Error("Settings should be nil when explicitly disabled")
	}
	if set.Rules != nil {
		t.Error("Rules should be nil when absent from config")
	}
}

func TestBuild_IsPure_FreshInstanceEveryCall(t *testing.T) {
	cfg := allComponentsEnabled(t.TempDir())
	projectDir := t.TempDir()

	a, err := Build(cfg, projectDir, Options{})
	if err != nil {
		t.Fatalf("Build() #1 error: %v", err)
	}
	b, err := Build(cfg, projectDir, Options{})
	if err != nil {
		t.Fatalf("Build() #2 error: %v", err)
	}
	// Two independent calls must return distinct installer pointers so
	// that state attached to one (e.g., SetApplyPolicy on MCP) does not
	// leak into the other. Zero-sized structs (Memory) share an address
	// by Go spec — use a struct with fields as the canary.
	if a.MCP == b.MCP {
		t.Error("Build should return a fresh MCP installer per call")
	}
	if a.Plugins == b.Plugins {
		t.Error("Build should return a fresh Plugins installer per call")
	}
}
