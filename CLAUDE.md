# CLAUDE.md — Development Standards

## 1. Dependency Rule (highest priority)

Dependencies flow **inward only**: `database/` and `http/` import `domain/`. `domain/` imports nothing outside stdlib.

| Package | May import | Must NOT import |
|---|---|---|
| `internal/domain/` | stdlib only | `database/`, `http/`, any infra lib |
| `internal/database/` | `domain/`, `sqlx`, `pq`, `migrate` | `http/` |
| `internal/http/` | `domain/`, `chi`, `jwt` | `database/` |
| `cmd/main.go` | everything | — (composition root only, no logic) |

**Repository interfaces live in `domain/`. PostgreSQL implementations live in `database/` and satisfy those interfaces. The domain never imports the database.**

Constructor pattern: `func New(db *sqlx.DB) transaction.Repository` — always return the interface type, never the concrete struct.

## 2. Package Roles

- `domain/<agg>/entity.go` — structs, types, constants, `Validate()`. Pure Go.
- `domain/<agg>/repository.go` — interface only. No implementation.
- `domain/<agg>/service.go` — business rules. Accepts repository interface via constructor. No SQL, no HTTP.
- `database/<agg>/repository.go` — implements domain interface. No business logic.
- `http/handler/` — decode request → call service → encode response. No business logic, no SQL.
- `http/middleware/` — JWT validation only. Sets claims in context.
- `http/client/` — outbound HTTP to other services. Signs requests with `JWT_INTERNAL_KEY`.
- `cmd/main.go` — wires concretes. Order: config → db → migrations → repository → service → handler → server.

## 3. TDD — Red → Green → Refactor

Never write implementation before a failing test. Sequence per task:
1. Write interface/signature (no body).
2. Write test → run → must **fail**.
3. Implement → run → must **pass**.
4. `go vet ./...` → zero warnings before moving on.

**Test isolation by layer — no cross-layer helpers:**
- `domain/*_test.go` — hand-written mock repositories, no DB, no HTTP.
- `database/*_test.go` — `testcontainers-go` with real PostgreSQL, run migrations before assertions.
- `http/handler/*_test.go` — `httptest` + mock domain services, assert status codes and response shape.
- `http/client/*_test.go` — `httptest.NewServer` to simulate downstream, assert Authorization header contains correct JWT.

## 4. Clean Code

- **One responsibility per function.** If you can describe it with "and", split it.
- **No magic strings or numbers.** Use typed constants (`type TransactionType string`).
- **All errors checked and wrapped:** `fmt.Errorf("creating transaction: %w", err)`.
- **Sentinel errors in domain:** `var ErrNotFound = errors.New("not found")`. HTTP handlers map these to status codes.
- **No `interface{}` or `any`** where a concrete or domain type is known.
- Functions over ~30 lines are a signal to refactor.

## 5. Type Safety — Gate After Every Step

Run after implementing any file or completing any feature branch:

```bash
go vet ./...      # zero warnings required
go build ./...    # zero errors required
go test ./...     # all tests pass, none skipped
```

Do not accumulate errors across steps. A file that does not compile is a broken state, not a partial implementation.

## 6. GitFlow

- `main` ← `release/*` only. `develop` ← `feature/*` via PR only. No direct commits.
- Branch naming: `feature/<service>-<layer>` (e.g. `feature/ms-transactions-domain`).
- Every PR must pass `go test ./...`. PR description states what was built and TDD steps completed.
- Commit format: `feat(scope): description` / `test(scope):` / `fix(scope):` / `refactor(scope):`.

## 7. Pre-Completion Checklist

- [ ] `domain/` has zero infra imports. Repository interface in domain, implementation in database.
- [ ] Handlers contain no business logic. `main.go` is the only composition root.
- [ ] Tests written before implementation. Domain=mocks, DB=testcontainers, HTTP=httptest.
- [ ] `go vet ./...` clean. `go build ./...` clean. `go test ./...` all pass.
- [ ] Work is on a `feature/` branch with a PR targeting `develop`.