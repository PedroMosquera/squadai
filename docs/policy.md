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

## Lockable Fields

The following fields can be locked by policy:

| Field Path | Controls |
|------------|----------|
| `adapters.opencode.enabled` | Whether OpenCode adapter is active |
| `adapters.claude-code.enabled` | Whether Claude Code adapter is active |
| `adapters.vscode-copilot.enabled` | Whether VS Code Copilot adapter is active |
| `adapters.cursor.enabled` | Whether Cursor adapter is active |
| `adapters.windsurf.enabled` | Whether Windsurf adapter is active |
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
| `hybrid` | Both active. Policy locked fields take precedence, other fields follow project > user. |

When a policy file exists and sets `mode: "team"`, the mode is forced to `team` regardless of user config.

## Validation

Run `squadai validate-policy` to check the policy file for:

- **Schema correctness** — required fields present, valid types
- **Lock consistency** — every entry in `locked` has a corresponding value in `required`
- **Required values** — locked values are valid (e.g., boolean for `enabled` fields)

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

Policy only governs team-lane adapters and shared components. Personal-lane adapters (Claude Code, VS Code Copilot, Cursor, Windsurf) are controlled by the user config and are not affected by policy locks unless explicitly locked.

A user can enable Claude Code, Cursor, or any other personal-lane adapter in their `~/.squadai/config.json` without affecting team compliance, as long as the team-required adapters and components remain enabled.

## Troubleshooting

**"Policy violation" in plan output**

This means a user or project config tried to override a locked field. The policy value is used instead. No action needed unless the policy itself is wrong.

**"validate-policy" fails with lock consistency errors**

Every field listed in `locked` must have a corresponding value in `required`. Add the missing required value or remove the lock.

**Personal adapter not appearing**

Personal-lane adapters (Claude Code, VS Code Copilot, Cursor, Windsurf) are only included when the binary or config directory is detected. They are not controlled by policy unless explicitly locked.
