## Project Memory Protocol

Apply the following rules when working in this repository:

- Search `docs/memory/` for prior decisions before any architecture, API, or
  infrastructure change. Run `/memory-search <query>` or execute
  `squadai memory search <query>` in the terminal.
- Capture decisions and learnings immediately after making them.
  Run `/memory-add <note>` or `squadai memory add "<note>"`.
- Notes are staged in `docs/memory/_inbox/`. Periodically run
  `/memory-promote` to move them into permanent topic folders.
- For multi-topic research, use the `@librarian` agent with a plain-language
  query to retrieve ranked excerpts from the memory index.
- Run `/memory-reindex` or `squadai memory reindex` after manually editing
  files in `docs/memory/` to keep the search index current.
