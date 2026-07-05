package assets_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/tokenizer"
)

// sizeCeilingModel is the tokenizer used for persona size regression checks.
const sizeCeilingModel = "claude-sonnet-4-6"

// personaBudget is a token ceiling plus the behavioral checklist flags for
// one embedded persona asset. Ceilings ratchet down only — raising one is a
// context-cost regression and needs an explicit decision.
type personaBudget struct {
	path       string
	maxTokens  int
	delegating bool // native/prompt variants delegate; solo executes inline
	native     bool // native variants carry YAML frontmatter
}

var personaBudgets = []personaBudget{
	{path: "teams/conventional/orchestrator-native.md", maxTokens: 1808, delegating: true, native: true},
	{path: "teams/conventional/orchestrator-prompt.md", maxTokens: 1513, delegating: true},
	{path: "teams/conventional/orchestrator-solo.md", maxTokens: 1303},
	{path: "teams/sdd/orchestrator-native.md", maxTokens: 2403, delegating: true, native: true},
	{path: "teams/sdd/orchestrator-prompt.md", maxTokens: 2072, delegating: true},
	{path: "teams/sdd/orchestrator-solo.md", maxTokens: 1794},
	{path: "teams/tdd/orchestrator-native.md", maxTokens: 2158, delegating: true, native: true},
	{path: "teams/tdd/orchestrator-prompt.md", maxTokens: 1910, delegating: true},
	{path: "teams/tdd/orchestrator-solo.md", maxTokens: 1560},
}

// squadaiInitBudget is the ceiling for the /squadai-init driver command.
const squadaiInitBudget = 4796

func TestOrchestratorPersonas_TokenCeilings(t *testing.T) {
	counter := tokenizer.ForModel(sizeCeilingModel)
	for _, b := range personaBudgets {
		t.Run(b.path, func(t *testing.T) {
			content := assets.MustRead(b.path)
			got := counter.Count(content)
			if got > b.maxTokens {
				t.Errorf("%s is %d tokens, ceiling is %d — slim the persona instead of raising the ceiling",
					b.path, got, b.maxTokens)
			}
		})
	}
}

func TestSquadaiInitCommand_TokenCeiling(t *testing.T) {
	counter := tokenizer.ForModel(sizeCeilingModel)
	content := assets.MustRead("commands/squadai-init.md")
	if got := counter.Count(content); got > squadaiInitBudget {
		t.Errorf("commands/squadai-init.md is %d tokens, ceiling is %d", got, squadaiInitBudget)
	}
}

// orchestratorChecklist asserts the behaviors that must survive any persona
// rewrite: proactive delegation before context exhaustion, per-role output
// summarization budgets, never pasting full sub-agent output, a compaction
// recovery pointer, and model/tier routing guidance.
func TestOrchestratorPersonas_BehaviorChecklist(t *testing.T) {
	type check struct {
		name    string
		re      *regexp.Regexp
		applies func(personaBudget) bool
	}
	all := func(personaBudget) bool { return true }
	delegating := func(b personaBudget) bool { return b.delegating }
	checks := []check{
		{
			name:    "delegate proactively at ~60% context",
			re:      regexp.MustCompile(`60% context`),
			applies: delegating,
		},
		{
			name:    "solo checkpoint before context exhaustion",
			re:      regexp.MustCompile(`80% context`),
			applies: func(b personaBudget) bool { return !b.delegating },
		},
		{
			name:    "per-role output summarization budgets",
			re:      regexp.MustCompile(`<\s*\d+ lines|\d+[-–]\d+ lines?\b|\d+[-–]\d+ line summary`),
			applies: all,
		},
		{
			name:    "never paste full sub-agent output",
			re:      regexp.MustCompile(`(?i)not the full output|never (paste|store) full`),
			applies: delegating,
		},
		{
			name:    "compaction recovery pointer",
			re:      regexp.MustCompile(`(?i)compact`),
			applies: all,
		},
		{
			name:    "model/tier routing guidance",
			re:      regexp.MustCompile(`\{\{[- ]*if \.ModelHint[ -]*\}\}`),
			applies: all,
		},
		{
			name:    "skill resolution from SkillsDir",
			re:      regexp.MustCompile(`\{\{\.SkillsDir\}\}`),
			applies: all,
		},
	}

	for _, b := range personaBudgets {
		content := assets.MustRead(b.path)
		for _, c := range checks {
			if !c.applies(b) {
				continue
			}
			if !c.re.MatchString(content) {
				t.Errorf("%s: missing behavior %q (pattern %s)", b.path, c.name, c.re)
			}
		}
	}
}

// Template variables and structure that installers depend on must remain in
// every orchestrator persona.
func TestOrchestratorPersonas_TemplateContract(t *testing.T) {
	requiredVars := []string{
		"{{.Methodology}}",
		"{{.SkillsDir}}",
		"{{ .Language }}",
		"{{.TestCommand}}",
		"{{ .ModelHint }}",
		".HasContext7",
	}
	for _, b := range personaBudgets {
		content := assets.MustRead(b.path)
		for _, v := range requiredVars {
			if !strings.Contains(content, v) {
				t.Errorf("%s: missing template construct %q", b.path, v)
			}
		}
		if b.native && !strings.HasPrefix(content, "---\n") {
			t.Errorf("%s: native orchestrator must keep YAML frontmatter", b.path)
		}
		if b.delegating && b.native && !strings.Contains(content, "{{.AgentsDir}}") {
			t.Errorf("%s: native orchestrator must reference {{.AgentsDir}}", b.path)
		}
	}
}

// The /squadai-init driver must keep its safety contract regardless of how
// much procedure detail moves into the playbook skill.
func TestSquadaiInitCommand_BehaviorChecklist(t *testing.T) {
	content := assets.MustRead("commands/squadai-init.md")
	for _, want := range []string{
		"squadai:refinement", // marker-block-only writes
		"Proceed?",           // token-cost disclosure + confirmation
		".squadai/project.json",
		".squad-refined",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("commands/squadai-init.md: missing required construct %q", want)
		}
	}
	lower := strings.ToLower(content)
	for _, phrase := range []string{"diff", "never"} {
		if !strings.Contains(lower, phrase) {
			t.Errorf("commands/squadai-init.md: missing %q language", phrase)
		}
	}
}
