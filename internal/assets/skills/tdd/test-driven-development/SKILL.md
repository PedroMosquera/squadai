---
name: test-driven-development
description: Red-green-refactor TDD implementation cycle
methodology: tdd
---

# Test-Driven Development Skill

Implement code using the red-green-refactor cycle. This skill is used by the
Implementer to write code driven by failing tests.

## Core Principle

Write the MINIMUM code to make a failing test pass. No more. Then refactor
to improve quality without changing behavior.

## Steps

### Phase 1: RED — Write a failing test

1. Pick the next test from the implementation plan.
2. Write the test code first, before any implementation.
3. Run the test: it MUST fail (red). If it passes without implementation, the
   test is not testing the right thing.
4. Read the failure message — it tells you exactly what to implement.

### Phase 2: GREEN — Make the test pass

1. Write the MINIMUM implementation to make the failing test pass.
   - No extra logic, no anticipated future use cases
   - Hardcoding is acceptable at this stage if it makes the test pass
   - Copy-paste is acceptable if it moves quickly to green
2. Run the test: it MUST pass (green).
3. Run ALL tests: ensure no regressions.
4. Do NOT refactor yet.

### Phase 3: REFACTOR — Improve the code

1. Only after tests are green, improve code quality.
   - Remove duplication
   - Improve naming
   - Extract helper functions
   - Simplify logic
2. Run tests after every change — refactoring must not break anything.
3. Do NOT add new behavior during refactoring.
4. When satisfied, move to the next test in the plan.

## Rules

- Never skip the red phase — always write the test first
- Never write more code than needed to pass the current test
- Never refactor while red — only refactor on green
- Commit after each green-refactor cycle (small, frequent commits)
- If a test is hard to write, it signals a design problem — fix the design

## Test Structure

Follow table-driven tests for multiple scenarios:

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {name: "happy path", input: ..., want: ..., wantErr: false},
        {name: "empty input", input: ..., want: ..., wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error: %v", err)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Output Format

For each RED-GREEN-REFACTOR cycle, produce:
1. The failing test code
2. The minimal implementation
3. Any refactoring applied
4. Confirmation that all tests pass
