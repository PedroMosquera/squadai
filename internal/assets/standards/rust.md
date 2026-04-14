## Team Standards

### Error Handling

- Use `Result<T, E>` for recoverable errors and `Option<T>` for optional values.
- Never use `.unwrap()` or `.expect()` in production code. Use `?` operator or explicit matching.
- Define custom error types with `thiserror` for library crates and `anyhow` for applications.
- Propagate errors with `?` and add context using `.context()` or `.with_context()`.
- Reserve `panic!` for programming errors (invariant violations), not runtime failures.

### Ownership and Borrowing

- Prefer borrowing (`&T`, `&mut T`) over transferring ownership when the callee does not need to store the value.
- Use `Clone` explicitly when shared ownership is needed. Avoid hidden clones in hot paths.
- Prefer `&str` over `String` in function parameters. Accept the most general type.
- Use `Cow<'_, str>` when a function may or may not need to allocate.
- Avoid lifetime annotations when the compiler can infer them. Add them only when required.

### Naming Conventions

- Types and traits in `PascalCase`. Functions, methods, and variables in `snake_case`.
- Constants and statics in `UPPER_SNAKE_CASE`.
- Modules in `snake_case`. File names match module names.
- Builder methods return `Self`. Conversion methods follow `from_*`, `to_*`, `into_*`, `as_*` conventions.
- Boolean-returning methods start with `is_`, `has_`, or `can_`.

### Testing

- Unit tests go in a `#[cfg(test)] mod tests` block at the bottom of each module.
- Integration tests go in the `tests/` directory at the crate root.
- Use `#[test]` attribute for test functions. Name them descriptively: `test_function_name_scenario`.
- Use `assert_eq!`, `assert_ne!`, and `assert!` with descriptive messages.
- Test error paths. Use `#[should_panic]` sparingly; prefer testing `Result` variants.
- Use `proptest` or `quickcheck` for property-based testing of complex logic.

### Linting and Formatting

- Run `cargo fmt` on all files. Never commit unformatted code.
- Run `cargo clippy` and fix all warnings. Use `#[allow(clippy::...)]` only with a justifying comment.
- Enable `#![deny(clippy::all)]` and `#![warn(clippy::pedantic)]` at the crate level.
- Keep `Cargo.toml` dependencies sorted alphabetically. Pin major versions.

### Unsafe Code

- Avoid `unsafe` unless absolutely necessary for FFI or performance-critical sections.
- Document every `unsafe` block with a `// SAFETY:` comment explaining the invariant.
- Encapsulate `unsafe` in safe abstractions. Never expose raw pointers in public APIs.
- Prefer safe alternatives from `std` or well-audited crates over hand-rolled unsafe code.

### Code Organization

- Keep crates focused on a single responsibility. Split large crates into workspace members.
- Use `pub(crate)` visibility by default. Only make items `pub` when they are part of the public API.
- Define traits for abstraction boundaries. Use generics with trait bounds over trait objects when performance matters.
- Avoid circular dependencies between workspace members.
- Prefer composition over deep type hierarchies.