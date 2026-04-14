## Team Standards

### PSR Standards

- Follow PSR-1 (basic coding standard) and PSR-12 (extended coding style).
- Use PSR-4 autoloading via Composer. Class names map to file paths.
- Declare strict types in every file: `declare(strict_types=1);`.
- Use one class per file. File name matches the class name exactly.

### Naming Conventions

- Classes in `PascalCase`. Methods and properties in `camelCase`.
- Constants in `UPPER_SNAKE_CASE`.
- Namespaces mirror directory structure. Use vendor prefix: `App\Models\User`.
- Boolean methods start with `is`, `has`, or `can`.
- Avoid Hungarian notation and type prefixes.

### Type Declarations

- Add type declarations to all function parameters and return types.
- Use union types (`string|int`) and intersection types (`Countable&Iterator`) where appropriate.
- Use `?Type` for nullable parameters. Use `void` for methods that return nothing.
- Prefer strict comparisons (`===`, `!==`) over loose ones.
- Use enums (PHP 8.1+) for fixed sets of values. Avoid magic strings and constants.

### Error Handling

- Throw specific exceptions with descriptive messages. Never throw bare `\Exception`.
- Catch specific exception types. Avoid empty catch blocks.
- Define custom exception classes for domain-specific failures.
- Use `try`/`catch`/`finally` for resource cleanup.
- Validate inputs at controller and service boundaries. Fail fast with clear messages.

### Testing

- Use PHPUnit for all tests. Follow Arrange-Act-Assert structure.
- Place tests in a `tests/` directory mirroring `src/` structure.
- Name test methods descriptively: `testGetUser_WithInvalidId_ThrowsException`.
- Use data providers (`@dataProvider`) for parameterized tests.
- Mock external dependencies. Use `createMock()` or Mockery for complex scenarios.
- Run `./vendor/bin/phpunit` before committing.

### Composer and Dependencies

- Use Composer for all dependency management. Never require packages globally.
- Pin versions with caret constraint (`^8.0`). Keep `composer.lock` committed.
- Organize autoload configuration in `composer.json`. Use PSR-4 for `src/` and `tests/`.
- Run `composer validate` to verify `composer.json` correctness.

### Documentation

- Use PHPDoc blocks for all public classes, methods, and properties.
- Include `@param`, `@return`, `@throws` tags. Keep descriptions concise.
- Do not duplicate type information already expressed in type declarations.
- Document non-obvious behavior, side effects, and edge cases.

### Laravel Conventions (if applicable)

- Follow Laravel naming conventions for models, controllers, and migrations.
- Keep controllers thin. Move business logic to service classes or actions.
- Use Form Request classes for validation. Never validate in controllers directly.
- Use Eloquent scopes for reusable query conditions.
- Prefer config files and environment variables over hardcoded values.

### Symfony Conventions (if applicable)

- Use attributes for routing, validation, and serialization (Symfony 6.2+).
- Register services via autowiring. Use explicit configuration only when needed.
- Keep controllers as invokable classes when they handle a single action.
- Use the Messenger component for async operations and CQRS patterns.

### Code Organization

- Organize by feature or domain concept, not by technical layer.
- Keep domain logic free of framework dependencies where possible.
- Use interfaces for service contracts at module boundaries.
- Avoid circular dependencies between namespaces.