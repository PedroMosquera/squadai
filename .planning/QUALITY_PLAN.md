# SquadAI Quality Plan

Self-contained work plan from the July 2026 codebase evaluation. Any session or
agent can pick up a workstream independently — each one lists motivation, exact
file targets, tasks, and acceptance criteria. Keep this file in sync as items
ship (mark checkboxes, note the commit).

Relationship to `.planning/IMPROVEMENT_PLAN.md`: that file tracks the
feature build-out on `feat/pi-adapter-brand-budget` (Pi adapter, brand,
budget phases). This file tracks **quality/trust debt** found in evaluation.
Locked decisions from IMPROVEMENT_PLAN.md still hold (tiktoken-go with
embedded BPE files; pure-Go TF-IDF stays; embeddings deferred).

## Ordering

```
WS1 (honest tokens)  ─┐
WS2 (fix skip tests) ─┼─ independent, ~1 day each, do in any order
WS4 (dedupe paths)   ─┘
WS2 ──▶ WS3 (split god files)   # trustworthy tests BEFORE the big mechanical refactor
WS5 (plugin guardrails)          # independent, slot in anywhere
```

Verification for every workstream: `go build ./... && go vet ./... && go test -race ./...`
plus `golangci-lint run` (same as CI).

---

## WS1 — Make token/cost reporting honest

**Problem.** The `token-budget` / `token-usage` surface reports misleading
numbers for its flagship target (Claude):

- `internal/tokenprofile/tokenizer/tokenizer.go:21-33` — tiktoken-go only has
  OpenAI encodings; every non-OpenAI model silently falls back to
  `ApproxCount` = 4 chars/token, presented with no estimate marker.
- `internal/tokenprofile/pricing/pricing.go:11` — `modelPricing` is a
  hardcoded "known 2025 price table" (stale as of 2026).
- `internal/tokenprofile/pricing/pricing.go:29` — `Lookup` returns a zero
  `ModelPricing{}` for unknown models → cost renders as $0.00 with no warning.

**Tasks.**

- [x] Add an `Approximate bool` (or method) to the token-count result so
      callers know whether a real encoder or the heuristic produced the number.
      (`Approximate()` on the `Counter` interface; also removed the dishonest
      `claude- → o200k_base` proxy mapping, which looked exact but undercounted
      Claude ~15–20%.)
- [x] In all CLI output (`internal/cli/token_budget.go`,
      `internal/cli/token_usage.go`): prefix approximate counts with `~` and
      print a single footnote line. (Deviation: `token-usage` counts come
      verbatim from API-reported session usage, so they are exact and stay
      unprefixed; `~` applies to `token-budget`, where counts are computed.)
- [x] `Lookup`: return `(ModelPricing, bool)` (or a sentinel) so unknown
      models render `cost: unknown` — never $0.00. Update all call sites.
      (Mixed known/unknown totals render as a `≥$` lower bound + footnote.)
- [x] Calibrate the fallback per model family: replace the flat 4 chars/token
      with a small divisor table. (Claude prose 3.7 / code 3.0 with a
      symbol-density heuristic; divisors ship inside pricing.json.)
- [x] Move the price table from Go code into an embedded JSON asset with a
      `generated_at` date (2026-07-12, refreshed 2026 prices). Warn on load
      when older than 90 days. (Deviation: no separate network refresh —
      `squadai update` replaces the whole binary, so the embedded asset
      refreshes with it; documented in `internal/update/update.go` and the
      stale warning points at `squadai update`.)
- [x] Tests: unknown-model → "unknown" not zero; approximate flag propagates
      to output; stale-pricing warning fires past the threshold.

**Acceptance.** No code path can print a confident-looking cost or token count
that is actually a guess. `squadai token-usage` against a Claude-only config
shows `~` counts and either a real or an "unknown" cost — never $0.00.

**Gain.** The usage-controls feature stops being a credibility liability; a
user comparing output against their real provider bill sees honest estimates
instead of silently wrong numbers.

---

## WS2 — Eliminate false-coverage tests

**Problem.** 19 conditional `t.Skip()` calls pass while asserting nothing when
setup doesn't produce the expected case, so planner regressions can sail
through CI:

- `internal/planner/render_test.go` — 14 skips ("no memory action produced —
  skipping" pattern)
- `internal/tui/tui_test.go` — 4 skips
- `internal/pipeline/executor_test.go` — 1 skip (plan with no create steps)

**Tasks.**

- [x] Inventory every conditional skip in the three files
      (`grep -n "t.Skip" <file>`); legitimate environment skips (missing
      binary, OS-specific) stay — only "setup didn't produce the case" skips
      are in scope. (All 19 were in scope; none were environment skips — the
      TUI skill catalog is embedded, so counts are deterministic.)
- [x] Convert each in-scope skip to a hard failure:
      `t.Fatalf("expected plan to produce a memory action, got none")`.
- [x] Fix each fixture/config until the intended action IS produced. (No
      fixture fixes needed: zero skips fired at runtime — all 19 were latent;
      the existing fixtures already produce every asserted action.)
- [x] Run `go test ./internal/planner/... ./internal/tui/... ./internal/pipeline/... -v`
      and confirm zero skips from the in-scope set.

**Acceptance.** `go test -v ./...` shows no "skipping" output from these
patterns; every previously-skipping test now asserts a real outcome.

**Gain.** CI signal becomes real exactly where it matters most — the planner
decides what gets written into users' repos. Prerequisite for WS3: the safety
net must be trustworthy before the large mechanical refactor.

---

## WS3 — Split the god files (do AFTER WS2)

**Problem.** `internal/cli/commands.go` is 5,525 lines / ~81 functions;
`internal/tui/tui.go` is 2,430 lines. Every feature lands there, every merge
conflicts there, and agent/review tooling degrades badly on files this size.

**Tasks.**

- [x] Mechanical split of `commands.go` by command family, same package, zero
      behavior change: 18 new files (`apply.go`, `status.go`, `remove.go`,
      `init.go` + `init_config.go`, `doctor.go`, `backup.go`, `policy.go`,
      `explain.go`, `schema.go`, `context.go`, `diff.go`, `verify.go`,
      `plugins.go`, `hooks_install.go`, `memory_tools.go`, `watch.go`,
      `audit.go`); shared helpers stay in a 112-line `commands.go`. Largest
      new file 522 lines.
- [x] Mechanical split of `tui.go`: `model.go`, `keys.go` (handleKey),
      `commands.go` (tea.Cmd runners), `view_menu.go`, `view_init.go`,
      `view_skill_browser.go`, `view_misc.go`; empty `tui.go` deleted.
      Largest file 661 lines; styles already lived in `styles.go`.
- [x] No function bodies change in the split commits — verified by
      byte-identical sorted function-signature sets plus a line-multiset
      check (cli) and a go/parser byte-range purity check (tui). One family
      per commit, build+tests after each.
- [ ] Follow-up (optional, separate commits): extract obvious duplicated
      helpers discovered during the split. (Not done — optional.)

**Acceptance.** No file in `internal/cli/` or `internal/tui/` exceeds ~800
lines; `git log --stat` shows pure-move commits; full test suite green with
`-race`.

**Gain.** Smaller diffs, fewer merge conflicts, and far better results from
sub-agents/code review working on 300-line files. Makes every future change
cheaper.

---

## WS4 — Deduplicate adapter path resolution

**Problem.** Near-identical Windows `APPDATA` + `runtime.GOOS` resolution is
copy-pasted three times:

- `internal/adapters/cursor/adapter.go:184-…`
- `internal/adapters/windsurf/adapter.go:190-…`
- `internal/adapters/vscode/adapter.go:196-…`

Divergence risk on every Windows path fix; the planned `GenericAdapter` +
JSON overrides (IMPROVEMENT_PLAN.md §B2) would otherwise be built on top of
triplicated logic.

**Tasks.**

- [x] Create `internal/adapters/paths` (or extend an existing shared adapters
      helper) with one function, e.g.
      `UserConfigDir(homeDir, goos, appName string) string` implementing the
      APPDATA → `homeDir\AppData\Roaming\<app>` fallback → XDG/darwin logic.
- [x] Point cursor/windsurf/vscode adapters at it; delete the three copies.
      (Behavioral note: only the Windows branches were identical; VS Code uses
      the full XDG/darwin convention while Cursor/Windsurf keep dot-directories
      on non-Windows — each difference preserved deliberately.)
- [x] Port the per-adapter tests to the shared helper (table-driven across
      GOOS values) and keep one thin test per adapter proving the right
      `appName` is passed.
- [ ] When implementing IMPROVEMENT_PLAN.md §B2 (`GenericAdapter`), build on
      this helper. (Future work — §B2 not in scope here.)

**Acceptance.** `grep -rn "APPDATA" internal/adapters/` matches only the
shared helper (plus its test). All adapter tests green.

**Gain.** Windows bugs get fixed once, not three times; the upcoming override
system inherits correct path handling for free. Directly reduces the
six-vendor maintenance tax.

---

## WS5 — Plugin install guardrails

**Problem.** The plugin path is the weakest link in the tool's own
security/permissions story:

- `internal/pluginsdk/git.go:77` — `git clone --depth 1 <repoURL>` of
  arbitrary remotes, no confirmation, no pinning.
- `internal/tui/tui.go:2266` — execs a catalog-supplied command under a
  `//nolint:gosec` waiver.

**Tasks.**

- [x] Before install, print the resolved repo URL plus every command the
      manifest declares, and require interactive confirmation. Add `--yes`
      for scripted use; non-TTY without `--yes` fails closed. (Manifest lives
      in the repo, so installs clone into a quarantine staging dir first —
      `git clone` runs no repo code; confirmation gates the move into place,
      decline deletes the staged clone. Non-TTY check runs before any network.)
- [x] Support (and prefer) commit-SHA pinning for catalog entries
      (`git:repo@<full-sha>`); warn loudly when installing an unpinned ref.
      Shallow clone kept for unpinned refs.
- [x] Record every plugin install/remove in the existing audit log
      (`plugin:install` / `plugin:remove`). Added `plugins remove-git` — the
      SDK's Remove had no CLI caller, and audited removal needs one.
- [x] Revisit the `tui.go` exec site: `validateSkillInstallCmd` checks argv
      against the embedded catalog (exact command + identifier + safe-token
      regex); nolint narrowed with the invariant documented.
- [x] Tests: non-TTY without `--yes` refuses; audit entry written on
      install/remove; unpinned-ref warning fires (suppressed when pinned);
      interactive decline leaves nothing on disk; pinned-install
      reproducibility; TUI command-tampering rejection.

**Acceptance.** No network-sourced code executes or lands on disk without an
explicit confirmation or `--yes`, every install is audited, and pinned
installs are reproducible.

**Gain.** Closes the gap between the enterprise-locked/permissions pitch and
actual behavior, using audit infrastructure that already exists.

---

## Explicitly deferred

- **TF-IDF rewrite** (`internal/memory/tfidf.go`): hand-rolled but working and
  tested (7 test files). Embeddings already deferred by IMPROVEMENT_PLAN.md.
  Rewriting a working secondary feature is worse ROI than everything above.

## Status log

| Date | Workstream | Commit | Notes |
|------|-----------|--------|-------|
| 2026-07-12 | — | — | Plan created from codebase evaluation |
| 2026-07-12 | WS2 | f7b7070 | Done on `worktree-ws2-fix-skip-tests` (worktree). 19 skips → hard failures; all latent (none fired); full suite green with `-race`, lint clean. Unblocks WS3. |
| 2026-07-12 | WS4 | 1237481, 08ff911 | `internal/adapters/paths.UserConfigDir`; three APPDATA copies deleted; per-OS behavior preserved exactly. Merged at 1312df7. |
| 2026-07-12 | WS5 | 1e48f4a, 2b80ed3, bb47aef | Staged installs + confirmation/`--yes`, SHA pinning, audit events, `plugins remove-git`, TUI exec validation. Merged at 164e356. |
| 2026-07-12 | WS1 | e2a777c, eee0c6a, 52c0e8a | Embedded pricing.json (2026 prices + divisors), `Lookup` ok-bool, `~` approx counts in token-budget, stale warning. Merged at 91aa637. |
| 2026-07-12 | WS3 | 7988b1e, d23f0b5 | Pure-move splits: tui.go → 7 files (max 661 lines), commands.go → 18 files + 112-line helpers (max 522). Purity machine-verified. |
| 2026-07-12 | ALL | 0034473 | All five workstreams merged to `main`; full `go build`/`vet`/`test -race`/`golangci-lint` green at every merge point. |
