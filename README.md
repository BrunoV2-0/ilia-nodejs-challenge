# README

This repository is a solution to the Ilia backend challenge. The original challenge brief and requirements are in [`CHALLENGE-README.md`](./CHALLENGE-README.md).

---

# Architecture

## Repository layout

```
ilia-nodejs-challenge/
├── ms-users/          # User management service  (port 3002)
├── ms-transactions/   # Wallet & transaction service (port 3001)
├── e2e/               # End-to-end test suite (separate Go module)
└── docker-compose.yml # Full-stack orchestration
```

Each service is an independent Go module with an identical internal structure:

```
internal/
  domain/<aggregate>/   # Pure business logic — no infra imports
  database/<aggregate>/ # PostgreSQL implementations
  http/
    handler/            # Decode → call service → encode
    middleware/         # JWT validation only
    client/             # Outbound HTTP (ms-users only)
cmd/main.go             # Composition root — wires everything, no logic
```

---

## Dependency rule

Dependencies flow **inward only**: `database/` and `http/` import `domain/`. `domain/` imports nothing outside stdlib.

Repository **interfaces** are declared inside `domain/`. PostgreSQL **implementations** live in `database/` and satisfy those interfaces. This means the domain can be tested with hand-written mocks and never touches a database.

Constructors always return the interface, never the concrete type:

```go
func New(db *sqlx.DB) wallet.Repository  // not *PostgresRepository
```

---

## Why the Wallet entity exists

Transactions are an immutable append-only log. A wallet is the read model: the current balance derived from that log. Keeping them separate makes the balance a first-class, directly queryable value rather than a `SUM()` computed on every read.

Owning the balance at the application level — rather than deriving it in SQL — means the debit guard (`wallet.CanDebit(amount)`) is a plain Go method that can be unit-tested without a database. Any rule that changes (e.g. allowing overdrafts up to a limit, enforcing a daily spend cap) is added to the `Wallet` struct and covered by a domain test, not by patching a query.

The entity is also the natural place to attach wallet-level metadata in the future: credit limits, overdraft flags, currency, freeze status. All of that belongs on `Wallet`, not on individual transaction rows.

The `Wallet` struct carries a `Version int64` field used for optimistic concurrency control (see below). A transaction entry never changes after it is inserted.

---

## Optimistic Concurrency Control (OCC)

Multiple concurrent requests can attempt to debit or credit the same wallet simultaneously. Instead of a row-level lock, the service uses a version counter:

1. Read the wallet and note its current `Version`.
2. Validate the operation (e.g. sufficient funds for a debit).
3. `UPDATE wallets SET balance = balance + delta, version = version + 1 WHERE id = $1 AND version = $2` — the `WHERE version = $2` is the guard.
4. If zero rows were updated, another request modified the wallet first → return `ErrConflict`.

The service retries up to **3 times** with exponential back-off + jitter on `ErrConflict`. This handles bursts of concurrent writes without ever blocking a thread on a database lock.

This approach was chosen under the assumption that **contention per wallet is low** — the typical user submits one transaction at a time and conflicts are rare. Under low contention OCC is cheaper than a pessimistic lock because it avoids the overhead of acquiring and holding a row lock on every write. If the system were to support high-frequency trading or bulk automated transactions against the same wallet, the retry loop would become a bottleneck and a pessimistic lock (`SELECT ... FOR UPDATE`) would be the better trade-off.

---

## Cross-service integration

`ms-users` must refuse to delete a user whose wallet still has a positive balance. Rather than coupling the domain to HTTP, a narrow interface is declared inside `domain/user`:

```go
type WalletChecker interface {
    HasBalance(userID uuid.UUID) (bool, error)
}
```

`http/client/` provides the concrete implementation: it mints a **short-lived (1 min) HS256 JWT** signed with `JWT_INTERNAL_KEY` and calls `GET /internal/wallets/balance` on `ms-transactions`. The domain never knows about HTTP.

The internal route is separate from public routes so that `JWT_INTERNAL_KEY` rotation can be done independently of the user-facing `JWT_KEY`.

---

## Guidelines for future work

| Concern | Rule |
|---|---|
| New feature | Start with the domain: entity + repository interface + service. Never touch the DB or HTTP layers until the domain tests are green. |
| New service-to-service call | Declare an interface in the calling service's `domain/`. Implement in `http/client/`. The domain never imports a client package. |
| Concurrent writes to a shared resource | Use OCC with a version column, not a DB lock. Add retry logic in the service, not the handler. |
| Adding a route | Handler decodes → calls service → encodes. No business logic in handlers. Map sentinel domain errors to HTTP status codes. |
| Error propagation | Wrap with `fmt.Errorf("context: %w", err)`. Declare sentinel errors in `domain/` (`var ErrNotFound = errors.New(...)`). HTTP handlers switch on `errors.Is`. |
| Testing | Domain tests use hand-written mocks. DB tests use `testcontainers-go` with real PostgreSQL. Handler tests use `httptest`. E2E tests in `e2e/` use the full docker-compose stack. |
| Branch | `feature/<service>-<layer>` → `develop` via PR. `main` receives only `release/*` branches. |
