## Memory Protocol

This protocol defines how you manage persistent context across sessions using AGENTS.md
and the `.agent-manager/` configuration directory.

### Session Start

1. Read the project's `AGENTS.md` for accumulated context, prior decisions, and conventions.
2. Check `.agent-manager/project.json` for project configuration, enabled components, and team policy.
3. Search existing memory for any notes relevant to the current task before beginning work.
4. Review the most recent session summary (if present) at the end of AGENTS.md.

### Save Triggers

Save important context to AGENTS.md after any of these events:

- **Architecture decisions** — record the decision, alternatives considered, and rationale.
- **Bug discoveries and fixes** — document the root cause, symptoms, and the fix applied.
- **New conventions or patterns** — note the pattern, where it applies, and examples.
- **Configuration changes** — record what changed, why, and any migration steps.
- **Dependency additions or removals** — note the package, version, and reason.
- **Performance findings** — document measurements, bottlenecks, and optimizations.
- **Security considerations** — record any security-relevant decisions or constraints.

### Search Protocol

- At session start, search AGENTS.md for context relevant to the current task.
- Before making architectural decisions, check for prior decisions on the same topic.
- Before modifying shared infrastructure, check for documented conventions.
- Use keyword search for specific topics (e.g., "error handling", "testing", "deployment").

### Session End

At the end of each session, append a summary to AGENTS.md including:

- **Goal**: what was the objective for this session
- **Accomplished**: what was completed
- **Decisions**: any architectural or design decisions made
- **Discoveries**: what was learned about the codebase or problem domain
- **Next Steps**: what remains to be done

Keep summaries concise (5-10 lines). Remove stale summaries when they are no longer relevant.
