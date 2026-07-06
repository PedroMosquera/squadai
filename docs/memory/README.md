# Project Memory

This directory stores structured notes about decisions, learnings, and incidents.

## Structure

- `_inbox/` — raw, unprocessed notes (add with `/memory-add` or `squadai memory add`)
- `decisions/` — promoted architectural and design decisions
- `learnings/` — promoted insights and lessons learned
- `incidents/` — promoted post-mortems and incident notes

## Workflow

1. Capture notes during a session: `/memory-add "Decision: ..."`
2. Review inbox periodically: `squadai memory status`
3. Promote useful notes: `squadai memory promote docs/memory/_inbox/<file>`
4. Search before starting significant work: `/memory-search <topic>`

Run `squadai memory reindex` to rebuild the search index after bulk changes.