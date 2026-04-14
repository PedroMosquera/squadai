## Team Standards

### Naming Conventions

- Types and protocols in `PascalCase`. Properties, methods, and variables in `camelCase`.
- Constants in `camelCase` (Swift convention, not `UPPER_SNAKE_CASE`).
- Enum cases in `camelCase`. Use descriptive names: `.invalidInput` over `.error1`.
- Name methods as verb phrases for actions (`fetchUser`), noun phrases for properties (`userName`).
- Omit needless words. Follow Swift API Design Guidelines for clarity at the point of use.

### Optional Handling

- Use `guard let` for early exits. Use `if let` for conditional binding within a scope.
- Never force-unwrap (`!`) in production code. Use `guard let` or `if let` instead.
- Prefer `??` (nil-coalescing) for default values over conditional checks.
- Use `Optional.map` and `Optional.flatMap` for chaining transformations on optionals.
- Mark APIs that cannot return nil with non-optional types. Reserve `?` for genuinely absent values.

### Error Handling

- Use `throws` for functions that can fail. Catch errors at the appropriate call site.
- Define error types as enums conforming to `Error`. Include associated values for context.
- Use `Result<Success, Failure>` for async callbacks or when errors must be stored.
- Avoid `try!` in production code. Use `try?` only when the error is intentionally discarded.
- Prefer `do`/`catch` blocks with pattern matching on specific error cases.

### Protocol-Oriented Programming

- Prefer protocols over base classes for shared behavior.
- Use protocol extensions to provide default implementations.
- Keep protocols focused. Prefer multiple small protocols over one large one.
- Use `some` (opaque types) and `any` (existential types) deliberately. Prefer `some` for return types.
- Conform to standard protocols (`Equatable`, `Hashable`, `Codable`) where appropriate.

### Testing

- Use XCTest for unit and integration tests.
- Name tests descriptively: `testFetchUser_WithInvalidID_ThrowsNotFoundError`.
- Follow Arrange-Act-Assert structure. One assertion concept per test.
- Use `XCTAssertEqual`, `XCTAssertThrowsError`, and `XCTAssertNil` with descriptive messages.
- Mock dependencies using protocols. Inject mocks through initializers.
- Place test files in a `Tests/` directory mirroring the source structure.

### Codable and Data

- Use `Codable` for JSON serialization. Define `CodingKeys` when property names differ from JSON keys.
- Prefer `struct` over `class` for data models. Use `class` only when reference semantics are needed.
- Make data types immutable by default (`let` properties). Use `var` only when mutation is required.
- Use `JSONDecoder`/`JSONEncoder` with explicit date and key strategies.

### Concurrency

- Use Swift concurrency (`async`/`await`, `Task`, `Actor`) over GCD and completion handlers.
- Mark shared mutable state with `@MainActor` or isolate with custom actors.
- Use `TaskGroup` for structured concurrency. Avoid detached tasks unless necessary.
- Handle cancellation with `Task.checkCancellation()` and `Task.isCancelled`.

### SwiftLint

- Run SwiftLint and fix all warnings before committing.
- Configure rules in `.swiftlint.yml`. Disable rules only with a justifying comment.
- Enable `force_unwrapping`, `force_cast`, and `force_try` rules as errors, not warnings.

### Code Organization

- Use extensions to group protocol conformances and related functionality.
- Mark access control explicitly: `private`, `fileprivate`, `internal`, `public`.
- Keep files focused. One primary type per file with extensions in the same file or separate files.
- Organize by feature: `Features/User/`, `Features/Settings/`.
- Use Swift Package Manager for dependency management and modularization.