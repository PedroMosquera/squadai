## Team Standards

### Functional Style

- Prefer immutable data structures. Use `val` by default; reach for `var` only when mutability is unavoidable.
- Use `case class` for value types — they get `equals`, `hashCode`, `copy`, and pattern matching for free.
- Use pattern matching over `instanceOf` checks and type casts. Exhaustive matching on sealed traits is enforced by the compiler.
- Use for-comprehensions (`for { a <- fa; b <- fb } yield expr`) for monadic composition — avoid nested `flatMap` chains.
- Keep functions pure where possible; isolate side effects at the boundary of the program.

### Naming Conventions

- Methods, values, and variables in `camelCase`. Types, objects, and traits in `PascalCase`.
- Avoid abbreviations — `userRepository` not `usrRepo`, `NetworkException` not `NetEx`.
- Type parameters should be meaningful when context allows (`Key`, `Value`) and single-letter only when the convention is universal (`F[_]`, `A`).
- Constants in companion objects in `PascalCase` (`MaxRetries`, `DefaultTimeout`).
- Boolean-returning methods start with `is`, `has`, or `can`.

### Error Handling

- Use `Either[Error, A]` (or `EitherT`) for expected failures that callers must handle.
- Use `Option[A]` for absent values — never `null` in Scala code.
- Use `Try` only at JVM interop boundaries where Java methods throw. Immediately convert to `Either` for internal use.
- Never catch `Throwable`, `Error`, or `Exception` broadly. Catch only the specific type you can actually handle.
- Define a sealed hierarchy of error types per domain module to make error handling exhaustive and self-documenting.

### Testing

- Use ScalaTest with `FunSuite` (unit) or `WordSpec` (BDD-style) consistently across the project.
- Use MUnit as a lightweight alternative for new projects — it integrates well with Scala 3 and Cats Effect.
- Add property-based tests with ScalaCheck for non-trivial logic. Derive `Arbitrary` instances for domain types.
- Name tests to describe behaviour, not implementation: `"returns Left when the user is not found"`.
- Co-locate test files with sources under `src/test/scala` mirroring the `src/main/scala` structure.

### Tooling

- Run `scalafmt` before every commit. Configure CI to fail on drift (`--check` mode).
- Use WartRemover or Scalafix rules for linting. Enable `DisableSyntax` to ban `null`, `throw`, and `return`.
- Build with sbt. Use incremental compilation (default) and enable `sbt-revolver` for fast dev cycles.
- Use Metals (VS Code or IntelliJ) for IDE support — it provides accurate type inference and navigation.
- Run `sbt test` in CI with parallelism enabled. Cache the `~/.ivy2` and `~/.sbt` directories.

### Type System

- Annotate all public method signatures explicitly — rely on inference only for local values.
- Model domain types with sealed trait hierarchies (ADTs): `sealed trait Shape; case class Circle(...) extends Shape`.
- Prefer `opaque type UserId = String` (Scala 3) over raw `String` aliases to enforce domain boundaries.
- Use `given`/`using` (Scala 3) over implicit parameters where possible — it is clearer and easier to trace.
- Avoid type projections and complex structural types; they compile slowly and confuse readers.
