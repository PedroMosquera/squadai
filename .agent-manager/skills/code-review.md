---
name: code-review
description: Structured code review with severity-based findings
---

# Code Review Skill

Perform a structured code review of the specified code. Follow this protocol
to produce consistent, actionable feedback.

## Steps

1. **Understand context**: Read the code and any related files to understand
   the purpose and design of the change.

2. **Check correctness**: Verify that the logic is correct.
   - Are all code paths handled?
   - Are edge cases covered (null, empty, boundary values)?
   - Are error conditions handled properly?

3. **Check error handling**: Review how errors are managed.
   - Are errors propagated with sufficient context?
   - Are there silent failures or swallowed exceptions?
   - Are retries and timeouts handled where appropriate?

4. **Check naming and clarity**: Evaluate readability.
   - Are variable and function names descriptive?
   - Is the code self-documenting or does it need comments?
   - Are there magic numbers or unexplained constants?

5. **Check test coverage**: Assess testing.
   - Are new code paths tested?
   - Are edge cases and error paths tested?
   - Do tests follow the project's testing conventions?

6. **Check security**: Look for security concerns.
   - Are there injection vulnerabilities (SQL, command, template)?
   - Are secrets or credentials hardcoded?
   - Is user input validated before use?

7. **Check performance**: Identify performance issues.
   - Are there unnecessary allocations or copies?
   - Are there N+1 query patterns or redundant I/O?
   - Are there unbounded loops or missing pagination?

## Output Format

List findings grouped by severity:

### Critical
Issues that must be fixed before merging (bugs, security, data loss).

### Warning
Issues that should be addressed but are not blocking (style, minor inefficiencies).

### Suggestion
Optional improvements (readability, alternative approaches, nice-to-haves).

If no issues are found in a category, omit that section.