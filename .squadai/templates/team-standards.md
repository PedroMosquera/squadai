## Team Standards

### Error Handling

- Always wrap errors with context using `fmt.Errorf("operation: %w", err)`.
- Define sentinel errors for expected failure conditions (`var ErrNotFound = errors.New(...)`).
- Never ignore error return values. If intentionally discarding, assign to `_` with a comment.
- Use `errors.Is()` and `errors.As()` for error inspection, not string matching.
- Return errors to callers rather than logging and continuing silently.

### Naming Conventions

- Use MixedCaps (exported) and mixedCaps (unexported). No underscores in Go names.
- Package names are short, lowercase, singular nouns (`config`, not `configs` or `configuration`).
- Interface names describe behavior (`Reader`, `Validator`), not implementation.
- Acronyms are all-caps when exported (`HTTPClient`, `ID`), lowercase when not (`httpClient`).
- Test helpers start with `test` or `helper` prefix and call `t.Helper()`.

### Package Design

- Keep packages small and focused on a single responsibility.
- Avoid circular dependencies. Use interfaces at package boundaries.
- Internal packages (`internal/`) are not importable by external consumers.
- Domain types go in `domain/` with no infrastructure dependencies.
- Accept interfaces, return concrete types.

### Testing

- Use table-driven tests for cases with multiple inputs.
- Test files live alongside implementation (`foo.go` → `foo_test.go`).
- Run `go test -race ./...` to detect data races.
- Use `t.TempDir()` for filesystem tests (automatic cleanup).
- Test behavior, not implementation details. Mock at boundaries, not internals.
- Name tests descriptively: `TestFunctionName_Scenario_ExpectedResult`.

### Concurrency

- Prefer channels for communicating between goroutines.
- Use `context.Context` for cancellation and timeouts.
- Protect shared state with `sync.Mutex` when channels are not appropriate.
- Never start goroutines without a clear shutdown path.

### Formatting and Tooling

- Run `gofmt` (or `goimports`) on all files. Never commit unformatted code.
- Use `go vet ./...` as a minimum static analysis check.
- Prefer `golangci-lint` for comprehensive linting when configured.
- Keep imports grouped: stdlib, external, internal — separated by blank lines.