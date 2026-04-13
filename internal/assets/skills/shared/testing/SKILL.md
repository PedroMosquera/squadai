---
name: testing
description: Test writing protocol with structured coverage analysis
---

# Testing Skill

Write tests for the specified code following a structured protocol. Produce
thorough, maintainable tests that cover the important behavior.

## Steps

1. **Identify what to test**: Analyze the code to determine test targets.
   - Public API functions and methods
   - Edge cases: empty inputs, nil/null values, boundary conditions
   - Error paths: invalid input, missing dependencies, timeout conditions
   - State transitions: before/after side effects

2. **Choose test structure**: Select the appropriate pattern.
   - **Table-driven tests**: When testing the same function with multiple inputs.
   - **Arrange-Act-Assert (AAA)**: For single-scenario tests.
   - **Given-When-Then**: For behavior-focused tests.

3. **Name tests descriptively**: Test names should explain the scenario.
   - Format: `TestFunctionName_Scenario_ExpectedResult`
   - Example: `TestValidate_EmptyInput_ReturnsError`
   - Failed test names should make the failure obvious without reading the test body.

4. **Set up test fixtures**: Prepare the test environment.
   - Use temp directories for filesystem tests.
   - Create minimal fixtures — only what the test needs.
   - Clean up after tests (use deferred cleanup or framework helpers).

5. **Write assertions**: Verify behavior clearly.
   - Assert on specific values, not just "no error."
   - Check both positive cases (correct output) and negative cases (expected errors).
   - Verify side effects (file created, state changed) when relevant.

6. **Mock boundaries, not internals**: Keep tests focused.
   - Mock external dependencies: HTTP, databases, filesystems, clocks.
   - Do not mock internal functions or private methods.
   - Use interfaces at boundaries to enable test substitution.

7. **Check coverage gaps**: Review what is not yet tested.
   - Concurrent access (if applicable).
   - Large inputs or performance-sensitive paths.
   - Integration between components (consider integration tests).

## Output Format

Produce test code that:
- Follows the project's existing test conventions.
- Includes a brief comment explaining each test's purpose.
- Groups related tests together.
- Is runnable without modification.
