package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

// CreateOrder handles POST /api/v1/orders.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input dtos.CreateOrderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	order, err := h.Order.CreateOrder(r.Context(), userID, input)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, dtos.NewOrderResponse(order))
}

// GetOrder handles GET /api/v1/orders/{id}.
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid order id")
		return
	}

	order, err := h.Order.GetOrder(r.Context(), userID, id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.NewOrderResponse(order))
}

// GetOrders handles GET /api/v1/orders - every where the caller is
// either the buyer or the seller.
func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	orders, err := h.Order.ListOrders(r.Context(), userID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.NewOrderResponses(orders))
}

// parseOrderRequest pulls out the two things every status endpoint needs and
// writes the error response itself if either is missin.
func parseOrderRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return uuid.Nil, uuid.Nil, false
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid order id")
		return uuid.Nil, uuid.Nil, false
	}

	return userID, id, true
}

// ConfirmOrder handles POST /api/v1/orders/{id}/confirm - seller accepts.
// pending -> confirm
func (h *Handler) ConfirmOrder(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := parseOrderRequest(w, r)
	if !ok {
		return
	}

	order, err := h.Order.ConfirmOrder(r.Context(), userID, id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.NewOrderResponse(order))
}

// HandoverOrder handles POST /api/v1/orders/{id}/handover - the seller records
// that they handed the goods over.
func (h *Handler) HandoverOrder(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := parseOrderRequest(w, r)
	if !ok {
		return
	}

	order, err := h.Order.HandoverOrder(r.Context(), userID, id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.NewOrderResponse(order))
}

// ReceiveOrder handles POST /api/v1/orders/{id}/receive - the buyer records
// that they got the goods.
func (h *Handler) ReceiveOrder(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := parseOrderRequest(w, r)
	if !ok {
		return
	}

	order, err := h.Order.ReceiveOrder(r.Context(), userID, id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.NewOrderResponse(order))
}

// CancelOrder handles POST /api/v1/orders/{id}/cancel - either side backs out.
// pending|confirmed -> cancelled (terminal, and restores the listings's stock)
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := parseOrderRequest(w, r)
	if !ok {
		return
	}

	order, err := h.Order.CancelOrder(r.Context(), userID, id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.NewOrderResponse(order))
}

// GetOrderEvents handles GET /api/v1/orders/{id}/events.
func (h *Handler) GetOrderEvents(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid order id")
		return
	}

	events, err := h.Order.ListEvents(r.Context(), userID, id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToOrderEventResponses(events))
}
