---
name: systematic-debugging
description: 4-phase debugging protocol for diagnosing and fixing failing tests
methodology: tdd
---

# Systematic Debugging Skill

Diagnose and fix failing tests using a structured 4-phase protocol.
This skill is used by the Debugger when tests are failing and the cause
is not immediately obvious.

## The 4-Phase Protocol

### Phase 1: REPRODUCE

1. Identify the failing test(s) exactly.
   - Run the test suite and capture the exact failure output
   - Note the assertion that failed and the expected vs actual values
   - Record whether the failure is consistent or flaky

2. Create a minimal reproduction.
   - Isolate the failing test from its suite
   - Remove any unrelated setup or fixtures
   - Confirm the minimal test still fails

3. Document what you know.
   - When did the test start failing? (after which commit?)
   - Does it fail on all machines or just some environments?
   - Is the failure deterministic?

### Phase 2: ISOLATE

1. Identify the failure boundary.
   - Is the bug in the test itself (wrong assertion) or in the code under test?
   - Add temporary logging or debug output at key points
   - Use a binary search approach: remove half the code, check if still failing

2. Check common causes first.
   - Off-by-one errors in loops
   - Nil pointer dereference or missing initialization
   - Incorrect error handling (error swallowed, wrong type)
   - State leaking between tests (missing cleanup)
   - Race condition (run with `-race` flag)
   - Wrong mock or stub behavior

3. Trace the data flow.
   - Follow the input through each transformation
   - Compare the actual intermediate values against expected values
   - Find the first point where actual diverges from expected

### Phase 3: FIX

1. Make the smallest targeted fix.
   - Change ONLY what is necessary to fix the identified root cause
   - Do not refactor or clean up while fixing
   - If the fix requires a larger change, note it but keep it separate

2. Verify the fix addresses the root cause.
   - Explain in a comment why the fix works
   - Ensure the fix does not introduce new edge cases

3. Run the previously-failing test — it must now pass.

### Phase 4: VERIFY

1. Run the full test suite — ensure no regressions.
2. Run with race detection: `go test -race ./...`
3. If the bug was subtle, add a regression test that would have caught it.
4. Document the root cause and fix for future reference.

## Output Format

```
## Debug Report

### Failure Summary
- Test: <test name>
- Assertion: expected <X>, got <Y>
- Consistent/Flaky: <consistent|flaky>

### Root Cause
<Explanation of what caused the failure>

### Fix Applied
<Description of the change and why it works>

### Verification
- Failing test now passes: ✓
- Full suite passing: ✓
- Race detector clean: ✓

### Regression Test Added
<Yes/No and why>
```
