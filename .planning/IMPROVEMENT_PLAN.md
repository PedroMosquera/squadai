# SquadAI Improvement Plan

Reference document for the multi-phase implementation on branch
`feat/pi-adapter-brand-budget`. Keep this file in sync as phases ship.

## Strategic framing

Competitors fall into two camps:

- **Rules-marketplace tools** (`cursor-rules`, `awesome-claude-code`, Pi's
  `pi-skills` package system) — share prompt snippets, no orchestration.
- **Agent orchestration frameworks** (`claude-flow`, `aider` architect/editor)
  — delegate well but are coupled to one runtime and don't standardize across
  editors.

SquadAI's wedge is **cross-editor config-as-code with a methodology team**.
Every workstream below reinforces that wedge rather than competing head-on with
either camp.

## Locked decisions (from planning conversation)

| Decision | Choice |
|---|---|
| Tokenizer | Bundle `tiktoken-go` with embedded BPE files (cl100k_base, o200k_base). Lazy-loaded. `len/4` model-calibrated fallback for unknown models. |
| Memory search | Pure-Go TF-IDF + cosine similarity now. Embeddings deferred to a later phase. |
| Adapter customization | Built-ins stay curated; `.squadai/adapters/<id>.json` overrides path fields only. Unknown IDs construct a `GenericAdapter`. |
| Banners | Per-agent themed variants (SquadAI standalone for OpenCode, co-branded for Pi). |
| Methodology authoring | Include in this plan as Phase 4. |

---

## Phase 1 — Pi Agent adapter, adapter overrides, branded banners

### B1. Pi adapter

New `internal/adapters/pi/adapter.go`, new `domain.AgentID` `AgentPi`.

Paths:
- `ConfigDir` → `~/.pi/agent`
- `SystemPromptFile` → `~/.pi/agent/AGENTS.md` (write one if missing — Pi reads project `AGENTS.md` too)
- `AgentsDir` → `~/.pi/agent/agents`
- `PromptsDir` → `~/.pi/agent/prompts` (Pi splits sub-agent *definitions* in `agents/` from *prompt templates* in `prompts/`; both populated)
- `SkillsDir` → `~/.pi/agent/skills`
- `SettingsPath` → `~/.pi/agent/settings.json`
- `SubAgentsDir` → `~/.pi/agent/agents`

Capabilities:
- `DelegationStrategy` → `DelegationNativeAgents`
- `SupportsSubAgents` → true
- `SupportsWorkflows` → false
- `SupportsComponent` → memory, rules, settings, mcp, agents, skills, commands, plugins, permissions (commands render into `prompts/` as prompt templates)

Detect: `exec.LookPath("pi")` + `statPath(~/.pi/agent)`.

Wire into `internal/app/registry`, `status`, `doctor`, TUI adapter summary screens.

### B2. Adapter JSON overrides

Built-ins stay curated. `.squadai/adapters/<id>.json` with optional path fields:
`config_dir`, `agents_subdir`, `prompts_subdir`, `skills_subdir`,
`settings_path`, `delegation`, `supports[]`.

- `internal/adapters/loader.go` builds an `OverrideAdapter` wrapping the
  built-in; unknown `AgentID` constructs a `GenericAdapter` from a full
  descriptor.
- `squadai explain adapter <id>` renders merged effective paths.
- `doctor` checks for override JSON and reports effective paths.

### C. Brand component

New `domain.ComponentID` → `ComponentBrand`.
New `internal/components/brand/` installer.
New `internal/assets/brand/` directory with **per-agent themed variants**:

- `banner-squadai.txt` — standalone, used by OpenCode/Claude/Cursor/Windsurf/VSCode.
- `banner-pi.txt` — co-branded "SquadAI · Pi".
- `banner-opencode.txt` — co-branded "SquadAI · OpenCode".

Per-agent dialect renderer:
- OpenCode → fenced ```` ```text ```` block in `.opencode/agents/orchestrator.md`.
- Pi → fenced block in `~/.pi/agent/agents/orchestrator.md` **and** mirrored
  `prompts/_squadai_banner.md` so Pi's prompt loader surfaces it at session start.
- Claude Code → marker block in `CLAUDE.md`.
- Cursor → fenced block in `.cursor/rules/squadai.mdc`.
- Windsurf / VS Code → fenced block in their rules file.

Banner sizing target: ≤72 cols, ≤8 lines, ~120 tokens each.

`project.json` toggle: `"brand": { "enabled": true, "style": "default" }`.
`token-budget` shows brand as its own row. Policy can lock `brand.enabled`.
`doctor` checks markers + printable-ASCII purity.

---

## Phase 2 — Real tokenizer, `--fit` budgeting, memory MCP

### A1. Bundled tiktoken-go

New `internal/tokenprofile/tokenizer`:
- `//go:embed` cl100k_base + o200k_base `.tiktoken` files; lazy `tiktoken.New` per family.
- `TokenizerFor(model string)` maps:
  - `claude-*`, `gpt-4o*` → o200k_base
  - `gpt-4*`, `gpt-3.5*` → cl100k_base
  - unknown → existing `len/4` heuristic with per-model divisor.
- Read each detected agent's model list (Pi's `~/.pi/agent/models.json`, OpenCode config) to auto-pick.
- Replace usage in `token_budget.go`, profiler, and the new fitter.
- Keep `ApproxTokens` exported as the cheap fallback.

### A2. `squadai apply --fit`

New `internal/planner/budget/` package.
Input: target model + token cap (from `--max-tokens`/`--model` or
`project.json` `budget.max_tokens` / `budget.model`).

Priority tiers (drop lowest first): plugins → commands → skills (full → summarised) → memory protocol (full → stub) → rules → orchestrator. Brand banner always kept; if even banners push over cap, fail loudly with guidance.

Three truncation modes per component: `full`, `summary` (one-paragraph + `see <file>` pointer — works for OpenCode & Pi), `omit`.

`internal/components/skills/` truncation: when a skill file > `per_component_cap`, fall back to its `summary` frontmatter field if present, else auto-summarize first paragraph.

Output layout persisted to `.squadai/.applied-budget.json`; `diff` and `doctor` read it to detect drift (e.g. agent swap invalidates the budget).

### A4. Progressive memory via MCP

`internal/mcpserver` gains a `squadai-memory` server (stdio):
- Wraps `internal/memory` as an MCP server. Default-enabled local MCP alongside Context7.
- Orchestrator gets only the **memory index** in its prompt (paths + first-line + tags), not full notes — saves the ~1k tokens currently in the README.
- Tool surface: `memory_search`, `memory_get`, `memory_add`, `memory_promote_status`.

Memory becomes pay-per-call context, not fixed install overhead.

---

## Phase 3 — Session telemetry, semantic memory, lifecycle

### A3. `squadai token-usage`

New `internal/tokenprofile/session`:
- Parse OpenCode + Pi session transcripts under their respective session dirs.
- Aggregate real system+completion tokens per project, last 7/30/all.
- New `internal/tokenprofile/pricing` table (USD/1M tokens by model).
- `--watch` tails the latest session and prints a live table; `--json` for CI.

### D1. TF-IDF search

Rewrite `internal/memory/search.go`:
- Pure-Go TF-IDF vectors per note, cosine similarity, freshness decay multiplier.
- `squadai memory reindex` writes `docs/memory/.index/<note>.tfidf.json` (diff-friendly JSON).
- Embeddings explicitly deferred.

### D2. Memory lifecycle

New `internal/memory/gc.go`:
- `squadai memory gc --older-than 180d --dry-run` archives unreferenced notes to `docs/memory/.archive/`, prunes the live index.
- `doctor --fix` runs `reindex` when index ↔ files drift detected.
- Notes referenced in `decisions/` ADRs are exempt from GC.

---

## Phase 4 — Methodology authoring, plugin SDK, CI hooks

### E1. User methodologies

`.squadai/methodologies/<name>.json` (roles, delegation graph, per-role skill pointers, prompts).
New `internal/methodology/loader.go`; built-ins `tdd`/`sdd`/`conventional` migrate from hardcoded `assets/teams/*.json` to the same format and ship as embedded defaults.

### E2. Plugin SDK

Promote `internal/marketplace/registry.go`:
- `squadai plugin add git:<url>` → `.squadai/plugins/<id>/` with `plugin.json` manifest declaring contributed methodologies/skills/commands/MCP/memory templates.
- Mirror Pi's `"packages": ["git:github.com/..."]` convention.
- `plugin list` / `plugin remove` / `plugin update`.

### F1. Git hook installer

`squadai hooks install`: `post-merge`/`post-checkout` run `squadai apply --quiet` when `.squadai/` changed; `pre-commit` runs `squadai doctor --strict` (configurable). Marker-wrapped so user lines survive.

### F2. Reusable GitHub Action

`.github/workflows/squadai-check.yml` running `doctor --strict` + `diff --exit-code` on PRs touching `.squadai/`.

---

## Cross-cutting

- **Testing**: every new package gets table-driven tests matching the existing style (`*_test.go` next to source, compare `profiler_test.go`, `search_test.go`). Phase 1's Pi adapter needs a full `adapter_test.go` mirroring `opencode/adapter_test.go`. Tokenizer tests must cover lazy-load path + fallback.
- **`doctor`**: each phase adds checks — Pi detection, banner markers, budget drift, index drift, plugin manifest validity. Keep the README's "~22 checks" claim updated or drop the count.
- **Docs**: update `docs/architecture.md` "Planned subsystems" section, `docs/MANUAL.md` with new commands (`apply --fit`, `token-usage`, `plugin add`, `hooks install`), README's adapter table (add Pi row).
- **Backward compat**: `project.json` `version: 1` stays valid; `budget` and `brand` default-on via sensible behavior; Pi/opencode adapter override JSON is purely additive.

## Additions beyond original plan

These were identified during implementation kickoff:

1. **`squadai explain` topics** for `budget`, `brand`, `adapter-overrides`, `pi`. The `explain` command already exists; keep its topic registry growing.
2. **Banner token-budget row** — when the brand component is installed, `squadai token-budget` must list `brand` as its own category. Without this the budget math silently hides the banner cost.
3. **Override adapter validation** — `doctor` must warn when an override JSON points at a non-existent path (e.g. `config_dir` typo) rather than silently producing broken installs.
4. **Pi `commands` component** — Pi has no native commands dir; render squadai commands into `prompts/` as `<name>.md` prompt templates with a squadai marker so they're discoverable via Pi's prompt picker. Document this in the adapter.
5. **Banner escape hatch** — `squadai apply --no-brand` flag for users who want the methodology without the visual. Cheap to add, useful in CI contexts.
6. **Test for ASCII purity** — banners must be printable ASCII only (no smart quotes, no emoji) because some terminal renderers mangle non-ASCII in fenced blocks. A unit test enforces this.
