package doctor

import (
	"context"
	"fmt"
	"strings"
)

const catEnv = "Environment"

// runEnvironment runs all environment checks: Go, npx, git, node.
func (d *Doctor) runEnvironment(_ context.Context) []CheckResult {
	return []CheckResult{
		d.checkGo(),
		d.checkNpx(),
		d.checkGit(),
		d.checkNode(),
	}
}

// checkGo verifies Go is on PATH and reports its version.
func (d *Doctor) checkGo() CheckResult {
	path, err := d.looker.LookPath("go")
	if err != nil {
		return fail(catEnv, "Go", "go not found in PATH", "", "Install Go from https://go.dev/dl/")
	}
	out, err := d.runner.Output("go", "version")
	if err != nil {
		return warn(catEnv, "Go", fmt.Sprintf("go found at %s but version check failed", path), "", "")
	}
	version := parseGoVersion(string(out))
	return pass(catEnv, "Go", fmt.Sprintf("go found at %s", path), version)
}

// checkNpx verifies npx is on PATH and reports its version.
func (d *Doctor) checkNpx() CheckResult {
	path, err := d.looker.LookPath("npx")
	if err != nil {
		return warn(catEnv, "npx", "npx not found in PATH (required for MCP servers)", "", "Install Node.js from https://nodejs.org/")
	}
	out, err := d.runner.Output("npx", "--version")
	if err != nil {
		return warn(catEnv, "npx", fmt.Sprintf("npx found at %s but version check failed", path), "", "")
	}
	version := strings.TrimSpace(string(out))
	return pass(catEnv, "npx", fmt.Sprintf("npx found at %s (required for MCP servers)", path), version)
}

// checkGit verifies git is on PATH and reports its version.
func (d *Doctor) checkGit() CheckResult {
	path, err := d.looker.LookPath("git")
	if err != nil {
		return warn(catEnv, "git", "git not found in PATH", "", "Install git from https://git-scm.com/")
	}
	out, err := d.runner.Output("git", "--version")
	if err != nil {
		return warn(catEnv, "git", fmt.Sprintf("git found at %s but version check failed", path), "", "")
	}
	version := parseGitVersion(string(out))
	return pass(catEnv, "git", fmt.Sprintf("git found at %s", path), version)
}

// checkNode verifies node is on PATH and warns if version < 20.
func (d *Doctor) checkNode() CheckResult {
	path, err := d.looker.LookPath("node")
	if err != nil {
		return warn(catEnv, "Node.js", "node not found in PATH (v20+ recommended for MCP compatibility)", "", "Install Node.js from https://nodejs.org/")
	}
	out, err := d.runner.Output("node", "--version")
	if err != nil {
		return warn(catEnv, "Node.js", fmt.Sprintf("node found at %s but version check failed", path), "", "")
	}
	versionStr := strings.TrimSpace(string(out)) // e.g. "v20.11.0"
	major := parseNodeMajor(versionStr)
	if major > 0 && major < 20 {
		return warn(catEnv, "Node.js",
			fmt.Sprintf("node found at %s (%s, v20+ recommended for MCP compatibility)", path, versionStr),
			versionStr,
			"Upgrade Node.js to v20+ from https://nodejs.org/")
	}
	return pass(catEnv, "Node.js", fmt.Sprintf("node found at %s", path), versionStr)
}

// parseGoVersion extracts "go X.Y.Z" from `go version` output.
func parseGoVersion(out string) string {
	// "go version go1.22.4 linux/amd64"
	parts := strings.Fields(out)
	if len(parts) >= 3 {
		return strings.TrimPrefix(parts[2], "go")
	}
	return strings.TrimSpace(out)
}

// parseGitVersion extracts version number from `git --version` output.
func parseGitVersion(out string) string {
	// "git version 2.44.0"
	parts := strings.Fields(out)
	if len(parts) >= 3 {
		return parts[2]
	}
	return strings.TrimSpace(out)
}

// parseNodeMajor extracts the major version integer from a string like "v20.11.0".
func parseNodeMajor(v string) int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 2)
	if len(parts) == 0 {
		return 0
	}
	var major int
	_, _ = fmt.Sscanf(parts[0], "%d", &major)
	return major
}
