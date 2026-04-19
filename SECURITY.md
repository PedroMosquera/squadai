# Security Policy

## Supported Versions

Only the latest released version receives security fixes.

| Version | Supported |
|---------|-----------|
| Latest (`v0.x.y`) | ✓ |
| Older releases | ✗ |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report them privately via GitHub Security Advisories:

1. Go to https://github.com/PedroMosquera/squadai/security/advisories
2. Click **"Report a vulnerability"**
3. Provide a clear description, reproduction steps, and impact assessment

You can expect:

- An initial acknowledgement within **5 business days**
- A status update at least every **14 days** until resolution
- Public disclosure coordinated after a fix is released

## Scope

squadai runs as a CLI on developer machines and modifies files inside the
current project directory and inside `~/.squadai/`. Security-relevant areas
include:

- Command execution (`squadai apply`, `squadai doctor --fix`, `squadai update`)
- File writes outside marker-managed regions
- Self-update download / signature verification
- Backup/restore manipulation
- Policy lock bypass
- MCP server configuration injection

Reports unrelated to the above (e.g. third-party dependency CVEs without an
exploit path through squadai) are still welcome but lower priority.

## Out of Scope

- Issues in third-party AI agents (OpenCode, Claude Code, Cursor, Windsurf,
  VS Code Copilot) — please report those upstream.
- Issues in MCP servers or plugins not authored by this project.
- Social-engineering attacks against humans on the team.
