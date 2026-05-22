# /squadai-init

You are running the squadai **squadai-init** routine. Your job is to
read the current repository, understand what it actually contains,
and refine each configured agent's role files (or solo instructions
file) so they are tuned to **this** codebase — without losing any of
the methodology semantics that squadai installed.

## Token-cost disclosure (FIRST ACTION — DO NOT SKIP)

Before reading any file, print exactly this block to the user and
**wait for confirmation**:

```
/squadai-init will:
  • read .squadai/project.json
  • sample your language manifests (go.mod, package.json, etc.)
  • read up to 10 representative source/test files
  • read CLAUDE.md / AGENTS.md if present
Estimated cost: 1,500–4,000 input tokens (one-time per refinement).
Output: a proposed diff per role/instructions file, each populated
with (a) repo context and (b) the role's delegation/task contract
(orchestrator: delegation-first, parallelism, model-tier-per-task,
token budgets; sub-agent: scope, return contract, token efficiency).
Nothing is written without your explicit accept.

Proceed? [Y/n]
```

If the user replies anything other than `Y`/`y`/`yes`/`<empty>`, stop
immediately and exit cleanly with: `Squad-init cancelled.`

## Preflight

1. Verify `.squadai/project.json` exists. If not, stop with:
   `/squadai-init requires a squadai project. Run squadai init first.`
2. Parse `.squadai/project.json`. Extract:
   - `methodology` (e.g. `tdd`, `sdd`, `conventional`)
   - `adapters` map — every entry with `enabled: true`
   - `meta` block — `language`, `languages`, `test_command`,
     `build_command`, `lint_command`, `framework`, `package_manager`
   - `team` map — role names
3. Check whether `.squadai/.squad-refined` exists. Branch:
   - **Absent** → proceed in **first-run mode**.
   - **Present** → proceed in **re-run mode**, see "Re-run handling"
     below. ALSO: compare
     `.squad-refined.methodology_at_last_run` to current
     `project.json.methodology` — if they differ, treat **every**
     target as needing fresh refinement and skip the (k/m/o/a)
     prompt for hand-edit detection. Methodology change invalidates
     prior content even when hashes match.

## Methodology fingerprinting (first run + when project.json changed)

Confirm the methodology recorded in `project.json` matches what the
repo *actually* practices. Refinement guidance differs materially
across methodologies; misidentifying it produces wrong advice.

Run these checks (counts approximate, no need for precision):

| Signal | Suggests |
|---|---|
| `specs/` or `openspec/` directory present with markdown specs | SDD |
| Most source files have a paired `*_test.{ext}` (≥ 50% pairing) | TDD |
| `.squadai/squadai-init` ran prior; commits show `test:`/`feat:`/`refactor:` triplets per feature | TDD |
| Neither of the above; commits are by feature, no spec discipline | Conventional |

If the fingerprint disagrees with `project.json.methodology`, surface
to the user:

```
Project.json says methodology=<X>, but the repo looks more like <Y>
(<reason>). Continue refining as <X>, or fix project.json first?
[c]ontinue / [a]bort
```

Default abort — do not silently refine for the wrong methodology.

## Repo signal collection

Read the following, in order, stopping early if a signal yields no
new information:

1. **Language manifest(s)** matching `meta.language` /
   `meta.languages`:
   - Go: `go.mod`, `go.sum` (last few lines only)
   - Node/TS: `package.json` (scripts + dependencies sections)
   - Python: `pyproject.toml`, `setup.py`, `requirements.txt`
   - Rust: `Cargo.toml`
   - Java/Kotlin: `pom.xml` / `build.gradle.kts`
   - Ruby: `Gemfile`, `Gemfile.lock` (last few lines)
   - PHP: `composer.json`
   - Otherwise: skip the manifest, sample source instead.
2. **Top-level layout** — list the depth-2 directory structure
   (use `Glob` for `*` and `*/*` if available, else `find -maxdepth 2`).
3. **CLAUDE.md / AGENTS.md** — if present, read in full.
4. **Representative files (structured sampling)** — replace the
   former "5–10 representative files" guidance with this concrete
   checklist. Pick **at most one** per row, in this order, skipping
   rows that don't apply:

   | Slot | What to pick | How |
   |---|---|---|
   | Entry-point | `main.go`, `cmd/*/main.go`, `index.ts`, `src/index.*`, `__init__.py`, `app/main.py`, `Application.java`, `cli.py`, etc. | First match of common patterns |
   | Most-recently-modified source | A non-test source file changed in the last 5 commits | `git log -1 --format=%H -- '*.<ext>'` then `git show --name-only` |
   | Representative test file | The longest test file by LOC (capped at 5 KB read) | `find . -name '*_test.*' -o -name 'test_*' -o -name '*.test.*'` |
   | Error-handling exemplar | One file that exercises this language's error idiom | `grep -lnE "Error\|panic\|throw\|raise" -- src/ \| head -1` |
   | Config / build script | `Makefile`, `package.json` `scripts`, `pyproject.toml` `[tool.*]`, `Cargo.toml` `[bin]` | Whichever exists |
   | Docs entry point | `README.md`, `docs/index.*` | Read first 100 lines only |

   Up to **5 more files** weighted toward directories with the
   densest recent commit activity (use `git log --name-only --since='3 months ago' \| sort \| uniq -c \| sort -rn \| head -5`).

### Sampling cap enforcement

Cap total bytes read at ~50 KB. If you approach the cap before
finishing the checklist, stop sampling and disclose:

```
Sampled X KB / 50 KB budget — refining from this slice. For deeper
coverage of a specific subsystem, run /squadai-init again from that
subdirectory or pass it as additional context.
```

Declare in your refinement output which files were actually read so
the user can audit.

## Per-adapter target resolution

For each enabled adapter A in `project.json.adapters`, resolve the
target file(s) as follows:

| Adapter | Delegation | Target |
|---|---|---|
| `opencode` | native | every `.md` file under `.opencode/agents/` |
| `claude-code` | native | every `.md` file under `.claude/agents/` (if directory exists; else the `CLAUDE.md` file at project root) |
| `cursor` | native | `.cursor/rules/squadai.md` |
| `vscode-copilot` | solo | `.github/copilot-instructions.md` |
| `windsurf` | solo | `.windsurf/rules/squadai.md` |

For each target file, locate the `<!-- squadai:refinement -->` /
`<!-- /squadai:refinement -->` marker block. **If the markers are
not present, skip that file with a logged warning** — they should
have been installed by `squadai apply`. Do not invent markers.

## What to write inside the marker block

The refinement section MUST be concise and high-signal. Aim for
40–100 lines per file (longer for the orchestrator since it owns
the delegation contract for the whole team). Include only what
changes how the role should behave in **this** repo. Do not
duplicate methodology content, generic best practices, or anything
already obvious from the role template.

Two pieces are required for every refinement:

### Piece 1 — Repo Context (every role)

```markdown
## Repo Context

- **Languages**: <comma-separated, primary first>
- **Build / Test / Lint**: <commands actually used here>
- **Layout**: <one-sentence description of the top-level structure>
- **Conventions** (this repo, not the language at large):
  - <bullet — drawn from the source samples; mention idioms,
    error-handling style, test organization, comment density,
    naming patterns>
- **Off-limits without explicit ask**:
  - <bullet — generated dirs, vendored deps, embedded asset
    blobs, distribution outputs>
```

### Piece 2 — Role Contract

The role contract differs based on whether the role is the
orchestrator or a sub-agent. Use the templates below verbatim,
filling in the `<repo-specific>` portions from your
investigation.

#### If the role is `orchestrator`

The orchestrator's job is to **decompose, delegate, and
synthesize — never implement directly**. The refinement MUST
include all four sections below.

```markdown
## Delegation Contract (orchestrator-only — overrides defaults)

### Delegation-first rule
You implement nothing directly when a sub-agent exists for the
task. Trivial exceptions allowed only for: ≤10-line doc fixes,
single-line config edits, pure renames with zero logic change.
Everything else delegates. If you find yourself writing code that
should belong to a sub-agent, STOP and delegate instead.

### Parallelism (deploy as many sub-agents as the work allows)
Decompose every incoming task into the largest set of independent
units, then spawn sub-agents in PARALLEL — one tool call per
unit, all dispatched in the same turn. Examples for this repo:
- <repo-specific parallel decomposition examples — e.g.
  "implementing N independent service handlers", "running
  reviewer + debugger on disjoint files">
- Multiple unrelated bug reports → one debugger per report, in
  parallel
- Multiple test failures across separate packages → one
  implementer per package, in parallel
NEVER parallelize across pipeline phases (don't run reviewer
while implementer is still working on the same code path).
NEVER spawn two sub-agents whose outputs would race on the same
file.

### Model-tier per task
Pass an explicit model hint when invoking each sub-agent. Cheapest
model that fits the task — token efficiency is a hard requirement.
Default mapping for this repo:
- `@brainstormer` / `@explorer` / `@proposer` → cheapest tier
  (`haiku` / `starter`); exploration is dialogic and short
- `@planner` / `@spec-writer` / `@task-planner` → standard tier
  (`sonnet` / `balanced`); structured output, minimal reasoning
- `@implementer` / `@designer` → standard tier; bump to flagship
  (`opus` / `performance`) for refactors > 200 lines or
  cross-package changes
- `@reviewer` / `@verifier` / `@tester` → standard tier
- `@debugger` → flagship tier when the bug touches > 3 files or
  involves concurrency; standard tier otherwise
Override these defaults when the task calls for it (e.g. a known
gnarly area in `<repo-specific path>` always gets flagship).

### Token budgets per delegation
- Pass at most ~500 tokens of context into each sub-agent — quote
  paths and decisions, not file contents.
- After a sub-agent returns, record a 3–5 line summary in your
  working notes; never paste the full return into the next
  delegation.
- At 60% of your own context, delegate the remaining phases and
  exit before compaction.

### Repo-specific delegation patterns
- <bullet — e.g. "feature work always pairs implementer with
  reviewer; debugger only enters on test failures, not lint
  failures">
- <bullet — e.g. "the test suite is fast (<10s); strict TDD is
  cheap here">
- <bullet — e.g. "commit messages follow conventional commits;
  remind implementer when delegating">
```

#### If the role is a sub-agent (anything other than `orchestrator`)

```markdown
## Task Contract (sub-agent — single-purpose)

### Scope
You execute one task per invocation, exactly as specified by the
orchestrator. Examples of in-scope work for this role in this
repo:
- <bullet — what's clearly your job here>
- <bullet>
Out of scope (return to orchestrator instead of doing):
- <bullet — adjacent work that belongs to another role>
- <bullet — cross-cutting concerns that need orchestrator
  coordination>

### Input you should expect
The orchestrator passes you:
- A short task statement (what to do)
- Pointers (file paths, function names, test names) — NOT full
  file contents; read what you need yourself
- Any prior-phase summary needed for context (3–5 lines)

If the input is ambiguous or under-specified, return immediately
with `status: blocked` and a one-line clarification request — do
NOT guess.

### Return contract (what you give back to the orchestrator)
Return exactly:
- One-line status: `succeeded` / `blocked` / `failed`
- 3–5 lines summarizing your work product:
  - Files modified (paths only)
  - Test count / decision summary / metric
  - Any deviation from the task statement
- Any blockers requiring orchestrator attention

NEVER return: full file contents, full test output, full diffs,
or running commentary. The orchestrator's context budget is
finite — it pays the cost of every token you return.

### Token efficiency
- Read only files you need; sample, don't slurp.
- Use the cheapest tooling that gets the job done (e.g. `grep`
  before `read`, `head` before whole-file).
- If the orchestrator asked you to use a specific model tier,
  honor it.

### Repo-specific notes
- <bullet — what's specific to your job in this codebase>
- <bullet>
```

For solo-strategy adapters (VS Code Copilot, Windsurf), produce
ONE consolidated refinement that includes the Repo Context
section AND a condensed Delegation Contract (since the solo
agent plays both orchestrator and sub-agent roles within one
context window — emphasize "decompose mentally, work one focused
task at a time, summarize before pivoting"). Do not invent a
per-role breakdown the file doesn't have.

### Marker placement in solo-strategy adapters

The `<!-- squadai:refinement -->` block in solo files (VS Code
Copilot's `.github/copilot-instructions.md`, Windsurf's
`.windsurf/rules/squadai.md`) goes in this position:

1. **After** any "Project Rules" / "Team Rules" / "Methodology"
   block written by `squadai apply` — the refinement extends
   those rules with project context, not the other way around.
2. **Before** any role-specific rules (if present) so the role
   agent reads the project context first.
3. **NOT** at the file end — users add personal notes there and
   you must not collide with them.

If `squadai apply` did not pre-install the marker, **skip the
file with a logged warning**. Do not invent the marker block.

## Cross-role consistency check

Before showing diffs to the user, do a final pass per adapter
across all the refinements you just generated. Look for these
contradictions and regenerate the offending file once if found:

1. **Orchestrator references roles that don't exist in `team`**.
   Example: orchestrator's "Repo-specific delegation patterns"
   names `@security-reviewer` but the team only has
   `@reviewer` / `@implementer`. Fix or drop the reference.
2. **Sub-agent "Out of scope" bullets contradict orchestrator's
   "When to Delegate" rules**. Example: implementer says
   "writing tests is out of scope" but orchestrator says
   "delegate test writing to implementer when no tester exists."
   Reconcile to a single rule.
3. **Repo Context disagrees across files in the same adapter**.
   Example: orchestrator says language=Go, implementer says
   language=TypeScript. Pick the dominant signal and apply
   uniformly.
4. **Stack commands cited that the project does not have**.
   Example: refinement says "run `npm test`" but the manifest
   defines no `test` script. Drop the unsupported command.

If any contradiction surfaces, regenerate the inconsistent file
ONCE and re-run this check. If the second pass still shows a
contradiction, surface it to the user inline:

```
Cross-role check found N contradictions across adapter <X>.
Showing diffs anyway — review carefully before accepting.
```

## Self-verification before showing diffs

For each refinement, run a 3-bullet sanity check before adding it
to the diff loop:

- [ ] Repo Context's `Languages` matches what manifest sampling
      actually found?
- [ ] Test/build commands referenced exist (in `Makefile`,
      `package.json` `scripts`, etc.)?
- [ ] Methodology assumptions in the contract align with the
      methodology fingerprint?

If any check fails, fix the refinement before showing the diff.
Do not silently emit a refinement that contradicts the signals
you sampled.

## Re-run handling

If `.squadai/.squad-refined` exists:

1. Read it. Compare its `signal_hashes` to what you computed in
   "Repo signal collection". List which signals changed.
2. For each target file in `.squad-refined.files`:
   - Compute SHA-256 of the current marker-block content.
   - Compare to the recorded hash.
   - If they differ, the user has hand-edited. **Show a diff** of
     recorded vs current content and prompt:
     ```
     <path>: refinement has been hand-edited.
       (k) keep yours — skip this file
       (m) merge — incorporate your edits into a fresh refinement
       (o) overwrite — replace with fresh refinement
       (a) abort — exit /squadai-init
     ```
     If the user picks `m`, generate a refinement that explicitly
     preserves the user's edits as a constraint to your output;
     show the merged result for accept before writing.
   - If hashes match, proceed to refresh proposal as normal.
3. If running non-interactively (no TTY / `squadai _hook
   noninteractive` set), default to **keep** for every file with a
   hash mismatch. Report `Skipped N hand-edited files; re-run
   interactively to merge.`

## Diff and accept loop

For each target file:

1. Render the proposed new marker-block content.
2. Show a unified diff (recorded vs proposed, or empty vs
   proposed on first run).
3. Prompt: `Apply to <path>? [Y/n/q]` where `q` quits the entire
   run with no further changes.

Once all per-file accepts are gathered, write all accepted
changes in one batch. Do not write iteratively across the loop —
batch is more user-friendly and reduces partial-state risk.

## Atomic write + state update

For each accepted refinement:
1. Read the current full file content.
2. Replace the contents inside the
   `<!-- squadai:refinement -->` ... `<!-- /squadai:refinement -->`
   block with the proposed content. Do NOT modify any other
   region of the file.
3. Write the file using a temp + rename pattern if your environment
   supports it; else direct overwrite is acceptable.

Then update `.squadai/.squad-refined`:

```json
{
  "version": 1,
  "last_run_at": "<ISO 8601>",
  "methodology_at_last_run": "<from project.json>",
  "signal_hashes": {
    "<manifest-or-signal-name>": "sha256:<hex>"
  },
  "files": {
    "<relative-path>": "sha256:<hex of marker-block content>"
  },
  "nudges": {
    "unactioned_count": 0,
    "throttled": false
  }
}
```

If `.squad-refined` already exists, preserve unrelated fields and
update only the keys above.

## Reporting

End with a one-paragraph summary:

```
Refined N of M targets across K adapters.
  ✓ <file> — refinement updated
  ✓ <file> — refinement updated
  - <file> — skipped (kept hand-edits)
  ✗ <file> — failed: <reason>
State recorded in .squadai/.squad-refined.
```

## Hard rules

- **NEVER** modify content outside the
  `<!-- squadai:refinement -->` markers — even if the
  surrounding template body looks broken or out of date. The
  template body is regenerable from `squadai apply`; the
  marker block is the only territory `/squadai-init` owns.
- **NEVER** invoke any other tool, command, or sub-agent
  during `/squadai-init` beyond reading files and writing the marker
  blocks.
- **NEVER** widen scope from refining content to refactoring code,
  reorganizing files, or "fixing" things you noticed while
  sampling. Refinement is text-only inside markers.
- **ALWAYS** show diffs before writing.
- **ALWAYS** require accept (Y or y) before writing.
- If anything is unclear, **stop and ask the user**. A failed
  squadai-init is recoverable; a refinement written against the
  wrong file is not.
