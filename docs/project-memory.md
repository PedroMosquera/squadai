# Project Memory

SquadAI includes a lightweight memory system for capturing and retrieving
project notes across sessions.

## Quick start

During a session, capture a decision:
    squadai memory add "Decision: chose Bubble Tea over tview for TUI"

Or use the slash command from inside your AI assistant:
    /memory-add Decision: chose Bubble Tea over tview for TUI

## Workflow

### 1. Add notes (inbox)
Notes land in `docs/memory/_inbox/`. Use them freely — don't over-structure.

    squadai memory add "<note>"
    /memory-add <note>

### 2. Search before starting work
Before a significant session, search for prior context:

    squadai memory search <topic>
    /memory-search <topic>

The `@librarian` agent can also be invoked by the orchestrator for richer lookups.

### 3. Promote inbox items
Periodically move raw notes to categorized directories:

    squadai memory promote docs/memory/_inbox/<file>
    /memory-promote

Categories: `decisions`, `learnings`, `incidents`.

### 4. Reindex
After bulk changes outside the CLI:

    squadai memory reindex

## Directory layout

    docs/memory/
    ├── README.md
    ├── _inbox/       ← unprocessed notes
    ├── decisions/    ← architectural decisions (promoted)
    ├── learnings/    ← insights and lessons (promoted)
    └── incidents/    ← post-mortems (promoted)

## Search index

The index lives at `.squadai/memory-index.json` and is rebuilt by `reindex`.
It's auto-rebuilt on the first `memory search` if the index is absent.
