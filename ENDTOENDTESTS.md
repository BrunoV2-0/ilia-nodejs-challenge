# End-to-End Test Scenarios

## Infrastructure
- Stack spun up via `testcontainers-go` compose module (self-contained, no manual `docker compose up`)
- Suite lives in `e2e/` at monorepo root as a separate Go module
- `TestMain` starts the full stack, waits for healthchecks, then tears it down after all tests run
- Each test uses a unique email (UUID suffix) to avoid cross-test interference

---

## User Service — `POST /users`

| # | Scenario | Input | Expected |
|---|---|---|---|
| U01 | Create user with valid fields | `first_name`, `last_name`, `email`, `password` | 201, all fields returned, `password` absent from response |
| U02 | Duplicate email | Same email as an existing user | 409 Conflict |
| U03 | Missing required field | Body omits `email` | 400 Bad Request |
| U04 | Invalid email format | `email: "not-an-email"` | 400 Bad Request |
| U05 | Empty body | `{}` | 400 Bad Request |

---

## User Service — `POST /auth`

| # | Scenario | Input | Expected |
|---|---|---|---|
| A01 | Valid credentials | Correct email + password | 201, `access_token` in response |
| A02 | Wrong password | Correct email, wrong password | 401 Unauthorized |
| A03 | Unknown email | Email not in database | 401 Unauthorized |
| A04 | Missing fields | Body omits password | 400 Bad Request |

---

## User Service — `GET /users`

| # | Scenario | Auth | Expected |
|---|---|---|---|
| L01 | List users | Valid JWT | 200, array contains the created user |
| L02 | No token | None | 401 Unauthorized |
| L03 | Invalid token | Malformed JWT | 401 Unauthorized |
| L04 | Expired token | Expired JWT | 401 Unauthorized |

---

## User Service — `GET /users/:id`

| # | Scenario | Auth | Expected |
|---|---|---|---|
| G01 | Get existing user | Valid JWT | 200, correct user fields |
| G02 | Unknown UUID | Valid JWT | 404 Not Found |
| G03 | Invalid UUID format | Valid JWT | 400 Bad Request |
| G04 | No token | None | 401 Unauthorized |

---

## User Service — `PATCH /users/:id`

| # | Scenario | Input | Expected |
|---|---|---|---|
| UP01 | Update first name | `{ "first_name": "New" }` | 200, updated field returned |
| UP02 | Update email to an already-taken email | `{ "email": "<other user's email>" }` | 409 Conflict |
| UP03 | Update with invalid email format | `{ "email": "bad" }` | 400 Bad Request |
| UP04 | Update non-existent user | Valid JWT, unknown UUID | 404 Not Found |
| UP05 | No token | None | 401 Unauthorized |
---

## User Service — `DELETE /users/:id`

| # | Scenario | Precondition | Expected |
|---|---|---|---|
| D01 | Delete user with empty wallet | No transactions ever created | 204 No Content |
| D02 | Delete user with positive balance | CREDIT 50 was posted | 422 Unprocessable Entity |
| D03 | Delete non-existent user | Unknown UUID | 404 Not Found |
| D04 | No token | None | 401 Unauthorized |
| D05 | Delete user with zero balance | CREDIT 50 was posted and DEBIT 50 was posted afterwards | 204 No Content |
---

## Transaction Service — `POST /transactions`

| # | Scenario | Input | Expected |
|---|---|---|---|
| T01 | Create CREDIT transaction | `{ "amount": 100, "type": "CREDIT" }` | 201, transaction ID returned |
| T02 | Create DEBIT after sufficient credit | CREDIT 100 first, then DEBIT 40 | 201 |
| T03 | DEBIT exceeds balance | CREDIT 10, then DEBIT 50 | 422 Unprocessable Entity |
| T04 | DEBIT on zero balance | No prior transactions | 422 Unprocessable Entity |
| T05 | Invalid type | `{ "amount": 10, "type": "INVALID" }` | 400 Bad Request |
| T06 | Zero amount | `{ "amount": 0, "type": "CREDIT" }` | 400 Bad Request |
| T07 | Negative amount | `{ "amount": -5, "type": "CREDIT" }` | 400 Bad Request |
| T08 | Missing amount | `{ "type": "CREDIT" }` | 400 Bad Request |
| T09 | No token | None | 401 Unauthorized |

---

## Transaction Service — `GET /transactions`

| # | Scenario | Query | Expected |
|---|---|---|---|
| TL01 | List all transactions | None | 200, array of all user's transactions |
| TL02 | Filter by CREDIT | `?type=CREDIT` | 200, only CREDIT entries |
| TL03 | Filter by DEBIT | `?type=DEBIT` | 200, only DEBIT entries |
| TL04 | Empty history | New user, no transactions | 200, empty array |
| TL05 | No token | None | 401 Unauthorized |

---

## Transaction Service — `GET /wallets/balance`

| # | Scenario | Precondition | Expected |
|---|---|---|---|
| B01 | Balance after credits only | CREDIT 100, CREDIT 50 | 200, `balance: 150` |
| B02 | Balance after credit and debit | CREDIT 100, DEBIT 30 | 200, `balance: 70` |
| B03 | Balance on new user | No transactions | 200, `balance: 0` (or 404 if wallet created on first transaction) |
| B04 | No token | None | 401 Unauthorized |

---

## Cross-Service Flows

| # | Scenario | Steps | Expected |
|---|---|---|---|
| X01 | Delete blocked by wallet balance | Create user → auth → CREDIT 50 → DELETE user | 422 from ms-users |
| X02 | Delete succeeds after zeroing wallet | Create user → auth → CREDIT 50 → DEBIT 50 → DELETE user | 204 from ms-users |
| X03 | Multiple credits then full debit then delete | CREDIT 30 + CREDIT 70 → DEBIT 100 → DELETE | 204 |

---

## Notes for Review

- Add scenarios here if anything is missing before implementation starts.
- Scenario IDs (U01, T03, etc.) will be used as test function name suffixes.
- Each test is independent: creates its own user with a unique email and manages its own data.