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
