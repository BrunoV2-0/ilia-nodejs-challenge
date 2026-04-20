# E2E Test Suite

Automated end-to-end tests for the `ms-users` and `ms-transactions` microservices.

## How it works

`TestMain` uses [testcontainers-go](https://golang.testcontainers.org/) to spin up the full `docker-compose.yml` stack (both databases + both services) before any test runs, and tears it down when the suite finishes. No manual `docker compose up` is needed.

## Prerequisites

- [Go 1.23+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) running
- `.env` files present at the monorepo root:
  - `ms-users/.env`
  - `ms-transactions/.env`

  Copy from the `.env.example` files if they don't exist:
  ```bash
  cp ../ms-users/.env.example ../ms-users/.env
  cp ../ms-transactions/.env.example ../ms-transactions/.env
  ```

## Running the suite

From the `e2e/` directory:

```bash
go test ./... -v -timeout 5m
```

The first run will build the Docker images — expect 2–3 minutes. Subsequent runs are faster because Docker caches the image layers.

## Running a single test

```bash
go test ./... -v -timeout 5m -run TestCreateUser_U01_OK
```

## Test scenarios

| File | Scenarios | Coverage |
|---|---|---|
| `users_test.go` | U01–U05, A01–A04, L01–L03, G01–G04, UP01–UP05, D01–D04 | User CRUD, auth |
| `transactions_test.go` | T01–T09, TL01–TL05, B01–B04 | Transactions, balance |
| `cross_service_test.go` | X01–X03 | Wallet-balance guard on user deletion |

Full scenario descriptions are in [`./TEST-SCENARIOS.md`](./TEST-SCENARIOS.md).
