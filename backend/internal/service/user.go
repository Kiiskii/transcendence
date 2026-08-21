package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/google/uuid"
)

type UserService struct {
	db *database.Queries
}

func NewUserService(db *database.Queries) *UserService {
	return &UserService{db: db}
}

func (s *UserService) Get(ctx context.Context, userID uuid.UUID) (database.User, error) {
	user, err := s.db.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.User{}, &NotFoundError{Message: "User not found"}
		}
		return database.User{}, err
	}
	return user, nil
}

func (s *UserService) SetShowOnlineStatus(ctx context.Context, userID uuid.UUID, show bool) (database.User, error) {
	user, err := s.db.UpdateShowOnlineStatus(ctx, database.UpdateShowOnlineStatusParams{
		ID:               userID,
		ShowOnlineStatus: show,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.User{}, &NotFoundError{Message: "User not found"}
		}
		return database.User{}, err
	}
	return user, nil
}
