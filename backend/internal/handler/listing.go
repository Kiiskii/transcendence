package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

func (h *Handler) CreateListing(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input dtos.CreateListingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	listing, err := h.Listing.CreateListing(r.Context(), userID, input)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, dtos.ToListingResponse(listing))
}

func (h *Handler) GetListings(w http.ResponseWriter, r *http.Request) {
	listings, err := h.Listing.ListListings(r.Context())
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	ids := make([]uuid.UUID, 0, len(listings))
	for _, l := range listings {
		ids = append(ids, l.ID)
	}

	byListing, err := h.ListingImage.ImagesByListing(r.Context(), ids)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToListingResponsesWithImages(listings, byListing))
}

func (h *Handler) GetListing(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid listing id")
		return
	}

	listing, err := h.Listing.GetListing(r.Context(), id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	imgs, err := h.ListingImage.ListImages(r.Context(), id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToListingResponseWithImages(listing, imgs))
}

func (h *Handler) UpdateListing(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid listing id")
		return
	}

	var input dtos.UpdateListingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	updated, err := h.Listing.UpdateListing(r.Context(), userID, id, input)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	imgs, err := h.ListingImage.ListImages(r.Context(), id)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, dtos.ToListingResponseWithImages(updated, imgs))
}

func (h *Handler) DeleteListing(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid listing id")
		return
	}

	if err := h.Listing.DeleteListing(r.Context(), userID, id); err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SearchListings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := dtos.ListingSearchQuery{
		Keyword:  q.Get("keyword"),
		Category: q.Get("category"),
		MinPrice: q.Get("min_price"),
		MaxPrice: q.Get("max_price"),
		Location: q.Get("location"),
		Sort:     q.Get("sort"),
		Page:     q.Get("page"),
		Limit:    q.Get("limit"),
	}

	result, err := h.Listing.SearchListings(r.Context(), query)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusOK, result)
}
