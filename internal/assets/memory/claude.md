## Memory Protocol

This protocol defines how you manage persistent context across sessions using CLAUDE.md
as the primary memory store.

### Session Start

1. Review `CLAUDE.md` for project context, prior decisions, and accumulated conventions.
2. Check the most recent session summary for continuity with prior work.
3. Search for any notes relevant to the current task before beginning.
4. If `.agent-manager/project.json` exists, review it for project configuration.

### Save Triggers

Update CLAUDE.md after any of these events:

- **Architecture decisions** — record the decision, alternatives considered, and rationale.
- **Bug discoveries and fixes** — document the root cause, symptoms, and the fix applied.
- **New conventions or patterns** — note the pattern, where it applies, and examples.
- **Configuration changes** — record what changed, why, and any migration steps.
- **Dependency additions or removals** — note the package, version, and reason.
- **Environment setup notes** — document any local setup steps or environment requirements.
- **Project-specific constraints** — record limitations, quirks, or known issues.

### Search Protocol

- At session start, search CLAUDE.md for context relevant to the current task.
- Before making architectural decisions, check for prior decisions on the same topic.
- Before modifying shared infrastructure, check for documented conventions.
- When encountering unfamiliar code, search for related notes or explanations.

### Session End

Append a session summary to the relevant section of CLAUDE.md including:

- **Goal**: what was the objective for this session
- **Accomplished**: what was completed
- **Decisions**: any architectural or design decisions made
- **Discoveries**: what was learned about the codebase or problem domain
- **Next Steps**: what remains to be done

Keep summaries concise (5-10 lines). Remove stale summaries when they are no longer relevant.
