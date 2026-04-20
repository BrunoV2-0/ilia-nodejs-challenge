package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const CallerKey contextKey = "caller"

// CallerFromContext returns the sub claim from a validated JWT.
// For ms-users (all-internal routes) this is always a service name, e.g. "ms-transactions".
func CallerFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(CallerKey).(string)
	return v, ok && v != ""
}

func JWT(secret string) func(http.Handler) http.Handler {
	key := []byte(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if raw == r.Header.Get("Authorization") || raw == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return key, nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			sub, err := token.Claims.GetSubject()
			if err != nil || sub == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), CallerKey, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
