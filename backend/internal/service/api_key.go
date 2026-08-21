package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/IbnBaqqi/transcendence/internal/auth"

	"github.com/IbnBaqqi/transcendence/internal/database"
)

const KeyPrefix = "fk_live_"

const prefixLength = len(KeyPrefix) + 4

const maxKeyNameLength = 60

type APIKeyService struct {
	db *database.Queries
}

func NewAPIKeyService(db *database.Queries) *APIKeyService {
	return &APIKeyService{db: db}
}

type IssuedKey struct {
	Key    string
	Record database.ApiKey
}

func (s *APIKeyService) Create(ctx context.Context, userID uuid.UUID, name string) (IssuedKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return IssuedKey{}, &ValidationError{Message: "Name is required"}
	}
	if len([]rune(name)) > maxKeyNameLength {
		return IssuedKey{}, &ValidationError{Message: "Name must be 60 characters or fewer"}
	}

	raw := KeyPrefix + randomHex()

	record, err := s.db.CreateKey(ctx, database.CreateKeyParams{
		ID:        database.NewID(),
		UserID:    userID,
		Name:      name,
		KeyHash:   hashKey(raw),
		KeyPrefix: raw[:prefixLength],
	})
	if err != nil {
		return IssuedKey{}, err
	}

	return IssuedKey{Key: raw, Record: record}, nil
}

func (s *APIKeyService) List(ctx context.Context, userID uuid.UUID) ([]database.ListKeysForUserRow, error) {
	return s.db.ListKeysForUser(ctx, userID)
}

func (s *APIKeyService) Revoke(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	rows, err := s.db.RevokeKey(ctx, database.RevokeKeyParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if rows == 0 {
		return &NotFoundError{Message: "API key not found"}
	}
	return nil
}

func (s *APIKeyService) Authenticate(ctx context.Context, raw string) (uuid.UUID, auth.User, error) {
	if !strings.HasPrefix(raw, KeyPrefix) {
		return uuid.Nil, auth.User{}, ErrKeyNotUsable
	}

	row, err := s.db.FindLiveKeyByHash(ctx, hashKey(raw))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, auth.User{}, ErrKeyNotUsable
		}
		return uuid.Nil, auth.User{}, err
	}

	return row.ID, auth.User{ID: row.UserID, Name: row.Username, Role: row.Role}, nil
}

var ErrKeyNotUsable = errors.New("API key is not usable")

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomHex() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read is documented never to fail
	return hex.EncodeToString(b)
}
