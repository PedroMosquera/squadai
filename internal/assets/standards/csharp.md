## Team Standards

### Naming Conventions

- Classes, methods, properties, and events in `PascalCase`.
- Parameters and local variables in `camelCase`.
- Private fields prefixed with `_` and in `camelCase` (`_userRepository`).
- Constants in `PascalCase` (not `UPPER_SNAKE_CASE` — follow .NET convention).
- Interfaces prefixed with `I`: `IUserRepository`, `ILogger`.
- Avoid abbreviations. Use `GetCustomer` not `GetCust`.

### Nullable Reference Types

- Enable nullable reference types project-wide (`<Nullable>enable</Nullable>`).
- Use `T?` only when null is a meaningful value. Default to non-nullable.
- Check nullable parameters at public API boundaries with `ArgumentNullException.ThrowIfNull()`.
- Avoid the null-forgiving operator (`!`) except in tests or where nullability is guaranteed by context.

### Error Handling

- Throw specific exceptions with descriptive messages. Never throw `Exception` directly.
- Catch specific exception types. Avoid bare `catch` or `catch (Exception)` without rethrowing.
- Use `when` filters for conditional catch: `catch (HttpRequestException ex) when (ex.StatusCode == 404)`.
- Wrap lower-level exceptions in domain exceptions before propagating across boundaries.
- Use `IResult` or `Result<T>` patterns for expected failures in business logic. Reserve exceptions for unexpected errors.

### Async/Await

- Use `async`/`await` for all I/O-bound operations. Never block with `.Result` or `.Wait()`.
- Suffix async methods with `Async`: `GetUserAsync()`.
- Pass `CancellationToken` through all async call chains.
- Use `ConfigureAwait(false)` in library code. Omit it in application code.
- Prefer `ValueTask<T>` over `Task<T>` for methods that frequently complete synchronously.

### Dependency Injection

- Use constructor injection exclusively. Avoid service locator patterns.
- Register services in `Program.cs` or dedicated `IServiceCollection` extension methods.
- Define dependencies as interfaces. Keep constructors focused (under 5 parameters).
- Use `IOptions<T>` for configuration. Avoid injecting raw configuration strings.

### Testing

- Use xUnit or NUnit. Do not mix frameworks within a project.
- Use Moq or NSubstitute for mocking. Prefer interface-based mocking.
- Follow Arrange-Act-Assert structure. One logical assertion per test.
- Name tests descriptively: `GetUser_WithInvalidId_ThrowsNotFoundException`.
- Use `[Theory]`/`[InlineData]` (xUnit) or `[TestCase]` (NUnit) for parameterized tests.
- Place test projects in a `tests/` directory mirroring the `src/` structure.

### LINQ

- Use LINQ for collection queries. Prefer method syntax over query syntax for consistency.
- Avoid LINQ in performance-critical loops. Benchmark when in doubt.
- Chain operations clearly. Put each clause on its own line for readability.
- Use `Any()` over `Count() > 0`. Use `FirstOrDefault()` over `Where().First()`.

### Project Structure

- One project per assembly. Keep `.csproj` files minimal.
- Organize by feature: `Features/Users/`, `Features/Orders/`.
- Separate domain logic from infrastructure. Use a `Domain/` project with no framework dependencies.
- Use `Directory.Build.props` for shared build settings across projects.

### Formatting

- Use the project `.editorconfig` for formatting rules.
- Run `dotnet format` before committing. Never commit unformatted code.
- Keep `using` directives sorted and inside the namespace (or use global usings).
- Use file-scoped namespaces (`namespace Foo;`) in C# 10+.