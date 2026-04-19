package doctor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

const catMCP = "MCP Servers"

// runMCP checks each entry in the MCP catalog.
func (d *Doctor) runMCP(_ context.Context) []CheckResult {
	if len(d.catalog) == 0 {
		return []CheckResult{skip(catMCP, "mcp", "no MCP servers in catalog")}
	}

	var results []CheckResult
	for _, server := range d.catalog {
		results = append(results, d.checkMCPServer(server))
	}
	return results
}

// checkMCPServer checks a single catalog MCP server entry.
func (d *Doctor) checkMCPServer(server CuratedMCPServer) CheckResult {
	name := server.Name

	// Check required env vars (RequiredEnvVars field on CuratedMCPServer).
	// If any required env var is missing → fail or warn depending on RequiresAuth.
	for _, envVar := range server.RequiredEnvVars {
		if os.Getenv(envVar) == "" {
			if server.RequiresAuth {
				return fail(catMCP, name,
					fmt.Sprintf("%s — %s not set", name, envVar),
					"",
					fmt.Sprintf("Set the %s environment variable", envVar))
			}
			return warn(catMCP, name,
				fmt.Sprintf("%s — %s not set (server will start but auth may fail)", name, envVar),
				"",
				fmt.Sprintf("Set the %s environment variable", envVar))
		}
	}

	// Also check legacy AuthEnvVars for backward compatibility.
	for _, envVar := range server.AuthEnvVars {
		if os.Getenv(envVar) == "" {
			if server.RequiresAuth {
				return fail(catMCP, name,
					fmt.Sprintf("%s — %s not set", name, envVar),
					"",
					fmt.Sprintf("Set the %s environment variable", envVar))
			}
			return warn(catMCP, name,
				fmt.Sprintf("%s — %s not set (server will start but auth may fail)", name, envVar),
				"",
				fmt.Sprintf("Set the %s environment variable", envVar))
		}
	}

	// Check Node version compatibility when MinNodeVersion is set.
	if server.MinNodeVersion != "" {
		if result := d.checkNodeVersionForMCP(name, server.MinNodeVersion); result != nil {
			return *result
		}
	}

	// For local servers, check that the command binary is available.
	if server.Type == "local" && server.Command != "" {
		if _, err := d.looker.LookPath(server.Command); err != nil {
			return fail(catMCP, name,
				fmt.Sprintf("%s — %s not found in PATH", name, server.Command),
				"",
				fmt.Sprintf("Install %s to run this MCP server", server.Command))
		}
	}

	return pass(catMCP, name, fmt.Sprintf("%s — ready", name), d.mcpDetail(server))
}

// checkNodeVersionForMCP checks whether the installed Node version meets the
// minimum required by the server. Returns nil when the check passes or node
// is not installed (already reported by environment checks).
func (d *Doctor) checkNodeVersionForMCP(serverName, minVersion string) *CheckResult {
	out, err := d.runner.Output("node", "--version")
	if err != nil {
		// node not installed — already flagged by environment check.
		return nil
	}
	versionStr := strings.TrimSpace(string(out))
	installedMajor := parseNodeMajor(versionStr)
	if installedMajor == 0 {
		return nil
	}
	var requiredMajor int
	_, _ = fmt.Sscanf(strings.TrimPrefix(minVersion, "v"), "%d", &requiredMajor)
	if requiredMajor > 0 && installedMajor < requiredMajor {
		r := warn(catMCP, serverName,
			fmt.Sprintf("%s — Node %s installed, v%s+ required", serverName, versionStr, minVersion),
			versionStr,
			fmt.Sprintf("Upgrade Node.js to v%s+ from https://nodejs.org/", minVersion))
		return &r
	}
	return nil
}

// mcpDetail returns a short detail string for a passing MCP server.
func (d *Doctor) mcpDetail(server CuratedMCPServer) string {
	if server.Command != "" {
		args := strings.Join(server.Args, " ")
		if args != "" {
			return server.Command + " " + args
		}
		return server.Command
	}
	if server.URL != "" {
		return server.URL
	}
	return ""
}

// CuratedMCPServer is the domain type imported locally for the MCP check.
// It is a type alias to avoid import cycles — the doctor package operates on
// domain.CuratedMCPServer which is passed in via the catalog slice.
type CuratedMCPServer = domain.CuratedMCPServer
