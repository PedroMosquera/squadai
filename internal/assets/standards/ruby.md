## Team Standards

### Naming Conventions

- Methods, variables, and symbols in `snake_case`.
- Classes and modules in `PascalCase`. Constants in `UPPER_SNAKE_CASE`.
- Predicate methods end with `?`. Dangerous (mutating) methods end with `!`.
- File names in `snake_case`, matching the class they define.
- Avoid abbreviations unless universally understood (`id`, `url`, `config`).

### Code Style

- Follow RuboCop defaults unless the project overrides them in `.rubocop.yml`.
- Run `rubocop` and fix all offenses before committing.
- Keep methods short. Extract when exceeding 15 lines.
- Prefer `each`, `map`, `select`, `reject` over manual loops.
- Use guard clauses to reduce nesting: `return unless valid?` over wrapping in `if valid?`.
- Use string interpolation (`"Hello, #{name}"`) over concatenation.

### Error Handling

- Rescue specific exceptions. Never use bare `rescue` without an exception class.
- Define custom error classes inheriting from `StandardError` for domain-specific failures.
- Use `raise` with a descriptive message. Include context about what operation failed.
- Avoid using exceptions for control flow. Use return values for expected conditions.
- Log errors with sufficient context for debugging before re-raising when appropriate.

### Testing

- Use RSpec or Minitest consistently within the project. Do not mix frameworks.
- RSpec: use `describe`/`context`/`it` structure. Keep descriptions readable as sentences.
- Follow Arrange-Act-Assert pattern. One expectation concept per example.
- Use `let` and `let!` for test setup. Prefer `build` over `create` in factory-based tests.
- Test edge cases: nil inputs, empty collections, boundary values.
- Name test files to mirror implementation: `app/models/user.rb` -> `spec/models/user_spec.rb`.

### Gem Management

- Use Bundler for all dependency management. Never install gems globally for project work.
- Pin gem versions in `Gemfile` using pessimistic constraints (`~> 1.2`).
- Run `bundle update` deliberately, not as part of routine development.
- Keep `Gemfile.lock` committed to version control.

### Documentation

- Use YARD-style documentation for public classes and methods.
- Include `@param`, `@return`, and `@raise` tags where applicable.
- Keep the first line as a concise summary (one sentence).
- Document non-obvious behavior and side effects.

### Rails Conventions (if applicable)

- Follow Rails conventions for directory structure and naming.
- Keep controllers thin. Move business logic to service objects or models.
- Use strong parameters. Never trust user input.
- Prefer scopes over class methods for query composition.
- Use database migrations for all schema changes. Never modify the schema manually.

### Metaprogramming

- Use metaprogramming sparingly. Prefer explicit code over `method_missing` and `define_method`.
- Document any dynamic method generation clearly.
- Always define `respond_to_missing?` alongside `method_missing`.
- Prefer `Module#prepend` over `alias_method` chains for method wrapping.

### Code Organization

- Keep classes focused on a single responsibility.
- Extract shared behavior into modules (mixins) or concerns.
- Avoid deep inheritance hierarchies. Prefer composition.
- Organize files by feature or domain concept, not by technical layer.