package handler

import (
	"encoding/json"
	"net/http"

	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input dtos.CreateAPIKeyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	issued, err := h.APIKey.Create(r.Context(), userID, input.Name)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	// The only response that carries the key. There is no endpoint to read it
	// back: the database holds a hash.
	respondWithJSON(w, http.StatusCreated, dtos.NewCreatedAPIKeyResponse(issued.Record, issued.Key))
}

func (h *Handler) GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	rows, err := h.APIKey.List(r.Context(), userID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToAPIKeyResponses(rows))
}

func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid API key id")
		return
	}

	if err := h.APIKey.Revoke(r.Context(), userID, id); err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
