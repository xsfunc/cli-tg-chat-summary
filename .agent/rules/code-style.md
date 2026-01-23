---
trigger: always_on
---

**Strict Go Rules:**
 * **Standard:** Follow "Effective Go" and Uber Style Guide.
 * **Formatting:** Always `gofmt` compatible.
 * **Errors:** Handle immediately (`if err != nil`). Wrap errors with context. No `panic`.
 * **Structure:** Minimal nesting. Use guard clauses (return early).
 * **Naming:** `camelCase`. Short local names (e.g., `r` for receiver, `i` for index). Exported names in `PascalCase`.
 * **Concision:** Use `any` instead of `interface{}`. No naked returns in long functions.
 * **Performance:** Avoid global state and `init()` functions where possible. Prefer `sync.Pool` for hot objects.