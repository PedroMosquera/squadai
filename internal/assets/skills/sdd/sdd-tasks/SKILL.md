---
name: sdd-tasks
description: Dependency-ordered task breakdown for spec-driven development
methodology: sdd
---

# SDD Tasks Skill

Break a design into a dependency-ordered list of implementation tasks.
Each task must be independently completable and testable.

## Steps

1. **Identify leaf-level tasks**: Find the smallest independently implementable units.
   - Each task should produce a working, tested component
   - Tasks should be completable in 30 min – 4 hours
   - Tasks that cannot be tested independently need to be split further

2. **Identify dependencies**: Map which tasks depend on others.
   - Task B depends on Task A if B requires A's output to compile or run
   - Mark dependencies explicitly (not just "nice to have A first")
   - Identify the critical path (longest chain of dependencies)

3. **Build a dependency graph**: Visualize the execution order.
   ```
   Task 1 (no deps)
   Task 2 (no deps)
   Task 3 (depends: Task 1)
   Task 4 (depends: Task 1, Task 2)
   Task 5 (depends: Task 3, Task 4)
   ```

4. **Group into phases**: Batch independent tasks for parallel execution.
   - Phase 1: Tasks with no dependencies
   - Phase 2: Tasks whose dependencies are complete after Phase 1
   - Continue until all tasks are assigned

5. **Estimate effort**: Assign complexity to each task.
   - **S** (Simple): ≤ 30 min — single function, no new abstraction
   - **M** (Medium): 30 min–2 hrs — new type or interface, some design needed
   - **L** (Large): 2–4 hrs — multiple components, requires careful integration
   - Tasks estimated **L** or larger should be split if possible

6. **Write acceptance criteria per task**: Each task needs a clear done condition.
   - What tests must pass?
   - What behavior must be observable?
   - What artifacts must exist (new file, new type, updated interface)?

## Output Format

```
## Task Plan: <feature name>

### Dependency Graph
```
<ASCII or text dependency diagram>
```

### Phase 1 (parallel)
- [ ] Task 1: <description> [S]
  Depends on: none
  Done when: <acceptance criteria>

- [ ] Task 2: <description> [M]
  Depends on: none
  Done when: <acceptance criteria>

### Phase 2 (parallel after Phase 1)
- [ ] Task 3: <description> [M]
  Depends on: Task 1
  Done when: <acceptance criteria>

### Summary
| Phase | Tasks | Effort |
|-------|-------|--------|
| 1 | 2 | S+M |
| 2 | 1 | M |
| Total | 3 | ~3-4 hrs |
```
