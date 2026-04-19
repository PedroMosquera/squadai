// Package doctor implements the pre-flight diagnostic command.
// It runs read-only checks across six categories and reports results in
// either human-readable colored terminal output or structured JSON.
package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// CheckStatus represents the outcome of a single diagnostic check.
type CheckStatus int

const (
	// CheckPass means the check succeeded with no issues.
	CheckPass CheckStatus = iota
	// CheckWarn means the check found a non-critical issue.
	CheckWarn
	// CheckFail means the check found a critical issue.
	CheckFail
	// CheckSkip means the check was not applicable and was skipped.
	CheckSkip
)

// String returns the lowercase string representation of the status.
func (s CheckStatus) String() string {
	switch s {
	case CheckPass:
		return "pass"
	case CheckWarn:
		return "warn"
	case CheckFail:
		return "fail"
	case CheckSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// CheckResult holds the outcome of a single diagnostic check.
type CheckResult struct {
	Category    string      `json:"category"`
	Name        string      `json:"name"`
	Status      CheckStatus `json:"status"`
	Message     string      `json:"message"`
	Detail      string      `json:"detail,omitempty"`
	FixHint     string      `json:"fix_hint,omitempty"`
	AutoFixable bool        `json:"auto_fixable,omitempty"`
}

// StatusString returns the JSON-serialisable status string.
func (r CheckResult) StatusString() string { return r.Status.String() }

// PathLooker resolves a binary name to an absolute path.
type PathLooker interface {
	LookPath(file string) (string, error)
}

// pathLookerFunc wraps a function as a PathLooker.
type pathLookerFunc struct{ fn func(string) (string, error) }

func (p pathLookerFunc) LookPath(file string) (string, error) { return p.fn(file) }

// CommandRunner executes a command and returns its combined output.
type CommandRunner interface {
	Output(name string, args ...string) ([]byte, error)
}

// commandRunnerFunc wraps a function as a CommandRunner.
type commandRunnerFunc struct {
	fn func(string, ...string) ([]byte, error)
}

func (c commandRunnerFunc) Output(name string, args ...string) ([]byte, error) {
	return c.fn(name, args...)
}

// defaultCommandRunner uses exec.Command under the hood.
type defaultCommandRunner struct{}

func (defaultCommandRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// Options configures a Doctor run.
type Options struct {
	// JSON outputs structured JSON instead of colored terminal output.
	JSON bool
	// Verbose shows sub-check details for ALL checks.
	Verbose bool
	// Category restricts the run to a single category (case-insensitive slug match).
	Category string
	// Check restricts the run to a single named check in "category.name" format.
	Check string
}

// Doctor runs pre-flight diagnostics.
type Doctor struct {
	homeDir    string
	projectDir string
	adapters   []domain.Adapter
	catalog    []domain.CuratedMCPServer
	looker     PathLooker
	runner     CommandRunner
}

// New returns a Doctor with production dependencies.
func New(homeDir, projectDir string, adapters []domain.Adapter, catalog []domain.CuratedMCPServer) *Doctor {
	return &Doctor{
		homeDir:    homeDir,
		projectDir: projectDir,
		adapters:   adapters,
		catalog:    catalog,
		looker:     pathLookerFunc{fn: exec.LookPath},
		runner:     defaultCommandRunner{},
	}
}

// NewWithDeps returns a Doctor with injected dependencies (for testing).
func NewWithDeps(
	homeDir, projectDir string,
	adapters []domain.Adapter,
	catalog []domain.CuratedMCPServer,
	looker PathLooker,
	runner CommandRunner,
) *Doctor {
	return &Doctor{
		homeDir:    homeDir,
		projectDir: projectDir,
		adapters:   adapters,
		catalog:    catalog,
		looker:     looker,
		runner:     runner,
	}
}

// categoryOrder defines the canonical display order for categories.
var categoryOrder = []string{
	"Environment",
	"AI Agents",
	"Project Configuration",
	"MCP Servers",
	"Filesystem",
	"Config Drift",
}

// categorySlug maps canonical category names to their slug equivalents for
// --category flag matching.
var categorySlug = map[string]string{
	"Environment":           "environment",
	"AI Agents":             "agents",
	"Project Configuration": "config",
	"MCP Servers":           "mcp",
	"Filesystem":            "filesystem",
	"Config Drift":          "drift",
}

// Run executes all checks (or a filtered subset) and returns ordered results.
func (d *Doctor) Run(ctx context.Context, opts Options) ([]CheckResult, error) {
	// Validate --category flag before running any checks.
	if opts.Category != "" {
		if !validCategory(opts.Category) {
			return nil, fmt.Errorf("unknown category %q — valid values: environment, agents, config, mcp, filesystem, drift", opts.Category)
		}
	}

	// Validate --check flag.
	var checkCat, checkName string
	if opts.Check != "" {
		parts := strings.SplitN(opts.Check, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("--check must be in <category>.<name> format, e.g. mcp.github")
		}
		checkCat, checkName = parts[0], parts[1]
		if !validCategory(checkCat) {
			return nil, fmt.Errorf("unknown category %q in --check flag", checkCat)
		}
	}

	var all []CheckResult

	// Run each category, respecting filters.
	type categoryFn struct {
		category string
		slug     string
		fn       func(context.Context) []CheckResult
	}
	categories := []categoryFn{
		{"Environment", "environment", d.runEnvironment},
		{"AI Agents", "agents", d.runAgents},
		{"Project Configuration", "config", d.runProjectConfig},
		{"MCP Servers", "mcp", d.runMCP},
		{"Filesystem", "filesystem", d.runFilesystem},
		{"Config Drift", "drift", d.runConfigDrift},
	}

	for _, cat := range categories {
		if opts.Category != "" && opts.Category != cat.slug {
			continue
		}
		if checkCat != "" && checkCat != cat.slug {
			continue
		}

		results := cat.fn(ctx)

		// Apply --check name filter.
		if checkName != "" {
			filtered := results[:0]
			for _, r := range results {
				if strings.EqualFold(r.Name, checkName) {
					filtered = append(filtered, r)
				}
			}
			if len(filtered) == 0 {
				return nil, fmt.Errorf("no check named %q in category %q", checkName, cat.slug)
			}
			results = filtered
		}

		all = append(all, results...)
	}

	// Sort by (category order index, name) for determinism.
	catIndex := make(map[string]int, len(categoryOrder))
	for i, c := range categoryOrder {
		catIndex[c] = i
	}
	sort.SliceStable(all, func(i, j int) bool {
		ci := catIndex[all[i].Category]
		cj := catIndex[all[j].Category]
		if ci != cj {
			return ci < cj
		}
		return all[i].Name < all[j].Name
	})

	return all, nil
}

// validCategory returns true when s is a known category slug.
func validCategory(s string) bool {
	slugs := []string{"environment", "agents", "config", "mcp", "filesystem", "drift"}
	for _, v := range slugs {
		if s == v {
			return true
		}
	}
	return false
}

// pass is a convenience constructor for a passing CheckResult.
func pass(cat, name, msg, detail string) CheckResult {
	return CheckResult{Category: cat, Name: name, Status: CheckPass, Message: msg, Detail: detail}
}

// warn is a convenience constructor for a warning CheckResult.
func warn(cat, name, msg, detail, hint string) CheckResult {
	return CheckResult{Category: cat, Name: name, Status: CheckWarn, Message: msg, Detail: detail, FixHint: hint}
}

// fail is a convenience constructor for a failing CheckResult.
func fail(cat, name, msg, detail, hint string) CheckResult {
	return CheckResult{Category: cat, Name: name, Status: CheckFail, Message: msg, Detail: detail, FixHint: hint}
}

// skip is a convenience constructor for a skipped CheckResult.
func skip(cat, name, msg string) CheckResult {
	return CheckResult{Category: cat, Name: name, Status: CheckSkip, Message: msg}
}
