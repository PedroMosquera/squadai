# Token Efficiency Wave 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Measure the per-session token cost of a squadai install, cut memory-protocol duplication across subagent files in half, and ship a `squadai token-budget` command so users can see their token overhead at a glance.

**Architecture:** Three independent layers: (1) a pure `internal/tokenprofile` utility package that estimates tokens from bytes and scans files on disk; (2) a targeted change to `internal/components/agents/installer.go` that injects a short stub into subagent roles instead of the full memory-protocol block (saving ~200 tokens × N subagents per session); (3) a new `squadai token-budget` CLI command in `internal/cli/` that runs the planner in dry-run mode, reads each planned file from disk, and reports a per-component token breakdown.

**Tech Stack:** Go 1.24, `internal/planner`, `internal/domain`, existing `marker`, `fileutil`, `assets` packages. No new external dependencies.

---

## File Map

| Status | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/tokenprofile/profiler.go` | Token estimation (`ApproxTokens`) + file scanning (`ScanPaths`) + `Report` type |
| Create | `internal/tokenprofile/profiler_test.go` | Unit tests for profiler |
| Modify | `internal/components/agents/installer.go` | Inject stub (not full block) for subagent roles |
| Modify | `internal/components/agents/installer_test.go` | Two new tests asserting orchestrator vs subagent memory content |
| Create | `internal/cli/token_budget.go` | `RunTokenBudget` command: loads config, runs planner, calls tokenprofile |
| Create | `internal/cli/token_budget_test.go` | Tests for RunTokenBudget |
| Modify | `internal/app/app.go` | Wire `token-budget` subcommand + add to command registry + printUsage |

---

## Task 1: tokenprofile package

**Files:**
- Create: `internal/tokenprofile/profiler.go`
- Create: `internal/tokenprofile/profiler_test.go`

- [ ] **Step 1.1: Write the failing tests**

Create `internal/tokenprofile/profiler_test.go`:

```go
package tokenprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApproxTokens_Empty(t *testing.T) {
	if got := ApproxTokens(nil); got != 0 {
		t.Errorf("ApproxTokens(nil) = %d, want 0", got)
	}
	if got := ApproxTokens([]byte{}); got != 0 {
		t.Errorf("ApproxTokens([]) = %d, want 0", got)
	}
}

func TestApproxTokens_Rounding(t *testing.T) {
	cases := []struct {
		bytes int
		want  int
	}{
		{1, 1},
		{4, 1},
		{5, 2},
		{8, 2},
		{9, 3},
		{400, 100},
	}
	for _, c := range cases {
		content := make([]byte, c.bytes)
		got := ApproxTokens(content)
		if got != c.want {
			t.Errorf("ApproxTokens(%d bytes) = %d, want %d", c.bytes, got, c.want)
		}
	}
}

func TestScanPaths_CountsTokens(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.md")
	p2 := filepath.Join(dir, "b.md")
	// Write 400 bytes to a.md (→ 100 tokens), 800 bytes to b.md (→ 200 tokens)
	if err := os.WriteFile(p1, make([]byte, 400), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, make([]byte, 800), 0644); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		p1: "agents",
		p2: "agents",
	}
	report, err := ScanPaths(paths)
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}

	if report.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", report.TotalTokens)
	}
	if report.TotalBytes != 1200 {
		t.Errorf("TotalBytes = %d, want 1200", report.TotalBytes)
	}
	cat := report.ByCategory["agents"]
	if cat.Tokens != 300 {
		t.Errorf("agents tokens = %d, want 300", cat.Tokens)
	}
	if cat.Files != 2 {
		t.Errorf("agents files = %d, want 2", cat.Files)
	}
}

func TestScanPaths_MissingFileIsSkipped(t *testing.T) {
	paths := map[string]string{
		"/nonexistent/path/file.md": "agents",
	}
	report, err := ScanPaths(paths)
	if err != nil {
		t.Fatalf("ScanPaths with missing file should not error: %v", err)
	}
	if report.TotalTokens != 0 {
		t.Errorf("expected 0 tokens for missing file, got %d", report.TotalTokens)
	}
	if report.Missing != 1 {
		t.Errorf("expected Missing = 1, got %d", report.Missing)
	}
}

func TestScanPaths_MultipleCategories(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "orchestrator.md")
	skill := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(agent, make([]byte, 800), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, make([]byte, 400), 0644); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		agent: "agents",
		skill: "skills",
	}
	report, err := ScanPaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if report.ByCategory["agents"].Files != 1 {
		t.Errorf("agents files = %d, want 1", report.ByCategory["agents"].Files)
	}
	if report.ByCategory["skills"].Files != 1 {
		t.Errorf("skills files = %d, want 1", report.ByCategory["skills"].Files)
	}
	if report.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", report.TotalTokens)
	}
}
```

- [ ] **Step 1.2: Run tests to confirm they fail**

```
go test ./internal/tokenprofile/... -v
```

Expected: compilation error (package does not exist yet).

- [ ] **Step 1.3: Implement tokenprofile/profiler.go**

Create `internal/tokenprofile/profiler.go`:

```go
package tokenprofile

import "os"

// ApproxTokens estimates the number of tokens in content using a 4 chars/token
// heuristic (ceiling division). Returns 0 for empty content.
func ApproxTokens(content []byte) int {
	n := len(content)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// Entry holds per-file token data.
type Entry struct {
	Path    string
	Category string
	Bytes   int
	Tokens  int
}

// CategorySummary aggregates token data for a category.
type CategorySummary struct {
	Files  int
	Bytes  int
	Tokens int
}

// Report is the full output of a ScanPaths call.
type Report struct {
	Entries    []Entry
	ByCategory map[string]CategorySummary
	TotalBytes  int
	TotalTokens int
	Missing    int // count of paths that did not exist on disk
}

// ScanPaths reads each file in paths (map[filepath]category) from disk,
// estimates tokens, and returns a Report.
// Missing files increment Report.Missing but are not an error.
func ScanPaths(paths map[string]string) (*Report, error) {
	r := &Report{
		ByCategory: make(map[string]CategorySummary),
	}
	for path, category := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				r.Missing++
				continue
			}
			return nil, err
		}
		tokens := ApproxTokens(data)
		r.Entries = append(r.Entries, Entry{
			Path:     path,
			Category: category,
			Bytes:    len(data),
			Tokens:   tokens,
		})
		sum := r.ByCategory[category]
		sum.Files++
		sum.Bytes += len(data)
		sum.Tokens += tokens
		r.ByCategory[category] = sum
		r.TotalBytes += len(data)
		r.TotalTokens += tokens
	}
	return r, nil
}
```

- [ ] **Step 1.4: Run tests and confirm they pass**

```
go test ./internal/tokenprofile/... -v
```

Expected: all 4 tests pass.

- [ ] **Step 1.5: Commit**

```
git add internal/tokenprofile/profiler.go internal/tokenprofile/profiler_test.go
git commit -m "feat(tokenprofile): token estimation + ScanPaths utility"
```

---

## Task 2: Memory-protocol dedup for subagents

**Files:**
- Modify: `internal/components/agents/installer.go`
- Modify: `internal/components/agents/installer_test.go`

**Context:** `applyNativeAgent` in installer.go currently calls `injectMemoryProtocol(translated, action.Agent)` for every role. The full protocol (`protocol-claude.md`) is ~914 bytes (~228 tokens). For a TDD team (6 roles), this wastes ~228 × 5 = ~1,140 tokens per session for the 5 subagents. The fix: orchestrator keeps the full block; subagents get a short 3-line stub (~25 tokens).

- [ ] **Step 2.1: Write failing tests**

Append to `internal/components/agents/installer_test.go`:

```go
// ─── Memory-protocol dedup ──────────────────────────────────────────────────

func TestApplyTeamNative_OrchestratorHasFullMemoryProtocol(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(project, ".opencode", "agents", "orchestrator.md"))
	if err != nil {
		t.Fatalf("read orchestrator: %v", err)
	}
	content := string(data)
	// Full protocol includes @librarian and /memory-promote references.
	if !strings.Contains(content, "@librarian") {
		t.Error("orchestrator should contain full memory protocol with @librarian reference")
	}
	if !strings.Contains(content, "/memory-promote") {
		t.Error("orchestrator should contain full memory protocol with /memory-promote reference")
	}
}

func TestApplyTeamNative_SubagentHasMemoryStub_NotFullProtocol(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(project, ".opencode", "agents", "implementer.md"))
	if err != nil {
		t.Fatalf("read implementer: %v", err)
	}
	content := string(data)
	// Subagent gets stub: has /memory-search but NOT @librarian or /memory-promote.
	if !strings.Contains(content, "/memory-search") {
		t.Error("subagent stub should contain /memory-search command")
	}
	if strings.Contains(content, "@librarian") {
		t.Error("subagent should NOT contain full protocol @librarian reference")
	}
}
```

- [ ] **Step 2.2: Run tests to confirm they fail**

```
go test ./internal/components/agents/... -run "TestApplyTeamNative_Orchestrator|TestApplyTeamNative_Subagent" -v
```

Expected: `TestApplyTeamNative_SubagentHasMemoryStub_NotFullProtocol` fails because subagents currently get the full protocol with `@librarian`.

- [ ] **Step 2.3: Add the stub constant and helper in installer.go**

In `internal/components/agents/installer.go`, add after the `const memorySectionID` line (around line 20):

```go
// memoryProtocolSubagentStub is the short memory-protocol block injected into
// subagent files. Subagents only need the two daily-use commands; the full
// protocol (librarian, promote, etc.) lives in the orchestrator.
const memoryProtocolSubagentStub = `## Project Memory Protocol

Before starting work, search memory: ` + "`/memory-search <query>`" + `.
After significant work, capture decisions: ` + "`/memory-add <note>`" + `.`
```

Then add a new function after `injectMemoryProtocol`:

```go
// injectMemoryProtocolStub injects the short subagent memory-protocol stub.
// Returns content unchanged for adapters that have no memory protocol asset.
func injectMemoryProtocolStub(content string, agentID domain.AgentID) string {
	if memoryProtocolAsset(agentID) == "" {
		return content
	}
	return marker.InjectSection(content, memorySectionID, memoryProtocolSubagentStub)
}
```

- [ ] **Step 2.4: Update applyNativeAgent to use the stub for subagents**

In `applyNativeAgent` (around line 497), replace:

```go
	// Inject the per-adapter memory-protocol block (idempotent).
	translated = injectMemoryProtocol(translated, action.Agent)
```

with:

```go
	// Inject memory protocol: full block for orchestrator, short stub for subagents.
	if roleName == "orchestrator" {
		translated = injectMemoryProtocol(translated, action.Agent)
	} else {
		translated = injectMemoryProtocolStub(translated, action.Agent)
	}
```

- [ ] **Step 2.5: Run all installer tests**

```
go test ./internal/components/agents/... -v
```

Expected: all tests pass, including both new dedup tests.

- [ ] **Step 2.6: Commit**

```
git add internal/components/agents/installer.go internal/components/agents/installer_test.go
git commit -m "feat(agents): inject memory-protocol stub for subagents, full block for orchestrator only"
```

---

## Task 3: `squadai token-budget` CLI command

**Files:**
- Create: `internal/cli/token_budget.go`
- Create: `internal/cli/token_budget_test.go`

- [ ] **Step 3.1: Write the failing tests**

Create `internal/cli/token_budget_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// writeTDDProjectForBudget writes a minimal .squadai/project.json to dir.
func writeTDDProjectForBudget(t *testing.T, dir string) {
	t.Helper()
	proj := domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(dir), &proj); err != nil {
		t.Fatalf("write project.json: %v", err)
	}
}

func TestRunTokenBudget_NoConfig_NoError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	// With no config, the planner returns 0 actions and we print a no-install note.
	err := RunTokenBudget([]string{}, &buf)
	if err != nil {
		t.Errorf("unexpected error with no config: %v", err)
	}
}

func TestRunTokenBudget_Help(t *testing.T) {
	var buf bytes.Buffer
	err := RunTokenBudget([]string{"--help"}, &buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "token-budget") {
		t.Error("help output should contain 'token-budget'")
	}
}

func TestRunTokenBudget_JSON_EmptyInstall(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	err := RunTokenBudget([]string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if _, ok := out["total_tokens"]; !ok {
		t.Error("JSON output should contain 'total_tokens'")
	}
}

func TestRunTokenBudget_HumanOutput_ContainsExpectedFields(t *testing.T) {
	dir := t.TempDir()
	writeTDDProjectForBudget(t, dir)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	err := RunTokenBudget([]string{}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Should contain a header and TOTAL line.
	if !strings.Contains(out, "Token Budget") {
		t.Error("output should contain 'Token Budget' header")
	}
	if !strings.Contains(out, "TOTAL") {
		t.Error("output should contain 'TOTAL' row")
	}
}
```

- [ ] **Step 3.2: Run tests to confirm they fail**

```
go test ./internal/cli/... -run "TestRunTokenBudget" -v
```

Expected: compilation error (RunTokenBudget not defined).

- [ ] **Step 3.3: Implement token_budget.go**

Create `internal/cli/token_budget.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
)

// RunTokenBudget reports the approximate token cost of the current squadai install.
func RunTokenBudget(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai token-budget [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Estimate the per-session token cost of the current squadai install.")
			fmt.Fprintln(stdout, "Reads installed files on disk and groups by component.")
			fmt.Fprintln(stdout, "Token count uses a 4 chars/token approximation.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json   Output as JSON")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		// No config is not an error for token-budget; just report 0.
		merged = nil
	}

	adapters := DetectAdapters(homeDir)

	pathToCategory := make(map[string]string)
	if merged != nil {
		p := planner.New()
		actions, planErr := p.Plan(merged, adapters, homeDir, projectDir)
		if planErr == nil {
			for _, a := range actions {
				if a.TargetPath == "" {
					continue
				}
				pathToCategory[a.TargetPath] = string(a.Component)
			}
		}
	}

	report, err := tokenprofile.ScanPaths(pathToCategory)
	if err != nil {
		return fmt.Errorf("scan paths: %w", err)
	}

	if jsonOut {
		return printTokenBudgetJSON(stdout, report)
	}
	printTokenBudgetHuman(stdout, report)
	return nil
}

type tokenBudgetJSON struct {
	TotalBytes   int                           `json:"total_bytes"`
	TotalTokens  int                           `json:"total_tokens"`
	Missing      int                           `json:"missing_files"`
	ByCategory   map[string]tokenprofile.CategorySummary `json:"by_category"`
}

func printTokenBudgetJSON(w io.Writer, r *tokenprofile.Report) error {
	out := tokenBudgetJSON{
		TotalBytes:  r.TotalBytes,
		TotalTokens: r.TotalTokens,
		Missing:     r.Missing,
		ByCategory:  r.ByCategory,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token budget: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func printTokenBudgetHuman(w io.Writer, r *tokenprofile.Report) {
	fmt.Fprintln(w, "Token Budget (approx. 4 chars/token)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n", "Component", "Files", "Bytes", "~Tokens")
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"─────────────────", "─────", "─────────", "────────")

	// Sort categories for deterministic output.
	cats := make([]string, 0, len(r.ByCategory))
	for k := range r.ByCategory {
		cats = append(cats, k)
	}
	sort.Strings(cats)

	for _, cat := range cats {
		s := r.ByCategory[cat]
		fmt.Fprintf(w, "%-16s  %5d  %9d  %8d\n", cat, s.Files, s.Bytes, s.Tokens)
	}
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"─────────────────", "─────", "─────────", "────────")
	fmt.Fprintf(w, "%-16s  %5d  %9d  %8d\n",
		"TOTAL", totalFiles(r), r.TotalBytes, r.TotalTokens)

	if r.Missing > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Note: %d planned file(s) not found on disk. Run `squadai apply` to install.\n", r.Missing)
	}
	if r.TotalTokens == 0 && r.Missing == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No installed files found. Run `squadai apply` to install.")
	}
}

func totalFiles(r *tokenprofile.Report) int {
	n := 0
	for _, s := range r.ByCategory {
		n += s.Files
	}
	return n
}
```

- [ ] **Step 3.4: Run tests and confirm they pass**

```
go test ./internal/cli/... -run "TestRunTokenBudget" -v
```

Expected: all 4 tests pass.

- [ ] **Step 3.5: Commit**

```
git add internal/cli/token_budget.go internal/cli/token_budget_test.go
git commit -m "feat(cli): squadai token-budget — per-session token cost report"
```

---

## Task 4: Wire token-budget into app.go

**Files:**
- Modify: `internal/app/app.go`

- [ ] **Step 4.1: Write a failing test**

Add to `internal/app/app_test.go` (the file testing top-level dispatch — check if it exists first; if not, add to `internal/cli/commands_test.go`):

Run this to check:
```
grep -rn "TestRun_" internal/app/ internal/cli/ --include="*_test.go" | head -5
```

If `internal/app/app_test.go` exists, add:
```go
func TestRun_TokenBudget_Help(t *testing.T) {
	var out bytes.Buffer
	err := app.Run([]string{"squadai", "token-budget", "--help"}, &out, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "token-budget") {
		t.Errorf("help output missing 'token-budget': %q", out.String())
	}
}
```

If there's no app_test.go, skip the app-level test and only write the CLI test in the step above (already written in Task 3).

- [ ] **Step 4.2: Add dispatch in app.go**

In `internal/app/app.go`, in the `Run` switch statement, after the `"memory"` case (or after the `"explain"` case), add:

```go
	case "token-budget":
		return cli.RunTokenBudget(args[1:], stdout)
```

- [ ] **Step 4.3: Add to command registry in app.go**

In `buildCommandRegistry()`, add after the `"memory"` entry:

```go
		{
			Name:        "token-budget",
			Description: "Estimate per-session token cost of the current squadai install.",
			Flags: []cmdFlag{
				{Name: "--json", Type: "bool", Description: "Output as JSON"},
			},
		},
```

- [ ] **Step 4.4: Add to printUsage in app.go**

In `printUsage`, add `token-budget` to the commands list:

```go
  token-budget       Estimate per-session token cost of current install (--json for JSON)
```

- [ ] **Step 4.5: Run full test suite**

```
go test ./... 
```

Expected: all tests pass.

- [ ] **Step 4.6: Smoke-test the command end-to-end**

```
go run ./cmd/squadai token-budget --help
go run ./cmd/squadai token-budget
go run ./cmd/squadai token-budget --json
```

Expected:
- `--help` prints usage with "token-budget" in the output
- No args: prints the table (or "No installed files found" if not applied in this repo)
- `--json`: outputs valid JSON with `total_tokens` key

- [ ] **Step 4.7: Commit**

```
git add internal/app/app.go
git commit -m "feat(app): wire squadai token-budget command"
```

---

## Self-Review

**Spec coverage check:**

| Rec | Requirement | Task |
|-----|-------------|------|
| 5a | Measure tokens per installed file | Task 3 (uses tokenprofile to scan planned files) |
| 5a | Break down by component bucket | Task 3 (groups by action.Component) |
| 5b.1 | Inject stub for subagents, not full protocol | Task 2 |
| 5b.1 | Orchestrator keeps full protocol | Task 2 (conditional on `roleName == "orchestrator"`) |
| 5c.1 | `squadai token-budget` CLI | Tasks 3+4 |
| 5c.1 | Per-component breakdown | Task 3 |
| 5c.1 | `--json` output | Task 3 |

**Placeholder scan:** None found. All code steps contain complete, runnable code.

**Type consistency check:**
- `tokenprofile.CategorySummary` used in `token_budget.go` matches definition in `profiler.go` ✓
- `tokenprofile.Report.Missing` referenced in both implementation and tests ✓
- `tokenprofile.Report.ByCategory` is `map[string]CategorySummary` — used consistently ✓
- `loadAndMerge(homeDir, projectDir)` signature matches existing function in commands.go ✓
- `DetectAdapters(homeDir)` signature matches existing function in commands.go ✓
- `planner.New().Plan(merged, adapters, homeDir, projectDir)` matches planner API used in RunPlan ✓
- `config.ProjectConfig` and `config.TeamRoleConfig` used in test helper — verify these type names are correct before implementing:
  ```
  grep -n "type ProjectConfig\|type TeamRoleConfig" internal/config/*.go
  ```
  If names differ, update the test helper accordingly.

**Open question for implementer:**
The `writeTDDProjectForBudget` test helper uses `config.ProjectConfig` and `config.TeamRoleConfig`. Run:
```
grep -n "type.*Config" internal/config/config.go | head -10
```
and adjust the struct names if needed. All other types and function signatures are derived from existing, verified code.
