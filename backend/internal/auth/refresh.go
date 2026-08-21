package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/google/uuid"
)

const (
	RefreshTokenTTL    = 7 * 24 * time.Hour
	RefreshGracePeriod = 30 * time.Second
)

const (
	reasonRotated = "rotated"
	reasonLogout  = "logout"
)

func hashSession(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Service) IssueSession(ctx context.Context, q *database.Queries, userID uuid.UUID) (string, error) {
	raw := MakeRefreshToken()

	if err := q.DeleteDeadSessionsForUser(ctx, database.DeleteDeadSessionsForUserParams{
		UserID:        userID,
		RevokedBefore: sql.NullTime{Time: time.Now().Add(-RefreshGracePeriod), Valid: true},
	}); err != nil {
		return "", err
	}

	if err := q.StoreSession(ctx, database.StoreSessionParams{
		TokenHash: hashSession(raw),
		UserID:    userID,
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
	}); err != nil {
		return "", err
	}

	return raw, nil
}

func (s *Service) RedeemSession(ctx context.Context, raw string) (LoginResult, error) {
	if raw == "" {
		return LoginResult{}, &AuthError{Message: "Invalid refresh token"}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginResult{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("refresh transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	session, err := qtx.FindLiveSession(ctx, database.FindLiveSessionParams{
		TokenHash:    hashSession(raw),
		RevokedAfter: sql.NullTime{Time: time.Now().Add(-RefreshGracePeriod), Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoginResult{}, &AuthError{Message: "Invalid refresh token"}
		}
		return LoginResult{}, err
	}

	rotate := !session.RevokedAt.Valid
	if rotate {
		rows, err := qtx.RevokeSession(ctx, database.RevokeSessionParams{
			TokenHash: session.TokenHash,
			Reason:    sql.NullString{String: reasonRotated, Valid: true},
		})
		if err != nil {
			return LoginResult{}, err
		}
		rotate = rows == 1
	}
	if !rotate {
		slog.Info("refresh token reused inside the grace window", "user_id", session.UserID)
	}

	user, err := qtx.GetUser(ctx, session.UserID)
	if err != nil {
		return LoginResult{}, err
	}

	var next string
	if rotate {
		next, err = s.IssueSession(ctx, qtx, user.ID)
		if err != nil {
			return LoginResult{}, err
		}
	}

	accessToken, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return LoginResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: next,
		User:         toUserInfo(user),
	}, nil
}

func (s *Service) EndSession(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}

	session, err := s.db.FindSessionByHash(ctx, hashSession(raw))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	return s.db.RevokeSessionsForUser(ctx, session.UserID)
}
