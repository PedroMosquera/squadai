---
name: sdd-apply
description: Spec-faithful implementation for spec-driven development
methodology: sdd
---

# SDD Apply Skill

Implement exactly what the specification says. No more, no less.
This skill is used by the Implementer after the Spec Writer and Designer
have produced their artifacts.

## Core Principle

Your job is to implement what is specified, not to improve the design.
If the spec is wrong, report it — do not fix it silently.

## Steps

1. **Read the full spec before writing any code**: Understand the complete
   interface contract before starting the first function.
   - Note all invariants and error conditions
   - Mark any ambiguities to report back to the orchestrator

2. **Implement interfaces first**: Create types and method stubs before logic.
   - Implement all interface methods (even stubs return placeholder errors)
   - This ensures compilation and enables parallel work by others
   - Run `go build ./...` after stubs are in place

3. **Implement one spec item at a time**: Work through the spec sequentially.
   - Pick one function or behavior from the spec
   - Write a test for it (even in SDD, tests validate spec compliance)
   - Implement it
   - Verify the test passes

4. **Respect the interface contract strictly**:
   - Return exactly the error types specified — no substitutions
   - Produce exactly the output format specified
   - Respect all invariants stated in the spec
   - If spec says "side effect X happens", make it happen

5. **Handle all edge cases in the spec**:
   - Check the "Edge Case Behavior" table in the spec
   - Each row must be implemented
   - Do not add undocumented behavior

6. **Do not add extras**:
   - No logging unless specified
   - No caching unless specified
   - No additional validation beyond what is specified
   - No convenience methods not in the interface

7. **Report spec gaps**: If you encounter behavior the spec does not cover:
   - Do not guess — make a conservative implementation (return error)
   - Document the gap explicitly with a `// TODO(spec): ...` comment
   - Report gaps to the orchestrator before finalizing

## Output Format

After implementing each spec item:
```
## Implementation Status

### Completed
- [x] <FunctionName>: implemented, tests passing

### Pending
- [ ] <FunctionName>: waiting for Task 2 (dependency)

### Spec Gaps Found
1. Spec does not address <condition>. Implemented conservative fallback: <what you did>.
   Needs decision before merge.
```
