package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/IbnBaqqi/transcendence/internal/auth"
	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

const refreshTokenCookie = "refresh_token"

const refreshCookiePath = "/api/v1/auth"

// Signup creates the account and starts a session.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var input dtos.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.Auth.Signup(r.Context(), input)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	h.setRefreshTokenCookie(w, result.RefreshToken, auth.RefreshTokenTTL)

	respondWithJSON(w, http.StatusCreated, dtos.AuthResponse{
		AccessToken: result.AccessToken,
		User:        result.User,
	})
}

// Login exchanges credentials for an access token and a session cookie.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req dtos.LoginRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.Auth.Login(r.Context(), req)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	h.setRefreshTokenCookie(w, result.RefreshToken, auth.RefreshTokenTTL)

	respondWithJSON(w, http.StatusOK, dtos.AuthResponse{
		AccessToken: result.AccessToken,
		User:        result.User,
	})
}

// Logout ends every session for the caller and expires the cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshTokenCookie); err == nil {
		if err := h.Auth.EndSession(r.Context(), cookie.Value); err != nil {
			slog.Error("revoking the session failed",
				"request_id", middleware.GetReqID(r.Context()), "error", err)
		}
	}

	h.setRefreshTokenCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshTokenCookie)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	result, err := h.Auth.RedeemSession(r.Context(), cookie.Value)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	if result.RefreshToken != "" {
		h.setRefreshTokenCookie(w, result.RefreshToken, auth.RefreshTokenTTL)
	}

	respondWithJSON(w, http.StatusOK, dtos.AuthResponse{
		AccessToken: result.AccessToken,
		User:        result.User,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	user, err := h.User.Get(r.Context(), userID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.UserInfo{
		ID:       user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	})
}

func (h *Handler) setRefreshTokenCookie(w http.ResponseWriter, value string, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	if ttl < 0 {
		maxAge = -1
	}
	// #nosec G124 -- Secure follows COOKIE_SECURE, true by default
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    value,
		Path:     refreshCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
