package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// The three states from the migration's CHECK constraint.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusDeclined = "declined"
)

const (
	maxMessageLength    = 2000
	defaultMessageLimit = 50
	maxMessageLimit     = 100
)

// ConversationService holds the chat rules: who may talk to whom, when.
type ConversationService struct {
	db *database.DB
}

func NewConversationService(db *database.DB) *ConversationService {
	return &ConversationService{db: db}
}

func checkParticipant(c database.Conversation, userID uuid.UUID) error {
	if c.BuyerID != userID && c.SellerID != userID {
		return &NotFoundError{Message: "Conversation not found"}
	}
	return nil
}

// checkCanDecide guards accept/decline: seller only, pending only.
func checkCanDecide(c database.Conversation, userID uuid.UUID) error {
	if err := checkParticipant(c, userID); err != nil {
		return err
	}

	if c.SellerID != userID {
		return &ForbiddenError{Message: "Only the seller can answer a chat request"}
	}
	if c.Status != StatusPending {
		return &ConflictError{Message: "This request has already been answered"}
	}
	return nil
}

// checkCanSend is the state machine for messaging.
func checkCanSend(c database.Conversation, userID uuid.UUID) error {
	if err := checkParticipant(c, userID); err != nil {
		return err
	}

	switch c.Status {
	case StatusAccepted:
		return nil
	case StatusPending:
		return &ConflictError{Message: "The seller has not accepted this chat request yet"}
	default:
		return &ConflictError{Message: "This chat request was declined"}
	}
}

func validateMessageBody(body string) (string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", &ValidationError{Message: "Message cannot be empty"}
	}
	if utf8.RuneCountInString(trimmed) > maxMessageLength {
		return "", &ValidationError{Message: "Message is too long"}
	}
	return trimmed, nil
}

// isUniqueViolation asks Postgres, via the driver, whether an insert failed
// because of a specific unique constrint.
func isUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return pqErr.Code == "23505" && pqErr.Constraint == constraint
}

// StartConversation is the buyer's first contact.
func (s *ConversationService) StartConversation(
	ctx context.Context,
	buyerID uuid.UUID,
	listingID uuid.UUID,
	body string,
) (database.Conversation, database.Message, error) {
	trimmed, err := validateMessageBody(body)
	if err != nil {
		return database.Conversation{}, database.Message{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.Conversation{}, database.Message{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("conversation start transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	listing, err := qtx.GetListing(ctx, listingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Conversation{}, database.Message{}, &NotFoundError{Message: "Listing not found"}
		}
		return database.Conversation{}, database.Message{}, err
	}

	if listing.SellerID == buyerID {
		return database.Conversation{}, database.Message{}, &ValidationError{
			Message: "You cannot start a chat about your own listing",
		}
	}

	conv, err := qtx.CreateConversation(ctx, database.CreateConversationParams{
		ID:           database.NewID(),
		ListingID:    uuid.NullUUID{UUID: listingID, Valid: true},
		ListingTitle: listing.Title,
		BuyerID:      buyerID,
		SellerID:     listing.SellerID,
	})
	if err != nil {
		if isUniqueViolation(err, "conversations_listing_buyer_uq") {
			return database.Conversation{}, database.Message{}, &ConflictError{
				Message: "You have already contacted this seller about this listing",
			}
		}
		return database.Conversation{}, database.Message{}, err
	}

	msg, err := qtx.CreateMessage(ctx, database.CreateMessageParams{
		ID:             database.NewID(),
		ConversationID: conv.ID,
		SenderID:       buyerID,
		Body:           trimmed,
	})
	if err != nil {
		return database.Conversation{}, database.Message{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.Conversation{}, database.Message{}, err
	}

	return conv, msg, nil
}

// decide is the shared body of Accept and Decline
func (s *ConversationService) decide(
	ctx context.Context,
	sellerID uuid.UUID,
	conversationID uuid.UUID,
	status string,
) (database.Conversation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.Conversation{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("conversation decide transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	conv, err := qtx.GetConversationForUpdate(ctx, conversationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Conversation{}, &NotFoundError{Message: "Conversation not found"}
		}
		return database.Conversation{}, err
	}

	if err := checkCanDecide(conv, sellerID); err != nil {
		return database.Conversation{}, err
	}

	updated, err := qtx.UpdateConversationStatus(ctx, database.UpdateConversationStatusParams{
		ID:     conversationID,
		Status: status,
	})
	if err != nil {
		return database.Conversation{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.Conversation{}, err
	}

	return updated, nil
}

func (s *ConversationService) Accept(ctx context.Context, sellerID uuid.UUID, conversationID uuid.UUID) (database.Conversation, error) {
	return s.decide(ctx, sellerID, conversationID, StatusAccepted)
}

func (s *ConversationService) Decline(ctx context.Context, sellerID uuid.UUID, conversationID uuid.UUID) (database.Conversation, error) {
	return s.decide(ctx, sellerID, conversationID, StatusDeclined)
}

// SendMessage appends to an accepted thread and bumps the conversation's
// updated_at, which is what sorts the inbox.
func (s *ConversationService) SendMessage(
	ctx context.Context,
	userID uuid.UUID,
	conversationID uuid.UUID,
	body string,
) (database.Message, error) {
	trimmed, err := validateMessageBody(body)
	if err != nil {
		return database.Message{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.Message{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("send message transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	conv, err := qtx.GetConversationForUpdate(ctx, conversationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Message{}, &NotFoundError{Message: "Conversation not found"}
		}
		return database.Message{}, err
	}

	if err := checkCanSend(conv, userID); err != nil {
		return database.Message{}, err
	}

	msg, err := qtx.CreateMessage(ctx, database.CreateMessageParams{
		ID:             database.NewID(),
		ConversationID: conversationID,
		SenderID:       userID,
		Body:           trimmed,
	})
	if err != nil {
		return database.Message{}, err
	}

	if err := qtx.TouchConversation(ctx, conversationID); err != nil {
		return database.Message{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.Message{}, err
	}

	return msg, nil
}

func (s *ConversationService) ListConversations(ctx context.Context, userID uuid.UUID) ([]database.ListConversationsForUserRow, error) {
	return s.db.ListConversationsForUser(ctx, userID)
}

func (s *ConversationService) GetConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) (database.Conversation, error) {
	conv, err := s.db.GetConversation(ctx, conversationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Conversation{}, &NotFoundError{Message: "Conversation not found"}
		}
		return database.Conversation{}, err
	}

	if err := checkParticipant(conv, userID); err != nil {
		return database.Conversation{}, err
	}

	return conv, nil
}

func (s *ConversationService) ListMessages(
	ctx context.Context,
	userID uuid.UUID,
	conversationID uuid.UUID,
	afterID uuid.UUID,
	limit int32,
) ([]database.Message, error) {
	if _, err := s.GetConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = defaultMessageLimit
	}
	if limit > maxMessageLimit {
		limit = maxMessageLimit
	}

	// uuid.Nil rather than 0: the caller sends no cursor on the first page.
	if afterID != uuid.Nil {
		return s.db.ListMessagesAfter(ctx, database.ListMessagesAfterParams{
			ConversationID: conversationID,
			AfterID:        afterID,
			MaxRows:        limit,
		})
	}

	rows, err := s.db.ListRecentMessages(ctx, database.ListRecentMessagesParams{
		ConversationID: conversationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}

	slices.Reverse(rows)
	return rows, nil
}

// MarkRead return how many messages were marked.
func (s *ConversationService) MarkRead(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) (int64, error) {
	if _, err := s.GetConversation(ctx, userID, conversationID); err != nil {
		return 0, err
	}

	return s.db.MarkMessagesRead(ctx, database.MarkMessagesReadParams{
		ConversationID: conversationID,
		ReaderID:       userID,
	})
}

func (s *ConversationService) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.db.CountUnreadForUser(ctx, userID)
}

func (s *ConversationService) GetConversationDetail(
	ctx context.Context,
	userID uuid.UUID,
	conversationID uuid.UUID,
) (database.Conversation, database.User, error) {
	conv, err := s.GetConversation(ctx, userID, conversationID)
	if err != nil {
		return database.Conversation{}, database.User{}, err
	}

	otherID := conv.SellerID
	if conv.SellerID == userID {
		otherID = conv.BuyerID
	}

	other, err := s.db.GetUser(ctx, otherID)
	if err != nil {
		return database.Conversation{}, database.User{}, err
	}

	return conv, other, nil
}
