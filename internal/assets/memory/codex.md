## Memory Protocol

This protocol defines how you manage persistent context across sessions.
Codex operates with limited file access, so memory management is lightweight.

### Session Start

1. Check project documentation for context and prior decisions.
2. Review any README, CHANGELOG, or project-level documentation for recent changes.
3. If instructions are available, search for notes relevant to the current task.

### Save Triggers

Save context after any of these events:

- **Architecture decisions** — record the decision and rationale.
- **Bug discoveries and fixes** — document the root cause and the fix applied.
- **New conventions or patterns** — note the pattern and where it applies.
- **Important configuration changes** — record what changed and why.
- **Dependency changes** — note the package, version, and reason.

### Search Protocol

- Before starting work, review available documentation for relevant context.
- Before making architectural decisions, check for prior decisions on the same topic.

### Session End

Save a brief summary of significant changes made during this session:

- **Goal**: what was the objective
- **Accomplished**: what was completed
- **Next Steps**: what remains to be done

Keep summaries concise (3-5 lines).
