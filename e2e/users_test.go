package e2e_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// ── POST /users ──────────────────────────────────────────────────────────────

func TestCreateUser_U01_OK(t *testing.T) {
	email := "u01-" + uuid.NewString() + "@example.com"
	resp := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "Alice",
		"last_name":  "Smith",
		"email":      email,
		"password":   "pass1234",
	}, "")

	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("U01: expected 201, got %d", resp.StatusCode)
	}
	if body["id"] == nil {
		t.Fatal("U01: response missing id")
	}
	if body["password"] != nil {
		t.Fatal("U01: password must not appear in response")
	}
}

func TestCreateUser_U02_DuplicateEmail(t *testing.T) {
	email := "u02-" + uuid.NewString() + "@example.com"
	payload := map[string]string{
		"first_name": "Bob",
		"last_name":  "Jones",
		"email":      email,
		"password":   "pass1234",
	}

	first := doJSON(t, http.MethodPost, usersURL+"/users", payload, "")
	first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("U02 setup: expected 201, got %d", first.StatusCode)
	}

	second := doJSON(t, http.MethodPost, usersURL+"/users", payload, "")
	second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("U02: expected 409, got %d", second.StatusCode)
	}
}

func TestCreateUser_U03_MissingField(t *testing.T) {
	resp := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "Carl",
		"last_name":  "Doe",
		"password":   "pass1234",
		// email omitted
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("U03: expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateUser_U04_InvalidEmail(t *testing.T) {
	resp := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "Dan",
		"last_name":  "Doe",
		"email":      "not-an-email",
		"password":   "pass1234",
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("U04: expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateUser_U05_EmptyBody(t *testing.T) {
	resp := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("U05: expected 400, got %d", resp.StatusCode)
	}
}

// ── POST /auth ───────────────────────────────────────────────────────────────

func TestAuth_A01_ValidCredentials(t *testing.T) {
	email := "a01-" + uuid.NewString() + "@example.com"
	create := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "Eve",
		"last_name":  "Doe",
		"email":      email,
		"password":   "mypassword",
	}, "")
	create.Body.Close()
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("A01 setup: expected 201, got %d", create.StatusCode)
	}

	resp := doJSON(t, http.MethodPost, usersURL+"/auth", map[string]string{
		"email":    email,
		"password": "mypassword",
	}, "")
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("A01: expected 200, got %d", resp.StatusCode)
	}
	if body["access_token"] == nil {
		t.Fatal("A01: access_token missing from response")
	}
}

func TestAuth_A02_WrongPassword(t *testing.T) {
	email := "a02-" + uuid.NewString() + "@example.com"
	create := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "Frank",
		"last_name":  "Doe",
		"email":      email,
		"password":   "correct",
	}, "")
	create.Body.Close()

	resp := doJSON(t, http.MethodPost, usersURL+"/auth", map[string]string{
		"email":    email,
		"password": "wrong",
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("A02: expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_A03_UnknownEmail(t *testing.T) {
	resp := doJSON(t, http.MethodPost, usersURL+"/auth", map[string]string{
		"email":    "ghost-" + uuid.NewString() + "@example.com",
		"password": "anypassword",
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("A03: expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_A04_MissingPassword(t *testing.T) {
	resp := doJSON(t, http.MethodPost, usersURL+"/auth", map[string]string{
		"email": "a04@example.com",
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("A04: expected 401 (bad credentials), got %d", resp.StatusCode)
	}
}

// ── GET /users ───────────────────────────────────────────────────────────────

func TestListUsers_L01_OK(t *testing.T) {
	id, token := createUser(t)

	resp := doJSON(t, http.MethodGet, usersURL+"/users", nil, token)
	var users []map[string]any
	decodeBody(t, resp, &users)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("L01: expected 200, got %d", resp.StatusCode)
	}

	found := false
	for _, u := range users {
		if u["id"] == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("L01: created user %s not in list", id)
	}
}

func TestListUsers_L02_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodGet, usersURL+"/users", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("L02: expected 401, got %d", resp.StatusCode)
	}
}

func TestListUsers_L03_InvalidToken(t *testing.T) {
	resp := doJSON(t, http.MethodGet, usersURL+"/users", nil, "not.a.jwt")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("L03: expected 401, got %d", resp.StatusCode)
	}
}

// ── GET /users/:id ───────────────────────────────────────────────────────────

func TestGetUser_G01_OK(t *testing.T) {
	id, token := createUser(t)

	resp := doJSON(t, http.MethodGet, usersURL+"/users/"+id, nil, token)
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("G01: expected 200, got %d", resp.StatusCode)
	}
	if body["id"] != id {
		t.Fatalf("G01: expected id %s, got %v", id, body["id"])
	}
}

func TestGetUser_G02_NotFound(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodGet, usersURL+"/users/"+uuid.NewString(), nil, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("G02: expected 404, got %d", resp.StatusCode)
	}
}

func TestGetUser_G03_InvalidUUID(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodGet, usersURL+"/users/not-a-uuid", nil, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("G03: expected 400, got %d", resp.StatusCode)
	}
}

func TestGetUser_G04_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodGet, usersURL+"/users/"+uuid.NewString(), nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("G04: expected 401, got %d", resp.StatusCode)
	}
}

// ── PATCH /users/:id ─────────────────────────────────────────────────────────

func TestUpdateUser_UP01_OK(t *testing.T) {
	id, token := createUser(t)

	newName := "Updated"
	resp := doJSON(t, http.MethodPatch, usersURL+"/users/"+id, map[string]string{
		"first_name": newName,
	}, token)
	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("UP01: expected 200, got %d", resp.StatusCode)
	}
	if body["first_name"] != newName {
		t.Fatalf("UP01: expected first_name %q, got %v", newName, body["first_name"])
	}
}

func TestUpdateUser_UP02_EmailTaken(t *testing.T) {
	id1, token1 := createUser(t)

	// Create a second user whose email we'll try to steal.
	email2 := "up02-" + uuid.NewString() + "@example.com"
	create2 := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "Second",
		"last_name":  "User",
		"email":      email2,
		"password":   "pass1234",
	}, "")
	create2.Body.Close()

	resp := doJSON(t, http.MethodPatch, usersURL+"/users/"+id1, map[string]string{
		"email": email2,
	}, token1)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("UP02: expected 409, got %d", resp.StatusCode)
	}
}

func TestUpdateUser_UP03_InvalidEmail(t *testing.T) {
	id, token := createUser(t)

	resp := doJSON(t, http.MethodPatch, usersURL+"/users/"+id, map[string]string{
		"email": "bad-email",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("UP03: expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateUser_UP04_NotFound(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodPatch, usersURL+"/users/"+uuid.NewString(), map[string]string{
		"first_name": "Ghost",
	}, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("UP04: expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateUser_UP05_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodPatch, usersURL+"/users/"+uuid.NewString(), map[string]string{
		"first_name": "X",
	}, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("UP05: expected 401, got %d", resp.StatusCode)
	}
}

// ── DELETE /users/:id ────────────────────────────────────────────────────────

func TestDeleteUser_D01_EmptyWallet(t *testing.T) {
	id, token := createUser(t)

	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+id, nil, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("D01: expected 204, got %d", resp.StatusCode)
	}
}

func TestDeleteUser_D02_WalletHasBalance(t *testing.T) {
	id, token := createUser(t)
	creditUser(t, token, 50)

	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+id, nil, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("D02: expected 422, got %d", resp.StatusCode)
	}
}

func TestDeleteUser_D03_NotFound(t *testing.T) {
	_, token := createUser(t)

	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+uuid.NewString(), nil, token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("D03: expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteUser_D04_NoToken(t *testing.T) {
	resp := doJSON(t, http.MethodDelete, usersURL+"/users/"+uuid.NewString(), nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("D04: expected 401, got %d", resp.StatusCode)
	}
}
