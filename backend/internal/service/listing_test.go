package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

func TestResolveSort(t *testing.T) {
	got, err := resolveSort("")
	if err != nil {
		t.Fatalf("resolveSort(\"\") unexpected error = %v", err)
	}
	if got != database.DefaultSort {
		t.Errorf("resolveSort(\"\") = %q, want %q", got, database.DefaultSort)
	}

	for _, key := range database.SortOptions() {
		got, err := resolveSort(key)
		if err != nil {
			t.Errorf("resolveSort(%q) unexpected error = %v", key, err)
		}
		if got != key {
			t.Errorf("resolveSort(%q) = %q, want %q", key, got, key)
		}
	}

	wantMessage := "Sort must be one of: " + strings.Join(database.SortOptions(), ", ")
	for _, key := range []string{"cheapest", "PRICE_ASC", "listings.price ASC"} {
		_, err := resolveSort(key)

		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("resolveSort(%q) err = %v, want *ValidationError", key, err)
		}
		if validation.Message != wantMessage {
			t.Errorf("message = %q, want %q", validation.Message, wantMessage)
		}
	}
}

func TestValidateSearchText(t *testing.T) {
	if err := validateSearchText("chanterelle", "mushrooms", "helsinki"); err != nil {
		t.Errorf("valid text rejected: %v", err)
	}

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"invalid utf-8", "\xff", true},
		{"null byte", "a\x00b", true},
		{"at the limit", strings.Repeat("a", maxSearchTextLength), false},
		{"over the limit", strings.Repeat("a", maxSearchTextLength+1), true},
		{"non-ASCII at the limit", strings.Repeat("ä", maxSearchTextLength), false},
		{"non-ASCII over the limit", strings.Repeat("ä", maxSearchTextLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchText(tt.value)

			var validation *ValidationError
			if got := errors.As(err, &validation); got != tt.want {
				t.Fatalf("rejected = %v, want %v (err = %v)", got, tt.want, err)
			}
		})
	}
}

func TestSearchListingsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		query dtos.ListingSearchQuery
	}{
		{"unknown sort", dtos.ListingSearchQuery{Sort: "cheapest"}},
		{"page overflows int32", dtos.ListingSearchQuery{Page: "6148914691236517207", Limit: "3"}},
		{"page beyond int32", dtos.ListingSearchQuery{Page: "2147483648"}},
		{"min price NaN", dtos.ListingSearchQuery{MinPrice: "NaN"}},
		{"max price infinite", dtos.ListingSearchQuery{MaxPrice: "inf"}},
		{"keyword is not utf-8", dtos.ListingSearchQuery{Keyword: "\xff"}},
		{"location has a null byte", dtos.ListingSearchQuery{Location: "hel\x00sinki"}},
		{"keyword is too long", dtos.ListingSearchQuery{Keyword: strings.Repeat("a", maxSearchTextLength+1)}},
	}

	svc := NewListingService(nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.SearchListings(context.Background(), tt.query)

			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
		})
	}
}
