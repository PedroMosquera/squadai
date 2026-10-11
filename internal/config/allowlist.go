package config

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/model"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// CheckAllowLists returns one message per declaration in cfg that falls
// outside cfg.Allowed, in a stable order. It checks enabled MCP servers,
// custom agent and command models, the model each team role resolves to for
// every enabled adapter that writes agents, hook commands, and the source of
// every enabled plugin: the marketplace named in its plugin_id and, when the
// declaration says where to fetch that marketplace, that location too, so an
// allowed name cannot be pointed at another repository. Plugins without a
// plugin_id ship inside squadai and have no source to check. A nil cat
// resolves tiers with the embedded catalog.
func CheckAllowLists(cfg *domain.MergedConfig, adapters []domain.Adapter, cat *modelcatalog.Catalog) []string {
	allow := cfg.Allowed
	var out []string

	for _, name := range slices.Sorted(maps.Keys(cfg.MCP)) {
		def := cfg.MCP[name]
		if !def.Enabled {
			continue
		}
		if v := allow.MCPServerViolation(name, strings.Join(def.Command, " "), def.URL); v != "" {
			out = append(out, v)
		}
	}

	if allow.Models != nil {
		out = append(out, modelViolations(cfg, adapters, cat)...)
	}

	for _, event := range slices.Sorted(maps.Keys(cfg.Hooks)) {
		for _, group := range cfg.Hooks[event] {
			for _, h := range group.Hooks {
				if !allow.HookCommandAllowed(h.Command) {
					out = append(out, fmt.Sprintf("%s hook command %q is not in allowed.hook_commands", event, h.Command))
				}
			}
		}
	}

	for _, name := range slices.Sorted(maps.Keys(cfg.Plugins)) {
		def := cfg.Plugins[name]
		_, source, ok := strings.Cut(def.PluginID, "@")
		if !def.Enabled || !ok {
			continue
		}
		if !allow.PluginSourceAllowed(source) {
			out = append(out, fmt.Sprintf("plugin %q comes from %q, which is not in allowed.plugin_sources", name, source))
		}
		if loc := def.Marketplace.Location(); loc != "" && !allow.PluginSourceAllowed(loc) {
			out = append(out, fmt.Sprintf("plugin %q marketplace %q is fetched from %q, which is not in allowed.plugin_sources", name, source, loc))
		}
	}
	return out
}

func modelViolations(cfg *domain.MergedConfig, adapters []domain.Adapter, cat *modelcatalog.Catalog) []string {
	allow := cfg.Allowed
	var out []string
	for _, name := range slices.Sorted(maps.Keys(cfg.Agents)) {
		if m := cfg.Agents[name].Model; m != "" && !allow.ModelAllowed(m) {
			out = append(out, fmt.Sprintf("agent %q model %q is not in allowed.models", name, m))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Commands)) {
		if m := cfg.Commands[name].Model; m != "" && !allow.ModelAllowed(m) {
			out = append(out, fmt.Sprintf("command %q model %q is not in allowed.models", name, m))
		}
	}

	if !cfg.Components[string(domain.ComponentAgents)].Enabled {
		return out
	}
	methodology := string(cfg.Methodology)
	for _, a := range adapters {
		if !cfg.Adapters[string(a.ID())].Enabled || !a.SupportsComponent(domain.ComponentAgents) {
			continue
		}
		for _, role := range slices.Sorted(maps.Keys(cfg.Team)) {
			m := model.ResolveRoleModelFor(cfg, methodology, role, cfg.Team[role].Model, a.ID(), cat)
			if m != "" && !allow.ModelAllowed(m) {
				out = append(out, fmt.Sprintf("team role %q resolves to model %q for %s, which is not in allowed.models", role, m, a.ID()))
			}
		}
	}
	return out
}

// claudeModelID matches a bare Claude Code model ID, optionally followed by a
// "-*" that stands for every later segment.
var claudeModelID = regexp.MustCompile(`^([a-z0-9][a-z0-9.]*(?:-[a-z0-9.]+)*)(-\*)?$`)

// emitClaudeAvailableModels writes allowed.models into the Claude Code
// availableModels setting, the only one of Claude's model controls that is
// honored in project settings (availableModelsMatch, deniedModels and
// allowedProviders are managed-only). availableModels matches by prefix on
// segment boundaries, so only literal IDs and a trailing "-*" translate.
// Any other pattern that could match a Claude ID would make the emitted list
// stricter than the policy, so nothing is emitted. Provider-qualified IDs
// (with a "/") belong to other harnesses and are skipped. An explicit
// availableModels in the policy's required settings always wins.
func emitClaudeAvailableModels(merged *domain.MergedConfig, policy *domain.PolicyConfig) {
	id := string(domain.AgentClaudeCode)
	ac, ok := merged.Adapters[id]
	if !ok || policy.Allowed.Models == nil {
		return
	}
	if _, explicit := policy.Required.Adapters[id].Settings["availableModels"]; explicit {
		return
	}
	var entries []string
	for _, p := range policy.Allowed.Models {
		if strings.Contains(p, "/") {
			continue
		}
		m := claudeModelID.FindStringSubmatch(p)
		if m == nil {
			return
		}
		entries = append(entries, m[1])
	}
	if len(entries) == 0 {
		return
	}
	ac = cloneAdapterConfig(ac)
	if ac.Settings == nil {
		ac.Settings = map[string]interface{}{}
	}
	ac.Settings["availableModels"] = entries
	merged.Adapters[id] = ac
}
