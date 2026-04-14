## Team Standards

### Naming Conventions

- Classes and interfaces in `PascalCase`. Methods and variables in `camelCase`.
- Constants in `UPPER_SNAKE_CASE` (`static final` fields).
- Package names are all lowercase, following reverse domain convention.
- Boolean methods start with `is`, `has`, or `can`. Avoid negated names like `isNotValid`.
- Kotlin files: use `PascalCase` for classes, `camelCase` for functions and properties.

### Null Safety

- Use `Optional<T>` for return types that may be absent. Never return `null` from public methods.
- Kotlin: leverage the type system. Use `T?` only when nullability is meaningful.
- Annotate Java parameters with `@NonNull` or `@Nullable` from `jakarta.annotation` or `org.jetbrains.annotations`.
- Never pass `null` as a method argument unless the API explicitly accepts it.
- Use `Objects.requireNonNull()` at public API boundaries to fail fast.

### Error Handling

- Catch specific exceptions. Never catch `Exception` or `Throwable` without rethrowing.
- Use checked exceptions for recoverable conditions and unchecked for programming errors.
- Wrap low-level exceptions with domain-specific types before propagating across module boundaries.
- Include the original exception as the cause when wrapping: `new DomainException("msg", cause)`.
- Kotlin: use `Result<T>` or sealed classes for typed error handling. Avoid checked exceptions.

### Dependency Injection

- Use constructor injection. Avoid field injection (`@Autowired` on fields).
- Define dependencies as interfaces, not concrete classes.
- Keep the number of constructor parameters below 5. Extract a parameter object if needed.
- Use `@Configuration` classes for bean definitions. Avoid component scanning of third-party packages.

### Testing

- Use JUnit 5 (`@Test`, `@BeforeEach`, `@ParameterizedTest`).
- Use Mockito for mocking. Prefer `@ExtendWith(MockitoExtension.class)` over manual setup.
- Follow Arrange-Act-Assert structure. One assertion concept per test.
- Name tests descriptively: `shouldReturnEmpty_whenInputIsNull`.
- Use `@Nested` classes to group related test scenarios.
- Kotlin: use JUnit 5. Consider `kotest` for property-based testing.

### Build and Dependencies

- Use Gradle (Kotlin DSL preferred) or Maven. Keep build scripts clean and well-structured.
- Pin dependency versions. Use a BOM (`dependencyManagement`) for consistent transitive versions.
- Run `./gradlew check` or `mvn verify` before committing.
- Keep the Java version explicit in build configuration. Use LTS releases (17, 21).

### Code Style

- Keep methods short. Extract when exceeding 30 lines.
- Prefer immutable objects. Use `final` on fields, `record` types (Java 16+), or Kotlin `data class`.
- Use streams and lambdas for collection transformations, but not for control flow.
- Avoid raw types. Always parameterize generic types.
- Follow the project formatter (google-java-format, Spotless, or ktlint). Never commit unformatted code.

### Package Structure

- Organize by feature, not by layer (`user/` over `controller/`, `service/`, `repository/`).
- Keep domain logic free of framework annotations where possible.
- Use `internal` or package-private visibility to enforce module boundaries.
- Avoid circular dependencies between packages.