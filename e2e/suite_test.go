package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	tccompose "github.com/testcontainers/testcontainers-go/modules/compose"
)

var (
	usersURL        = "http://localhost:3002"
	transactionsURL = "http://localhost:3001"

	// internalToken is minted once before the test suite runs.
	// It identifies this test process as "ms-e2e" to ms-users, which now
	// requires the internal JWT on all routes.
	internalToken string
)

const (
	// usersInternalJWTKey must match JWT_INTERNAL_KEY in ms-users/.env.
	usersInternalJWTKey = "ILIACHALLENGE_INTERNAL"

	// e2ePassword is a password that satisfies the strength policy
	// (≥8 chars, letter+digit, zxcvbn score ≥2).
	e2ePassword = "E2eStr0ng!Pass99"
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	stack, err := tccompose.NewDockerCompose("../docker-compose.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "compose init: %v\n", err)
		return 1
	}

	if err := stack.Up(ctx, tccompose.Wait(true)); err != nil {
		fmt.Fprintf(os.Stderr, "compose up: %v\n", err)
		return 1
	}
	defer stack.Down(ctx, tccompose.RemoveOrphans(true)) //nolint:errcheck

	if err := waitForHTTP(usersURL+"/users", 30); err != nil {
		fmt.Fprintf(os.Stderr, "ms-users not ready: %v\n", err)
		return 1
	}
	if err := waitForHTTP(transactionsURL+"/transactions", 30); err != nil {
		fmt.Fprintf(os.Stderr, "ms-transactions not ready: %v\n", err)
		return 1
	}

	tok, err := mintInternalToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint internal token: %v\n", err)
		return 1
	}
	internalToken = tok

	return m.Run()
}

// mintInternalToken creates a short-lived HS256 JWT identifying this process
// as "ms-e2e", signed with the shared internal key.
func mintInternalToken() (string, error) {
	claims := jwt.MapClaims{
		"sub": "ms-e2e",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(usersInternalJWTKey))
}

// waitForHTTP polls url until it gets any HTTP response or maxAttempts is reached.
// A 401 counts as "ready" — the service is up and rejecting unauthenticated requests.
func waitForHTTP(url string, maxAttempts int) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := range maxAttempts {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Second)
		}
	}
	return fmt.Errorf("not ready after %d attempts", maxAttempts)
}

// doJSON sends a JSON request and returns the response. Caller must close resp.Body.
func doJSON(t *testing.T, method, url string, body any, token string) *http.Response {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, url, err)
	}
	return resp
}

// decodeBody decodes the response body into dst and closes it.
func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

// createUser creates a unique user via the internal API and returns (userID, userJWT).
// The returned token is the user-facing JWT (JWT_KEY) for use with ms-transactions.
// Use internalToken for subsequent ms-users calls.
func createUser(t *testing.T) (id, token string) {
	t.Helper()
	email := "e2e-" + uuid.NewString() + "@example.com"

	resp := doJSON(t, http.MethodPost, usersURL+"/users", map[string]string{
		"first_name": "E2E",
		"last_name":  "User",
		"email":      email,
		"password":   e2ePassword,
	}, internalToken)
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("createUser: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	decodeBody(t, resp, &created)
	id = created["id"].(string)

	authResp := doJSON(t, http.MethodPost, usersURL+"/auth", map[string]string{
		"email":    email,
		"password": e2ePassword,
	}, internalToken)
	if authResp.StatusCode != http.StatusOK {
		authResp.Body.Close()
		t.Fatalf("createUser/auth: expected 200, got %d", authResp.StatusCode)
	}
	var auth map[string]any
	decodeBody(t, authResp, &auth)
	token = auth["access_token"].(string)

	return id, token
}

// creditUser posts a CREDIT transaction of the given amount.
func creditUser(t *testing.T, token string, amount float64) {
	t.Helper()
	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": amount,
		"type":   "CREDIT",
	}, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("creditUser: expected 201, got %d", resp.StatusCode)
	}
}

// debitUser posts a DEBIT transaction of the given amount.
func debitUser(t *testing.T, token string, amount float64) {
	t.Helper()
	resp := doJSON(t, http.MethodPost, transactionsURL+"/transactions", map[string]any{
		"amount": amount,
		"type":   "DEBIT",
	}, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("debitUser: expected 201, got %d", resp.StatusCode)
	}
}
