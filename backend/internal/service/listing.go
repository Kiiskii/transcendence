package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/IbnBaqqi/transcendence/internal/dtos"
	"github.com/google/uuid"
)

// ListingService contains the business logic for listings: validation,
// ownership rules, and orchestration of database calls.
type ListingService struct {
	db    *database.DB
	files fileStore
}

func NewListingService(db *database.DB, files fileStore) *ListingService {
	return &ListingService{db: db, files: files}
}

func validateListingInput(title, category, unit string, price float64, quantity int32) error {
	if title == "" || len(title) > 100 {
		return &ValidationError{Message: "Title is required and must be under 100 characters"}
	}
	if category == "" {
		return &ValidationError{Message: "Category is required"}
	}
	if unit == "" {
		return &ValidationError{Message: "Unit is required"}
	}
	if price <= 0 {
		return &ValidationError{Message: "Price must be greater than 0"}
	}
	if quantity <= 0 {
		return &ValidationError{Message: "Quantity must be greater than 0"}
	}
	return nil
}

func (s *ListingService) CreateListing(ctx context.Context, sellerID uuid.UUID, input dtos.CreateListingInput) (database.Listing, error) {
	if err := validateListingInput(input.Title, input.Category, input.Unit, input.Price, input.Quantity); err != nil {
		return database.Listing{}, err
	}

	return s.db.CreateListing(ctx, database.CreateListingParams{
		ID:          database.NewID(),
		SellerID:    sellerID,
		Title:       input.Title,
		Description: sql.NullString{String: input.Description, Valid: input.Description != ""},
		Category:    input.Category,
		Price:       strconv.FormatFloat(input.Price, 'f', 2, 64),
		Quantity:    input.Quantity,
		Unit:        input.Unit,
	})
}

func (s *ListingService) GetListing(ctx context.Context, id uuid.UUID) (database.Listing, error) {
	listing, err := s.db.GetListing(ctx, id)
	if err != nil {
		return database.Listing{}, &NotFoundError{Message: "Listing not found"}
	}
	return listing, nil
}

func (s *ListingService) ListListings(ctx context.Context) ([]database.Listing, error) {
	return s.db.ListListings(ctx)
}

// UpdateListing edits a listing the caller owns.
func (s *ListingService) UpdateListing(ctx context.Context, userID uuid.UUID, listingID uuid.UUID, input dtos.UpdateListingInput) (database.Listing, error) {
	if err := validateListingInput(input.Title, input.Category, input.Unit, input.Price, input.Quantity); err != nil {
		return database.Listing{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.Listing{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("listing update transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	existing, err := qtx.GetListingForUpdate(ctx, listingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Listing{}, &NotFoundError{Message: "Listing not found"}
		}
		return database.Listing{}, err
	}

	if existing.SellerID != userID {
		return database.Listing{}, &ForbiddenError{Message: "You do not own this listing"}
	}

	if existing.Quantity == 0 {
		return database.Listing{}, &ConflictError{Message: "Listing is sold out and can no longer be edited; create new listing"}
	}

	updated, err := qtx.UpdateListing(ctx, database.UpdateListingParams{
		ID:          listingID,
		Title:       input.Title,
		Description: sql.NullString{String: input.Description, Valid: input.Description != ""},
		Category:    input.Category,
		Price:       strconv.FormatFloat(input.Price, 'f', 2, 64),
		Quantity:    input.Quantity,
		Unit:        input.Unit,
	})
	if err != nil {
		return database.Listing{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.Listing{}, err
	}

	return updated, nil
}

// DeleteListing removes a listing the caller owns.
func (s *ListingService) DeleteListing(ctx context.Context, userID uuid.UUID, listingID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("listing delete transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	existing, err := qtx.GetListingForUpdate(ctx, listingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Message: "Listing not found"}
		}
		return err
	}
	if existing.SellerID != userID {
		return &ForbiddenError{Message: "You do not own this listing"}
	}

	orderCount, err := qtx.CountOrdersForListing(ctx, listingID)
	if err != nil {
		return err
	}
	if orderCount > 0 {
		return &ConflictError{Message: "This listing has orders and cannot be deleted; its order history has to be kept"}
	}

	filenames, err := qtx.DeleteImagesForListing(ctx, listingID)
	if err != nil {
		return err
	}

	if err := qtx.DeleteListing(ctx, listingID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	for _, name := range filenames {
		if err := s.files.Delete(name); err != nil {
			slog.Error("failed to delete image file", "filename", name, "error", err)
		}
	}

	return nil
}

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 50

	maxSearchTextLength = 200
)

func resolveSort(sortKey string) (string, error) {
	if sortKey == "" {
		return database.DefaultSort, nil
	}
	if !database.IsValidSort(sortKey) {
		return "", &ValidationError{
			Message: "Sort must be one of: " + strings.Join(database.SortOptions(), ", "),
		}
	}
	return sortKey, nil
}

func validateSearchText(values ...string) error {
	for _, value := range values {
		if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			return &ValidationError{Message: "Search text must be valid UTF-8 without null bytes"}
		}
		if utf8.RuneCountInString(value) > maxSearchTextLength {
			return &ValidationError{Message: "Search text is too long"}
		}
	}
	return nil
}

func (s *ListingService) SearchListings(ctx context.Context, q dtos.ListingSearchQuery) (dtos.PaginatedListings, error) {
	if err := validateSearchText(q.Keyword, q.Category, q.Location); err != nil {
		return dtos.PaginatedListings{}, err
	}

	page := defaultPage
	if q.Page != "" {
		p, err := strconv.Atoi(q.Page)
		if err != nil || p < 1 || p > math.MaxInt32 {
			return dtos.PaginatedListings{}, &ValidationError{Message: "Page must be a positive integer"}
		}
		page = p
	}

	limit := defaultLimit
	if q.Limit != "" {
		l, err := strconv.Atoi(q.Limit)
		if err != nil || l < 1 {
			return dtos.PaginatedListings{}, &ValidationError{Message: "Limit must be a positive integer"}
		}
		limit = l
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	var minPrice, maxPrice sql.NullString
	var minVal, maxVal float64

	if q.MinPrice != "" {
		v, err := strconv.ParseFloat(q.MinPrice, 64)
		if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return dtos.PaginatedListings{}, &ValidationError{Message: "Min price must be a non-negative number"}
		}
		minVal = v
		minPrice = sql.NullString{String: strconv.FormatFloat(v, 'f', 2, 64), Valid: true}
	}
	if q.MaxPrice != "" {
		v, err := strconv.ParseFloat(q.MaxPrice, 64)
		if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return dtos.PaginatedListings{}, &ValidationError{Message: "Max price must be a non-negative number"}
		}
		maxVal = v
		maxPrice = sql.NullString{String: strconv.FormatFloat(v, 'f', 2, 64), Valid: true}
	}
	if minPrice.Valid && maxPrice.Valid && minVal > maxVal {
		return dtos.PaginatedListings{}, &ValidationError{Message: "Min price must not exceed max_price"}
	}

	sortKey, err := resolveSort(q.Sort)
	if err != nil {
		return dtos.PaginatedListings{}, err
	}

	offset := (page - 1) * limit
	if offset < 0 || offset > math.MaxInt32 {
		return dtos.PaginatedListings{}, &ValidationError{Message: "Page is too large"}
	}

	params := database.SearchListingsParams{
		Keyword:  q.Keyword,
		Category: q.Category,
		Location: q.Location,
		Sort:     sortKey,
		Offset:   int32(offset),
		Limit:    int32(limit),
	}
	if minPrice.Valid {
		params.MinPrice = minPrice.String
	}
	if maxPrice.Valid {
		params.MaxPrice = maxPrice.String
	}

	items, err := s.db.SearchListingsDynamic(ctx, params)
	if err != nil {
		return dtos.PaginatedListings{}, err
	}
	total, err := s.db.CountSearchListingsDynamic(ctx, params)
	if err != nil {
		return dtos.PaginatedListings{}, err
	}

	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	byListing, err := imagesByListing(ctx, s.db.Queries, ids)
	if err != nil {
		return dtos.PaginatedListings{}, err
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	return dtos.PaginatedListings{
		Items:      dtos.ToListingResponsesWithImages(items, byListing),
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}
