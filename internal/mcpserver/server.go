// Package mcpserver implements a JSON-RPC 2.0 / MCP stdio server for SquadAI.
// It exposes core SquadAI operations as MCP tools that LLM agents can call
// directly, making SquadAI self-configurable via Claude Code MCP integrations.
package mcpserver

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ─── JSON-RPC 2.0 wire types ──────────────────────────────────────────────────

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ─── MCP types ────────────────────────────────────────────────────────────────

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type capabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type initResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolRunner is a function that runs a SquadAI command and writes output to the given writer.
type ToolRunner func(args []string, stdout io.Writer) error

// Server is the MCP stdio server.
type Server struct {
	version string
	tools   map[string]ToolRunner
	defs    []toolDef
}

// New creates a new MCP server with the given version and tool runners.
func New(version string, runners map[string]ToolRunner) *Server {
	s := &Server{
		version: version,
		tools:   runners,
	}
	s.defs = buildToolDefs()
	return s
}

// Serve reads JSON-RPC requests from r and writes responses to w until EOF.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	enc := json.NewEncoder(w)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = enc.Encode(response{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error: " + err.Error()},
			})
			continue
		}

		resp := s.handle(req)
		_ = enc.Encode(resp)
	}

	return scanner.Err()
}

func (s *Server) handle(req request) response {
	base := response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		base.Result = initResult{
			ProtocolVersion: "2024-11-05",
			Capabilities:    capabilities{Tools: &toolsCapability{ListChanged: false}},
			ServerInfo:      serverInfo{Name: "squadai", Version: s.version},
		}

	case "initialized":
		// Notification — no response ID needed.
		return response{JSONRPC: "2.0"}

	case "tools/list":
		base.Result = map[string]any{"tools": s.defs}

	case "tools/call":
		return s.handleToolCall(base, req.Params)

	default:
		base.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}

	return base
}

func (s *Server) handleToolCall(base response, rawParams json.RawMessage) response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		base.Error = &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
		return base
	}

	runner, ok := s.tools[params.Name]
	if !ok {
		base.Error = &rpcError{Code: -32601, Message: "unknown tool: " + params.Name}
		return base
	}

	// Parse arguments as key→value map, flatten to CLI args.
	var argMap map[string]json.RawMessage
	if len(params.Arguments) > 0 && string(params.Arguments) != "null" {
		if err := json.Unmarshal(params.Arguments, &argMap); err != nil {
			base.Error = &rpcError{Code: -32602, Message: "invalid arguments: " + err.Error()}
			return base
		}
	}
	cliArgs := flattenArgs(argMap)

	var buf bytes.Buffer
	runErr := runner(cliArgs, &buf)
	output := buf.String()

	if runErr != nil {
		base.Result = callResult{
			Content: []textContent{{Type: "text", Text: fmt.Sprintf("Error: %v\n\n%s", runErr, output)}},
			IsError: true,
		}
		return base
	}

	if output == "" {
		output = "OK\n"
	}
	base.Result = callResult{
		Content: []textContent{{Type: "text", Text: output}},
	}
	return base
}

// flattenArgs converts a JSON argument map to CLI flag strings.
// Boolean true → --flag; string/number → --flag=value.
func flattenArgs(m map[string]json.RawMessage) []string {
	if len(m) == 0 {
		return nil
	}
	args := make([]string, 0, len(m))
	for k, v := range m {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			args = append(args, fmt.Sprintf("--%s=%s", k, s))
			continue
		}
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			if b {
				args = append(args, "--"+k)
			}
			continue
		}
		// Fallback: use raw JSON value.
		args = append(args, fmt.Sprintf("--%s=%s", k, strings.Trim(string(v), `"`)))
	}
	return args
}

// buildToolDefs returns the static tool definitions exposed by the server.
func buildToolDefs() []toolDef {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	boolProp := func(desc string) map[string]any {
		return map[string]any{"type": "boolean", "description": desc}
	}
	schema := func(required []string, props map[string]any) map[string]any {
		s := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}

	return []toolDef{
		{
			Name:        "plan",
			Description: "Compute the SquadAI action plan. Returns what apply would do without writing files.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return plan as JSON"),
			}),
		},
		{
			Name:        "apply",
			Description: "Execute the SquadAI plan: write agent configs, inject MCP servers, install hooks.",
			InputSchema: schema(nil, map[string]any{
				"json":    boolProp("Return apply report as JSON"),
				"dry-run": boolProp("Preview changes without writing"),
				"force":   boolProp("Apply even without project.json"),
			}),
		},
		{
			Name:        "verify",
			Description: "Run compliance and health checks. Returns pass/fail for each component.",
			InputSchema: schema(nil, map[string]any{
				"json":   boolProp("Return verify report as JSON"),
				"strict": boolProp("Also check for drift since last apply"),
			}),
		},
		{
			Name:        "status",
			Description: "Return project status: adapters, components, MCP servers, health summary, last backup.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return status as JSON"),
				"fix":  boolProp("Run apply automatically if health checks fail"),
			}),
		},
		{
			Name:        "context",
			Description: "Dump the merged SquadAI configuration as LLM-ready context.",
			InputSchema: schema(nil, map[string]any{
				"format":  strProp("Output format: prompt, json, or mcp (default: prompt)"),
				"adapter": strProp("Scope output to a specific adapter ID"),
			}),
		},
		{
			Name:        "init",
			Description: "Initialize or re-initialize .squadai/project.json for this project.",
			InputSchema: schema(nil, map[string]any{
				"methodology": strProp("Development methodology: tdd, sdd, or conventional"),
				"model-tier":  strProp("Model tier: balanced, performance, starter, or manual"),
				"json":        boolProp("Return init result as JSON"),
				"merge":       boolProp("Merge with existing config instead of replacing"),
				"force":       boolProp("Overwrite existing config without merging"),
			}),
		},
		{
			Name:        "validate_policy",
			Description: "Validate policy.json schema and lock/required field consistency.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return validation result as JSON"),
			}),
		},
		{
			Name:        "schema_export",
			Description: "Export JSON Schema for project.json and/or policy.json.",
			InputSchema: schema(nil, map[string]any{
				"format": strProp("Which schema: project, policy, or all (default: all)"),
			}),
		},
		{
			Name:        "doctor",
			Description: "Run pre-flight diagnostics: environment, agents, config, MCP servers, filesystem, drift.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return diagnostics as JSON"),
				"fix":  boolProp("Attempt to auto-fix detected issues"),
			}),
		},
		{
			Name:        "plugins_sync",
			Description: "Fetch the latest plugin registry from the community marketplace.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return result as JSON"),
			}),
		},
		{
			Name:        "plugins_list",
			Description: "List available plugins from the registry. Shows which are installed.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return list as JSON"),
			}),
		},
		{
			Name:        "install_hooks",
			Description: "Install a Git pre-commit hook that runs 'squadai verify --strict'.",
			InputSchema: schema(nil, map[string]any{
				"json": boolProp("Return result as JSON"),
			}),
		},
	}
}

// RunMCPServer starts the MCP stdio server and blocks until stdin is closed.
func RunMCPServer(args []string, stdout, stderr io.Writer, version string, runners map[string]ToolRunner) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(stdout, "Usage: squadai mcp-server")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Start SquadAI as an MCP (Model Context Protocol) stdio server.")
			fmt.Fprintln(stdout, "Reads JSON-RPC 2.0 requests from stdin, writes responses to stdout.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "This allows Claude Code (or any MCP-compatible client) to call")
			fmt.Fprintln(stdout, "SquadAI tools directly: plan, apply, verify, status, context, etc.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "To register with Claude Code, add to .claude/settings.json:")
			fmt.Fprintln(stdout, `  "mcpServers": {`)
			fmt.Fprintln(stdout, `    "squadai": {`)
			fmt.Fprintln(stdout, `      "command": "squadai",`)
			fmt.Fprintln(stdout, `      "args": ["mcp-server"]`)
			fmt.Fprintln(stdout, `    }`)
			fmt.Fprintln(stdout, `  }`)
			return nil
		}
	}

	fmt.Fprintln(stderr, "squadai mcp-server: ready (stdin → stdout)")
	s := New(version, runners)
	return s.Serve(os.Stdin, stdout)
}
