# SDD Development Workflow

> Windsurf workflow for SDD (Specification-Driven Development) methodology. Each phase below represents a step in the development pipeline.

## Phase 1: Exploration

- Survey existing codebase for related types, interfaces, and patterns.
- Identify architectural constraints: module boundaries, dependency rules, API contracts.
- Document current behavior that the change must preserve or extend.
- List open questions and unknowns that the proposal must address.
- Record findings in a structured exploration summary.

## Phase 2: Proposal

- Draft an approach document describing the intended change and its scope.
- Include at least two alternatives with pros, cons, and risk assessment.
- State the recommended option with clear justification.
- Identify dependencies on other teams, services, or external systems.
- Get the proposal approved (or self-approve with rationale) before proceeding.

## Phase 3: Specification Writing

- Define all new and modified types, interfaces, and function signatures.
- Specify preconditions, postconditions, and invariants for each contract.
- Document error cases with expected error types and messages.
- Include example inputs and outputs for non-trivial operations.
- Write the spec in a format that can be directly verified during implementation.

## Phase 4: Design

- Map the spec to concrete packages, files, and component boundaries.
- Define data flow between components with sequence or flow diagrams.
- Choose patterns (repository, adapter, pipeline) and justify each choice.
- Identify interfaces that enable testing via dependency injection.
- Produce a design document that an implementer can follow without ambiguity.

## Phase 5: Task Planning

- Decompose the design into ordered implementation tasks with clear boundaries.
- Define the dependency graph: which tasks block others and which are parallel.
- Assign acceptance criteria per task derived from the specification.
- Estimate effort and flag tasks that require spike or prototype work.
- Write the plan as a sequential checklist with verification steps.

## Phase 6: Implementation

- Implement each task strictly following the design and specification.
- Write tests that verify the spec contracts, not just code paths.
- Do not deviate from the design without updating the design document first.
- Commit each task separately with a conventional commit referencing the spec.
- Run the full test suite after each task to catch integration issues early.

## Phase 7: Verification

- Verify every spec contract has a corresponding passing test.
- Check that all preconditions and postconditions are enforced in code.
- Run linting, static analysis, and race detection on the final implementation.
- Compare the implementation against the design document for drift.
- Produce a verification report listing each spec item and its test status.

## Phase 8: Orchestration

- Enforce phase ordering: no implementation before design approval.
- Track spec compliance throughout every phase transition.
- Delegate to the appropriate phase when context window exceeds 60% capacity.
- On compaction or context loss, read AGENTS.md, the spec, and git log to recover.
- Maintain a traceability matrix from spec items to tests to commits.
