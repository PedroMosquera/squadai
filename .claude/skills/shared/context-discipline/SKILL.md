---
name: context-discipline
description: Recover from context compaction and keep long agent sessions token-efficient. Load on demand — after a compaction event, or when the session context is filling with tool/MCP output.
---

# Context Discipline Skill

On-demand procedures for context recovery and long-session hygiene. Load this
skill only when needed — after a compaction, or when context pressure builds.

## Compaction Recovery Procedure

When context is compacted or truncated mid-task, re-anchor before acting:

1. **Session state** — read `AGENTS.md` or `CLAUDE.md` for recorded decisions;
   in solo sessions, look for `=== PHASE: <name> ===` markers in the visible
   context to find your position.
2. **Recent history** — run `git log --oneline -10` to see the latest commits
   and infer phase progress from their prefixes (`test:` = red, `feat:` =
   green, `refactor:`, `docs: add spec` = spec phase, etc.).
3. **Ground truth** — run the project test command; the pass/fail state is a
   reliable phase indicator.
4. **Artifacts** — check `specs/` for written specification documents and read
   the most recently modified files to see what was last changed.
5. **Resume, don't restart** — continue from the last completed phase. Never
   restart a pipeline from scratch after a compaction.
6. **Confirm when unsure** — tell the user: "Context was compacted. Last
   commit was X. Shall I continue with [phase]?"

## Summarizing Tool and MCP Output

Tool results and MCP responses (Context7 documentation, search results, API
lookups) are the fastest way to fill a context window. Rules:

- Extract the relevant findings from any tool output longer than ~30 lines;
  never leave a full dump in the transcript.
- After an MCP documentation lookup, keep only the signatures, parameters,
  and constraints you need — drop prose, examples you won't use, and
  unrelated sections.
- Record what you learned in 3-5 lines in your working notes, then reference
  the note instead of re-querying.
- When delegating, pass file paths and one-line conclusions, not the raw
  output you collected.

## Working-Notes Protocol for Long Sessions

- Keep a running checklist of pipeline phases; tick them off as they finish.
- After each phase or sub-agent result, write a 3-5 line summary and drop the
  detail from active use.
- Write large intermediate products (specs, plans, analyses) to disk and
  reference them by path — the file system is your external memory.
- Prefer re-reading a small file over keeping its content in context "just in
  case".