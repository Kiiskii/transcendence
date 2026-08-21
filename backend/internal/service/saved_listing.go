package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/google/uuid"
)

// SavedListingService holds the wishlist rules.
type SavedListingService struct {
	db *database.Queries
}

func NewSavedListingService(db *database.Queries) *SavedListingService {
	return &SavedListingService{db: db}
}

// SaveListing bookmarks a listing for a user.
func (s *SavedListingService) SaveListing(ctx context.Context, userID uuid.UUID, listingID uuid.UUID) error {
	if _, err := s.db.GetListing(ctx, listingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Message: "Listing not found"}
		}
		return err
	}

	return s.db.SaveListing(ctx, database.SaveListingParams{
		UserID:    userID,
		ListingID: listingID,
	})
}

// UnsaveListing removes a bookmark, reporting 404 when there wasn't one.
func (s *SavedListingService) UnsaveListing(ctx context.Context, userID uuid.UUID, listingID uuid.UUID) error {
	rows, err := s.db.UnsaveListing(ctx, database.UnsaveListingParams{
		UserID:    userID,
		ListingID: listingID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return &NotFoundError{Message: "Listing was not saved"}
	}
	return nil
}

// ListSaved returns the user's wishlist, most recently saved first.
func (s *SavedListingService) ListSaved(ctx context.Context, userID uuid.UUID) ([]database.Listing, error) {
	return s.db.ListSavedListings(ctx, userID)
}
