package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/IbnBaqqi/transcendence/internal/auth"
	"github.com/google/uuid"
)

func sanitizeLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

type keyStore interface {
	Authenticate(ctx context.Context, raw string) (uuid.UUID, auth.User, error)
}

const keyHeader = "X-API-Key"

type apiKeyIDKey struct{}

func apiKeyID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(apiKeyIDKey{}).(uuid.UUID)
	return id, ok
}

func Authenticate(authService *auth.JwtService, keys keyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, err := auth.GetBearerToken(r.Header)
			if err != nil {
				if presented := r.Header.Get(keyHeader); presented != "" && keys != nil {
					id, user, err := keys.Authenticate(r.Context(), presented)
					if err != nil {
						slog.Warn("rejected API key", "path", sanitizeLog(r.URL.Path)) // #nosec G706 -- path sanitized by sanitizeLog
						writeAuthzError(w, http.StatusUnauthorized, "Invalid API key")
						return
					}

					ctx := auth.WithUser(r.Context(), user)
					ctx = context.WithValue(ctx, apiKeyIDKey{}, id)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				slog.Debug("no auth token", "path", sanitizeLog(r.URL.Path), "error", err.Error()) // #nosec G706 -- path sanitized by sanitizeLog
				next.ServeHTTP(w, r)
				return
			}

			if r.Header.Get(keyHeader) != "" {
				slog.Debug("both a bearer token and an API key were sent; using the token")
			}

			claims, err := authService.VerifyAccessToken(tokenStr)
			if err != nil {
				slog.Warn("invalid auth token", "path", sanitizeLog(r.URL.Path), "error", err.Error()) // #nosec G706 -- path sanitized by sanitizeLog

				if errors.Is(err, auth.ErrExpiredToken) {
					r = r.WithContext(auth.WithExpiredToken(r.Context()))
				}

				next.ServeHTTP(w, r)
				return
			}

			id, err := uuid.Parse(claims.Subject)
			if err != nil {
				slog.Error("invalid user ID in token", "subject", sanitizeLog(claims.Subject), "error", err.Error()) // #nosec G706 -- subject sanitized by sanitizeLog
				next.ServeHTTP(w, r)
				return
			}

			user := auth.User{
				ID:   id,
				Role: claims.Role,
				Name: claims.Name,
			}

			slog.Debug("authenticated request", "user_id", user.ID, "role", user.Role, "path", sanitizeLog(r.URL.Path)) // #nosec G706 -- path sanitized by sanitizeLog
			ctx := auth.WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SessionOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, viaKey := apiKeyID(r.Context()); viaKey {
			writeAuthzError(w, http.StatusForbidden, "Log in to manage API keys")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequiredAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.UserFromContext(r.Context()); !ok {
			if auth.TokenExpired(r.Context()) {
				w.Header().Set("WWW-Authenticate",
					`Bearer error="invalid_token", error_description="The access token expired"`)
			} else {
				w.Header().Set("WWW-Authenticate", `Bearer`)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Authentication required"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
