## Project Memory Protocol

`docs/memory/` is this project's indexed memory store for decisions, learnings,
and incidents. Sub-agents should use it as follows:

**Before any research, planning, or implementation task:**
Delegate to the `librarian` sub-agent to search memory for relevant prior
decisions, ADRs, runbooks, or context. Pass findings as context to other agents.

**After any non-trivial task:**
Delegate to the `librarian` again with a summary of decisions made, asking it
to propose new memory entries. The librarian writes drafts to
`docs/memory/_inbox/`; the user runs `squadai memory promote` to accept them.

**Available slash commands:**
- `/memory-search <query>` — search and return ranked snippets
- `/memory-add <note>` — add a new entry to the inbox
- `/memory-reindex` — regenerate the index after manual edits
- `/memory-promote` — promote inbox drafts to permanent topic folders
