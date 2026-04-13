---
name: sdd-explore
description: Codebase analysis and context gathering for spec-driven development
methodology: sdd
---

# SDD Explore Skill

Analyze an existing codebase to gather context before proposing solutions.
This is the first phase of the SDD pipeline — understand before proposing.

## Steps

1. **Identify entry points**: Find where the relevant system behavior begins.
   - Main functions, HTTP handlers, CLI commands, event listeners
   - Public API surfaces (exported functions, interfaces)
   - Configuration loading points

2. **Trace data flow**: Follow data through the system.
   - How does input enter the system?
   - What transformations does it undergo?
   - Where is data persisted or returned?
   - What are the side effects?

3. **Map dependencies**: Understand what depends on what.
   - Internal package dependencies
   - External library dependencies
   - Interface boundaries and their implementations
   - Circular dependency risks

4. **Identify hotspots**: Flag areas of concern.
   - Files changed frequently in git log
   - Code with high complexity or low test coverage
   - Undocumented magic behavior
   - Areas where the feature request will require changes

5. **Document constraints**: Note existing commitments.
   - Public API contracts that must not break
   - Performance SLAs or resource limits
   - Security requirements (auth, input validation)
   - Compatibility requirements (old data formats, deprecated paths)

6. **Summarize for the Proposer**: Produce a context document.
   - Overview of relevant system components
   - Key data structures and their relationships
   - Constraints the Proposer must respect
   - Open questions that need clarification

## Output Format

```
## Exploration Report: <topic>

### Entry Points
- <file/function>: <description of role>

### Data Flow
<brief description or diagram>

### Key Data Structures
- <type>: <description and purpose>

### Constraints
- <constraint>: <why it exists>

### Hotspots
- <area>: <risk or complexity reason>

### Open Questions
1. <question needing clarification before proposing>
```
