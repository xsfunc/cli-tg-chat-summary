# AGENTS.md

## Go Guidelines
- Run gofmt on any Go code changes.
- Prefer the standard library; avoid new third-party dependencies unless necessary.
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Do not change public APIs without explicit request.
- Keep package layout consistent (e.g., `cmd/`, `internal/`, `pkg/`); place new code accordingly.
- Be explicit about concurrency ownership: context cancellation, goroutine lifetimes, channel closure.

## Tests
- If you add behavior, add or update a test when it is simple and nearby.
- Prefer table-driven tests; avoid flaky tests and `time.Sleep` unless necessary.

## Dependencies
- Do not add new dependencies without a clear need and a brief justification.
- Do not change `go.mod` or `go.sum` unless required by the change.

## Git (Local)
- Commit directly to `main`.
- Make small, frequent commits; use `git add -p` to stage chunks.
- Before confirming changes, check `git status -sb`, `git diff`, and `git diff --staged`.
- For quick rollback, prefer `git restore --staged .` and `git restore .`.
- Use Conventional Commits: `type(scope): summary` (e.g., `feat(cli): add --json output`).
- After implementing code changes (before commit), run `make lint`.

## Tooling
- Provide short aliases in `Makefile` or `Taskfile` for build, test, and lint.
- Consider pre-commit hooks for `gofmt` and a fast `go test` subset.
