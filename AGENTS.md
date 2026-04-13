<!-- agent-manager:memory -->
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
<!-- /agent-manager:memory -->

---

### Session: V2-Enhanced Agent Configs (Complete)

- **Goal**: Rewrite all 9 OpenCode agent/subagent configs (`.opencode/agents/`) from scratch — the originals were for a React/Strato project, not this Go CLI tool — and embed deep V2 roadmap knowledge so agents can implement V2 features.
- **Accomplished**: All 9 configs rewritten with V2 knowledge:
  - `orchestrator.md` — V2-aware routing with session-commit tracking and agent capability matrix
  - `planner.md` — V2 session-commit map, test trajectory, adapter capability matrix
  - `researcher.md` — full V2 reference: MCP strategies, methodology teams, plugin catalog, delegation semantics
  - `implementer.md` — V2 implementation patterns: delegation branching, template rendering, new adapters
  - `reviewer.md` — V2 review criteria: delegation correctness, methodology exclusions, MCP validation
  - `tester.md` — V2 test expectations: per-commit test counts, smoke scenarios 6-10
  - `checker.md` — V2 impact analysis: 5-adapter matrix, go:embed completeness
  - `writer.md` — V2 asset authoring: 9 orchestrator templates, 15 sub-agent defs, template variable system
  - `specialist-adapters.md` — full 5-adapter reference: 18-method interface, paths, detection, MCP strategies, delegation branching
- **Decisions**: writer.md was already V2-complete from prior session; specialist-adapters.md needed full rewrite (was V1-only with 13-method interface)
- **Discoveries**: Original configs referenced React, Vanilla Extract, Jest, Playwright — completely wrong tech stack
- **Next Steps**: Begin V2 implementation starting with Commit 2 (3 new adapter packages). All agent configs are now ready to support the full V2 roadmap.
