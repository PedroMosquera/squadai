package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestPrintUsage_GroupedSections verifies the help text renders the four
// command groups in order.
func TestPrintUsage_GroupedSections(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()

	prev := -1
	for _, section := range []string{"Daily:", "Setup:", "Advanced:", "Internal:"} {
		idx := strings.Index(out, section)
		if idx < 0 {
			t.Fatalf("printUsage missing section %q, got:\n%s", section, out)
		}
		if idx < prev {
			t.Errorf("section %q rendered out of order", section)
		}
		prev = idx
	}
}

// TestPrintUsage_RenderedFromRegistry verifies every registry command appears
// in the help text — the sections are generated, not hand-maintained.
func TestPrintUsage_RenderedFromRegistry(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()

	for _, c := range buildCommandRegistry().Commands {
		if !strings.Contains(out, c.Name) {
			t.Errorf("printUsage missing registry command %q", c.Name)
		}
	}
}

// TestBuildCommandRegistry_EveryCommandGrouped ensures no registry entry is
// left without a group (they would silently fall into Advanced).
func TestBuildCommandRegistry_EveryCommandGrouped(t *testing.T) {
	valid := map[string]bool{groupDaily: true, groupSetup: true, groupAdvanced: true, groupInternal: true}
	for _, c := range buildCommandRegistry().Commands {
		if !valid[c.Group] {
			t.Errorf("command %q has invalid or missing group %q", c.Name, c.Group)
		}
	}
}

// TestHelpJSON_BackwardCompatible decodes `help --json` output with the OLD
// struct shape (pre-Group) and asserts nothing breaks and all names survive.
func TestHelpJSON_BackwardCompatible(t *testing.T) {
	var buf bytes.Buffer
	if err := printUsageJSON(&buf); err != nil {
		t.Fatalf("printUsageJSON: %v", err)
	}

	// The registry shape consumers depended on before the Group field existed.
	type oldFlag struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type,omitempty"`
		Default     string `json:"default,omitempty"`
	}
	type oldEntry struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		Flags       []oldFlag  `json:"flags,omitempty"`
		Subcommands []oldEntry `json:"subcommands,omitempty"`
	}
	type oldHelp struct {
		Version  string     `json:"version"`
		Commands []oldEntry `json:"commands"`
	}

	var decoded oldHelp
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("help --json no longer decodes with the old struct: %v", err)
	}
	if len(decoded.Commands) == 0 {
		t.Fatal("expected commands in help --json output")
	}

	names := make(map[string]bool, len(decoded.Commands))
	for _, c := range decoded.Commands {
		names[c.Name] = true
	}
	for _, want := range []string{"init", "plan", "apply", "verify", "status", "doctor", "backup", "restore", "remove", "update", "version", "help"} {
		if !names[want] {
			t.Errorf("help --json missing command %q under old-struct decoding", want)
		}
	}
}

// TestHelpJSON_ExposesGroups verifies the additive Group field is emitted.
func TestHelpJSON_ExposesGroups(t *testing.T) {
	var buf bytes.Buffer
	if err := printUsageJSON(&buf); err != nil {
		t.Fatalf("printUsageJSON: %v", err)
	}
	var decoded struct {
		Commands []struct {
			Name  string `json:"name"`
			Group string `json:"group"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range decoded.Commands {
		if c.Group == "" {
			t.Errorf("command %q missing group in help --json", c.Name)
		}
	}
}
