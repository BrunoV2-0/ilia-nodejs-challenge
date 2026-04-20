package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ilia/ms-users/internal/http/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const internalKey = "PRIVATEKEY_INTERNAL"

func parseToken(t *testing.T, authHeader, secret string) *jwt.Token {
	t.Helper()
	raw := strings.TrimPrefix(authHeader, "Bearer ")
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	require.NoError(t, err)
	return tok
}

func TestTransactionsClient_HasBalance_True(t *testing.T) {
	var gotHeader, gotUserID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		gotUserID = r.URL.Query().Get("user_id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]float64{"balance": 150.00})
	}))
	defer srv.Close()

	userID := uuid.New()
	c := client.New(srv.URL, internalKey)

	has, err := c.HasBalance(userID)
	require.NoError(t, err)
	assert.True(t, has)

	tok := parseToken(t, gotHeader, internalKey)
	assert.True(t, tok.Valid)
	sub, _ := tok.Claims.GetSubject()
	assert.Equal(t, "ms-users", sub, "sub must identify the calling service, not the user")

	assert.Equal(t, userID.String(), gotUserID, "user_id must be passed as a query parameter")
}

func TestTransactionsClient_HasBalance_False(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]float64{"balance": 0})
	}))
	defer srv.Close()

	has, err := client.New(srv.URL, internalKey).HasBalance(uuid.New())
	require.NoError(t, err)
	assert.False(t, has)
}

func TestTransactionsClient_HasBalance_UsesInternalKey(t *testing.T) {
	var gotHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]float64{"balance": 0})
	}))
	defer srv.Close()

	client.New(srv.URL, internalKey).HasBalance(uuid.New())

	// token must NOT be valid when checked against the external key
	raw := strings.TrimPrefix(gotHeader, "Bearer ")
	_, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		return []byte("PRIVATEKEY"), nil // external key — must fail
	})
	assert.Error(t, err, "internal token must not be verifiable with the external key")
}
