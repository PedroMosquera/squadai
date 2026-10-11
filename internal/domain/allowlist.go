package domain

import "fmt"

// AllowLists restricts what project config may declare. Every list follows
// the same rule: nil (key absent from policy.json) means no restriction, and
// a present but empty list allows nothing. Entries are globs where * matches
// any run of characters (slashes and spaces included) and ? matches one.
type AllowLists struct {
	MCPServers    *MCPServerAllowList `json:"mcp_servers,omitempty"`
	Models        []string            `json:"models,omitempty"`
	HookCommands  []string            `json:"hook_commands,omitempty"`
	PluginSources []string            `json:"plugin_sources,omitempty"`
}

// MCPServerAllowList restricts MCP servers. A server must pass every list
// that is set: Names checks the server name, Commands checks the full
// command line (command and args joined by spaces) of stdio servers, and
// URLs checks the URL of remote servers. Names alone is weak, because a
// hand-added server can reuse an allowed name.
type MCPServerAllowList struct {
	Names    []string `json:"names,omitempty"`
	Commands []string `json:"commands,omitempty"`
	URLs     []string `json:"urls,omitempty"`
}

// MCPServerViolation returns why the server is not allowed, or "" when it is.
func (a AllowLists) MCPServerViolation(name, commandLine, url string) string {
	m := a.MCPServers
	if m == nil {
		return ""
	}
	if m.Names != nil && !MatchAnyGlob(m.Names, name) {
		return fmt.Sprintf("MCP server %q is not in allowed.mcp_servers.names", name)
	}
	if m.Commands != nil && commandLine != "" && !MatchAnyGlob(m.Commands, commandLine) {
		return fmt.Sprintf("MCP server %q command %q is not in allowed.mcp_servers.commands", name, commandLine)
	}
	if m.URLs != nil && url != "" && !MatchAnyGlob(m.URLs, url) {
		return fmt.Sprintf("MCP server %q URL %q is not in allowed.mcp_servers.urls", name, url)
	}
	return ""
}

// ModelAllowed reports whether a concrete model ID passes allowed.models.
func (a AllowLists) ModelAllowed(id string) bool {
	return a.Models == nil || MatchAnyGlob(a.Models, id)
}

// HookCommandAllowed reports whether a hook command passes allowed.hook_commands.
func (a AllowLists) HookCommandAllowed(command string) bool {
	return a.HookCommands == nil || MatchAnyGlob(a.HookCommands, command)
}

// PluginSourceAllowed reports whether a marketplace or repository identifier
// passes allowed.plugin_sources.
func (a AllowLists) PluginSourceAllowed(source string) bool {
	return a.PluginSources == nil || MatchAnyGlob(a.PluginSources, source)
}

// MatchAnyGlob reports whether s matches at least one pattern.
func MatchAnyGlob(patterns []string, s string) bool {
	for _, p := range patterns {
		if MatchGlob(p, s) {
			return true
		}
	}
	return false
}

// MatchGlob matches the whole of s against pattern. Unlike path.Match, * also
// crosses "/", because model IDs, command lines and URLs all contain slashes.
func MatchGlob(pattern, s string) bool {
	p, i := 0, 0
	star, mark := -1, 0
	for i < len(s) {
		switch {
		case p < len(pattern) && (pattern[p] == '?' || pattern[p] == s[i]):
			p++
			i++
		case p < len(pattern) && pattern[p] == '*':
			star, mark = p, i
			p++
		case star >= 0:
			p = star + 1
			mark++
			i = mark
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}
