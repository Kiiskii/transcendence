package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

// allowedImageTypes maps a DETECTED content type to the extension we stored it under.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// http.DetectContentType looks at (at most) the first 512 bytes.
const sniffLen = 512

func (h *Handler) UploadListingImage(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	listingID, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid listing id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

	file, _, err := r.FormFile("image")
	if err != nil {
		if isTooLarge(err) {
			respondWithError(w, http.StatusRequestEntityTooLarge, "Image is too large")
			return
		}
		respondWithError(w, http.StatusBadRequest, `expected a multipart form with an "image" file field`)
		return
	}
	defer file.Close()

	head := make([]byte, sniffLen)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		if isTooLarge(err) {
			respondWithError(w, http.StatusRequestEntityTooLarge, "Image is too large")
			return
		}
		respondWithError(w, http.StatusBadRequest, "Could not read uploaded file")
		return
	}
	head = head[:n]

	ext, ok := detectImageExt(head)
	if !ok {
		respondWithError(w, http.StatusUnsupportedMediaType, "Only JPEG, PNG and WebP images are allowed")
		return
	}

	full := io.MultiReader(bytes.NewReader(head), file)

	img, err := h.ListingImage.AddImage(r.Context(), userID, listingID, full, ext)
	if err != nil {
		if isTooLarge(err) {
			respondWithError(w, http.StatusRequestEntityTooLarge, "Image is too large")
			return
		}
		respondWithServiceError(w, r, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, dtos.ToListingImageResponse(img))
}

func (h *Handler) GetListingImages(w http.ResponseWriter, r *http.Request) {
	listingID, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid listing id")
		return
	}

	imgs, err := h.ListingImage.ListImages(r.Context(), listingID)
	if err != nil {
		respondWithServiceError(w, r, err)
		return
	}
	respondWithJSON(w, http.StatusOK, dtos.ToListingImageResponses(imgs))
}

func (h *Handler) DeleteListingImage(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	listingID, err := parseIDParam(r)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid listing id")
		return
	}

	imageID, err := parseUUIDParam(r, "imageID")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid image id")
		return
	}

	if err := h.ListingImage.DeleteImage(r.Context(), userID, listingID, imageID); err != nil {
		respondWithServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// isTooLarge reports whether an error came from http.MaxBytesReader hitting
// its limit.
func isTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// detectImageExt decides which extension a file is stored under., based on its
// leading bytes.
func detectImageExt(head []byte) (string, bool) {
	ext, ok := allowedImageTypes[http.DetectContentType(head)]
	return ext, ok
}
