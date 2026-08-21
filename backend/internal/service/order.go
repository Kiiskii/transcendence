package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/IbnBaqqi/transcendence/internal/database"
	"github.com/IbnBaqqi/transcendence/internal/dtos"
	"github.com/google/uuid"
)

type OrderService struct {
	db *database.DB
}

func NewOrderService(db *database.DB) *OrderService {
	return &OrderService{db: db}
}

func (s *OrderService) CreateOrder(ctx context.Context, buyerID uuid.UUID, input dtos.CreateOrderInput) (database.Order, error) {
	if input.ListingID == uuid.Nil {
		return database.Order{}, &ValidationError{Message: "Listing id is required"}
	}
	if input.Quantity <= 0 {
		return database.Order{}, &ValidationError{Message: "Quantity must be greater than 0"}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.Order{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			slog.Error("order transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	listing, err := qtx.GetListingForUpdate(ctx, input.ListingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Order{}, &NotFoundError{Message: "Listing not found"}
		}
		return database.Order{}, err
	}

	if listing.SellerID == buyerID {
		return database.Order{}, &ValidationError{Message: "You cannot order your own listing"}
	}
	if listing.Quantity < input.Quantity {
		return database.Order{}, &ConflictError{Message: "Not enough stock available"}
	}

	if _, err := qtx.DecrementListingQuantity(ctx, database.DecrementListingQuantityParams{
		ID:       listing.ID,
		Quantity: input.Quantity,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Order{}, &ConflictError{Message: "Not enough stock available"}
		}
		return database.Order{}, err
	}

	order, err := qtx.CreateOrder(ctx, database.CreateOrderParams{
		ID:           database.NewID(),
		ListingID:    listing.ID,
		BuyerID:      buyerID,
		SellerID:     listing.SellerID,
		Quantity:     input.Quantity,
		UnitPrice:    listing.Price,
		ListingTitle: listing.Title,
	})
	if err != nil {
		if isForeignKeyViolation(err, buyerConstraint) {
			return database.Order{}, &NotFoundError{Message: "User not found"}
		}
		return database.Order{}, err
	}

	if err := recordEvent(ctx, qtx, order.ID, buyerID, sql.NullString{}, order.Status, ""); err != nil {
		return database.Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.Order{}, err
	}

	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, userID uuid.UUID, orderID uuid.UUID) (database.Order, error) {
	order, err := s.db.GetOrder(ctx, orderID)
	if err != nil {
		return database.Order{}, &NotFoundError{Message: "Order not found"}
	}
	if order.BuyerID != userID && order.SellerID != userID {
		return database.Order{}, &ForbiddenError{Message: "You are not part of this order"}
	}
	return order, nil
}

func (s *OrderService) ListOrders(ctx context.Context, userID uuid.UUID) ([]database.Order, error) {
	return s.db.ListOrdersForUser(ctx, userID)
}

type orderActor int

const (
	actorSeller orderActor = iota
	actorBuyer
	actorEither
)

type handshakeMark int

const (
	markNone handshakeMark = iota
	markSeller
	markBuyer
)

type orderAction struct {
	name             string
	from             []string
	to               string
	actor            orderActor
	restoresStock    bool
	mark             handshakeMark
	blockedAfterMark bool
}

var (
	actionConfirm = orderAction{
		name:  "confirm",
		from:  []string{"pending"},
		to:    "confirmed",
		actor: actorSeller,
	}
	actionHandover = orderAction{
		name:  "hand over",
		from:  []string{"confirmed"},
		to:    "completed",
		actor: actorSeller,
		mark:  markSeller,
	}
	actionReceive = orderAction{
		name:  "confirm receipt of",
		from:  []string{"confirmed"},
		to:    "completed",
		actor: actorBuyer,
		mark:  markBuyer,
	}
	actionCancel = orderAction{
		name:             "cancel",
		from:             []string{"pending", "confirmed"},
		to:               "cancelled",
		actor:            actorEither,
		restoresStock:    true,
		blockedAfterMark: true,
	}
)

func (s *OrderService) ConfirmOrder(ctx context.Context, userID uuid.UUID, orderID uuid.UUID) (database.Order, error) {
	return s.applyAction(ctx, userID, orderID, actionConfirm)
}

func (s *OrderService) HandoverOrder(ctx context.Context, userID uuid.UUID, orderID uuid.UUID) (database.Order, error) {
	return s.applyAction(ctx, userID, orderID, actionHandover)
}

func (s *OrderService) ReceiveOrder(ctx context.Context, userID uuid.UUID, orderID uuid.UUID) (database.Order, error) {
	return s.applyAction(ctx, userID, orderID, actionReceive)
}

func (s *OrderService) CancelOrder(ctx context.Context, userID uuid.UUID, orderID uuid.UUID) (database.Order, error) {
	return s.applyAction(ctx, userID, orderID, actionCancel)
}

func (s *OrderService) applyAction(ctx context.Context, userID uuid.UUID, orderID uuid.UUID, action orderAction) (database.Order, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return database.Order{}, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			slog.Error("order status transaction rollback failed", "error", err)
		}
	}()

	qtx := s.db.Queries.WithTx(tx.Tx)

	order, err := qtx.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return database.Order{}, &NotFoundError{Message: "Order not found"}
		}
		return database.Order{}, err
	}

	if err := checkOrderActor(order, userID, action); err != nil {
		return database.Order{}, err
	}
	if !slices.Contains(action.from, order.Status) {
		return database.Order{}, &ConflictError{
			Message: fmt.Sprintf("Cannot %s an order that is %s", action.name, order.Status),
		}
	}

	if err := checkHandshakeLock(order, action); err != nil {
		return database.Order{}, err
	}

	if action.restoresStock {
		if _, err := qtx.IncrementListingQuantity(ctx, database.IncrementListingQuantityParams{
			ID:       order.ListingID,
			Quantity: order.Quantity,
		}); err != nil {
			return database.Order{}, err
		}
	}

	if action.mark != markNone {
		marked, err := markHandshake(ctx, qtx, order, action)
		if err != nil {
			return database.Order{}, err
		}

		if err := recordEvent(ctx, qtx, order.ID, userID,
			sql.NullString{String: order.Status, Valid: true}, order.Status, markNote(action.mark)); err != nil {
			return database.Order{}, err
		}

		if !bothSidesMarked(marked) {
			if err := tx.Commit(); err != nil {
				return database.Order{}, err
			}
			return marked, nil
		}
	}

	updated, err := qtx.UpdateOrderStatus(ctx, database.UpdateOrderStatusParams{
		ID:     order.ID,
		Status: action.to,
	})
	if err != nil {
		return database.Order{}, err
	}

	if err := recordEvent(ctx, qtx, order.ID, userID,
		sql.NullString{String: order.Status, Valid: true}, action.to, ""); err != nil {
		return database.Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return database.Order{}, err
	}

	return updated, nil
}

func (s *OrderService) ListEvents(ctx context.Context, userID uuid.UUID, orderID uuid.UUID) ([]database.OrderEvent, error) {
	if _, err := s.GetOrder(ctx, userID, orderID); err != nil {
		return nil, err
	}

	return s.db.ListOrderEvents(ctx, orderID)
}

func recordEvent(
	ctx context.Context,
	qtx *database.Queries,
	orderID uuid.UUID,
	actorID uuid.UUID,
	from sql.NullString,
	to string,
	note string,
) error {
	err := qtx.CreateOrderEvent(ctx, database.CreateOrderEventParams{
		ID:         database.NewID(),
		OrderID:    orderID,
		ActorID:    uuid.NullUUID{UUID: actorID, Valid: true},
		FromStatus: from,
		ToStatus:   to,
		Note:       sql.NullString{String: note, Valid: note != ""},
	})
	if isForeignKeyViolation(err, actorConstraint) {
		return &NotFoundError{Message: "User not found"}
	}
	return err
}

const (
	buyerConstraint = "orders_buyer_id_fkey"
	actorConstraint = "order_events_actor_id_fkey"
)

const (
	noteSellerHandover = "seller_handover"
	noteBuyerReceipt   = "buyer_receipt"
)

func markNote(m handshakeMark) string {
	switch m {
	case markSeller:
		return noteSellerHandover
	case markBuyer:
		return noteBuyerReceipt
	default:
		return ""
	}
}

func markHandshake(ctx context.Context, qtx *database.Queries, order database.Order, action orderAction) (database.Order, error) {
	switch action.mark {
	case markSeller:
		if order.SellerHandedOverAt.Valid {
			return database.Order{}, &ConflictError{Message: "You have already marked this order as handed over"}
		}
		return qtx.MarkOrderHandedOver(ctx, order.ID)
	case markBuyer:
		if order.BuyerReceivedAt.Valid {
			return database.Order{}, &ConflictError{Message: "You have already confirmed receipt of this order"}
		}
		return qtx.MarkOrderReceived(ctx, order.ID)
	}
	return order, nil
}

func bothSidesMarked(o database.Order) bool {
	return o.SellerHandedOverAt.Valid && o.BuyerReceivedAt.Valid
}

func checkOrderActor(order database.Order, userID uuid.UUID, action orderAction) error {
	isBuyer := order.BuyerID == userID
	isSeller := order.SellerID == userID

	if !isBuyer && !isSeller {
		return &ForbiddenError{Message: "You are not part of this order"}
	}

	switch action.actor {
	case actorSeller:
		if !isSeller {
			return &ForbiddenError{Message: "Only the seller can " + action.name + " this order"}
		}
	case actorBuyer:
		if !isBuyer {
			return &ForbiddenError{Message: "Only the buyer can " + action.name + " this order"}
		}
	case actorEither:
	}

	return nil
}

func checkHandshakeLock(order database.Order, action orderAction) error {
	if action.blockedAfterMark &&
		(order.SellerHandedOverAt.Valid || order.BuyerReceivedAt.Valid) {
		return &ConflictError{
			Message: "Cannot cancel an order once handover has started",
		}
	}
	return nil
}
