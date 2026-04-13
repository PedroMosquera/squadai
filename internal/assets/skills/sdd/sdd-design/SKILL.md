---
name: sdd-design
description: Architecture and interface design for spec-driven development
methodology: sdd
---

# SDD Design Skill

Translate a specification into a concrete architecture. Define module
boundaries, interface contracts, and data flow before any code is written.

## Steps

1. **Identify modules**: Break the system into coherent units.
   - Each module should have a single responsibility
   - Modules communicate through defined interfaces (not direct access)
   - Internal implementation of each module is hidden from others

2. **Define interfaces**: Specify the contract between modules.
   - Interface name and methods with full signatures
   - Which module implements each interface
   - Which modules consume each interface
   - Avoid leaking implementation details through interfaces

3. **Design data flow**: Map how data moves through the system.
   - Input sources and entry points
   - Transformation steps
   - Storage points (databases, files, caches)
   - Output destinations

4. **Draw architecture diagrams**: Use text-based diagrams.
   ```
   [Input] → [Parser] → [Validator] → [Processor] → [Output]
                                            ↓
                                       [Storage]
   ```

5. **Specify error propagation**: Define how errors flow.
   - Which errors are recoverable vs fatal?
   - Where are errors wrapped with context?
   - Which errors bubble up to the caller vs are handled locally?
   - Sentinel error values for expected failures

6. **Identify extension points**: Design for known future needs.
   - Where will new implementations be added (registry, plugin patterns)?
   - Which interfaces should be kept minimal to avoid coupling?
   - What should remain internal to allow refactoring?

7. **Validate against the spec**: Check each design decision against constraints.
   - Does the design satisfy all interface contracts in the spec?
   - Does it meet non-functional requirements?
   - Is it testable (can dependencies be injected or mocked)?

## Output Format

```
## Design: <feature name>

### Module Structure
- **<module>**: <responsibility>
  - Implements: <interface>
  - Depends on: <interfaces>

### Interfaces

#### <InterfaceName>
```go
type InterfaceName interface {
    Method(param Type) (ReturnType, error)
}
```
Implemented by: <types>
Consumed by: <modules>

### Data Flow
```
[source] → [module] → [module] → [destination]
```

### Error Handling
| Error | Type | Where handled | How propagated |
|-------|------|---------------|----------------|

### Architecture Diagram
```
<ASCII diagram>
```

### Design Decisions
1. <decision>: <rationale and alternatives considered>
```
