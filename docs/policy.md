# Policy

Team policy controls what configuration can and cannot be overridden by individual users or project configs. This document explains how to author, validate, and enforce policies.

## Overview

A policy file (`.squadai/policy.json`) is optional. When present, it:

1. Sets the operational mode to `team`
2. Locks specific fields so they cannot be overridden
3. Defines required values that are enforced during merge

The policy file is committed to the repository and applies to all team members.

## Policy File

Path: `.squadai/policy.json`

```json
{
  "version": 1,
  "mode": "team",
  "locked": [
    "adapters.opencode.enabled",
    "components.memory.enabled",
    "copilot.instructions_template"
  ],
  "required": {
    "adapters": {
      "opencode": { "enabled": true }
    },
    "components": {
      "memory": { "enabled": true }
    },
    "copilot": {
      "instructions_template": "standard"
    }
  }
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | integer | Schema version, must be `1` |
| `mode` | string | Operational mode, typically `"team"` |
| `locked` | string array | Dot-notation paths of fields that cannot be overridden |
| `required` | object | Values enforced for locked fields |
| `allowed` | object | Optional allow-lists for MCP servers, models, hook commands and plugin sources. See [Allow-lists](#allow-lists). |

## Allow-lists

The `allowed` block restricts what project config and committed harness files may declare. Each list is optional. An absent list means no restriction, so a policy without `allowed` behaves exactly as before. A present but empty list (`[]`) allows nothing.

```json
{
  "version": 1,
  "mode": "team",
  "locked": [],
  "required": {},
  "allowed": {
    "mcp_servers": {
      "names": ["squadai", "context7", "docs"],
      "commands": ["squadai mcp-server", "npx -y @upstash/context7-mcp@*"],
      "urls": ["https://mcp.example.com/*"]
    },
    "models": ["claude-sonnet-5-*", "claude-haiku-4-5", "anthropic/claude-*"],
    "hook_commands": ["squadai *", "./scripts/hooks/*"],
    "plugin_sources": ["claude-plugins-official", "team-tools", "github.com/example-org/*"]
  }
}
```

Entries are globs matched against the whole value: `*` matches any run of characters, including `/` and spaces, and `?` matches one character. Matching is case-sensitive.

| List | Checked against |
|------|-----------------|
| `mcp_servers.names` | The server name. |
| `mcp_servers.commands` | The full command line of a stdio server: command and args joined by single spaces. Remote servers are not checked by this list. |
| `mcp_servers.urls` | The URL of a remote server. Stdio servers are not checked by this list. |
| `models` | Concrete model IDs: `model` on custom agents and commands, and the model each team role resolves to for every enabled agent that writes agent files (OpenCode and Pi use provider-qualified IDs such as `anthropic/claude-sonnet-5-5`). |
| `hook_commands` | Every hook command string. |
| `plugin_sources` | The marketplace of each enabled plugin (the part after `@` in `plugin_id`, or in an `enabledPlugins` key). When a declaration sets `marketplace`, its location is checked too: `github.com/<repo>` for a `github` source, otherwise its `url` or `path`. This stops an allowed marketplace name from being pointed at another repository. A `package` plugin is checked by where its package comes from: `npm:<name>` for npm (match a scope with `npm:@example-org/*`) and `<host>/<repo>` for git, the same form as a `github` marketplace, so `github.com/example-org/*` covers both. Bundled plugins without a `plugin_id` are not checked. |

A server must pass every `mcp_servers` list that is set. Set `commands` or `urls` as well as `names`: a hand-added server can reuse an allowed name.

The lists apply to everything, including what SquadAI itself configures. If the project registers SquadAI's own MCP server, allow `squadai` and `squadai mcp-server`. If SquadAI installs hooks for you, allow `squadai *`.

Where each list is enforced:

| Command | What it checks | Exit code on a violation |
|---------|----------------|--------------------------|
| `squadai validate-policy` | `.squadai/project.json` merged with the policy (user config is ignored) | `3` (`E-201`) |
| `squadai verify` and `squadai verify --ci` | The merged config, one `policy-allow-list` failure per declaration | `3` (`E-201`) under `--ci`, ahead of drift |
| `squadai scan` | Committed harness files, as rule `SCAN-007` (high): MCP servers in every harness MCP file, hooks and `enabledPlugins` in `.claude/settings.json`, and package entries in `.pi/settings.json` `packages` and `opencode.json` `plugin` (local paths are skipped). This catches servers, hooks and packages added by hand that SquadAI never sees in `project.json`. | `3` (`E-203`) at the default `--fail-on high` |

### Native controls

Claude Code has model controls of its own. Only `availableModels` is honored in project settings, so it is the only one SquadAI writes:

- When `allowed.models` is set and every pattern can be expressed as an `availableModels` entry, `apply` writes `availableModels` into `.claude/settings.json` through the settings component (enable the `settings` component), with managed-key ownership. A literal ID such as `claude-haiku-4-5` is written as is, and `claude-sonnet-5-*` becomes `claude-sonnet-5`, because Claude Code matches entries by prefix on segment boundaries. Provider-qualified patterns (containing `/`) are for other agents and are skipped. If any other pattern is present (for example `*sonnet*` or `claude-sonnet-5*`), nothing is written, because a partial list would block models the policy allows.
- An explicit `required.adapters.claude-code.settings.availableModels` in the policy always wins over the derived list. Use it to write a different list, such as family aliases.
- Project-scope `availableModels` is a guardrail, not enforcement: Claude Code merges the list across user, project and local settings, so a developer can widen it. It also restricts the interactive `/model` picker, not only agent files.

The enforcing controls are managed-only. Claude Code ignores them in user, project and local settings, so SquadAI never writes them. To enforce the same allow-lists for an organization, deploy them in managed settings ([managed settings](https://code.claude.com/docs/en/managed-settings), [settings reference](https://code.claude.com/docs/en/settings-reference)):

| SquadAI list | Managed setting |
|--------------|-----------------|
| `models` | `availableModels` with `availableModelsMatch: "exact"` (so a listed ID does not also admit later versions), plus `deniedModels` for versions to block. `allowedProviders` limits the API providers a machine may use. |
| `mcp_servers` | `allowedMcpServers` with `allowManagedMcpServersOnly: true`, plus `deniedMcpServers`. |
| `hook_commands` | `allowManagedHooksOnly: true`, delivering the allowed hooks from managed settings. |
| `plugin_sources` | `strictKnownMarketplaces` (and `blockedMarketplaces`). |

`maxEffortLevel` is honored from any scope (the lowest cap wins) but has no allow-list to map from; it belongs to a budget profile, not to `allowed`.

## Lockable Fields

The following fields can be locked by policy:

| Field Path | Controls |
|------------|----------|
| `adapters.opencode.enabled` | Whether OpenCode adapter is active |
| `adapters.claude-code.enabled` | Whether Claude Code adapter is active |
| `adapters.vscode-copilot.enabled` | Whether VS Code Copilot adapter is active |
| `adapters.cursor.enabled` | Whether Cursor adapter is active |
| `adapters.windsurf.enabled` | Whether Windsurf adapter is active |
| `adapters.pi.enabled` | Whether Pi adapter is active |
| `adapters.codex.enabled` | Whether Codex adapter is active |
| `components.memory.enabled` | Whether memory component is installed |
| `copilot.instructions_template` | Which copilot instructions template is used |

## How Locking Works

### Config Merge Precedence

Configuration is resolved in three layers:

1. **Policy locked fields** (highest priority) — cannot be overridden
2. **Project config** (`.squadai/project.json`)
3. **User config** (`~/.squadai/config.json`, lowest priority)

When a user or project config sets a value for a locked field, the policy value wins. The conflict is recorded as a violation and reported in the output of `plan` and `verify`.

### Example

User config sets `adapters.opencode.enabled: false`, but policy locks it to `true`:

```
$ squadai plan
Policy overrides:
  - field "adapters.opencode.enabled" locked to true, overriding false

Mode: team
...
```

The merged config will have `adapters.opencode.enabled: true` regardless of what the user configured.

## Operational Modes

| Mode | Behavior |
|------|----------|
| `team` | Policy file governs all settings. Locked fields enforced. |
| `personal` | User config only. No policy enforcement. |

> **Note:** `hybrid` mode is deprecated. SquadAI resolves it to `team` (if `policy.json` exists) or `personal` (if not), and prints a deprecation warning.

When a policy file exists and sets `mode: "team"`, the mode is forced to `team` regardless of user config.

## Validation

Run `squadai validate-policy` to check the policy file for:

- **Schema correctness** — required fields present, valid types
- **Lock consistency** — every entry in `locked` has a corresponding value in `required`
- **Required values** — locked values are valid (e.g., boolean for `enabled` fields)
- **Allow-lists**: no empty patterns in `allowed`, and nothing in `.squadai/project.json` outside them (exit `3`)

```sh
$ squadai validate-policy
Policy is valid. No issues found.
```

If issues are found:

```sh
$ squadai validate-policy
Policy validation found 2 issue(s):
  1. locked field "adapters.unknown.enabled" has no corresponding required value
  2. required copilot.instructions_template is empty but field is locked
```

## Creating a Policy

Use `squadai init --with-policy` to generate a policy template:

```sh
squadai init --with-policy
```

This creates `.squadai/policy.json` with the default locked fields (OpenCode enabled, memory enabled, copilot instructions set to standard).

Edit the file to match your team's requirements, then commit it to the repository.

## Personal Lane Compatibility

Policy only governs team-lane adapters and shared components. Personal-lane adapters (Claude Code, VS Code Copilot, Cursor, Windsurf, Pi, Codex) are controlled by the user config and are not affected by policy locks unless explicitly locked.

A user can enable Claude Code, Cursor, or any other personal-lane adapter in their `~/.squadai/config.json` without affecting team compliance, as long as the team-required adapters and components remain enabled.

## Troubleshooting

**"Policy violation" in plan output**

This means a user or project config tried to override a locked field. The policy value is used instead. No action needed unless the policy itself is wrong.

**"validate-policy" fails with lock consistency errors**

Every field listed in `locked` must have a corresponding value in `required`. Add the missing required value or remove the lock.

**Personal adapter not appearing**

Personal-lane adapters (Claude Code, VS Code Copilot, Cursor, Windsurf, Pi, Codex) are only included when the binary or config directory is detected. They are not controlled by policy unless explicitly locked.
