package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

var errInvalidNumber = errors.New("invalid number")

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
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

	respondWithJSON(w, http.StatusOK, dtos.ToUserSettingsResponse(user))
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input dtos.UpdateSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.ShowOnlineStatus == nil {
		respondWithError(w, http.StatusBadRequest, "No settings to update")
		return
	}

	user, err := h.User.SetShowOnlineStatus(r.Context(), userID, *input.ShowOnlineStatus)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToUserSettingsResponse(user))
}

func (h *Handler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	count, err := h.Conversation.CountUnread(r.Context(), userID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.UnreadCountResponse{UnreadCount: count})
}
