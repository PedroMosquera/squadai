## Memory Protocol

You have access to persistent memory tools. Follow these rules to maintain useful
context across sessions.

### Session Start

1. Search memory for context relevant to the current task before beginning work.
2. Review any prior session summaries for continuity.
3. Check project documentation for recent decisions and conventions.
4. If configuration files exist, review them for project-specific settings.

### Save Triggers

Save context after any of these events:

- **Architecture decisions** — record the decision, alternatives considered, and rationale.
- **Bug discoveries and fixes** — document the root cause, symptoms, and the fix applied.
- **New conventions or patterns** — note the pattern, where it applies, and examples.
- **Configuration changes** — record what changed, why, and any migration steps.
- **Dependency additions or removals** — note the package, version, and reason.
- **Performance findings** — document measurements, bottlenecks, and optimizations.
- **Security considerations** — record any security-relevant decisions or constraints.

### Search Protocol

- At session start, search memory for relevant context.
- Before making architectural decisions, check for prior decisions on the same topic.
- Before modifying shared infrastructure, check for documented conventions.
- Use keyword search for specific topics (e.g., "error handling", "testing", "deployment").

### Session End

At the end of each session, save a summary including:

- **Goal**: what was the objective
- **Accomplished**: what was completed
- **Decisions**: any architectural or design decisions made
- **Discoveries**: what was learned
- **Next Steps**: what remains to be done

Keep summaries concise (5-10 lines). Remove stale context when it is no longer relevant.
