#!/usr/bin/env bash
# scripts/smoke-test.sh — End-to-end smoke test for agent-manager.
#
# Builds the binary, creates temp projects (Go, Node, Python), and runs
# init -> apply -> verify through each scenario. Exits non-zero on first failure.
#
# Usage:
#   ./scripts/smoke-test.sh           # run all scenarios
#   ./scripts/smoke-test.sh --keep    # keep temp dirs for inspection (prints paths)

set -euo pipefail

# ── Config ──────────────────────────────────────────────────────────────────

KEEP=false
[[ "${1:-}" == "--keep" ]] && KEEP=true

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
RESET='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0
FAILURES=()

# ── Helpers ─────────────────────────────────────────────────────────────────

log()  { printf "${BOLD}==> %s${RESET}\n" "$*"; }
pass() { printf "  ${GREEN}PASS${RESET} %s\n" "$*"; PASS_COUNT=$((PASS_COUNT + 1)); }
fail() { printf "  ${RED}FAIL${RESET} %s\n" "$*"; FAIL_COUNT=$((FAIL_COUNT + 1)); FAILURES+=("$*"); }
warn() { printf "  ${YELLOW}WARN${RESET} %s\n" "$*"; }
skip() { printf "  ${YELLOW}SKIP${RESET} %s\n" "$*"; }

assert_file_exists() {
  if [[ -f "$1" ]]; then
    pass "$2"
  else
    fail "$2 — file not found: $1"
  fi
}

assert_file_contains() {
  if grep -q "$2" "$1" 2>/dev/null; then
    pass "$3"
  else
    fail "$3 — '$2' not found in $1"
  fi
}

assert_output_contains() {
  if echo "$1" | grep -q "$2"; then
    pass "$3"
  else
    fail "$3 — expected '$2' in output"
  fi
}

assert_dir_exists() {
  if [[ -d "$1" ]]; then
    pass "$2"
  else
    fail "$2 — directory not found: $1"
  fi
}

cleanup() {
  if [[ "$KEEP" == true ]]; then
    warn "Temp dirs preserved (--keep):"
    warn "  HOME:    $FAKE_HOME"
    warn "  Go:      ${GO_PROJECT:-n/a}"
    warn "  Node:    ${NODE_PROJECT:-n/a}"
    warn "  Python:  ${PYTHON_PROJECT:-n/a}"
  else
    rm -rf "$TMPROOT"
  fi
}

# ── Setup ───────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

log "Building agent-manager binary"
BIN="$REPO_ROOT/agent-manager"
go build -o "$BIN" ./cmd/agent-manager
if [[ ! -x "$BIN" ]]; then
  echo "FATAL: build failed" >&2
  exit 1
fi
pass "Binary built at $BIN"

TMPROOT="$(mktemp -d)"
FAKE_HOME="$TMPROOT/home"
mkdir -p "$FAKE_HOME"

# Override HOME so we don't touch the real ~/.agent-manager
export HOME="$FAKE_HOME"

trap cleanup EXIT

# ── Scenario 1: Go project ─────────────────────────────────────────────────

log "Scenario 1: Go project"

GO_PROJECT="$TMPROOT/go-project"
mkdir -p "$GO_PROJECT"
cat > "$GO_PROJECT/go.mod" <<'EOF'
module github.com/example/myapp

go 1.24
EOF
mkdir -p "$GO_PROJECT/cmd/myapp"
cat > "$GO_PROJECT/cmd/myapp/main.go" <<'EOF'
package main

import "fmt"

func main() { fmt.Println("hello") }
EOF

# -- init --
cd "$GO_PROJECT"
log "  Running: agent-manager init"
INIT_OUT=$("$BIN" init 2>&1) || true

assert_file_exists "$GO_PROJECT/.agent-manager/project.json" "init creates project.json"
assert_file_exists "$FAKE_HOME/.agent-manager/config.json" "init creates user config"
assert_file_exists "$GO_PROJECT/.agent-manager/templates/team-standards.md" "init creates team standards"
assert_file_exists "$GO_PROJECT/.agent-manager/skills/code-review.md" "init creates code-review skill"
assert_file_exists "$GO_PROJECT/.agent-manager/skills/testing.md" "init creates testing skill"
assert_file_exists "$GO_PROJECT/.agent-manager/skills/pr-description.md" "init creates pr-description skill"
assert_file_contains "$GO_PROJECT/.agent-manager/project.json" '"language"' "project.json has language"
assert_file_contains "$GO_PROJECT/.agent-manager/project.json" '"Go"' "project.json detects Go"
assert_file_contains "$GO_PROJECT/.agent-manager/templates/team-standards.md" "Error Handling" "Go standards contain Error Handling"
assert_output_contains "$INIT_OUT" "Go" "init output mentions Go"

# -- apply --dry-run --
log "  Running: agent-manager apply --dry-run"
DRY_OUT=$("$BIN" apply --dry-run 2>&1) || true
assert_output_contains "$DRY_OUT" "action(s) would be executed" "dry run reports actions"

# -- apply --
log "  Running: agent-manager apply"
APPLY_OUT=$("$BIN" apply 2>&1) || true

assert_file_exists "$GO_PROJECT/AGENTS.md" "apply creates AGENTS.md"
assert_file_exists "$GO_PROJECT/.github/copilot-instructions.md" "apply creates copilot instructions"
assert_file_contains "$GO_PROJECT/AGENTS.md" "agent-manager" "AGENTS.md has managed marker"
assert_file_contains "$GO_PROJECT/.github/copilot-instructions.md" "myapp" "copilot instructions contain project name"
assert_output_contains "$APPLY_OUT" "written" "apply output contains 'written'"
assert_output_contains "$APPLY_OUT" "Applied" "apply output contains summary line"

# -- verify --
log "  Running: agent-manager verify"
VERIFY_OUT=$("$BIN" verify 2>&1) || true
assert_output_contains "$VERIFY_OUT" "passed" "verify output contains 'passed'"
assert_output_contains "$VERIFY_OUT" "checks" "verify output contains check summary"

# -- idempotency --
log "  Testing idempotency (second apply)"
APPLY2_OUT=$("$BIN" apply 2>&1) || true
assert_output_contains "$APPLY2_OUT" "0 failed" "second apply has 0 failures"

# -- user content preservation --
log "  Testing user content preservation"
echo -e "\n## My Custom Notes\n\nDo not delete this." >> "$GO_PROJECT/AGENTS.md"
APPLY3_OUT=$("$BIN" apply 2>&1) || true
assert_file_contains "$GO_PROJECT/AGENTS.md" "My Custom Notes" "user content preserved after apply"
assert_file_contains "$GO_PROJECT/AGENTS.md" "agent-manager" "managed content still present"

# -- init --force --
log "  Testing init --force"
FORCE_OUT=$("$BIN" init --force 2>&1) || true
assert_output_contains "$FORCE_OUT" "overwritten" "init --force reports overwritten"

# -- backup list --
log "  Testing backup list"
BACKUP_OUT=$("$BIN" backup list 2>&1) || true
# Backups were created by the apply calls above.
if echo "$BACKUP_OUT" | grep -qE "[0-9]{8}T[0-9]{6}Z-[0-9a-f]+|No backups"; then
  pass "backup list produces output"
else
  fail "backup list — unexpected output: $BACKUP_OUT"
fi

# -- verify --json --
log "  Testing verify --json"
JSON_OUT=$("$BIN" verify --json 2>&1) || true
assert_output_contains "$JSON_OUT" '"check"' "verify --json produces JSON with check field"
assert_output_contains "$JSON_OUT" '"passed"' "verify --json produces JSON with passed field"

# ── Scenario 2: Node/TypeScript project ────────────────────────────────────

log "Scenario 2: Node/TypeScript project"

NODE_PROJECT="$TMPROOT/node-project"
mkdir -p "$NODE_PROJECT"
cat > "$NODE_PROJECT/package.json" <<'EOF'
{
  "name": "my-web-app",
  "version": "1.0.0",
  "scripts": {
    "test": "jest",
    "build": "tsc",
    "lint": "eslint ."
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}
EOF
cat > "$NODE_PROJECT/tsconfig.json" <<'EOF'
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "strict": true
  }
}
EOF

cd "$NODE_PROJECT"
INIT_OUT=$("$BIN" init 2>&1) || true
assert_file_exists "$NODE_PROJECT/.agent-manager/project.json" "node: init creates project.json"
assert_file_contains "$NODE_PROJECT/.agent-manager/project.json" '"TypeScript"' "node: detects TypeScript"
assert_file_contains "$NODE_PROJECT/.agent-manager/templates/team-standards.md" "TypeScript" "node: JS/TS standards selected"

APPLY_OUT=$("$BIN" apply 2>&1) || true
assert_file_exists "$NODE_PROJECT/AGENTS.md" "node: apply creates AGENTS.md"
assert_file_exists "$NODE_PROJECT/.github/copilot-instructions.md" "node: apply creates copilot instructions"

VERIFY_OUT=$("$BIN" verify 2>&1) || true
assert_output_contains "$VERIFY_OUT" "passed" "node: verify has passing checks"

# ── Scenario 3: Python project ─────────────────────────────────────────────

log "Scenario 3: Python project"

PYTHON_PROJECT="$TMPROOT/python-project"
mkdir -p "$PYTHON_PROJECT"
cat > "$PYTHON_PROJECT/pyproject.toml" <<'EOF'
[project]
name = "my-ml-tool"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = [
    "fastapi>=0.100",
    "pytest>=7.0",
]

[tool.ruff]
line-length = 100
EOF

cd "$PYTHON_PROJECT"
INIT_OUT=$("$BIN" init 2>&1) || true
assert_file_exists "$PYTHON_PROJECT/.agent-manager/project.json" "python: init creates project.json"
assert_file_contains "$PYTHON_PROJECT/.agent-manager/project.json" '"Python"' "python: detects Python"
assert_file_contains "$PYTHON_PROJECT/.agent-manager/templates/team-standards.md" "Type Hints" "python: Python standards selected"

APPLY_OUT=$("$BIN" apply 2>&1) || true
assert_file_exists "$PYTHON_PROJECT/AGENTS.md" "python: apply creates AGENTS.md"

VERIFY_OUT=$("$BIN" verify 2>&1) || true
assert_output_contains "$VERIFY_OUT" "passed" "python: verify has passing checks"

# ── Scenario 4: Empty project (no language detected) ───────────────────────

log "Scenario 4: Empty project (generic fallback)"

EMPTY_PROJECT="$TMPROOT/empty-project"
mkdir -p "$EMPTY_PROJECT"

cd "$EMPTY_PROJECT"
INIT_OUT=$("$BIN" init 2>&1) || true
assert_file_exists "$EMPTY_PROJECT/.agent-manager/project.json" "empty: init creates project.json"
assert_file_contains "$EMPTY_PROJECT/.agent-manager/templates/team-standards.md" "Code Quality" "empty: generic standards selected"

APPLY_OUT=$("$BIN" apply 2>&1) || true
assert_file_exists "$EMPTY_PROJECT/AGENTS.md" "empty: apply creates AGENTS.md"

# ── Scenario 5: Version and help ───────────────────────────────────────────

log "Scenario 5: CLI basics"

VERSION_OUT=$("$BIN" version 2>&1) || true
assert_output_contains "$VERSION_OUT" "dev" "version reports 'dev' for local build"

HELP_OUT=$("$BIN" help 2>&1) || true
assert_output_contains "$HELP_OUT" "init" "help lists init command"
assert_output_contains "$HELP_OUT" "apply" "help lists apply command"
assert_output_contains "$HELP_OUT" "verify" "help lists verify command"

# ── Summary ─────────────────────────────────────────────────────────────────

echo ""
printf "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}\n"
printf "${BOLD}Results: ${GREEN}%d passed${RESET}, ${RED}%d failed${RESET}\n" "$PASS_COUNT" "$FAIL_COUNT"
printf "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}\n"

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  echo ""
  printf "${RED}Failures:${RESET}\n"
  for f in "${FAILURES[@]}"; do
    printf "  - %s\n" "$f"
  done
fi

echo ""
exit "$FAIL_COUNT"
