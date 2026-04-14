## Team Standards

### Functional Style

- Use the pipe operator (`|>`) to express data transformations as readable left-to-right pipelines.
- Keep functions small and pure — a function that transforms data should not produce side effects.
- Prefer pattern matching over `if`/`cond` chains when matching on data shapes.
- Reach for `Enum` and `Stream` functions (`map`, `filter`, `reduce`) before writing explicit recursion.
- Avoid mutable state; use process state in GenServers when mutation is genuinely required.

### Naming Conventions

- Modules in `PascalCase` (`MyApp.Accounts.User`). All other identifiers in `snake_case`.
- Functions returning booleans suffixed with `?` (`valid?/1`, `admin?/1`).
- Functions that raise on failure suffixed with `!` (`get!/1`, `create!/1`).
- Private functions defined with `defp`. Keep public API surface minimal.
- Use descriptive names — Elixir is readable; abbreviations obscure intent.

### Error Handling

- Return `{:ok, result}` and `{:error, reason}` tuples as the standard success/failure protocol.
- Use `with` for chaining multiple `{:ok, _}` operations — avoid deeply nested `case` expressions.
- Reserve `raise/1` for truly unexpected, unrecoverable errors (programming mistakes, not user errors).
- Embrace the "let it crash" philosophy: supervisor trees restart failing processes; don't defensively handle every edge case.
- Log errors with `Logger` before returning `{:error, reason}` when context is useful for debugging.

### Testing

- Write ExUnit tests with descriptive names: `test "returns error when user not found"`.
- Use `describe/2` blocks to group related tests for the same function.
- Add doctests (`iex> MyModule.greet("world")`) to public functions — they double as documentation and tests.
- Mock external dependencies using `Mox` — define behaviours and inject them.
- Use `ExUnit.DataCase` (database) and `ExUnit.ConnCase` (HTTP) for integration tests in Phoenix projects.

### Tooling

- Run `mix format` before every commit. Configure CI to fail on unformatted code.
- Run `mix credo --strict` for static analysis. Fix all issues; suppress only with inline `# credo:disable-for-next-line` and a comment.
- Run `mix dialyzer` to catch type errors. Add `@spec` annotations to all public functions.
- Run `mix deps.audit` in CI to flag known-vulnerable dependencies.
- Use `mix test --cover` and track coverage trends over time.

### OTP Patterns

- Model stateful processes as `GenServer` modules. Keep `handle_call/handle_cast` implementations thin — delegate to pure helper functions.
- Define supervision trees explicitly. Use `Supervisor.child_spec/2` and `:one_for_one` / `:rest_for_one` strategies intentionally.
- Avoid module-level global state. Use `Registry` for named process lookup and `ETS` for shared read-heavy data.
- Use `Task` for one-off async work and `Task.Supervisor` for fault-tolerant async operations.
- Name processes with `{:via, Registry, ...}` — avoid bare atom registration in library code.
