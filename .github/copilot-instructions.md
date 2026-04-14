<!-- squadai:copilot-instructions -->
## Team Standards

### Project: squadai

**Stack**: Go

### General Principles

- Follow existing patterns in the codebase. Consistency matters more than personal preference.
- Make minimal, focused changes. Avoid unrelated modifications in the same changeset.
- Every change should be reviewable: clear purpose, limited scope, easy to verify.

### Verification Commands
- Tests: `go test ./...`
- Build: `go build ./...`
- Lint: `go vet ./...`

### Code Style

- Follow existing patterns in the codebase
- Write tests for new functionality
- Use clear, descriptive naming
- Keep functions focused on a single responsibility
- Remove dead code and unused imports

### Error Handling

- Handle errors explicitly. Never silently ignore failures.
- Provide descriptive error messages with context for debugging.
- Validate inputs at system boundaries.

### Commit Messages

- Use conventional commit format (feat:, fix:, docs:, test:, refactor:)
- Keep the first line under 72 characters
- Explain why the change was made, not just what changed

### Architecture

- Respect package boundaries and import rules
- Keep domain logic free of infrastructure concerns
- All mutating operations must support dry-run mode
<!-- /squadai:copilot-instructions -->
