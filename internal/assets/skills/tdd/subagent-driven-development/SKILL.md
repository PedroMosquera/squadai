---
name: subagent-driven-development
description: Orchestrating sub-agents through TDD phases for complex features
methodology: tdd
---

# Sub-Agent Driven Development Skill

Coordinate a team of specialized sub-agents through the full TDD lifecycle
for a complex feature. This skill is used by the TDD Orchestrator.

## Overview

Sub-agent driven development applies TDD at the team level. Rather than one
agent doing all phases, each phase is handled by a specialist:

```
Feature Request
  → Brainstormer: scenarios and edge cases
  → Planner: ordered test plan + implementation dependencies
  → Implementer: red-green-refactor cycles
  → Reviewer: two-stage code review
  → Debugger: (if tests fail after implementation)
```

## Orchestration Steps

### 1. Receive and clarify the feature request

Before delegating, gather:
- Clear success criteria
- Acceptance tests (user-visible behavior)
- Non-functional requirements (performance, security)
- Constraints (deadline, API compatibility)

If any are missing, ask before starting delegation.

### 2. Delegate to Brainstormer

Provide: the feature description + success criteria.
Expect back: numbered list of test scenarios grouped by category.
Validate: covers happy paths, edge cases, and error scenarios.

### 3. Delegate to Planner

Provide: the brainstorming output + feature description.
Expect back: ordered test plan + implementation dependencies.
Validate: tests are ordered simple → complex, dependencies listed.

### 4. Delegate to Implementer

Provide: the full test plan.
Expect back: passing test suite + minimal implementation.
Validate: all tests pass, no skipped tests, no TODO comments.

### 5. Delegate to Reviewer

Provide: the diff (all changed files).
Expect back: Critical/Warning/Suggestion review findings.
Validate: no Critical findings before accepting. Address Warnings.

### 6. Handle failures via Debugger

If Implementer reports failing tests, delegate to Debugger.
Provide: exact failure output + relevant code.
Expect back: root cause analysis + fix.
Then re-delegate to Implementer or apply fix directly.

## Sub-Agent Isolation Rules

- Each sub-agent starts with a fresh context
- Pass ONLY what the sub-agent needs — no irrelevant history
- Summarize results after each phase (do not carry full sub-agent output)
- If a sub-agent exceeds its scope, correct and redirect

## Phase Gate Criteria

| Phase | Gate Before Next Phase |
|-------|----------------------|
| Brainstorming | ≥ 3 happy paths, ≥ 3 edge cases, ≥ 2 error scenarios |
| Planning | All scenarios have test names, implementation deps listed |
| Implementation | All tests pass, no skipped tests |
| Review | No Critical findings |

## Output Format

After completing all phases, produce a summary:

```
## Feature Complete: <feature name>

### Phases Completed
- Brainstorming: <N> scenarios generated
- Planning: <N> tests planned, <N> implementation deps
- Implementation: <N> tests written, all passing
- Review: <N> warnings addressed, no criticals

### Files Changed
- <list of modified/created files>

### Test Coverage
- <test file>: <N> tests
```
