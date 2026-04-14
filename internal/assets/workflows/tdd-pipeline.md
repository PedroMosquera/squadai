# TDD Development Workflow

> Windsurf workflow for TDD methodology. Each phase below represents a step in the development pipeline.

## Phase 1: Brainstorming

- Analyze the feature request or bug report to identify the core problem.
- Generate at least two alternative approaches with trade-off analysis.
- Identify edge cases, failure modes, and integration points early.
- Capture assumptions that need validation before implementation begins.
- Produce a short summary of the chosen approach with rationale.

## Phase 2: Planning

- Break the chosen approach into discrete, testable tasks ordered by dependency.
- Define test cases for each task before writing any production code.
- Identify shared fixtures, mocks, or test helpers required across tasks.
- Estimate complexity per task and flag anything requiring spike work.
- Write the plan as a checklist with acceptance criteria per item.

## Phase 3: Implementation

- Write a failing test that captures the next requirement (Red).
- Implement the minimum code to make the test pass (Green).
- Refactor for clarity, duplication, and naming without changing behavior.
- Run the full test suite after each Red-Green-Refactor cycle.
- Commit at each green state with a conventional commit message.

## Phase 4: Review

- Verify every public function has at least one test covering the happy path.
- Check for missing edge-case tests: nil inputs, empty collections, boundary values.
- Review naming, package structure, and adherence to project conventions.
- Confirm no test relies on execution order or shared mutable state.
- Flag any TODO or skip markers that should be resolved before merge.

## Phase 5: Debugging

- Reproduce the failure with a minimal test case that isolates the bug.
- Use `git bisect` or recent commit history to locate the regression point.
- Fix the root cause, not the symptom; add a regression test.
- Run the full suite to confirm no secondary breakage.

## Phase 6: Orchestration

- Track phase completion status and transition between phases in order.
- Delegate to the appropriate phase when context window exceeds 60% capacity.
- On compaction or context loss, read AGENTS.md and git log to recover state.
- Ensure every phase produces a committed artifact before advancing.
- Maintain a running checklist of completed and remaining tasks.
