## Team Standards

### Type Safety

- Enable TypeScript strict mode (`"strict": true` in tsconfig.json).
- Avoid `any` type. Use `unknown` when the type is genuinely uncertain.
- Define explicit return types for public functions and exported APIs.
- Use discriminated unions over optional properties where possible.
- Prefer `interface` for object shapes, `type` for unions and intersections.

### Async Patterns

- Use `async/await` over raw Promises and callbacks.
- Always handle promise rejections. Never leave promises unhandled.
- Use `Promise.all()` for independent concurrent operations.
- Avoid mixing callbacks and promises in the same code path.
- Set timeouts on network requests and external service calls.

### Error Handling

- Throw typed errors with descriptive messages, not bare strings.
- Catch specific errors rather than using bare `catch` blocks.
- Never swallow errors silently. Log or rethrow with context.
- Use custom error classes for domain-specific failure conditions.
- Validate external input at system boundaries (API handlers, CLI parsers).

### Modules and Imports

- Use ES modules (`import`/`export`), not CommonJS (`require`/`module.exports`).
- Prefer named exports over default exports for better refactoring support.
- Keep import paths consistent. Use path aliases for deep imports.
- Avoid circular dependencies between modules.

### Testing

- Use `describe`/`it` structure for organizing tests.
- Follow Arrange-Act-Assert (AAA) pattern within each test.
- Mock external boundaries (HTTP, database), not internal modules.
- Test edge cases: empty inputs, null values, error conditions.
- Name test cases descriptively: "should return X when Y".
- Aim for meaningful coverage, not coverage percentage targets.

### Code Style

- Use `const` by default. Use `let` only when reassignment is needed. Never use `var`.
- Prefer arrow functions for callbacks and inline functions.
- Use template literals for string interpolation.
- Destructure objects and arrays when accessing multiple properties.
- Keep functions short and focused. Extract when exceeding ~30 lines.

### Formatting and Linting

- Configure ESLint and Prettier. Run both before committing.
- Use consistent semicolons and quotes across the project.
- Name variables and functions in `camelCase`, types and classes in `PascalCase`.
- Constants in `UPPER_SNAKE_CASE` only for true compile-time constants.
