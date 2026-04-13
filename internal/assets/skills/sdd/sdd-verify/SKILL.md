---
name: sdd-verify
description: Spec compliance verification for spec-driven development
methodology: sdd
---

# SDD Verify Skill

Verify that the implementation matches the specification exactly. This is
the final phase of the SDD pipeline — a systematic compliance check.

## Steps

1. **Set up the verification matrix**: Create a checklist from the spec.
   - List every interface contract from the spec
   - List every success criterion
   - List every edge case behavior
   - List every non-functional requirement

2. **Verify interface contracts**: Check each function signature and behavior.
   - Does the function exist with the correct signature?
   - Does it return the correct types?
   - Does it return the correct error types as specified?
   - Are all side effects present?
   - Are all invariants maintained?

3. **Verify success criteria**: Test each acceptance criterion.
   - Write a test for each "Given/When/Then" in the spec if not already present
   - Run the tests — each must pass
   - If a criterion has no test, flag it as untested

4. **Verify edge case behavior**: Test the edge case table.
   - For each row in the spec's Edge Case Behavior table, confirm the
     implementation produces the specified behavior
   - If any edge case is not tested, write a test and run it

5. **Verify non-functional requirements**: Check constraints.
   - Performance: run a benchmark if latency/throughput was specified
   - Security: verify input validation for all untrusted inputs
   - Reliability: check retry and timeout behavior if specified

6. **Identify deviations**: Flag any mismatch between spec and implementation.
   - **Omission**: spec item not implemented
   - **Addition**: behavior not in spec was added
   - **Deviation**: behavior differs from what spec says
   - **Ambiguity**: spec is unclear; implementation made an assumption

7. **Produce the verification report**: Summarize findings for the orchestrator.

## Output Format

```
## Verification Report: <feature name>

### Interface Contracts
| Contract | Status | Notes |
|----------|--------|-------|
| FunctionName signature | ✓ PASS | |
| FunctionName error types | ✓ PASS | |
| SideEffect X | ✗ FAIL | Missing in implementation |

### Success Criteria
| Criterion | Test | Status |
|-----------|------|--------|
| Given X, when Y, then Z | TestFunctionName_Z | ✓ PASS |

### Edge Cases
| Condition | Expected | Actual | Status |
|-----------|----------|--------|--------|
| empty input | return ErrEmpty | returns nil | ✗ FAIL |

### Non-Functional Requirements
| Requirement | Verified By | Status |
|-------------|-------------|--------|
| latency < 100ms | BenchmarkFunctionName | ✓ PASS |

### Deviations
1. **Omission**: <spec item> not implemented. Severity: blocking/non-blocking.
2. **Addition**: <behavior> not in spec. Should be removed or spec updated.

### Verdict
PASS / FAIL — <summary>

### Required Actions Before Merge
1. <action>
```
