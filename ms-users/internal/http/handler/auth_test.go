package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	domain "github.com/ilia/ms-users/internal/domain/user"
	"github.com/ilia/ms-users/internal/http/handler"
)

const testJWTKey = "PRIVATEKEY"

type mockAuthService struct {
	authenticateFn func(email, password string) (domain.User, error)
}

func (m *mockAuthService) Authenticate(email, password string) (domain.User, error) {
	return m.authenticateFn(email, password)
}

func TestAuth_Success(t *testing.T) {
	userID := uuid.New()
	svc := &mockAuthService{
		authenticateFn: func(email, password string) (domain.User, error) {
			return domain.User{ID: userID, FirstName: "John", LastName: "Doe", Email: email}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{"email": "john@example.com", "password": "secret"})
	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.NewAuthHandler(svc, testJWTKey).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		User        domain.User `json:"user"`
		AccessToken string      `json:"access_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.User.ID != userID {
		t.Errorf("expected user ID %v, got %v", userID, resp.User.ID)
	}
	if resp.User.Password != "" {
		t.Error("password must not appear in response")
	}
	if resp.AccessToken == "" {
		t.Fatal("expected access_token in response")
	}

	// token must be valid and carry the correct sub
	tok, err := jwt.Parse(resp.AccessToken, func(t *jwt.Token) (any, error) {
		return []byte(testJWTKey), nil
	})
	if err != nil || !tok.Valid {
		t.Fatalf("access_token is not a valid JWT: %v", err)
	}
	sub, _ := tok.Claims.GetSubject()
	if sub != userID.String() {
		t.Errorf("expected sub %v, got %v", userID, sub)
	}
}

func TestAuth_InvalidCredentials(t *testing.T) {
	svc := &mockAuthService{
		authenticateFn: func(string, string) (domain.User, error) {
			return domain.User{}, domain.ErrUnauthorized
		},
	}

	body, _ := json.Marshal(map[string]string{"email": "x@x.com", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.NewAuthHandler(svc, testJWTKey).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_BadJSON(t *testing.T) {
	svc := &mockAuthService{
		authenticateFn: func(string, string) (domain.User, error) {
			t.Error("service must not be called on bad JSON")
			return domain.User{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()

	handler.NewAuthHandler(svc, testJWTKey).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAuth_InternalError(t *testing.T) {
	svc := &mockAuthService{
		authenticateFn: func(string, string) (domain.User, error) {
			return domain.User{}, errors.New("db down")
		},
	}

	body, _ := json.Marshal(map[string]string{"email": "x@x.com", "password": "p"})
	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.NewAuthHandler(svc, testJWTKey).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}
