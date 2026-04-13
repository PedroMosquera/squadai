---
name: sdd-spec
description: Formal specification document authoring for spec-driven development
methodology: sdd
---

# SDD Spec Skill

Write a formal specification document that the Designer and Implementer will
use as their source of truth. The spec must be complete enough that two
different developers could implement the same behavior independently.

## Steps

1. **State the problem**: Define what problem is being solved.
   - What user need or system requirement motivates this change?
   - What is currently broken or missing?
   - What is out of scope (explicit exclusions)?

2. **Define success criteria**: List measurable acceptance tests.
   - Each criterion must be testable (observable and verifiable)
   - Use "Given/When/Then" or precondition/action/postcondition format
   - Cover both functional and non-functional criteria

3. **Specify the interface contract**: Define the public surface precisely.
   - Function/method signatures with parameter and return types
   - Error types and when they are returned
   - Side effects (files created, state changed, events emitted)
   - Invariants that must hold before and after each operation

4. **Specify data structures**: Define all types involved.
   - Fields, types, and optionality
   - Validation rules (allowed values, format constraints)
   - Serialization format (JSON schema, protobuf, etc.)

5. **Specify behavior for edge cases**: Cover boundary conditions explicitly.
   - What happens with empty input?
   - What happens with maximum-size input?
   - What happens if a dependency is unavailable?
   - What partial failure behavior is acceptable?

6. **Specify non-functional requirements**: Add constraints beyond behavior.
   - Performance: maximum latency, throughput, memory usage
   - Security: authentication, authorization, input sanitization
   - Reliability: retry behavior, timeout, fallback

## Output Format

```
## Specification: <feature name>

### Problem Statement
<1-3 sentences describing what problem this solves>

### Scope
**In scope**: <what this spec covers>
**Out of scope**: <explicit exclusions>

### Success Criteria
1. Given <precondition>, when <action>, then <expected outcome>
2. ...

### Interface Contract

#### <FunctionName>(params) (returnType, error)
- **Purpose**: <what it does>
- **Parameters**: <name type — description>
- **Returns**: <description of return value>
- **Errors**: <error type — when it is returned>
- **Side effects**: <what changes as a result>
- **Invariants**: <what must be true before/after>

### Data Structures

#### <TypeName>
| Field | Type | Required | Description |
|-------|------|----------|-------------|

### Edge Case Behavior
| Condition | Expected Behavior |
|-----------|------------------|

### Non-Functional Requirements
- **Performance**: <requirement>
- **Security**: <requirement>
- **Reliability**: <requirement>
```
