## Team Standards

### Code Quality

- Write clear, self-documenting code. Use meaningful variable and function names.
- Keep functions focused on a single responsibility.
- Avoid deep nesting. Extract logic into helper functions when depth exceeds 3 levels.
- Remove dead code and unused imports. Do not comment out code for later.
- Prefer explicit over clever. Readability counts more than conciseness.

### Error Handling

- Handle errors explicitly. Never silently ignore failures.
- Provide descriptive error messages with enough context for debugging.
- Validate inputs at system boundaries (API handlers, CLI, file I/O).
- Distinguish between expected errors (user input) and unexpected errors (bugs).
- Log errors with context: what was attempted, what failed, and what input caused it.

### Testing

- Write tests for all new functionality before considering it complete.
- Follow Arrange-Act-Assert (or Given-When-Then) structure.
- Test edge cases: empty inputs, boundary values, error conditions.
- Name tests descriptively so failures are self-explanatory.
- Mock external dependencies (APIs, databases, filesystems), not internal logic.
- Keep tests independent. No test should depend on another test's state.

### Naming Conventions

- Use descriptive names that convey purpose, not implementation.
- Be consistent with the project's existing naming conventions.
- Avoid abbreviations unless they are universally understood (e.g., `id`, `url`).
- Name boolean variables as questions: `isValid`, `hasPermission`, `canRetry`.

### Version Control

- Use conventional commit format: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`.
- Keep commits atomic. Each commit should represent one logical change.
- Write commit messages that explain why, not just what.
- Keep the first line under 72 characters.

### Code Organization

- Respect existing project structure and patterns.
- Keep domain logic free of infrastructure concerns.
- Avoid circular dependencies between modules.
- DRY (Don't Repeat Yourself), but not at the cost of readability.
- Document public APIs. Internal code should be self-explanatory.
