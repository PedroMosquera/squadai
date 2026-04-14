## Team Standards

### Null Safety

- Enable sound null safety across the entire codebase — no `// ignore: null_safety` suppressions.
- Avoid the `!` (bang) operator except where non-nullability is provably guaranteed at the call site.
- Use null-aware operators by default: `??` for fallback values, `?.` for safe member access, `??=` for lazy initialization.
- Use `late` only when the variable is guaranteed to be initialized before first read and `final` alone cannot express it.
- Prefer nullable return types with explicit null handling over throwing on absent values.

### Naming Conventions

- Variables, functions, and named parameters in `lowerCamelCase`.
- Types (classes, enums, extensions, mixins) in `UpperCamelCase`.
- File names in `snake_case` (`user_repository.dart`, not `UserRepository.dart`).
- Private fields and methods prefixed with `_`. No `m_` or Hungarian-style prefixes.
- Boolean-returning functions start with `is`, `has`, or `can` (`isLoading`, `hasError`).

### Error Handling

- Throw typed exceptions (`class NetworkException implements Exception`) — never `throw 'string'`.
- Catch specific exception types, not the generic `Exception` or `Object`.
- Use a Result/Either pattern (`sealed class Result<T>`) for expected failures in domain logic.
- Document thrown exceptions in `///` doc comments with `@throws` annotations.
- Avoid silently swallowing exceptions in `catch` blocks — always log or rethrow.

### Testing

- Use `dart test` for pure Dart packages; `flutter_test` for Flutter widgets.
- Write widget tests for every non-trivial widget; use `WidgetTester` and `find.*` matchers.
- Use golden tests (`matchesGoldenFile`) for layout-critical UI components.
- Mock dependencies with `mockito` (`@GenerateMocks`) or `mocktail`; avoid manual stub classes.
- Aim for test file co-location: `lib/src/foo.dart` → `test/src/foo_test.dart`.

### Formatting and Linting

- Run `dart format .` before every commit. The formatter is non-negotiable.
- Run `dart analyze` with `strict-casts`, `strict-inference`, and `strict-raw-types` enabled in `analysis_options.yaml`.
- Prefer `final` over `var` for all values that are not reassigned.
- Prefer `const` constructors everywhere they are applicable — the linter will tell you when.
- Avoid `dynamic` except at explicit serialization/deserialization boundaries.

### Flutter Patterns

- Use BLoC (`flutter_bloc`) or Riverpod for state management — avoid raw `setState` beyond local widget state.
- Compose UI from small, single-responsibility widgets. Prefer composition over subclassing `Widget`.
- Always use `const` constructors for stateless widgets and static subtrees.
- Keep business logic out of `build()` methods — extract to BLoC, notifier, or service classes.
- Use `ThemeData` and design tokens for colors and typography — no hardcoded hex values in widgets.
