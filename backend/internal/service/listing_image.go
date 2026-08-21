package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/google/uuid"
)

// fileStore is the slice of storage.Local this service actually uses.
type fileStore interface {
	Save(r io.Reader, ext string) (string, error)
	Delete(name string) error
}

// ListingImageService holds the rules for listing photos.
type ListingImageService struct {
	db            *database.DB
	files         fileStore
	maxPerListing int
}

func NewListingImageService(db *database.DB, files fileStore, maxPerListing int) *ListingImageService {
	return &ListingImageService{
		db:            db,
		files:         files,
		maxPerListing: maxPerListing,
	}
}

func (s *ListingImageService) ownedListing(ctx context.Context, userID uuid.UUID, listingID uuid.UUID) error {
	listing, err := s.db.GetListing(ctx, listingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Message: "Listing not found"}
		}
		return err
	}
	if listing.SellerID != userID {
		return &ForbiddenError{Message: "You do not own this listing"}
	}
	return nil
}

// AddImage stores an uploaded file and records it against the listing.
func (s *ListingImageService) AddImage(
	ctx context.Context,
	userID uuid.UUID,
	listingID uuid.UUID,
	r io.Reader,
	ext string,
) (database.ListingImage, error) {
	if err := s.ownedListing(ctx, userID, listingID); err != nil {
		return database.ListingImage{}, err
	}

	count, err := s.db.CountListingImages(ctx, listingID)
	if err != nil {
		return database.ListingImage{}, err
	}
	if count >= int64(s.maxPerListing) {
		return database.ListingImage{}, &ConflictError{
			Message: fmt.Sprintf("A listing can have at most %d images", s.maxPerListing),
		}
	}

	filename, err := s.files.Save(r, ext)
	if err != nil {
		return database.ListingImage{}, err
	}

	img, err := s.createImageRow(ctx, listingID, filename)
	if err != nil {
		if delErr := s.files.Delete(filename); delErr != nil {
			slog.Error("orphaned upload: file written but row insert failed",
				"filename", filename, "error", delErr)
		}
		return database.ListingImage{}, err
	}

	return img, nil
}

// createImageRow locks the listing so concurrent uploads can't read the same
// position or slip past the per-listing cap.
func (s *ListingImageService) createImageRow(
	ctx context.Context,
	listingID uuid.UUID,
	filename string,
) (database.ListingImage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.ListingImage{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("add image transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	if _, err := qtx.GetListingForUpdate(ctx, listingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.ListingImage{}, &NotFoundError{Message: "Listing not found"}
		}
		return database.ListingImage{}, err
	}

	count, err := qtx.CountListingImages(ctx, listingID)
	if err != nil {
		return database.ListingImage{}, err
	}
	if count >= int64(s.maxPerListing) {
		return database.ListingImage{}, &ConflictError{
			Message: fmt.Sprintf("A listing can have at most %d images", s.maxPerListing),
		}
	}

	img, err := qtx.CreateListingImage(ctx, database.CreateListingImageParams{
		ID:        database.NewID(),
		ListingID: listingID,
		Filename:  filename,
	})
	if err != nil {
		return database.ListingImage{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.ListingImage{}, err
	}

	return img, nil
}

// ListImages returns one listing's photos in display order.
func (s *ListingImageService) ListImages(ctx context.Context, listingID uuid.UUID) ([]database.ListingImage, error) {
	return s.db.ListListingImages(ctx, listingID)
}

// ImagesByListing groups photos for MANY listings using a single query.
func (s *ListingImageService) ImagesByListing(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID][]database.ListingImage, error) {
	return imagesByListing(ctx, s.db.Queries, listingIDs)
}

// imagesByListing groups one batch query's rows by listing, so any service can
// attach photos to a page of listings without an N+1.
func imagesByListing(ctx context.Context, db *database.Queries, listingIDs []uuid.UUID) (map[uuid.UUID][]database.ListingImage, error) {
	out := make(map[uuid.UUID][]database.ListingImage, len(listingIDs))
	if len(listingIDs) == 0 {
		return out, nil
	}

	rows, err := db.ListImagesForListings(ctx, listingIDs)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		out[row.ListingID] = append(out[row.ListingID], row)
	}
	return out, nil
}

// DeleteImage remove one photo from a listing the caller owns.
func (s *ListingImageService) DeleteImage(ctx context.Context, userID uuid.UUID, listingID, imageID uuid.UUID) error {
	if err := s.ownedListing(ctx, userID, listingID); err != nil {
		return err
	}

	filename, err := s.db.DeleteListingImage(ctx, database.DeleteListingImageParams{
		ID:        imageID,
		ListingID: listingID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Message: "Image not found"}
		}
		return err
	}

	if err := s.files.Delete(filename); err != nil {
		slog.Error("failed to delete image file", "filename", filename, "error", err)
	}

	return nil
}
