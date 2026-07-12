# /squadai-init

You are running the squadai **squadai-init** routine: read the current
repository, understand what it actually contains, and refine each
configured agent's role files (or solo instructions file) so they are tuned
to **this** codebase — without losing any methodology semantics squadai
installed.

**Playbook**: the detailed procedure for every phase lives in the
`squadai-init-playbook` skill — find `SKILL.md` under your skills
directory at `shared/squadai-init-playbook/` (glob for
`**/shared/squadai-init-playbook/SKILL.md` if unsure). Load it once after
the user confirms, then follow the phase procedures from it as you go.

## Token-cost disclosure (FIRST ACTION — DO NOT SKIP)

Before reading any file, print exactly this block and **wait for
confirmation**:

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

Any reply other than `Y`/`y`/`yes`/`<empty>` → stop immediately:
`Squad-init cancelled.`

## Preflight

1. `.squadai/project.json` must exist, else stop:
   `/squadai-init requires a squadai project. Run squadai init first.`
2. Parse it: `methodology`, enabled `adapters`, `meta` block (language(s),
   test/build/lint commands, framework, package manager), `team` roles.
3. `.squadai/.squad-refined` absent → **first-run mode**; present →
   **re-run mode** (playbook: "Re-run handling"). If
   `methodology_at_last_run` differs from the current methodology, every
   target needs fresh refinement — skip hand-edit prompts.

## Phases (procedures in the playbook)

1. **Methodology fingerprinting** — confirm `project.json.methodology`
   matches what the repo actually practices; on mismatch ask
   `[c]ontinue / [a]bort` (default abort).
2. **Repo signal collection** — manifests, depth-2 layout,
   CLAUDE.md/AGENTS.md, structured file sampling; ~50 KB read cap with
   disclosure; declare which files were read.
3. **Per-adapter target resolution** — resolve role/instruction files per
   adapter; only files with `<!-- squadai:refinement -->` markers are
   targets; absent markers → skip with a logged warning, never invent.
4. **Refinement content** — inside each marker block write (a) Repo
   Context and (b) the Role Contract (orchestrator vs sub-agent vs solo
   consolidated) using the playbook templates verbatim, filling
   repo-specific portions. 40-100 lines per file, high-signal only.
5. **Cross-role consistency check** + **self-verification** — reconcile
   contradictions across roles; verify languages/commands/methodology
   against sampled signals before showing diffs.
6. **Diff and accept loop** — unified diff per target,
   `Apply to <path>? [Y/n/q]`; batch all accepted writes at the end.
7. **Atomic write + state update** — replace only marker-block content;
   update `.squadai/.squad-refined` (schema in playbook).
8. **Report** — summary of refined/skipped/failed targets.

## Hard rules

- **NEVER** modify content outside the `<!-- squadai:refinement -->`
  markers — the template body is regenerable from `squadai apply`; the
  marker block is the only territory `/squadai-init` owns.
- **NEVER** invoke any other tool, command, or sub-agent beyond reading
  files and writing the marker blocks.
- **NEVER** widen scope from refining content to refactoring code or
  "fixing" things noticed while sampling. Refinement is text-only inside
  markers.
- **ALWAYS** show diffs before writing; **ALWAYS** require accept (Y/y)
  before writing.
- If anything is unclear, **stop and ask the user**. A failed squadai-init
  is recoverable; a refinement written against the wrong file is not.
