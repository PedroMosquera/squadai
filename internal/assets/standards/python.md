## Team Standards

### Type Hints

- Add type hints to all public function signatures and class methods.
- Use `-> None` for functions that do not return a value.
- Prefer `list[str]` over `List[str]` (Python 3.9+ built-in generics).
- Use `Optional[X]` or `X | None` for nullable parameters.
- Define `TypedDict` or dataclasses for structured data, not raw dicts.

### Formatting and Linting

- Use `black` for code formatting. Never commit unformatted code.
- Use `ruff` or `flake8` for linting. Fix all warnings before committing.
- Configure `isort` for import ordering (compatible with black).
- Keep line length at the project default (88 for black, 79 for PEP 8).

### Virtual Environments

- Always use a virtual environment (`venv`, `poetry`, or `conda`).
- Pin dependency versions in `requirements.txt` or `pyproject.toml`.
- Never install packages globally for project work.
- Document environment setup in README or a setup script.

### Testing

- Use `pytest` as the test runner.
- Prefer fixtures over `setUp`/`tearDown` methods.
- Use `tmp_path` fixture for filesystem tests.
- Name test functions descriptively: `test_function_name_scenario_expected`.
- Test edge cases: empty inputs, None values, boundary conditions.
- Use `pytest.raises` for expected exceptions with specific exception types.

### Naming Conventions

- Functions and variables in `snake_case`.
- Classes in `PascalCase`.
- Constants in `UPPER_SNAKE_CASE`.
- Private attributes and methods prefixed with `_`.
- Avoid single-letter variable names except in comprehensions and loops.

### Error Handling

- Catch specific exceptions, never bare `except:` or `except Exception:`.
- Raise exceptions with descriptive messages.
- Use custom exception classes for domain-specific errors.
- Use context managers (`with` statements) for resource management.
- Log errors with sufficient context for debugging.

### Docstrings

- Use Google-style docstrings for all public functions, classes, and modules.
- Include `Args:`, `Returns:`, and `Raises:` sections where applicable.
- Keep the first line as a concise summary (one sentence).
- Document non-obvious behavior and side effects.

### Code Organization

- Keep modules focused on a single responsibility.
- Use `__init__.py` to define public APIs for packages.
- Avoid circular imports. Use late imports if necessary.
- Separate business logic from I/O and infrastructure concerns.
