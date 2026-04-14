## Team Standards

### Memory Management

- Use `std::unique_ptr` for exclusive ownership and `std::shared_ptr` for shared ownership — never raw `new`/`delete` in application code.
- Follow RAII: acquire resources in constructors, release in destructors. Never hold resources outside an owning object.
- Prefer stack allocation over heap allocation. Reach for `std::vector` before `new T[]`.
- Document ownership transfers explicitly in function signatures (`std::unique_ptr` parameter = callee takes ownership).
- Use `std::make_unique` and `std::make_shared` — never call `new` directly alongside smart pointer constructors.

### Naming Conventions

- Types (classes, structs, enums) in `PascalCase`; functions and variables in `snake_case`.
- Constants and compile-time values prefixed with `k` (`kMaxRetries`, `kDefaultTimeout`).
- Macros in `ALL_CAPS`; avoid macros where `constexpr` or `inline` functions suffice.
- Private member variables suffixed with `_` (`config_`, `handler_`).
- Follow Google C++ Style or LLVM style consistently — pick one per project.

### Error Handling

- Prefer exceptions for error propagation in application code — they compose cleanly with RAII.
- Mark move constructors and move assignment operators `noexcept` unconditionally.
- Use `std::expected<T, E>` (C++23) or a Result-style wrapper for expected failures in library APIs.
- Never use errno or return-code patterns in new C++ code unless wrapping C APIs.
- Catch exceptions by `const` reference (`catch (const std::exception& e)`), never by value.

### Testing

- Use Google Test (`gtest`) or Catch2. Choose one and apply it consistently across the codebase.
- Test files named `*_test.cpp`, co-located with or mirroring the source structure.
- Use `TEST_F` with fixture classes for stateful setup; use `TEST` for pure unit tests.
- Wire integration and end-to-end tests through CTest (`ctest --test-dir build`).
- Mock collaborators with Google Mock (`gmock`). Define mock classes in `*_mock.h` headers.
- Run tests with AddressSanitizer (`-fsanitize=address`) in CI to catch memory errors early.

### Tooling

- Run `clang-format` (project `.clang-format`) on every file before committing. Never push unformatted code.
- Run `clang-tidy` with a `.clang-tidy` config. Fix all warnings; suppress individually with a justifying comment.
- Use CMake with modern target-based configuration (`target_link_libraries` with access specifiers, no global flags).
- Enable ASan and UBSan in debug/CI builds (`-fsanitize=address,undefined`). Fix every report.
- Use `ccache` for faster incremental builds in CI.

### Modern C++ Practices

- Prefer algorithms (`std::transform`, `std::find_if`) over raw index loops.
- Use `auto` to avoid type repetition, but annotate where the type aids readability.
- Mark functions `constexpr` when their result can be computed at compile time.
- Use C++20 ranges and structured bindings to reduce boilerplate.
- Avoid `std::endl` — use `'\n'`; `endl` flushes and is usually unnecessary overhead.
- Prefer `enum class` over unscoped `enum` to prevent namespace pollution.
