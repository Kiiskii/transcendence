package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/IbnBaqqi/transcendence/internal/oauth"
)

type OAuthLogin struct {
	Provider       string
	ProviderUserID string
	Email          string
}

const (
	usernameAttempts   = 5
	maxDerivedUsername = 30
)

const (
	identityConstraint     = "oauth_identities_pkey"
	userProviderConstraint = "oauth_identities_user_provider_uq"
)

func (s *Service) LoginWithIdentity(ctx context.Context, in OAuthLogin) (LoginResult, error) {
	found, err := s.db.FindUserByProviderIdentity(ctx, database.FindUserByProviderIdentityParams{
		Provider:       in.Provider,
		ProviderUserID: in.ProviderUserID,
	})
	if err == nil {
		return s.sessionFor(ctx, found.User)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return LoginResult{}, fmt.Errorf("oauth login: find identity: %w", err)
	}

	if in.Email == "" {
		return LoginResult{}, &ValidationError{
			Message: fmt.Sprintf("Your %s account has no verified email address", providerLabel(in.Provider)),
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("oauth login transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	user, err := qtx.GetUserByEmail(ctx, in.Email)
	switch {
	case err == nil:
		// Signup does not verify addresses, so a password account may belong to
		// someone who does not own this one. Only link password-less rows.
		if user.Password.Valid {
			return LoginResult{}, &AccountExistsError{
				Message: fmt.Sprintf(
					"An account with this email already exists — sign in with your password to link %s",
					providerLabel(in.Provider)),
			}
		}
	case errors.Is(err, sql.ErrNoRows):
		user, err = createOAuthUser(ctx, qtx, in.Email)
		if err != nil {
			return LoginResult{}, err
		}
	default:
		return LoginResult{}, fmt.Errorf("oauth login: look up email: %w", err)
	}

	if err := qtx.LinkIdentity(ctx, database.LinkIdentityParams{
		Provider:       in.Provider,
		ProviderUserID: in.ProviderUserID,
		UserID:         user.ID,
	}); err != nil {
		if conflict := duplicateIdentityError(err, in.Provider); conflict != nil {
			return LoginResult{}, conflict
		}
		return LoginResult{}, fmt.Errorf("oauth login: link identity: %w", err)
	}

	refreshToken, err := s.IssueSession(ctx, qtx, user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: store session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: commit: %w", err)
	}

	accessToken, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: issue token: %w", err)
	}

	info, err := s.UserInfo(ctx, user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: %w", err)
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         info,
	}, nil
}

func (s *Service) sessionFor(ctx context.Context, user database.User) (LoginResult, error) {
	accessToken, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: issue token: %w", err)
	}

	refreshToken, err := s.IssueSession(ctx, s.db.Queries, user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: store session: %w", err)
	}

	info, err := s.UserInfo(ctx, user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("oauth login: %w", err)
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         info,
	}, nil
}

func createOAuthUser(ctx context.Context, q *database.Queries, email string) (database.User, error) {
	username, err := pickUsername(ctx, q, email)
	if err != nil {
		return database.User{}, err
	}

	user, err := q.CreateUser(ctx, database.CreateUserParams{
		ID:       database.NewID(),
		Username: username,
		Email:    email,
		Password: sql.NullString{},
	})
	if err != nil {
		if duplicateUserError(err) != nil {
			return database.User{}, &RetryError{Message: "That email was just registered — try again"}
		}
		return database.User{}, fmt.Errorf("oauth login: create user: %w", err)
	}

	if err := q.EnsureProfile(ctx, user.ID); err != nil {
		return database.User{}, fmt.Errorf("oauth login: create profile: %w", err)
	}

	return user, nil
}

func pickUsername(ctx context.Context, q *database.Queries, email string) (string, error) {
	base := usernameFromEmail(email)

	for attempt := range usernameAttempts {
		candidate := base
		if attempt > 0 {
			candidate = base + "-" + randomSuffix()
		}

		taken, err := q.EmailOrUsernameTaken(ctx, database.EmailOrUsernameTakenParams{
			Email:    email,
			Username: candidate,
		})
		if err != nil {
			return "", fmt.Errorf("oauth login: check username: %w", err)
		}
		if !taken.UsernameTaken {
			return candidate, nil
		}
	}

	return "", errors.New("oauth login: no free username after retries")
}

func usernameFromEmail(email string) string {
	local, _, _ := strings.Cut(email, "@")
	local = strings.ToLower(local)
	local, _, _ = strings.Cut(local, "+")

	var b strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}

	name := strings.Trim(b.String(), "._-")
	if len(name) > maxDerivedUsername {
		name = strings.Trim(name[:maxDerivedUsername], "._-")
	}
	if name == "" {
		name = "forager"
	}
	return name
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func duplicateIdentityError(err error, provider string) error {
	switch {
	case isUniqueViolation(err, identityConstraint):
		return &RetryError{Message: "That sign-in was already completed — try again"}
	case isUniqueViolation(err, userProviderConstraint):
		return &ConflictError{
			Message: fmt.Sprintf("This account is already linked to a different %s account", providerLabel(provider)),
		}
	}
	return nil
}

func providerLabel(provider string) string {
	switch provider {
	case oauth.ProviderGoogle:
		return "Google"
	case oauth.ProviderGitHub:
		return "GitHub"
	}
	return provider
}
