package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/IbnBaqqi/transcendence/internal/dtos"
)

type Service struct {
	db  *database.DB
	jwt *JwtService
}

func NewService(db *database.DB, jwt *JwtService) *Service {
	return &Service{
		db:  db,
		jwt: jwt,
	}
}

type SignupResponse struct {
	AccessToken  string
	RefreshToken string
	User         dtos.UserInfo
}

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         dtos.UserInfo
}

func signupFailed(step string, err error) error {
	return fmt.Errorf("signup: %s: %w", step, err)
}

func (s *Service) Signup(ctx context.Context, input dtos.CreateUserRequest) (SignupResponse, error) {
	input = normalizeSignupInput(input)

	if err := validateSignupInput(input); err != nil {
		return SignupResponse{}, err
	}

	taken, err := s.db.EmailOrUsernameTaken(ctx, database.EmailOrUsernameTakenParams{
		Email:    input.Email,
		Username: input.Username,
	})
	if err != nil {
		return SignupResponse{}, signupFailed("look up credentials", err)
	}
	if taken.EmailTaken {
		return SignupResponse{}, &ConflictError{Message: emailTakenMessage}
	}
	if taken.UsernameTaken {
		return SignupResponse{}, &ConflictError{Message: usernameTakenMessage}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return SignupResponse{}, signupFailed("hash password", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SignupResponse{}, signupFailed("begin transaction", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("signup transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	user, err := qtx.CreateUser(ctx, database.CreateUserParams{
		ID:       database.NewID(),
		Username: input.Username,
		Email:    input.Email,
		Password: string(hashed),
	})
	if err != nil {
		if conflict := duplicateUserError(err); conflict != nil {
			return SignupResponse{}, conflict
		}
		return SignupResponse{}, signupFailed("create user", err)
	}

	if err := qtx.EnsureProfile(ctx, user.ID); err != nil {
		return SignupResponse{}, signupFailed("create profile", err)
	}

	refreshToken, err := s.IssueSession(ctx, qtx, user.ID)
	if err != nil {
		return SignupResponse{}, signupFailed("store session", err)
	}

	if err := tx.Commit(); err != nil {
		return SignupResponse{}, signupFailed("commit", err)
	}

	accessToken, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return SignupResponse{}, fmt.Errorf("signup: issue token: %w", err)
	}

	return SignupResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserInfo(user),
	}, nil
}

func (s *Service) Login(ctx context.Context, input dtos.LoginRequest) (LoginResult, error) {
	input.Email = strings.TrimSpace(input.Email)

	if input.Email == "" || input.Password == "" {
		return LoginResult{}, &ValidationError{Message: "Email and password are required"}
	}

	user, err := s.db.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return LoginResult{}, &AuthError{Message: "Invalid email or password"}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return LoginResult{}, &AuthError{Message: "Invalid email or password"}
	}

	accessToken, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("login: issue token: %w", err)
	}

	refreshToken, err := s.IssueSession(ctx, s.db.Queries, user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("login: store session: %w", err)
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserInfo(user),
	}, nil
}

const (
	maxUsernameLength = 50
	maxEmailLength    = 150
	minPasswordLength = 8
	maxPasswordLength = 72
)

const (
	emailTakenMessage    = "Email already in use"
	usernameTakenMessage = "Username already taken"
)

func normalizeSignupInput(in dtos.CreateUserRequest) dtos.CreateUserRequest {
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	return in
}

func validateSignupInput(input dtos.CreateUserRequest) error {
	if input.Username == "" {
		return &ValidationError{Message: "Username is required"}
	}
	if utf8.RuneCountInString(input.Username) > maxUsernameLength {
		return &ValidationError{Message: tooLong("Username", maxUsernameLength)}
	}
	if input.Email == "" {
		return &ValidationError{Message: "Email is required"}
	}
	if utf8.RuneCountInString(input.Email) > maxEmailLength {
		return &ValidationError{Message: tooLong("Email", maxEmailLength)}
	}
	if len(input.Password) < minPasswordLength {
		return &ValidationError{
			Message: fmt.Sprintf("Password must be at least %d bytes", minPasswordLength),
		}
	}
	if len(input.Password) > maxPasswordLength {
		return &ValidationError{Message: passwordTooLong(maxPasswordLength)}
	}
	return nil
}

func tooLong(field string, limit int) string {
	return fmt.Sprintf("%s must be %d characters or fewer", field, limit)
}

func passwordTooLong(limit int) string {
	return fmt.Sprintf("Password must be %d bytes or fewer", limit)
}

const (
	usernameConstraint = "users_username_lower_uq"
	emailConstraint    = "users_email_lower_uq"
)

func duplicateUserError(err error) error {
	switch {
	case isUniqueViolation(err, usernameConstraint):
		return &ConflictError{Message: usernameTakenMessage}
	case isUniqueViolation(err, emailConstraint):
		return &ConflictError{Message: emailTakenMessage}
	}
	return nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return pqErr.Code == "23505" && pqErr.Constraint == constraint
}

func toUserInfo(user database.User) dtos.UserInfo {
	return dtos.UserInfo{
		ID:       user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}
}
