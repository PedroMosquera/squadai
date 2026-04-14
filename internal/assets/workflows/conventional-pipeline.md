# Conventional Development Workflow

> Windsurf workflow for Conventional methodology. Each phase below represents a step in the development pipeline.

## Phase 1: Implementation

- Understand the requirement fully before writing code.
- Follow existing project conventions for naming, structure, and patterns.
- Write unit tests alongside production code; do not defer testing.
- Keep changes focused: one logical change per commit.
- Use conventional commit messages (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`).

## Phase 2: Review

- Run the linter and fix all warnings before requesting review.
- Check test coverage for new and modified code paths.
- Verify error handling: no swallowed errors, no bare panics.
- Confirm public API changes are backward-compatible or properly versioned.
- Review for clarity: can another developer understand this without context?

## Phase 3: Testing

- Run the full test suite and confirm all tests pass.
- Add missing tests for edge cases: nil inputs, empty collections, concurrency.
- Fix flaky tests immediately; do not skip or retry around them.
- Measure coverage delta and ensure it does not decrease.
- Run race detection (`go test -race`) on concurrent code.

## Phase 4: Orchestration

- Coordinate phase transitions: implement, then review, then test.
- Delegate remaining work when context window exceeds 60% capacity.
- On compaction or context loss, read AGENTS.md and git log to recover state.
- Ensure each phase produces a committed artifact before advancing.
