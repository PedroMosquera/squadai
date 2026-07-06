---
name: squadai-init-playbook
description: Detailed procedures for the /squadai-init refinement routine — signal collection, refinement templates, consistency checks, re-run handling, and write protocol. Loaded on demand by the /squadai-init driver command.
---

# /squadai-init Playbook

Detailed procedures for each `/squadai-init` phase. The driver command
defines the phase order and hard rules; this playbook holds the full
procedure per phase.

## Phase: Methodology fingerprinting (first run + when project.json changed)

Confirm the methodology in `project.json` matches what the repo actually
practices — misidentifying it produces wrong advice.

| Signal | Suggests |
|---|---|
| `specs/` or `openspec/` directory with markdown specs | SDD |
| Most source files have a paired `*_test.{ext}` (≥ 50% pairing) | TDD |
| Commits show `test:`/`feat:`/`refactor:` triplets per feature | TDD |
| Neither; commits are by feature, no spec discipline | Conventional |

If the fingerprint disagrees with `project.json.methodology`, surface:

```
Project.json says methodology=<X>, but the repo looks more like <Y>
(<reason>). Continue refining as <X>, or fix project.json first?
[c]ontinue / [a]bort
```

Default abort — never silently refine for the wrong methodology.

## Phase: Repo signal collection

Read in order, stopping early when a signal yields nothing new:

1. **Language manifest(s)** matching `meta.language`/`meta.languages` —
   Go: `go.mod` (+ `go.sum` tail); Node/TS: `package.json` (scripts +
   dependencies); Python: `pyproject.toml`/`setup.py`/`requirements.txt`;
   Rust: `Cargo.toml`; Java/Kotlin: `pom.xml`/`build.gradle.kts`; Ruby:
   `Gemfile` (+ lock tail); PHP: `composer.json`; otherwise skip and
   sample source instead.
2. **Top-level layout** — depth-2 directory structure (Glob `*` and `*/*`,
   else `find -maxdepth 2`).
3. **CLAUDE.md / AGENTS.md** — read in full if present.
4. **Representative files (structured sampling)** — pick at most one per
   row, in order, skipping rows that don't apply:

   | Slot | What to pick | How |
   |---|---|---|
   | Entry-point | `main.go`, `cmd/*/main.go`, `index.ts`, `src/index.*`, `__init__.py`, `app/main.py`, `Application.java`, `cli.py` | first match |
   | Most-recently-modified source | non-test source changed in last 5 commits | `git log` + `git show --name-only` |
   | Representative test file | longest test file by LOC (cap 5 KB read) | `find` by test-name patterns |
   | Error-handling exemplar | one file exercising the language's error idiom | grep for `Error\|panic\|throw\|raise` |
   | Config / build script | `Makefile`, `package.json` scripts, `pyproject.toml [tool.*]`, `Cargo.toml [bin]` | whichever exists |
   | Docs entry point | `README.md`, `docs/index.*` | first 100 lines only |

   Plus up to **5 more files** weighted toward directories with the densest
   recent commit activity (`git log --name-only --since='3 months ago' |
   sort | uniq -c | sort -rn | head -5`).

**Sampling cap**: ~50 KB total. If approaching the cap early, stop and
disclose: `Sampled X KB / 50 KB budget — refining from this slice. For
deeper coverage of a specific subsystem, run /squadai-init again from that
subdirectory.` Always declare which files were actually read.

## Phase: Per-adapter target resolution

| Adapter | Delegation | Target |
|---|---|---|
| `opencode` | native | every `.md` under `.opencode/agents/` |
| `claude-code` | native | every `.md` under `.claude/agents/` (else root `CLAUDE.md`) |
| `cursor` | native | `.cursor/rules/squadai.md` |
| `vscode-copilot` | solo | `.github/copilot-instructions.md` |
| `windsurf` | solo | `.windsurf/rules/squadai.md` |

Locate the `<!-- squadai:refinement -->` / `<!-- /squadai:refinement -->`
block in each target. **If markers are absent, skip the file with a logged
warning** — never invent markers.

## Phase: Refinement content

Aim for 40-100 lines per file (longer for the orchestrator). Include only
what changes how the role behaves in THIS repo — no methodology content, no
generic best practices. Two pieces are required:

### Piece 1 — Repo Context (every role)

```markdown
## Repo Context

- **Languages**: <comma-separated, primary first>
- **Build / Test / Lint**: <commands actually used here>
- **Layout**: <one-sentence top-level structure>
- **Conventions** (this repo, not the language at large):
  - <idioms, error-handling style, test organization, naming>
- **Off-limits without explicit ask**:
  - <generated dirs, vendored deps, asset blobs, dist outputs>
```

### Piece 2 — Role Contract (orchestrator)

```markdown
## Delegation Contract (orchestrator-only — overrides defaults)

### Delegation-first rule
You implement nothing directly when a sub-agent exists for the task.
Trivial exceptions: ≤10-line doc fixes, single-line config edits, pure
renames. Everything else delegates — if you're writing code that belongs
to a sub-agent, STOP and delegate.

### Parallelism (deploy as many sub-agents as the work allows)
Decompose into the largest set of independent units; spawn sub-agents in
PARALLEL — one tool call per unit, all in the same turn. Examples:
- <repo-specific parallel decomposition examples>
- Multiple unrelated bug reports → one debugger per report, in parallel
- Test failures across separate packages → one implementer per package
NEVER parallelize across pipeline phases; NEVER let two sub-agents race
on the same file.

### Model-tier per task
Pass an explicit model hint per delegation — cheapest tier that fits:
- brainstormer/explorer/proposer → cheapest (exploration is short)
- planner/spec-writer/task-planner → standard (structured output)
- implementer/designer → standard; flagship for refactors > 200 lines
  or cross-package changes
- reviewer/verifier/tester → standard
- debugger → flagship when > 3 files or concurrency; else standard
Override when the task calls for it (<repo-specific hot spots>).

### Token budgets per delegation
- Pass ≤ ~500 tokens of context per sub-agent — paths and decisions,
  not file contents.
- Record a 3-5 line summary per return; never paste full returns.
- At 60% of your own context, delegate remaining phases and exit.

### Repo-specific delegation patterns
- <e.g. "feature work pairs implementer with reviewer">
- <e.g. "test suite is fast (<10s); strict TDD is cheap here">
```

### Piece 2 — Role Contract (sub-agent)

```markdown
## Task Contract (sub-agent — single-purpose)

### Scope
One task per invocation, exactly as specified. In scope here:
- <what's clearly this role's job in this repo>
Out of scope (return to orchestrator instead):
- <adjacent work belonging to another role>

### Input you should expect
Short task statement + pointers (paths, function/test names — NOT full
contents; read what you need) + prior-phase summary (3-5 lines). If
ambiguous, return `status: blocked` with a one-line clarification
request — do NOT guess.

### Return contract
Exactly: one-line status (`succeeded`/`blocked`/`failed`) + 3-5 lines
(files modified as paths, test count / decision / metric, deviations) +
blockers. NEVER return full file contents, full test output, full
diffs, or running commentary — the orchestrator pays for every token.

### Token efficiency
Read only what you need (`grep` before `read`, `head` before
whole-file). Honor the orchestrator's model-tier hint.

### Repo-specific notes
- <what's specific to this role in this codebase>
```

### Solo-strategy adapters (VS Code Copilot, Windsurf)

Produce ONE consolidated refinement: Repo Context + a condensed Delegation
Contract (the solo agent plays both roles — emphasize "decompose mentally,
work one focused task at a time, summarize before pivoting"). Marker block
position: AFTER any Project/Team Rules or Methodology block, BEFORE any
role-specific rules, NOT at file end (users add personal notes there).

## Phase: Cross-role consistency check

Before showing diffs, check per adapter across all generated refinements;
regenerate the offending file once if found:

1. Orchestrator references roles that don't exist in `team`.
2. Sub-agent "Out of scope" contradicts orchestrator's delegation rules.
3. Repo Context disagrees across files (e.g. different languages).
4. Stack commands cited that the project does not define.

If a second pass still shows a contradiction: `Cross-role check found N
contradictions across adapter <X>. Showing diffs anyway — review carefully.`

## Phase: Self-verification before diffs

- [ ] Repo Context `Languages` matches manifest sampling?
- [ ] Test/build commands referenced actually exist?
- [ ] Contract assumptions align with the methodology fingerprint?

Fix failures before showing the diff — never emit a refinement that
contradicts the sampled signals.

## Phase: Re-run handling

If `.squadai/.squad-refined` exists:

1. Compare its `signal_hashes` to freshly computed ones; list changes.
2. Per target file: compute SHA-256 of the current marker-block content
   and compare to the recorded hash. On mismatch (hand-edited), show a
   diff and prompt: `(k) keep yours / (m) merge / (o) overwrite /
   (a) abort`. For `m`, preserve the user's edits as a constraint and show
   the merged result before writing.
3. Non-interactive runs default to **keep** for every mismatch; report
   `Skipped N hand-edited files; re-run interactively to merge.`
4. If `methodology_at_last_run` differs from current methodology, treat
   every target as needing fresh refinement and skip hand-edit prompts —
   methodology change invalidates prior content even when hashes match.

## Phase: Diff, accept, atomic write, state

Per target: render proposed marker-block content → show unified diff →
prompt `Apply to <path>? [Y/n/q]` (`q` quits with no further changes).
Batch all accepted writes at the end (no iterative writes). Replace ONLY
the content inside the refinement markers, temp+rename when supported.

Then update `.squadai/.squad-refined` (preserve unrelated fields):

```json
{
  "version": 1,
  "last_run_at": "<ISO 8601>",
  "methodology_at_last_run": "<from project.json>",
  "signal_hashes": { "<signal>": "sha256:<hex>" },
  "files": { "<relative-path>": "sha256:<hex of marker-block content>" },
  "nudges": { "unactioned_count": 0, "throttled": false }
}
```

## Phase: Reporting

```
Refined N of M targets across K adapters.
  ✓ <file> — refinement updated
  - <file> — skipped (kept hand-edits)
  ✗ <file> — failed: <reason>
State recorded in .squadai/.squad-refined.
```
