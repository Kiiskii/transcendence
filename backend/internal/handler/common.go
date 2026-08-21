package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/IbnBaqqi/transcendence/internal/auth"
	"github.com/IbnBaqqi/transcendence/internal/service"
)

// getUserID returns the id of the authenticated caller.
func getUserID(r *http.Request) (uuid.UUID, error) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		return uuid.Nil, errors.New("no authenticated user in context")
	}
	return user.ID, nil
}

// parseUUIDParam reads a named URL segment as a UUID. Every id in the API is
// one, so this replaced the int32 and uuid variants that used to sit alongside
// each other.
func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}

// parseIDParam reads the {id} segment, which is the common case.
func parseIDParam(r *http.Request) (uuid.UUID, error) {
	return parseUUIDParam(r, "id")
}

// parseOptionalUUID reads a UUID from a query string, treating an empty value
// as absent. Cursors use it: no "after" means the first page.
func parseOptionalUUID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(raw)
}

func statusFromServiceError(err error) int {
	var validationErr *service.ValidationError
	var notFoundErr *service.NotFoundError
	var forbiddenErr *service.ForbiddenError
	var conflictErr *service.ConflictError

	var authValidationErr *auth.ValidationError
	var authConflictErr *auth.ConflictError
	var authErr *auth.AuthError

	switch {
	case errors.As(err, &validationErr), errors.As(err, &authValidationErr):
		return http.StatusBadRequest
	case errors.As(err, &notFoundErr):
		return http.StatusNotFound
	case errors.As(err, &forbiddenErr):
		return http.StatusForbidden
	case errors.As(err, &conflictErr), errors.As(err, &authConflictErr):
		return http.StatusConflict
	case errors.As(err, &authErr):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// respondWithServiceError turns a service error into a response, choosing the
// audience by status.
func respondWithServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status := statusFromServiceError(err)

	if status >= http.StatusInternalServerError {
		slog.Error("unhandled error",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"error", err)

		respondWithError(w, status, "Something went wrong")
		return
	}

	respondWithError(w, status, err.Error())
}
