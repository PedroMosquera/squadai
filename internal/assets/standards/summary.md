## Team Standards (summary)

Condensed under a token budget — the full standards live in `.squadai/`.

- Follow the dominant naming, layout, and error-handling conventions already
  present in this codebase; consistency beats personal preference.
- Every behavior change ships with a test; run the project's test, lint, and
  build commands before declaring work done.
- Keep functions small and intention-revealing; extract instead of nesting.
- Handle errors explicitly — no silent catches, no swallowed failures.
- Never commit secrets, credentials, or generated artifacts.
- Prefer small, reviewable changes with clear commit messages.
