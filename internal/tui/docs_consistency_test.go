package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// These tests guard user-facing docs against drift from the adapter and preset
// sets declared in internal/domain/types.go. They live in package tui because
// agentDisplayName and allCanonicalAgents are the canonical display mapping.

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above test directory")
		}
		dir = parent
	}
}

// domainConstants parses internal/domain/types.go and returns the string
// values of every constant declared with the named type, so a new adapter or
// preset is picked up without editing this test.
func domainConstants(t *testing.T, root, typeName string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(root, "internal", "domain", "types.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		ident, ok := vs.Type.(*ast.Ident)
		if !ok || ident.Name != typeName {
			return true
		}
		for _, v := range vs.Values {
			lit, ok := v.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return true
	})
	if len(out) == 0 {
		t.Fatalf("no %s constants found in internal/domain/types.go", typeName)
	}
	sort.Strings(out)
	return out
}

func readDoc(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDocsConsistency_CanonicalAgentsMatchDomain(t *testing.T) {
	want := domainConstants(t, repoRoot(t), "AgentID")
	var got []string
	for _, id := range allCanonicalAgents {
		got = append(got, string(id))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("allCanonicalAgents = %v, domain AgentID constants = %v", got, want)
	}
}

func TestDocsConsistency_READMEAdapterTableListsEveryAdapter(t *testing.T) {
	root := repoRoot(t)
	readme := readDoc(t, root, "README.md")
	for _, id := range domainConstants(t, root, "AgentID") {
		name := agentDisplayName(domain.AgentID(id))
		row := regexp.MustCompile(`(?m)^\| ` + regexp.QuoteMeta(name) + ` \|`)
		if !row.MatchString(readme) {
			t.Errorf("README.md adapter table has no row for %q (id %q)", name, id)
		}
	}
}

func TestDocsConsistency_TroubleshootingDetectionTableListsEveryAdapter(t *testing.T) {
	root := repoRoot(t)
	doc := readDoc(t, root, "docs/troubleshooting.md")
	for _, id := range domainConstants(t, root, "AgentID") {
		if !strings.Contains(doc, "| `"+id+"` |") {
			t.Errorf("docs/troubleshooting.md detection table has no row for `%s`", id)
		}
	}
}

func TestDocsConsistency_OpenspecListsEveryAdapterID(t *testing.T) {
	root := repoRoot(t)
	doc := readDoc(t, root, "openspec/config.yaml")
	m := regexp.MustCompile(`Adapters: \d+ supported \(([^)]*)\)`).FindStringSubmatch(doc)
	if m == nil {
		t.Fatal(`openspec/config.yaml has no "Adapters: N supported (...)" line`)
	}
	listed := map[string]bool{}
	for _, s := range strings.Split(m[1], ",") {
		listed[strings.TrimSpace(s)] = true
	}
	for _, id := range domainConstants(t, root, "AgentID") {
		if !listed[id] {
			t.Errorf("openspec/config.yaml adapter list %q is missing %q", m[1], id)
		}
	}
}

var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

func TestDocsConsistency_StatedAdapterCountsMatchDomain(t *testing.T) {
	root := repoRoot(t)
	want := len(domainConstants(t, root, "AgentID"))
	countRe := regexp.MustCompile(`(?i)\b(\d+|one|two|three|four|five|six|seven|eight|nine|ten) supported(?: agents\b| adapters\b| \()`)

	files := []string{"README.md", "openspec/config.yaml"}
	topLevelDocs, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range topLevelDocs {
		rel, _ := filepath.Rel(root, p)
		files = append(files, rel)
	}

	found := 0
	for _, rel := range files {
		for _, m := range countRe.FindAllStringSubmatch(readDoc(t, root, rel), -1) {
			found++
			n, err := strconv.Atoi(m[1])
			if err != nil {
				n = numberWords[strings.ToLower(m[1])]
			}
			if n != want {
				t.Errorf("%s states %q, but there are %d adapters", rel, m[0], want)
			}
		}
	}
	// openspec/config.yaml always carries a count; finding none means the
	// regex broke, not that the docs are clean.
	if found == 0 {
		t.Fatal("no adapter count claims found; the pattern no longer matches the docs")
	}
}

func TestDocsConsistency_PresetListsIncludeEveryPreset(t *testing.T) {
	root := repoRoot(t)
	presets := domainConstants(t, root, "SetupPreset")
	for _, rel := range []string{"README.md", "docs/commands.md"} {
		doc := readDoc(t, root, rel)
		for _, p := range presets {
			if !regexp.MustCompile("(?m)--preset=" + regexp.QuoteMeta(p) + `\b|^- ` + "`" + regexp.QuoteMeta(p) + "`").MatchString(doc) {
				t.Errorf("%s does not document preset %q", rel, p)
			}
		}
	}
}
