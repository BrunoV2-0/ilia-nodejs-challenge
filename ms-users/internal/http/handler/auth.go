package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	domain "github.com/ilia/ms-users/internal/domain/user"
)

type authService interface {
	Authenticate(email, password string) (domain.User, error)
}

type AuthHandler struct {
	svc    authService
	jwtKey []byte
}

func NewAuthHandler(svc authService, jwtKey string) *AuthHandler {
	return &AuthHandler{svc: svc, jwtKey: []byte(jwtKey)}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	u, err := h.svc.Authenticate(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	token, err := h.mintToken(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":         u,
		"access_token": token,
	})
}

func (h *AuthHandler) mintToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.jwtKey)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
