package domain

import (
	"strings"
	"testing"
)

// The community knowledge-graph MCP server (config key "memory") is renamed
// and de-emphasized: display name changes, config key stays for compat, and
// the entry sorts last in the catalog.

func TestDefaultMCPCatalog_MemoryRenamedToKnowledgeGraph(t *testing.T) {
	for _, s := range DefaultMCPCatalog() {
		if s.Name != "memory" {
			continue
		}
		if s.DisplayName != "knowledge-graph (community)" {
			t.Errorf("memory DisplayName = %q, want %q", s.DisplayName, "knowledge-graph (community)")
		}
		if s.Display() != "knowledge-graph (community)" {
			t.Errorf("memory Display() = %q, want the renamed label", s.Display())
		}
		if !strings.Contains(s.Description, "Overlaps with SquadAI Project Memory") {
			t.Errorf("memory description should point at Project Memory, got %q", s.Description)
		}
		return
	}
	t.Fatal("memory server missing from catalog (config key must stay for compat)")
}

func TestDefaultMCPCatalog_MemorySortedLast(t *testing.T) {
	catalog := DefaultMCPCatalog()
	if got := catalog[len(catalog)-1].Name; got != "memory" {
		t.Errorf("last catalog entry = %q, want the de-emphasized memory server", got)
	}
}

func TestCuratedMCPServer_Display_FallsBackToName(t *testing.T) {
	s := CuratedMCPServer{Name: "context7"}
	if s.Display() != "context7" {
		t.Errorf("Display() = %q, want the Name fallback", s.Display())
	}
}
