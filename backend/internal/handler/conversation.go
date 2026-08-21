package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/IbnBaqqi/transcendence/internal/dtos"
	"github.com/google/uuid"
)

func (h *Handler) StartConversation(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input dtos.StartConversationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	conv, _, err := h.Conversation.StartConversation(r.Context(), userID, input.ListingID, input.Body)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	h.respondWithConversation(w, r, userID, conv.ID, http.StatusCreated)
}

func (h *Handler) GetConversations(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	rows, err := h.Conversation.ListConversations(r.Context(), userID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToConversationListItems(rows, userID))
}

func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid conversation id")
		return
	}

	h.respondWithConversation(w, r, userID, id, http.StatusOK)
}

func (h *Handler) AcceptConversation(w http.ResponseWriter, r *http.Request) {
	h.decideConversation(w, r, true)
}

func (h *Handler) DeclineConversation(w http.ResponseWriter, r *http.Request) {
	h.decideConversation(w, r, false)
}

func (h *Handler) decideConversation(w http.ResponseWriter, r *http.Request, accept bool) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid conversation id")
		return
	}

	if accept {
		_, err = h.Conversation.Accept(r.Context(), userID, id)
	} else {
		_, err = h.Conversation.Decline(r.Context(), userID, id)
	}
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	h.respondWithConversation(w, r, userID, id, http.StatusOK)
}

func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid conversation id")
		return
	}

	afterID, err := parseOptionalUUID(r.URL.Query().Get("after"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "After must be a UUID")
		return
	}

	limit, err := parseOptionalInt32(r.URL.Query().Get("limit"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Limit must be a positive integer")
		return
	}

	messages, err := h.Conversation.ListMessages(r.Context(), userID, id, afterID, limit)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToMessageResponses(messages))
}

func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid conversation id")
		return
	}

	var input dtos.SendMessageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	msg, err := h.Conversation.SendMessage(r.Context(), userID, id, input.Body)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, dtos.ToMessageResponse(msg))
}

func (h *Handler) MarkConversationRead(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid conversation id")
		return
	}

	if _, err := h.Conversation.MarkRead(r.Context(), userID, id); err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondWithConversation(
	w http.ResponseWriter,
	r *http.Request,
	userID uuid.UUID,
	conversationID uuid.UUID,
	status int,
) {
	conv, other, err := h.Conversation.GetConversationDetail(r.Context(), userID, conversationID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, status, dtos.ToConversationResponse(conv, other, userID))
}

// parseOptionalInt32 reads a non-negative integer from a query string, treating
// an empty value as absent. Still an integer: `limit` is a count, not an id.
func parseOptionalInt32(raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < 0 {
		return 0, errInvalidNumber
	}
	return int32(value), nil
}
