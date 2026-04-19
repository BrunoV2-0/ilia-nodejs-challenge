# Job Description: Wallet & Users Microservices (Go)

> This file is intended to be read by Claude Code to plan and scaffold the project.
> All endpoint definitions below were extracted directly from `ms-transactions.yaml` and `ms-users.yaml` — do not re-read those files, use the definitions here.

---

## Overview

Build two microservices in **Go** following **Domain-Driven Design (DDD)**, **Clean Architecture**, and **Test-Driven Development (TDD)**. The domain is the heart of each service: business rules, entities, and repository interfaces live in the domain layer and have zero dependencies on frameworks, databases, or HTTP. Infrastructure packages implement domain interfaces — never the other way around. Every interface is defined first, tests are written second, and the implementation is written third.

---

## Architectural Rules (enforced across both services)

These rules are non-negotiable and must be respected in every file Claude Code creates:

1. **Domain layer has no outward dependencies.** Packages under `internal/domain/` must not import anything from `internal/database/`, `internal/http/`, or any third-party infrastructure library. Only the Go standard library and domain-internal packages are allowed there.

2. **Repositories are domain-level contracts.** Each aggregate root defines a repository *interface* inside `internal/domain/<aggregate>/repository.go`. The interface describes what the domain needs; it does not know how it is fulfilled.

3. **The database package implements domain repository interfaces.** Packages under `internal/database/` hold the PostgreSQL implementation of each repository interface. They import the domain to satisfy the interface — the domain never imports the database.

4. **The HTTP package is a delivery mechanism only.** Packages under `internal/http/` translate HTTP requests into domain calls and domain results into HTTP responses. Handlers hold no business logic.

5. **Dependency injection happens at the composition root.** `cmd/main.go` is the only place where concrete implementations are instantiated and wired together.

6. **No circular imports.** The architecture must make it structurally impossible.

---

## TDD Mandate — Non-Negotiable

Every implementation step follows the **Red → Green → Refactor** cycle without exception:

1. **Define the interface or contract first** (entity, repository interface, service signature).
2. **Write failing tests** (`_test.go` files) that specify the expected behaviour.
3. **Let the user review the tests** before proceding with the next step
4. **Run the tests — they must fail** (Red). Never write an implementation before a failing test exists.
5. **Write the minimum implementation** to make the tests pass (Green).
6. **Refactor** without breaking tests.
7. **Run `go vet ./...` and `go build ./...`** after every implementation step to catch type errors immediately.

### Test Separation Rules

- **Domain tests** (`internal/domain/<aggregate>/*_test.go`) use only Go standard library and mocks. No real DB, no HTTP. They test entities, domain validation, and domain service logic in complete isolation using mock implementations of repository interfaces.
- **Database integration tests** (`internal/database/<aggregate>/*_test.go`) use `testcontainers-go` to spin up a real PostgreSQL container. They test that the concrete repository implementation satisfies the domain interface contract correctly.
- **HTTP handler tests** (`internal/http/handler/*_test.go`) use `net/http/httptest` and mock domain services. They test request decoding, response encoding, status codes, and middleware behaviour. No real DB, no real domain logic.
- Tests in different layers must never share test helpers across package boundaries — each layer's tests are self-contained for readability and independent maintenance.

### Mock Strategy

- Use hand-written mocks for repository interfaces in domain tests (avoid code generation to keep domain tests dependency-free).
- Example mock for domain tests:
  ```go
  type mockTransactionRepository struct {
      createFn     func(tx transaction.Transaction) (transaction.Transaction, error)
      findAllFn    func(userID string, txType string) ([]transaction.Transaction, error)
      getBalanceFn func(userID string) (int, error)
  }
  ```

### Testcontainers Pattern (integration tests)

```go
func setupTestDB(t *testing.T) *sqlx.DB {
    ctx := context.Background()
    container, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
    )
    require.NoError(t, err)
    t.Cleanup(func() { container.Terminate(ctx) })

    connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
    db, err := sqlx.Connect("postgres", connStr)
    require.NoError(t, err)
    // run migrations here before returning
    return db
}
```

---

## Monorepo Structure

```
/
├── ms-transactions/
├── ms-users/
├── docker-compose.yml
├── ms-transactions.yaml
├── ms-users.yaml
└── JOBDESCRIPTION.md
```

---

## Part 1 — Wallet Microservice (`ms-transactions/`)

### Constraints

| Item | Value |
|---|---|
| Language | Go |
| Port | `3001` |
| Database | PostgreSQL |
| Auth (external users) | JWT signed with env var `JWT_PRIVATE_KEY` = `ILIACHALLENGE` |
| Auth (internal service calls) | JWT signed with env var `JWT_INTERNAL_KEY` = `ILIACHALLENGE_INTERNAL` |
| Container | Docker + Docker Compose |

### Directory Structure

```
ms-transactions/
├── cmd/
│   └── main.go                                         # Composition root — wires everything, no logic
│
├── internal/
│   │
│   ├── domain/                                         # THE CORE — zero external dependencies
│   │   └── transaction/
│   │       ├── entity.go                               # Transaction struct, TransactionType, domain validation
│   │       ├── entity_test.go                          # Unit tests for entity validation rules
│   │       ├── repository.go                           # Repository interface (port)
│   │       ├── service.go                              # Domain service — business rules
│   │       └── service_test.go                         # Unit tests using mock repository
│   │
│   ├── database/                                       # Infrastructure — implements domain interfaces
│   │   ├── postgres.go                                 # DB connection pool setup
│   │   ├── migrations/
│   │   │   ├── 000001_create_transactions_table.up.sql
│   │   │   └── 000001_create_transactions_table.down.sql
│   │   └── transaction/
│   │       ├── repository.go                           # PostgresTransactionRepository
│   │       └── repository_integration_test.go          # Integration tests with testcontainers
│   │
│   └── http/                                           # Delivery layer
│       ├── server/
│       │   └── server.go
│       ├── middleware/
│       │   ├── auth.go
│       │   └── auth_test.go                            # Unit tests for JWT middleware
│       ├── handler/
│       │   ├── transaction.go
│       │   └── transaction_test.go                     # Unit tests with mock service + httptest
│       └── docs/
│           └── swagger.go
│
├── Dockerfile
├── .env.example
└── README.md
```

### Dependency Flow

```
cmd/main.go
    │
    ├── instantiates → internal/database/postgres.go
    ├── instantiates → internal/database/transaction/repository.go  (implements domain interface)
    ├── injects into → internal/domain/transaction/service.go       (domain service)
    └── injects into → internal/http/handler/transaction.go

internal/http/handler   → calls → internal/domain/transaction/service
internal/domain/service → calls → internal/domain/transaction/repository (interface)
internal/database/repo  → implements → internal/domain/transaction/repository
```

### Domain: `internal/domain/transaction/`

**`entity.go`**
```go
package transaction

import "github.com/google/uuid"

type TransactionType string

const (
    Credit TransactionType = "CREDIT"
    Debit  TransactionType = "DEBIT"
)

type Transaction struct {
    ID     uuid.UUID
    UserID uuid.UUID
    Amount int
    Type   TransactionType
}

func (t *Transaction) Validate() error { ... }
```

**`repository.go`** — interface only, no implementation
```go
package transaction

type Repository interface {
    Create(tx Transaction) (Transaction, error)
    FindAll(userID string, txType string) ([]Transaction, error)
    GetBalance(userID string) (int, error)
}
```

**`service.go`** — accepts the interface, never a concrete DB type
```go
package transaction

type Service struct { repo Repository }

func NewService(repo Repository) *Service
func (s *Service) CreateTransaction(userID string, amount int, txType TransactionType) (Transaction, error)
func (s *Service) ListTransactions(userID string, txType string) ([]Transaction, error)
func (s *Service) GetBalance(userID string) (int, error)
```

### Infrastructure: `internal/database/transaction/repository.go`

```go
package transactiondb

// PostgresRepository implements domain/transaction.Repository
type PostgresRepository struct { db *sqlx.DB }

func New(db *sqlx.DB) transaction.Repository // returns the interface

// GetBalance MUST use a single SQL query:
// SELECT COALESCE(SUM(CASE WHEN type='CREDIT' THEN amount ELSE -amount END), 0)
// FROM transactions WHERE user_id = $1
```

### Endpoints

#### `POST /transactions` — Auth required
Request: `{ "user_id": "string", "amount": 0, "type": "CREDIT" }` (all required, type enum CREDIT|DEBIT)
Response 200: `{ "id": "string", "user_id": "string", "amount": 0, "type": "CREDIT" }`

#### `GET /transactions?type=CREDIT` — Auth required
Response 200: array of transactions.

#### `GET /balance` — Auth required
Response 200: `{ "amount": 0 }` — computed by a single DB-level query.

### Database Schema

```sql
CREATE TABLE transactions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    amount     INTEGER NOT NULL,
    type       VARCHAR(10) NOT NULL CHECK (type IN ('CREDIT', 'DEBIT')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Implementation Tasks (TDD order — strictly followed)

**Step 1 — Domain entity**
1a. Write `internal/domain/transaction/entity.go` — struct, types, `Validate()` signature only (no body yet).
1b. Write `internal/domain/transaction/entity_test.go` — tests for all validation rules (empty UserID, zero amount, invalid type, etc). Run: tests must **fail**.
1c. Implement `Validate()` body. Run: tests must **pass**.
1d. Run `go vet ./internal/domain/...` and `go build ./internal/domain/...`.

**Step 2 — Repository interface**
2a. Write `internal/domain/transaction/repository.go` — interface only, no implementation.

**Step 3 — Domain service**
3a. Write `internal/domain/transaction/service.go` — struct and method signatures only (no bodies).
3b. Write `internal/domain/transaction/service_test.go` — tests using a hand-written mock of `Repository`. Cover: successful create, invalid entity rejected before hitting repo, list with type filter, balance calculation delegated to repo. Run: tests must **fail**.
3c. Implement service method bodies. Run: tests must **pass**.
3d. Run `go vet ./internal/domain/...`.

**Step 4 — Database infrastructure**
4a. Write `internal/database/postgres.go` — connection, ping, close.
4b. Write SQL migration files.
4c. Write `internal/database/transaction/repository_integration_test.go` — full integration test suite using `testcontainers-go`. Cover: Create persists and returns entity, FindAll with and without type filter, GetBalance with mixed CREDIT/DEBIT rows, GetBalance on empty table returns 0. Run: tests must **fail** (no implementation yet).
4d. Write `internal/database/transaction/repository.go`. Run: integration tests must **pass**.
4e. Run `go vet ./internal/database/...`.

**Step 5 — HTTP middleware**
5a. Write `internal/http/middleware/auth_test.go` — test that valid JWT_PRIVATE_KEY token passes, valid JWT_INTERNAL_KEY token passes, missing token returns 401, tampered token returns 401. Run: must **fail**.
5b. Write `internal/http/middleware/auth.go`. Run: must **pass**.
5c. Run `go vet ./internal/http/middleware/...`.

**Step 6 — HTTP handlers**
6a. Write `internal/http/handler/transaction_test.go` — test each handler using `httptest` and a mock domain service. Cover: correct status codes, response body shape, 401 propagation from middleware. Run: must **fail**.
6b. Write `internal/http/handler/transaction.go`. Run: must **pass**.
6c. Run `go vet ./internal/http/...`.

**Step 7 — Wiring & build verification**
7a. Write `internal/http/server/server.go` — route registration, middleware application.
7b. Write `internal/http/docs/swagger.go` — swag annotations.
7c. Write `cmd/main.go` — composition root, dependency injection.
7d. Run `go build ./...` — must produce zero errors.
7e. Run the full test suite: `go test ./...`.

**Step 8 — Dockerfile and README**
8a. Write multi-stage `Dockerfile`.
8b. Write `README.md` with curl examples for all three endpoints.

---

## Part 2 — Users Microservice (`ms-users/`)

### Constraints

| Item | Value |
|---|---|
| Language | Go |
| Port | `3002` |
| Database | PostgreSQL |
| Auth (external) | JWT signed with `JWT_PRIVATE_KEY` = `ILIACHALLENGE` |
| Auth (internal) | JWT signed with `JWT_INTERNAL_KEY` = `ILIACHALLENGE_INTERNAL` |
| Communication with ms-transactions | REST over HTTP, signed with `JWT_INTERNAL_KEY` |
| Container | Docker + Docker Compose |

### Directory Structure

```
ms-users/
├── cmd/
│   └── main.go
│
├── internal/
│   │
│   ├── domain/
│   │   └── user/
│   │       ├── entity.go
│   │       ├── entity_test.go
│   │       ├── repository.go
│   │       ├── service.go
│   │       └── service_test.go
│   │
│   ├── database/
│   │   ├── postgres.go
│   │   ├── migrations/
│   │   │   ├── 000001_create_users_table.up.sql
│   │   │   └── 000001_create_users_table.down.sql
│   │   └── user/
│   │       ├── repository.go
│   │       └── repository_integration_test.go
│   │
│   └── http/
│       ├── server/
│       │   └── server.go
│       ├── middleware/
│       │   ├── auth.go
│       │   └── auth_test.go
│       ├── handler/
│       │   ├── user.go
│       │   ├── user_test.go
│       │   ├── auth.go
│       │   └── auth_test.go
│       ├── client/
│       │   ├── transactions.go
│       │   └── transactions_test.go                    # Unit tests with mock HTTP server
│       └── docs/
│           └── swagger.go
│
├── Dockerfile
├── .env.example
└── README.md
```

### Domain: `internal/domain/user/`

**`entity.go`**
```go
package user

type User struct {
    ID        uuid.UUID
    FirstName string
    LastName  string
    Email     string
    Password  string // always bcrypt hash in storage; service is responsible for hashing
}

func (u *User) Validate() error { ... }
```

**`repository.go`**
```go
package user

type Repository interface {
    Create(u User) (User, error)
    FindAll() ([]User, error)
    FindByID(id string) (User, error)
    FindByEmail(email string) (User, error)
    Update(id string, fields UpdateFields) (User, error)
    Delete(id string) error
}

type UpdateFields struct {
    FirstName *string
    LastName  *string
    Email     *string
    Password  *string // hashed by the service before reaching repo
}
```

**`service.go`**
```go
package user

type Service struct { repo Repository }

func NewService(repo Repository) *Service
func (s *Service) CreateUser(firstName, lastName, email, password string) (User, error)  // bcrypt here
func (s *Service) Authenticate(email, password string) (User, error)                      // bcrypt compare
func (s *Service) GetUser(id string) (User, error)
func (s *Service) ListUsers() ([]User, error)
func (s *Service) UpdateUser(id string, fields UpdateFields) (User, error)                // bcrypt if password set
func (s *Service) DeleteUser(id string) error
```

### HTTP Client: `internal/http/client/transactions.go`

```go
package client

type TransactionsClient struct {
    baseURL        string
    internalJWTKey string
}

func NewTransactionsClient(baseURL, internalKey string) *TransactionsClient
func (c *TransactionsClient) GetBalance(userID string) (int, error)
func (c *TransactionsClient) CreateTransaction(userID string, amount int, txType string) error
// All requests: Authorization: Bearer <JWT signed with internalJWTKey>
```

### Endpoints

#### `POST /users` — Public
Request: `{ "first_name", "last_name", "password", "email" }` (all required)
Response 200: `{ "id", "first_name", "last_name", "email" }` (no password field ever)

#### `GET /users` — Auth required
Response 200: array of users (no password).

#### `GET /users/:id` — Auth required
Response 200: single user.

#### `PATCH /users/:id` — Auth required
Request: partial fields, all optional.
Response 200: updated user.

#### `DELETE /users/:id` — Auth required
Response 200: OK.

#### `POST /auth` — Public
Request: `{ "user": { "email": "string", "password": "string" } }`
Response 200:
```json
{
  "user": { "id": "string", "first_name": "string", "last_name": "string", "email": "string" },
  "access_token": "<JWT signed with JWT_PRIVATE_KEY>"
}
```
Response 401: wrong credentials.

### Database Schema

```sql
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name VARCHAR(100) NOT NULL,
    last_name  VARCHAR(100) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Implementation Tasks (TDD order — strictly followed)

**Step 1 — Domain entity**
1a. Write `entity.go` — struct and `Validate()` signature only.
1b. Write `entity_test.go` — invalid email, empty required fields, etc. Run: must **fail**.
1c. Implement `Validate()`. Run: must **pass**.
1d. `go vet ./internal/domain/...`.

**Step 2 — Repository interface**
2a. Write `repository.go` — interface and `UpdateFields` struct only.

**Step 3 — Domain service**
3a. Write `service.go` — signatures only.
3b. Write `service_test.go` with mock repository. Cover: CreateUser hashes password (verify stored password != input), Authenticate rejects wrong password, UpdateUser hashes new password, DeleteUser delegates to repo. Run: must **fail**.
3c. Implement service. Run: must **pass**.
3d. `go vet ./internal/domain/...`.

**Step 4 — Database infrastructure**
4a. Write `postgres.go` and migration files.
4b. Write `repository_integration_test.go` with testcontainers. Cover: Create, FindAll, FindByID, FindByEmail, Update (partial fields), Delete, unique email constraint. Run: must **fail**.
4c. Write `repository.go`. Run: must **pass**.
4d. `go vet ./internal/database/...`.

**Step 5 — Outbound HTTP client**
5a. Write `client/transactions_test.go` — use `httptest.NewServer` to simulate ms-transactions. Verify: correct JWT_INTERNAL_KEY in Authorization header, correct request body, correct response parsing. Run: must **fail**.
5b. Write `client/transactions.go`. Run: must **pass**.
5c. `go vet ./internal/http/client/...`.

**Step 6 — HTTP middleware**
6a. Write `middleware/auth_test.go`. Run: must **fail**.
6b. Write `middleware/auth.go`. Run: must **pass**.
6c. `go vet ./internal/http/middleware/...`.

**Step 7 — HTTP handlers**
7a. Write `handler/auth_test.go` — POST /auth success, wrong password 401, missing fields 400. Run: must **fail**.
7b. Write `handler/auth.go`. Run: must **pass**.
7c. Write `handler/user_test.go` — all CRUD endpoints with mock service. Run: must **fail**.
7d. Write `handler/user.go`. Run: must **pass**.
7e. `go vet ./internal/http/handler/...`.

**Step 8 — Wiring & build**
8a. Write `server/server.go`, `docs/swagger.go`, `cmd/main.go`.
8b. `go build ./...` — zero errors.
8c. `go test ./...` — all tests pass.

**Step 9 — Dockerfile and README**
9a. Write multi-stage `Dockerfile`.
9b. Write `README.md` with inter-service ASCII diagram and curl examples.

---

## Root Docker Compose

| Service | Host port | Container port | Depends on |
|---|---|---|---|
| `db-transactions` | 5432 | 5432 | — |
| `ms-transactions` | 3001 | 3001 | `db-transactions` (health check) |
| `db-users` | 5433 | 5432 | — |
| `ms-users` | 3002 | 3002 | `db-users` (health check), `ms-transactions` |

- All services on Docker bridge network: `wallet-network`
- DB services declare `healthcheck` with `pg_isready`; app services use `depends_on: condition: service_healthy`

---

## Environment Variables

### `ms-transactions/.env.example`
```env
PORT=3001
JWT_PRIVATE_KEY=ILIACHALLENGE
JWT_INTERNAL_KEY=ILIACHALLENGE_INTERNAL
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=transactions
```

### `ms-users/.env.example`
```env
PORT=3002
JWT_PRIVATE_KEY=ILIACHALLENGE
JWT_INTERNAL_KEY=ILIACHALLENGE_INTERNAL
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=users
TRANSACTIONS_BASE_URL=http://ms-transactions:3001
```

---

## Recommended Go Libraries

| Purpose | Library |
|---|---|
| HTTP router | `github.com/go-chi/chi/v5` |
| JWT | `github.com/golang-jwt/jwt/v5` |
| PostgreSQL driver | `github.com/lib/pq` |
| SQL helpers | `github.com/jmoiron/sqlx` |
| Migrations | `github.com/golang-migrate/migrate/v4` |
| Password hashing | `golang.org/x/crypto/bcrypt` |
| Env loading | `github.com/joho/godotenv` |
| UUID | `github.com/google/uuid` |
| Integration testing | `github.com/testcontainers/testcontainers-go` |
| Test assertions | `github.com/stretchr/testify` |
| Swagger | `github.com/swaggo/swag` + `github.com/swaggo/http-swagger` |

---

## GitFlow — Mandatory Process

GitFlow is **required**. Direct commits to `main` or `develop` are not allowed.

### Branch Types

| Type | Pattern | Example |
|---|---|---|
| Feature | `feature/<scope>-<description>` | `feature/ms-transactions-domain` |
| Fix | `fix/<scope>-<description>` | `fix/ms-users-auth-middleware` |
| Release | `release/<version>` | `release/1.0.0` |
| Hotfix | `hotfix/<description>` | `hotfix/balance-query` |

### Protected Branches

- `main` — production only. Merges from `release/*` and `hotfix/*` only.
- `develop` — integration branch. All `feature/*` PRs target `develop`.

### Required Branch Sequence

Each step is a separate feature branch merged into `develop` via PR before the next begins.

```
Step 01 — feature/project-setup
          Monorepo scaffold, go.mod, docker-compose skeleton, .env.example files.

Step 02 — feature/ms-transactions-domain
          entity + entity_test, repository interface, service + service_test.
          TDD cycle complete. go vet passes. No infrastructure code.

Step 03 — feature/ms-transactions-database
          postgres.go, migrations, repository.go + repository_integration_test.go.
          TDD cycle complete (testcontainers). go vet passes.

Step 04 — feature/ms-transactions-http
          middleware + test, handlers + test, server, docs, cmd/main.go.
          go build ./... passes. go test ./... passes.

Step 05 — feature/ms-users-domain
          entity + test, repository interface, service + test. go vet passes.

Step 06 — feature/ms-users-database
          postgres.go, migrations, repository.go + integration test. go vet passes.

Step 07 — feature/ms-users-http-auth
          client/transactions.go + test, middleware + test, handler/auth.go + test.

Step 08 — feature/ms-users-http-users
          handler/user.go + test, server, docs, cmd/main.go.
          go build ./... passes. go test ./... passes.

Step 09 — feature/docker-compose
          Complete root docker-compose.yml. Both services boot with docker-compose up.

Step 10 — release/1.0.0
          Merge develop → main via PR. Tag the release.
```

### PR Requirements

- Clear title matching branch name.
- Description: what was added, why, design decisions.
- All tests must pass before merge.
- No force-pushes to `develop` or `main`.

---

## Acceptance Criteria

- [ ] `docker-compose up` starts all four containers.
- [ ] All three endpoints of ms-transactions respond correctly on port 3001.
- [ ] All six endpoints of ms-users respond correctly on port 3002.
- [ ] Protected routes return `401` without a valid token.
- [ ] `POST /auth` returns a JWT that unlocks protected routes.
- [ ] `GET /balance` computes balance in a single DB-level SQL query.
- [ ] Internal calls from ms-users to ms-transactions carry a JWT signed with `JWT_INTERNAL_KEY`.
- [ ] ms-transactions accepts both `JWT_PRIVATE_KEY` and `JWT_INTERNAL_KEY` tokens.
- [ ] Passwords are stored as bcrypt hashes — never plain text.
- [ ] `internal/domain/` has zero imports from `internal/database/` or `internal/http/`.
- [ ] `internal/database/` implements domain repository interfaces; domain never imports database.
- [ ] Each layer has its own isolated test files. Domain tests use only mocks. DB tests use testcontainers.
- [ ] `go test ./...` passes across both services with no skipped tests.
- [ ] `go vet ./...` produces zero warnings across both services.
- [ ] Git history shows all 10 feature branches and PRs into `develop`, plus the release PR into `main`.
- [ ] Each service has a `README.md` with setup, env vars, and curl examples.