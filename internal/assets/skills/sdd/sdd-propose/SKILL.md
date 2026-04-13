---
name: sdd-propose
description: Solution proposals with tradeoff analysis for spec-driven development
methodology: sdd
---

# SDD Propose Skill

Generate and evaluate solution options before committing to a design.
This skill produces a tradeoff table that the team can use to choose
the best approach.

## Steps

1. **Review the exploration report**: Understand constraints and context.
   - What must not change (API contracts, data formats)?
   - What are the performance/security requirements?
   - What areas of the codebase will be affected?

2. **Generate 2-4 solution options**: Propose meaningfully different approaches.
   - Option 1: Minimal change (least disruption, may be limited)
   - Option 2: Standard approach (industry-standard pattern)
   - Option 3: Clean-slate (ideal design, higher effort)
   - Option 4 (optional): Hybrid or incremental migration

3. **Evaluate each option**: Analyze tradeoffs honestly.
   For each option, assess:
   - **Effort**: person-hours or story points
   - **Risk**: probability and severity of breaking changes
   - **Reversibility**: how hard is it to undo if it goes wrong?
   - **Testability**: how easily can this be tested?
   - **Maintainability**: how easy to extend or modify in future?
   - **Alignment with constraints**: does it fit all requirements?

4. **Produce a recommendation**: Choose one option with reasoning.
   - State which option you recommend and why
   - Acknowledge the main tradeoff you are accepting
   - Note any blocking assumptions that must be validated first

5. **List open questions**: Flag anything that must be decided externally.
   - Questions for the product owner (scope/priority)
   - Technical unknowns requiring a spike
   - Dependencies on other teams or systems

## Output Format

```
## Proposals: <feature/change>

### Option 1: <name>
**Summary**: <one sentence description>
**Effort**: <estimate>
**Risk**: <low|medium|high> — <reason>
**Reversibility**: <easy|hard|irreversible>
**Testability**: <easy|medium|hard>
**Pros**: <bullet list>
**Cons**: <bullet list>

### Option 2: <name>
...

### Tradeoff Table

| Option | Effort | Risk | Reversible | Testable |
|--------|--------|------|------------|---------|
| Option 1 | | | | |
| Option 2 | | | | |

### Recommendation
**Recommended**: Option <N> — <rationale in 2-3 sentences>

### Open Questions
1. <question>
```
