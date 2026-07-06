package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// renderTOMLServers renders MCP server definitions as TOML [<rootKey>.<name>]
// tables for adapters whose config is TOML (Codex). Output is deterministic:
// server names and env/header keys are emitted in sorted order.
//
// Shape per server (stdio):
//
//	[mcp_servers.context7]
//	command = "npx"
//	args = ["-y", "@upstash/context7-mcp"]
//	env = { API_KEY = "value" }
//
// Remote servers emit url (and http_headers when headers are set) instead of
// command/args.
func renderTOMLServers(rootKey string, servers map[string]domain.MCPServerDef) string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteString("\n")
		}
		def := servers[name]
		fmt.Fprintf(&b, "[%s.%s]\n", rootKey, tomlKey(name))

		if def.URL != "" {
			fmt.Fprintf(&b, "url = %s\n", tomlString(def.URL))
			if len(def.Headers) > 0 {
				fmt.Fprintf(&b, "http_headers = %s\n", tomlInlineTable(def.Headers))
			}
		} else if len(def.Command) > 0 {
			fmt.Fprintf(&b, "command = %s\n", tomlString(def.Command[0]))
			if len(def.Command) > 1 {
				fmt.Fprintf(&b, "args = %s\n", tomlStringArray(def.Command[1:]))
			}
		}

		if len(def.Environment) > 0 {
			fmt.Fprintf(&b, "env = %s\n", tomlInlineTable(def.Environment))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// tomlKey renders a table-key segment. Bare keys (letters, digits, dashes,
// underscores) are emitted as-is; anything else is quoted.
func tomlKey(key string) string {
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return tomlString(key)
		}
	}
	if key == "" {
		return tomlString(key)
	}
	return key
}

// tomlString renders a basic (double-quoted) TOML string with escaping.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// tomlStringArray renders a string slice as a TOML array of basic strings.
func tomlStringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = tomlString(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// tomlInlineTable renders a string map as a TOML inline table with sorted keys.
func tomlInlineTable(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, len(keys))
	for i, k := range keys {
		pairs[i] = fmt.Sprintf("%s = %s", tomlKey(k), tomlString(m[k]))
	}
	return "{ " + strings.Join(pairs, ", ") + " }"
}
