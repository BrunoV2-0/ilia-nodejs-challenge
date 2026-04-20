package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ilia/ms-users/internal/http/middleware"
)

const testSecret = "PRIVATEKEY"

func makeToken(secret, sub string, exp time.Time) string {
	claims := jwt.MapClaims{"sub": sub}
	if !exp.IsZero() {
		claims["exp"] = exp.Unix()
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return tok
}

func TestJWT_MissingHeader(t *testing.T) {
	h := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWT_InvalidToken(t *testing.T) {
	h := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	h := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(testSecret, "ms-caller", time.Now().Add(-time.Hour)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestJWT_ValidToken_InjectsCaller(t *testing.T) {
	sub := "ms-transactions"
	var gotCaller string

	h := middleware.JWT(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, ok := middleware.CallerFromContext(r.Context())
		if !ok {
			t.Error("expected caller in context")
		}
		gotCaller = caller
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(testSecret, sub, time.Time{}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if gotCaller != sub {
		t.Errorf("expected %q, got %q", sub, gotCaller)
	}
}
