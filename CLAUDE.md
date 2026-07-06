<!-- squadai:memory:claude-code -->
## Project Memory Protocol

`docs/memory/` is this project's persistent, indexed memory store for
decisions, learnings, and incidents. It survives across sessions — use it.

**Search first.** Before starting significant work, run `/memory-search <query>`
(or `squadai memory search <query>` in the terminal) and pass any findings into
your plan. Never skip memory-search before architecture or API decisions.

**Capture as you go.** After a decision, fix, or discovery, run
`/memory-add <note>` (or `squadai memory add "<note>"`). Notes land in
`docs/memory/_inbox/` as drafts until promoted.

**Housekeeping.** Run `/memory-promote` periodically to graduate inbox drafts
into permanent topic folders, and `/memory-reindex` after manual edits under
`docs/memory/` to keep the search index current.

For deeper multi-query research, ask the `@librarian` agent with a
plain-language question; it returns ranked excerpts from the memory index.
<!-- /squadai:memory:claude-code -->

<!-- squadai:brand:claude-code -->
```text
                          *
    o      o      o     .---.
   /|\____/|\____/|\____|o o|    S Q U A D A I
   / \    / \    / \    '---'    x Claude Code
```

<!-- /squadai:brand:claude-code -->

<!-- squadai:efficiency:claude-code -->
## Session Efficiency Protocol

Work token-efficiently. These rules apply to every task in this repository.

**Search before read.** Locate code with grep/glob first; read only the files
and line ranges you need. Never read a whole file when a targeted range works.

**Never re-read a file you just edited.** The edit either succeeded or
errored; trust that result instead of re-opening the file to check.

**Summarize long output.** When a tool returns more than ~30 lines, extract
the relevant findings instead of pasting the whole output into the transcript.

**Delegate exploration.** Send open-ended codebase exploration to sub-agents
and request a compact report (files, symbols, one-line conclusions) — keep
raw file dumps out of the main context.

**Memory first.** Run a memory search before exploring the codebase — prior
decisions often answer the question faster than fresh exploration.

**Response discipline.** Answer, then stop. Prefer code over prose; do not
restate the request or narrate obvious steps.

**Checkpoint at ~60% context.** When roughly 60% of the context window is
used, stop exploring, write down what you know, and finish the current step
before starting anything new.
<!-- /squadai:efficiency:claude-code -->
