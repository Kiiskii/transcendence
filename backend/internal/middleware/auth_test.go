package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/IbnBaqqi/transcendence/internal/auth"
	"github.com/IbnBaqqi/transcendence/internal/database"
)

func TestExpiredTokenIsDistinguishableFromAbsent(t *testing.T) {
	user := database.User{ID: uuid.New(), Username: "aino", Email: "aino@example.test", Role: "USER"}

	live := auth.NewJwtService("test-secret", time.Hour)
	valid, err := live.IssueAccessToken(user)
	if err != nil {
		t.Fatal(err)
	}

	brief := auth.NewJwtService("test-secret", 2*time.Second)
	stale, err := brief.IssueAccessToken(user)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)

	protected := Authenticate(live, nil)(RequiredAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	tests := []struct {
		name       string
		bearer     string
		wantStatus int
		wantExpiry bool
	}{
		{"a valid token", valid, http.StatusOK, false},
		{"an expired token", stale, http.StatusUnauthorized, true},
		{"no token at all", "", http.StatusUnauthorized, false},
		{"a forged token", "not.a.jwt", http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/me/settings", nil)
			if tt.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tt.bearer)
			}

			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				return
			}

			header := rec.Header().Get("WWW-Authenticate")
			if header == "" {
				t.Fatal("no WWW-Authenticate header on a 401")
			}
			if got := strings.Contains(header, "invalid_token"); got != tt.wantExpiry {
				t.Errorf("WWW-Authenticate = %q, expiry signalled = %v, want %v", header, got, tt.wantExpiry)
			}

			if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"Authentication required"}` {
				t.Errorf("body = %s, want the unchanged generic 401", body)
			}
		})
	}
}
