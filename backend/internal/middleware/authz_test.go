package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/IbnBaqqi/transcendence/internal/auth"
)

type fakeRoleStore struct {
	role string
	err  error

	askedFor uuid.UUID
}

func (f *fakeRoleStore) GetUserRole(_ context.Context, id uuid.UUID) (string, error) {
	f.askedFor = id
	return f.role, f.err
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name          string
		authenticated bool
		store         fakeRoleStore
		wantStatus    int
		wantNext      bool
		wantBody      string
	}{
		{"an admin passes through", true, fakeRoleStore{role: auth.RoleAdmin}, http.StatusOK, true, ""},
		{"an ordinary user is refused", true, fakeRoleStore{role: auth.RoleUser}, http.StatusForbidden, false, "Forbidden"},
		{"a deleted account is refused, not a 500", true, fakeRoleStore{err: sql.ErrNoRows}, http.StatusForbidden, false, "Forbidden"},
		{"a database failure is a 500", true, fakeRoleStore{err: errors.New("pq: connection refused")}, http.StatusInternalServerError, false, "Internal server error"},
		{"nobody authenticated", false, fakeRoleStore{role: auth.RoleAdmin}, http.StatusUnauthorized, false, "Authentication required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

			id := uuid.New()
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if tt.authenticated {
				req = req.WithContext(auth.WithUser(req.Context(),
					auth.User{ID: id, Role: auth.RoleUser}))
			}
			rec := httptest.NewRecorder()

			store := tt.store
			RequireRole(&store, auth.RoleAdmin)(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantNext {
				t.Errorf("handler ran = %v, want %v", called, tt.wantNext)
			}
			if tt.wantBody == "" {
				return
			}

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
			}
			if body.Error != tt.wantBody {
				t.Errorf("error = %q, want %q", body.Error, tt.wantBody)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			// The 500 must not carry the driver's message out to the client.
			if strings.Contains(rec.Body.String(), "connection refused") {
				t.Errorf("body leaks the database error: %s", rec.Body.String())
			}
			if tt.authenticated && store.askedFor != id {
				t.Errorf("looked up %v, want the authenticated user %v", store.askedFor, id)
			}
		})
	}
}

func TestRequireRoleIsNotAHierarchy(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/members-only", nil)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: uuid.New()}))
	rec := httptest.NewRecorder()

	store := fakeRoleStore{role: auth.RoleAdmin}
	RequireRole(&store, auth.RoleUser)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
