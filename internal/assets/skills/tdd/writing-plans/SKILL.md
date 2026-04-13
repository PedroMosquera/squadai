---
name: writing-plans
description: Convert test scenarios into ordered implementation plans for TDD
methodology: tdd
---

# Writing Plans Skill

Convert a list of test scenarios into an ordered implementation plan. This
skill is used by the Planner after the Brainstormer produces scenarios.

## Steps

1. **Review scenarios**: Read the brainstorming output carefully.
   - Identify all test scenarios that need coverage
   - Note dependencies between scenarios (some tests require others to pass first)
   - Identify which scenarios share setup/teardown

2. **Group into test suites**: Organize scenarios by the code unit they test.
   - Group tests for the same function/method together
   - Identify shared fixtures that should be extracted
   - Note where table-driven tests are appropriate (multiple inputs → same function)

3. **Order the tests**: Sequence tests from simplest to most complex.
   - Start with happy path tests (positive cases)
   - Follow with edge cases
   - End with error scenarios
   - Ensure each test builds on knowledge from previous tests

4. **Write the test plan**: For each test, specify:
   - Test name (format: `TestFunctionName_Scenario_ExpectedResult`)
   - Setup required (fixtures, mocks, temp dirs)
   - The specific action to invoke
   - The assertion to make (exact values, not just "no error")
   - Teardown needed (if any)

5. **Identify implementation dependencies**: List what code must be written.
   - New functions or methods needed
   - Interface changes required
   - New types or structs
   - Dependency injection points for testability

6. **Estimate complexity**: Flag high-effort items.
   - Simple (< 30 min): straightforward logic, no external dependencies
   - Medium (30 min – 2 hrs): requires new abstraction or refactoring
   - Complex (> 2 hrs): involves multiple components or tricky concurrency

## Output Format

```
## Test Plan

### Suite: <FunctionName>
- [ ] Test: TestFunctionName_HappyPath_ReturnsResult
  Setup: <what to prepare>
  Action: call FunctionName with <inputs>
  Assert: result equals <expected>

- [ ] Test: TestFunctionName_EmptyInput_ReturnsError
  ...

## Implementation Dependencies
1. <what needs to be implemented>
2. ...

## Complexity Estimates
- TestFunctionName_HappyPath: Simple
- ...
```

Hand this plan to the Implementer. Do NOT write implementation code.
