# SquadAI roadmap (October 2026)

Status as of 2026-10-09, main at `03b720d`. Supersedes the uncommitted September relevance draft.

## 1. Thesis, revised

The September draft framed SquadAI as "the portable operating layer across harnesses". Two things changed since then:

- Instruction and hook sync is now a commodity. rulesync went from v16 to v29 between 2026-09-17 and 2026-10-08, added about 15 targets, and covers Codex permissions, Codex skills, nested AGENTS.md, Pi MCP, Pi stop hooks and Claude model-switch hooks.
- Cross-harness usage is solved. ccusage now reports Codex, OpenCode, Amp, Droid, pi-agent, Copilot CLI and Gemini CLI.

What nobody else does well is **prove** that a repo's agent setup is correct, safe and within budget, every day, in CI. That is the wedge:

> SquadAI is the agent config you can verify. Declare it once, apply it to every harness, and fail CI when it drifts, leaks, or overspends.

Everything below is ranked by how much it moves that sentence.

## 2. What shipped since September

| Change | Commit |
|---|---|
| Stale cleanup no longer deletes AGENTS.md shared with an enabled adapter | `a25c353` |
| Post-apply install-hooks nudge, memory slash commands, Pi round-trip test | `c5b8e56`..`7270618` |
| `token-usage --against-budget` (exit 9 under `block`), working `--watch` | `9fe2bd5`..`c11a8ea` |
| Docs synced to seven adapters, with a consistency test that can fail | `4ad2536`, `11cbcec` |
| Verify warns when AGENTS.md exceeds Codex `project_doc_max_bytes` (32 KiB) | `4a1a1f9` |
| Model catalog refreshed: Claude 5.5 family, Fable 5.1, GPT-6 / 6.1, Gemini 3.8 Flash, corrected Opus 4.5 to 4.7 prices | `3345d2c` |
| VS Code MCP moved to root `.mcp.json`, Pi MCP to `.pi/mcp.json` (the old Pi target was never read by Pi), with migration that keeps user servers | `03b720d` |

## 3. Status of the September items

| Item | Status | Note |
|---|---|---|
| Codex adapter | Partial | AGENTS.md, global `~/.codex/config.toml` MCP, size check. No skills, hooks or subagents. |
| AGENTS.md canonical | Not started | Claude gets full content in CLAUDE.md. VS Code gets a file it does not read (see 4.1). |
| Skills in one place | Partial | SKILL.md format, but written per harness. Nothing in `.agents/skills`; Codex gets none. |
| Harness-native plugin install | Partial | Claude plugins are declared in `enabledPlugins`. Registry still pinned to `wshobson/agents`. |
| Cross-harness usage | Partial, now deprioritized | Reads Claude, OpenCode, Pi. ccusage covers the rest. |
| Budget enforcement | CLI only | `--against-budget`. No hooks. |
| Verify in CI | Partial and broken | `verify --strict` exists. No reusable action, no `--ci`, no allow-lists. The repo's own workflow is broken (4.2). |
| Config security scan | Not started | |
| Memory over MCP | Done | `squadai mcp-server` exposes `memory_search` and `memory_add`, auto-registered in every agent. No capture hook. |
| Session handoff | Not started | `context --format prompt` dumps config, not session state. |
| Runtime profile switch | Done at config time | `squadai profile`, `apply --profile`. Include/exclude globs not enforced. |
| Context providers | Partial | MCP catalog has context7, github, sentry. No providers component. |
| Attention events | Partial | `watch` streams governance events, not agent state. |
| Removal list | Only `enforcement=warn` done | Banner still on by default, `explain` still exists, per-role files still generated. |

## 4. Phase 0: correctness (do first)

Bugs found during the audit. Each is small, each breaks a user today, and the wedge is "config you can verify", so shipping known-wrong config undercuts it.

1. **VS Code instructions are never read.** The adapter writes `<project>/.instructions.md`. VS Code reads `.github/copilot-instructions.md`, `.github/instructions/**/*.instructions.md`, and root `AGENTS.md`, and documents no root `.instructions.md`. Fix by targeting AGENTS.md (Phase 1) or, as a stopgap, `.github/copilot-instructions.md`.
2. **`squadai-check.yml` cannot pass.** It runs `squadai diff --exit-code`, which errors with `unknown flag "--exit-code"`, and `doctor --strict`, which has no strict flag. Either add the flags or fix the workflow; then reuse the result as the public action in Phase 2.
3. **MCP ownership is per key, not per server.** SquadAI claims the whole `mcpServers` key in `.mcp.json`, so the conflict prompt's "overwrite" replaces servers the user added. Since `03b720d` this covers VS Code and Pi as well as Claude, Cursor and Windsurf. Track ownership per server name.
4. **Disabling an adapter deletes whole user files.** A disabled Codex removes the user's entire `~/.codex/config.toml` and `~/.codex/AGENTS.md`. Strip SquadAI's marker blocks instead, and delete only files SquadAI created.
5. **Dead code and dead flags.** `assets/hooks/opencode-nudge.ts` is embedded but never installed. Help lists `watch --daemon`, which is not parsed. `pricing.json` price rows duplicate `models.json` and are no longer read for pricing. Delete them.

Size: 4 agent sessions in parallel, about one day of wall-clock including review. Items 3 and 4 touch user data (durability hazard): each needs a test that seeds user-owned content and proves it survives.

## 5. Phase 1: one instruction file, one skills directory

1. **AGENTS.md is the canonical instruction file.** `apply` writes all shared content to AGENTS.md. CLAUDE.md becomes `@AGENTS.md` plus Claude-only blocks. VS Code, Cursor, Windsurf, Pi, OpenCode and Codex read AGENTS.md natively. Claude Code also reads AGENTS.md when no CLAUDE.md exists (2.1.277), so the import keeps working for users who keep their own CLAUDE.md. This fixes 4.1 properly and removes one copy of every instruction block. Keep the 32 KiB check in mind: one file now carries everything.
2. **Skills in `.agents/skills`.** Write SKILL.md once to `.agents/skills` (Agent Skills standard, read by Codex and Pi), then link or copy into harness-specific directories only where a harness does not read it. Codex gains skills.
3. **Shrink role files after skills land.** Today: tdd 6, sdd 8, conventional 4 role files per harness for Claude, Cursor, OpenCode and Pi. With roles as skills, generate one orchestrator stub per harness.

Size: 2 to 3 sessions, about one day. Bottleneck: a live check in Claude Code, VS Code and Codex that each actually loads the new files.

## 6. Phase 2: the differentiators

Ranked by fit with the thesis.

1. **Verify in CI as a product.** A reusable GitHub Action (`action.yml`) that runs `squadai verify --ci`: machine-readable output, GitHub annotations, non-zero on drift, missing components or policy violations. Policy gains allow-lists for MCP servers, models, hook commands and plugin sources. Where a harness has a native control, emit it: Claude's managed `deniedModels`, `availableModelsMatch` and `allowedProviders` (2026-09-25) map directly onto the model allow-list, and `maxEffortLevel` (2026-09-09) onto a budget profile. 2 sessions.
2. **`squadai scan`.** Inspect hooks, MCP configs, skills and plugins for shell injection, secrets in env blocks, remote code fetched at runtime, and untrusted sources. Gate it in the same action. Flag project settings Claude now ignores, such as `defaultMode: "bypassPermissions"` (ignored since 2026-09-02), so users stop relying on them. 2 sessions.
3. **Budgets enforced through hooks.** The CLI enforcement exists; move it into the session. Claude: `PreToolUse` and `Stop`, plus `PreModelSwitch` (2026-08-28) to refuse a switch above the budget tier, and `onFailure: "block"` (2026-10-08) so a broken hook fails closed. Codex: hooks, including `Interrupt` (2026-08-26). Pi: an extension on `agent_before_settle` (0.87.0). Read usage from ccusage's JSON output when installed instead of writing new log readers. 1 session per harness, in parallel, plus live verification in each.
4. **Memory capture at session end.** The MCP tools exist. Add a per-harness end-of-session hook that calls `squadai memory add` with a short summary, landing in `docs/memory/_inbox`. 1 session.

Size: 6 to 8 sessions, 3 to 4 days of wall-clock. Bottleneck: hook behavior has to be verified in live sessions of each harness, which an agent cannot fully do headless.

## 7. Removals

Each removal is its own small change. Do them alongside Phase 0.

| Item | Decision |
|---|---|
| Banner and `brand` component | Default off in every preset. Keep the flag. |
| `explain` | Remove. `--help` and docs cover it. |
| `install-commands` | Fold into `apply` as the commands component for Claude (today Claude's `ProjectCommandsDir` is empty, which is why the separate command exists). Done: the commands component writes the slash commands, the agents component writes `squadai-manager`, and the command is removed. |
| `plugins sync` from `wshobson/agents` | Replace with declarations in project.json that delegate to each harness (`enabledPlugins` for Claude, marketplace installs for Codex and Pi). |

## 8. Dropped or deferred

| Item | Why |
|---|---|
| New usage readers for Codex and others | ccusage covers them. Integrate, do not rebuild. |
| Session handoff (`handoff` / `resume`) | Valuable but not part of the thesis. Revisit after Phase 2. |
| Context providers component | The MCP catalog already covers the useful part. |
| Agent attention events | Not part of the thesis. |
| Project-scoped Codex config, Codex subagent delegation | The September Codex rework assumed these. Main's Codex adapter is deliberately global and solo. Reopen only with a design note. |

## 9. Risks

- **Harness formats keep moving.** Two of seven adapters (VS Code rules, Pi MCP) were silently writing files their harness never read. Mitigation: a weekly changelog check, and round-trip tests per adapter against each harness's documented paths, with the doc URL in the test.
- **rulesync reaches policy and CI.** Mitigation: ship the action and scan first; consider emitting rulesync-compatible input rather than racing it on targets.
- **Review is the bottleneck.** Agents produce faster than review. Mitigation: one change per numbered item, every test shown red before green.

## 10. Decisions needed

1. Phase 0 before anything else. Recommended: yes.
2. AGENTS.md canonical (5.1) changes what lands in users' CLAUDE.md. It is a contract change for existing installs: ship it as a migration with a clear note, not silently.
3. Whether the public positioning moves from "one control plane" to "agent config you can verify". This drives README and the action's marketing, not code.

## Sources

- Claude Code changelog: https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md
- Codex releases: https://github.com/openai/codex/releases ; AGENTS.md cap in `codex-rs/config/defaults.toml`
- Pi MCP: https://github.com/earendil-works/pi (packages/coding-agent/docs/mcp.md)
- VS Code MCP: https://code.visualstudio.com/docs/copilot/customization/mcp-servers
- VS Code instructions: https://code.visualstudio.com/docs/copilot/customization/custom-instructions
- Anthropic pricing: https://platform.claude.com/docs/en/about-claude/pricing
- OpenAI pricing: https://developers.openai.com/api/docs/pricing
- rulesync: https://github.com/dyoshikawa/rulesync ; ccusage: https://github.com/ryoppippi/ccusage
