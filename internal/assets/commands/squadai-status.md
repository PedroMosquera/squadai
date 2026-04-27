Run `squadai status --json` and display a rich project status summary.

Show:
- Detected adapters (Claude Code, Cursor, Windsurf, etc.)
- Active components and managed file counts
- MCP server names
- Health check summary (pass/fail counts)
- Last backup info

If health checks are failing, suggest running `/squadai-apply` or `squadai status --fix`.
