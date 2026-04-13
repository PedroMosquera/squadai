package agents

import (
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

func TestRenderTemplate_ValidData(t *testing.T) {
	tmpl := "Hello {{.Language}} — methodology: {{.Methodology}}"
	data := TemplateData{
		Language:    "Go",
		Methodology: "tdd",
	}
	out, err := renderTemplate("test", tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Hello Go — methodology: tdd" {
		t.Errorf("got %q, want %q", out, "Hello Go — methodology: tdd")
	}
}

func TestRenderTemplate_InvalidTemplate_ReturnsError(t *testing.T) {
	_, err := renderTemplate("bad", "{{.Unclosed", TemplateData{})
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestRenderTemplate_MissingField_RendersEmpty(t *testing.T) {
	// missingkey=zero means missing map key renders as empty string, not error.
	tmpl := "lang={{.Language}} test={{.TestCommand}}"
	data := TemplateData{Language: "Go"} // TestCommand is zero-value ""
	out, err := renderTemplate("test", tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "lang=Go") {
		t.Errorf("expected lang=Go in %q", out)
	}
	if !strings.Contains(out, "test=") {
		t.Errorf("expected test= in %q", out)
	}
}

func TestBuildTemplateData_HasContext7_True(t *testing.T) {
	adapter := opencode.New()
	cfg := &domain.MergedConfig{
		Methodology: domain.MethodologyTDD,
		Meta: domain.ProjectMeta{
			Language:     "Go",
			TestCommand:  "go test ./...",
			BuildCommand: "go build ./...",
			LintCommand:  "golangci-lint run",
		},
		MCP: map[string]domain.MCPServerDef{
			"context7": {Type: "local", Enabled: true},
		},
		Team: domain.DefaultTeam(domain.MethodologyTDD),
	}
	data := buildTemplateData(adapter, cfg, "/home/user", "/proj")
	if !data.HasContext7 {
		t.Error("HasContext7 should be true when context7 is in MCP map")
	}
	if data.Language != "Go" {
		t.Errorf("Language = %q, want Go", data.Language)
	}
	if data.Methodology != "tdd" {
		t.Errorf("Methodology = %q, want tdd", data.Methodology)
	}
	if data.DelegationStrategy != "native" {
		t.Errorf("DelegationStrategy = %q, want native", data.DelegationStrategy)
	}
}

func TestBuildTemplateData_HasContext7_False(t *testing.T) {
	adapter := opencode.New()
	cfg := &domain.MergedConfig{
		Methodology: domain.MethodologySDD,
		Meta:        domain.ProjectMeta{Language: "TypeScript"},
		MCP:         map[string]domain.MCPServerDef{},
	}
	data := buildTemplateData(adapter, cfg, "/home/user", "/proj")
	if data.HasContext7 {
		t.Error("HasContext7 should be false when context7 is not in MCP map")
	}
	if data.Language != "TypeScript" {
		t.Errorf("Language = %q, want TypeScript", data.Language)
	}
}

func TestBuildTemplateData_NilMCP_HasContext7False(t *testing.T) {
	adapter := opencode.New()
	cfg := &domain.MergedConfig{
		Methodology: domain.MethodologyConventional,
		MCP:         nil,
	}
	data := buildTemplateData(adapter, cfg, "/home/user", "/proj")
	if data.HasContext7 {
		t.Error("HasContext7 should be false when MCP map is nil")
	}
}
