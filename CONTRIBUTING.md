# Contributing to squadai

Thanks for your interest in contributing! This document covers what you need
to know to make a productive change.

## Before You Start

- **Search [issues](https://github.com/PedroMosquera/squadai/issues) first.**
  Avoid duplicate work.
- **For non-trivial changes, open an issue first** to discuss the design before
  writing code. This is especially important for new components, adapters, or
  changes to the policy/config schema.
- **Bug reports** should include: `squadai version`, OS, reproduction steps,
  and `squadai doctor --json` output if the problem is environment-related.

## Development Setup

```sh
# Requirements: Go 1.24+
git clone https://github.com/PedroMosquera/squadai.git
cd squadai
go build ./cmd/squadai
go test ./...
```

## Project Layout

| Path | Purpose |
|------|---------|
| `cmd/squadai/` | Entry point |
| `internal/domain/` | Types, interfaces, errors (no side effects) |
| `internal/config/` | Three-layer config merge with policy enforcement |
| `internal/adapters/` | Per-agent file generators (one package per agent) |
| `internal/components/` | Per-component installers (one package per component) |
| `internal/planner/` | Compute actions from merged config |
| `internal/pipeline/` | Execute actions with backup/rollback |
| `internal/verify/` | Post-apply compliance checks |
| `internal/doctor/` | Health checks (`squadai doctor`) |
| `internal/cli/` | CLI command handlers |
| `internal/tui/` | Bubbletea TUI wizard |
| `docs/` | User-facing documentation |
| `openspec/` | Spec-driven development workflow (specs, changes, archive) |

Layer dependencies flow downward only: cli → planner → config → domain.
Adapters and components import only `domain`.

## Coding Conventions

- **Error wrapping**: `fmt.Errorf("context: %w", err)`, never bare returns.
- **Atomic file writes**: use `internal/fileutil.WriteAtomic` for every write.
- **Deterministic iteration**: use `sortedKeys` helpers for map ranges that
  produce user-visible output.
- **Marker blocks**: managed regions wrapped with
  `<!-- agent-manager:SECTION -->` markers; user content outside markers must
  never be modified.
- **Idempotent**: `apply` re-runs must produce no diff if state matches.
- **Test command**: `go test -race ./...` must pass.
- **Lint**: `go vet ./...` and `golangci-lint run ./...` must produce zero
  findings.
- **Tests**: table-driven where possible; use `t.Chdir` (Go 1.24+) instead of
  `os.Chdir` to avoid cross-platform cwd cleanup issues.

## Pull Request Checklist

Before opening a PR, run locally:

```sh
go vet ./...
go test -race ./...
golangci-lint run ./...
go build -o /tmp/squadai-build ./cmd/squadai
bash scripts/smoke-test.sh
```

All five must pass. CI runs the same on macOS and Linux.

In your PR description, include:

- **What** the change does and **why** it's needed
- Linked issue (if any): `Fixes #123` / `Closes #123`
- Any breaking changes to the public API or config schema
- Screenshots for TUI/output changes

## Spec-Driven Development

For larger changes (new components, new adapters, schema migrations), follow
the SDD workflow under `openspec/`:

1. Write a change proposal under `openspec/changes/<change-id>/`
2. Get the proposal reviewed before implementing
3. After implementation + verification, archive under `openspec/archive/`

See `openspec/config.yaml` for workflow rules.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body explaining what and why>
```

Examples:

- `feat(doctor): add MCP connectivity check`
- `fix(adapters/cursor): preserve user content outside markers`
- `docs(readme): clarify three-layer config precedence`
- `refactor(pipeline): extract step executor`

Do **not** add `Co-Authored-By` or AI attribution lines.

## Release Process

Releases are tag-driven. Maintainers run:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

`.github/workflows/release.yml` invokes goreleaser to build cross-platform
binaries, publish them to GitHub Releases, generate `.deb` and `.rpm`
packages, and update the Homebrew tap automatically.

## License

By contributing, you agree that your contributions will be licensed under the
MIT License (see `LICENSE`).
