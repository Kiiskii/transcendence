package service

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/IbnBaqqi/transcendence/internal/database"
)

const (
	followeeConstraint = "follows_followee_id_fkey"
	followerConstraint = "follows_follower_id_fkey"
)

type FollowService struct {
	db *database.Queries
}

func NewFollowService(db *database.Queries) *FollowService {
	return &FollowService{db: db}
}

func (s *FollowService) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if followerID == followeeID {
		return &ValidationError{Message: "You cannot follow yourself"}
	}

	err := s.db.FollowUser(ctx, database.FollowUserParams{
		FollowerID: followerID,
		FolloweeID: followeeID,
	})
	if isForeignKeyViolation(err, followeeConstraint, followerConstraint) {
		return &NotFoundError{Message: "User not found"}
	}
	return err
}

func (s *FollowService) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	_, err := s.db.UnfollowUser(ctx, database.UnfollowUserParams{
		FollowerID: followerID,
		FolloweeID: followeeID,
	})
	return err
}

func (s *FollowService) ListFollowing(ctx context.Context, userID uuid.UUID) ([]database.ListFollowingRow, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.db.ListFollowing(ctx, userID)
}

func (s *FollowService) ListFollowers(ctx context.Context, userID uuid.UUID) ([]database.ListFollowersRow, error) {
	if err := s.requireUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.db.ListFollowers(ctx, userID)
}

func (s *FollowService) requireUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.db.GetUser(ctx, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Message: "User not found"}
		}
		return err
	}
	return nil
}

func isForeignKeyViolation(err error, constraints ...string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23503" {
		return false
	}
	return slices.Contains(constraints, pqErr.Constraint)
}
