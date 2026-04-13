---
name: brainstorming
description: Requirements exploration and test scenario generation for TDD
methodology: tdd
---

# Brainstorming Skill

Explore requirements and generate comprehensive test scenarios before any code
is written. This skill is the first phase of the TDD pipeline.

## Steps

1. **Understand the problem**: Ask clarifying questions before generating scenarios.
   - What is the expected input and output?
   - What are the success criteria?
   - Are there performance or security constraints?
   - Who are the consumers of this functionality?

2. **Identify happy paths**: List the primary usage flows.
   - Core feature usage with typical inputs
   - Most common call patterns
   - Expected return values and side effects

3. **Identify edge cases**: Enumerate boundary and exceptional conditions.
   - Empty inputs, nil/null values, zero values
   - Maximum/minimum boundary values
   - Concurrent access scenarios (if applicable)
   - Large inputs or long-running operations

4. **Identify error scenarios**: List all failure modes.
   - Invalid input (wrong type, out-of-range, malformed)
   - Missing dependencies (unavailable service, missing file)
   - Permission or authentication failures
   - Partial failures (some items succeed, some fail)

5. **Prioritize scenarios**: Rank by risk and importance.
   - Critical: covers behavior that breaks the system if wrong
   - Important: covers common usage patterns
   - Nice-to-have: covers unusual but valid edge cases

## Output Format

Produce a numbered list of test scenarios grouped by category:

### Happy Paths
1. [Scenario: input → expected output]

### Edge Cases
1. [Scenario: edge condition → expected behavior]

### Error Scenarios
1. [Scenario: error condition → expected error or fallback]

Do NOT write code. Only produce the scenario list. The Planner converts this
into a test plan; the Implementer writes the code.
