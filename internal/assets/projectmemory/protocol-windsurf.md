## Project Memory Protocol

I maintain an indexed knowledge base at `docs/memory/` that captures prior
decisions, learnings, and incidents for this project.

Before I begin significant work, I search memory for relevant context:
`squadai memory search <query>`. I pass any findings into my planning.

When I complete a meaningful task — an implementation, a design decision, a
debugging session — I capture what I learned: `squadai memory add "<note>"`.
Notes land in `docs/memory/_inbox/` as drafts until promoted.

I use the `@librarian` agent for deeper research when a single search is not
enough. I invoke it with a plain query and it returns ranked file excerpts.

I run `squadai memory promote` periodically to graduate inbox drafts into
permanent topic folders. I keep the inbox tidy so it stays useful.

I run `squadai memory reindex` after any manual edits to `docs/memory/` to
keep the full-text search index in sync.
