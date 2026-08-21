package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestFollowRejectsYourself(t *testing.T) {
	svc := NewFollowService(nil)

	aino := uuid.New()
	err := svc.Follow(context.Background(), aino, aino)

	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if validation.Message != "You cannot follow yourself" {
		t.Errorf("message = %q, want %q", validation.Message, "You cannot follow yourself")
	}
}
