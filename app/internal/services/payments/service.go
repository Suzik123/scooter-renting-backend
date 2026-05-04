package payments

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	stripeclient "github.com/uniscoot/scooter-renting-backend/app/clients/stripe"
	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	rentalsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/rentals"
	paymentsrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/payments"
)

// PaymentMethodView mirrors the stripe client view but lives in the service
// layer so handlers do not import the client package directly.
type PaymentMethodView struct {
	ID        string `json:"id"`
	Brand     string `json:"brand"`
	Last4     string `json:"last4"`
	ExpMonth  int    `json:"exp_month"`
	ExpYear   int    `json:"exp_year"`
	IsDefault bool   `json:"is_default"`
}

// StripeClient captures the subset of the stripe client used here, defined
// against the stripeclient package types.
type StripeClient interface {
	CreateCustomer(ctx context.Context, email, name string) (string, error)
	CreateSetupIntent(ctx context.Context, customerID string) (clientSecret, setupIntentID string, err error)
	ListPaymentMethods(ctx context.Context, customerID string) ([]stripeclient.PaymentMethodView, error)
	DetachPaymentMethod(ctx context.Context, paymentMethodID string) error
	GetCustomerDefaultPaymentMethod(ctx context.Context, customerID string) (string, error)
	CreateAndConfirmPaymentIntent(ctx context.Context, p stripeclient.ChargeParams) (*stripeclient.ChargeResult, error)
	VerifySignature(payload []byte, sigHeader string) (*stripeclient.Event, error)
}

// PaymentsRepo is the subset of the payments repository used here.
type PaymentsRepo interface {
	Create(ctx context.Context, in paymentsrepo.CreateInput) (*models.Payment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)
	GetByProviderID(ctx context.Context, providerID string) (*models.Payment, error)
	AttachIntent(ctx context.Context, in paymentsrepo.AttachIntentInput) (*models.Payment, error)
	MarkByIDFailed(ctx context.Context, id uuid.UUID, reason string) (*models.Payment, error)
	MarkSucceeded(ctx context.Context, providerID string) (*models.Payment, error)
	MarkFailed(ctx context.Context, providerID, reason string) (*models.Payment, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Payment, int, error)
	HasUnpaidRentals(ctx context.Context, userID uuid.UUID) (bool, error)
	InsertWebhookEvent(ctx context.Context, eventID, eventType string, payload []byte) (bool, error)
}

// UsersRepo is the subset of the users repository used here.
type UsersRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	SetStripeCustomerID(ctx context.Context, id uuid.UUID, customerID string) (*models.User, error)
}

// Service orchestrates Stripe-backed payments.
type Service struct {
	stripe   StripeClient
	payments PaymentsRepo
	users    UsersRepo
	pool     *pgxpool.Pool
	cfg      *config.Config
	log      *zap.Logger
}

func New(stripe StripeClient, payments PaymentsRepo, users UsersRepo, pool *pgxpool.Pool, cfg *config.Config, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{
		stripe:   stripe,
		payments: payments,
		users:    users,
		pool:     pool,
		cfg:      cfg,
		log:      log,
	}
}

// EnsureCustomer returns the user's stripe_customer_id, creating one via
// Stripe and persisting it on first use.
func (s *Service) EnsureCustomer(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if u.StripeCustomerID != nil && *u.StripeCustomerID != "" {
		return *u.StripeCustomerID, nil
	}

	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	custID, err := s.stripe.CreateCustomer(ctx, u.Email, name)
	if err != nil {
		return "", err
	}
	if _, err := s.users.SetStripeCustomerID(ctx, userID, custID); err != nil {
		return "", err
	}
	return custID, nil
}

// CreateSetupIntent returns a client_secret usable by the frontend to
// collect a card via Stripe Elements.
func (s *Service) CreateSetupIntent(ctx context.Context, userID uuid.UUID) (string, string, error) {
	custID, err := s.EnsureCustomer(ctx, userID)
	if err != nil {
		return "", "", err
	}
	return s.stripe.CreateSetupIntent(ctx, custID)
}

// ListPaymentMethods returns the cards attached to the user's customer.
func (s *Service) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]PaymentMethodView, error) {
	custID, err := s.EnsureCustomer(ctx, userID)
	if err != nil {
		return nil, err
	}
	views, err := s.stripe.ListPaymentMethods(ctx, custID)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentMethodView, len(views))
	for i, v := range views {
		out[i] = PaymentMethodView{
			ID:        v.ID,
			Brand:     v.Brand,
			Last4:     v.Last4,
			ExpMonth:  v.ExpMonth,
			ExpYear:   v.ExpYear,
			IsDefault: v.IsDefault,
		}
	}
	return out, nil
}

// DetachPaymentMethod ensures the PM belongs to the user before detaching it.
func (s *Service) DetachPaymentMethod(ctx context.Context, userID uuid.UUID, paymentMethodID string) error {
	custID, err := s.EnsureCustomer(ctx, userID)
	if err != nil {
		return err
	}
	views, err := s.stripe.ListPaymentMethods(ctx, custID)
	if err != nil {
		return err
	}
	owned := false
	for _, v := range views {
		if v.ID == paymentMethodID {
			owned = true
			break
		}
	}
	if !owned {
		return apperrors.Forbidden("payment method not owned by user")
	}
	return s.stripe.DetachPaymentMethod(ctx, paymentMethodID)
}

// UserHasPaymentMethod reports whether the user has at least one card.
func (s *Service) UserHasPaymentMethod(ctx context.Context, userID uuid.UUID) (bool, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	if u.StripeCustomerID == nil || *u.StripeCustomerID == "" {
		return false, nil
	}
	views, err := s.stripe.ListPaymentMethods(ctx, *u.StripeCustomerID)
	if err != nil {
		return false, err
	}
	return len(views) > 0, nil
}

// UserHasUnpaidRentals reports whether the user owes for any prior rental.
func (s *Service) UserHasUnpaidRentals(ctx context.Context, userID uuid.UUID) (bool, error) {
	return s.payments.HasUnpaidRentals(ctx, userID)
}

// ChargeRental records a payment row up-front, attempts the Stripe charge,
// and reconciles the row with the resulting intent. It is designed to never
// return a *business* error to callers: card-missing, Stripe declines, and
// network failures all surface as a persisted 'failed' payment plus a
// PaymentResult with a failure reason. EndRental therefore always closes
// the rental cleanly; the user is reblocked on the next ride via the
// outstanding-balance precondition.
//
// Only programmer/system errors (DB failures, nil inputs) propagate as err.
func (s *Service) ChargeRental(ctx context.Context, rental *models.Rental, user *models.User) (*rentalsvc.PaymentResult, error) {
	if rental == nil || user == nil {
		return nil, apperrors.Internal("nil rental or user")
	}

	currency := strings.ToLower(s.cfg.Stripe.Currency)
	if currency == "" {
		currency = "usd"
	}
	currencyUpper := strings.ToUpper(currency)

	// $0 rides: still record a succeeded zero-amount payment row for
	// uniform downstream handling on the FE and audit trail.
	if rental.TotalCost.Sign() <= 0 {
		row, err := s.payments.Create(ctx, paymentsrepo.CreateInput{
			UserID:        user.ID,
			RentalID:      &rental.ID,
			Amount:        decimal.Zero,
			Currency:      currencyUpper,
			PaymentMethod: models.PaymentMethodCard,
			Status:        models.PaymentSucceeded,
		})
		if err != nil {
			return nil, err
		}
		return &rentalsvc.PaymentResult{
			ID:     row.ID,
			Status: models.PaymentSucceeded,
		}, nil
	}

	// 1. Insert pending payment row first so a row always exists before
	//    money can move. provider_payment_id stays NULL until Stripe
	//    responds.
	pending, err := s.payments.Create(ctx, paymentsrepo.CreateInput{
		UserID:        user.ID,
		RentalID:      &rental.ID,
		Amount:        rental.TotalCost,
		Currency:      currencyUpper,
		PaymentMethod: models.PaymentMethodCard,
		Status:        models.PaymentPending,
	})
	if err != nil {
		return nil, err
	}

	// 2. Resolve customer + payment method. If either is missing, persist
	//    the failure on the existing row and return a non-error result.
	if user.StripeCustomerID == nil || *user.StripeCustomerID == "" {
		return s.failPaymentRow(ctx, pending.ID, "add_card_required")
	}
	custID := *user.StripeCustomerID

	pmID, err := s.stripe.GetCustomerDefaultPaymentMethod(ctx, custID)
	if err != nil {
		s.log.Warn("payments.ChargeRental: get default pm",
			zap.String("rental_id", rental.ID.String()),
			zap.Error(err),
		)
		return s.failPaymentRow(ctx, pending.ID, "payment_method_lookup_failed")
	}
	if pmID == "" {
		views, err := s.stripe.ListPaymentMethods(ctx, custID)
		if err != nil {
			s.log.Warn("payments.ChargeRental: list pms",
				zap.String("rental_id", rental.ID.String()),
				zap.Error(err),
			)
			return s.failPaymentRow(ctx, pending.ID, "payment_method_lookup_failed")
		}
		if len(views) == 0 {
			return s.failPaymentRow(ctx, pending.ID, "card_removed")
		}
		pmID = views[0].ID
	}

	amountMinor := rental.TotalCost.Mul(decimal.NewFromInt(100)).Round(0).IntPart()

	// 3. Call Stripe. If it returns nil + err (e.g. network), the row stays
	//    NULL provider_payment_id and is flipped to failed below.
	res, stripeErr := s.stripe.CreateAndConfirmPaymentIntent(ctx, stripeclient.ChargeParams{
		CustomerID:      custID,
		PaymentMethodID: pmID,
		AmountMinor:     amountMinor,
		Currency:        currency,
		IdempotencyKey:  rental.ID.String(),
		Metadata: map[string]string{
			"rental_id":  rental.ID.String(),
			"user_id":    user.ID.String(),
			"payment_id": pending.ID.String(),
		},
	})
	if stripeErr != nil || res == nil || res.IntentID == "" {
		reason := "payment_failed"
		if stripeErr != nil {
			reason = stripeErr.Error()
		}
		s.log.Error("payments.ChargeRental: stripe call failed",
			zap.String("rental_id", rental.ID.String()),
			zap.String("payment_id", pending.ID.String()),
			zap.Error(stripeErr),
		)
		return s.failPaymentRow(ctx, pending.ID, reason)
	}

	domainStatus, failure := mapIntentStatus(res.Status, res.FailureReason)
	var failurePtr *string
	if failure != "" {
		v := failure
		failurePtr = &v
	}

	// 4. Attach the intent id and the mapped status to the existing row.
	updated, err := s.payments.AttachIntent(ctx, paymentsrepo.AttachIntentInput{
		PaymentID:         pending.ID,
		ProviderPaymentID: res.IntentID,
		Status:            domainStatus,
		FailureReason:     failurePtr,
	})
	if err != nil {
		s.log.Error("payments.ChargeRental: attach intent",
			zap.String("rental_id", rental.ID.String()),
			zap.String("payment_id", pending.ID.String()),
			zap.String("intent_id", res.IntentID),
			zap.Error(err),
		)
		return nil, err
	}

	out := &rentalsvc.PaymentResult{
		ID:     updated.ID,
		Status: domainStatus,
	}
	if failurePtr != nil {
		v := *failurePtr
		out.FailureReason = &v
	}
	if res.ClientSecret != "" {
		v := res.ClientSecret
		out.ClientSecret = &v
	}
	return out, nil
}

// failPaymentRow flips the previously inserted pending row to failed and
// returns a PaymentResult. DB errors here propagate so the rental flow can
// log them; the rental itself has already been closed by the caller.
func (s *Service) failPaymentRow(ctx context.Context, paymentID uuid.UUID, reason string) (*rentalsvc.PaymentResult, error) {
	row, err := s.payments.MarkByIDFailed(ctx, paymentID, reason)
	if err != nil {
		return nil, err
	}
	r := reason
	return &rentalsvc.PaymentResult{
		ID:            row.ID,
		Status:        models.PaymentFailed,
		FailureReason: &r,
	}, nil
}

// HandleWebhookEvent verifies and processes a Stripe webhook delivery.
//
// Idempotency story: we record the event row up-front (ON CONFLICT DO
// NOTHING) for audit, but we DO NOT short-circuit on duplicates. The
// downstream UPDATEs are themselves idempotent — MarkPaymentSucceeded /
// MarkPaymentFailed only fire when the payment row is still 'pending', so
// a redelivered webhook either no-ops (already terminal) or reconciles a
// row that the API path missed (e.g. attach_intent crashed before
// committing). Skipping duplicates entirely would silently lose a redrive
// after a partial failure.
func (s *Service) HandleWebhookEvent(ctx context.Context, payload []byte, sigHeader string) error {
	event, err := s.stripe.VerifySignature(payload, sigHeader)
	if err != nil {
		return apperrors.Invalid("invalid webhook signature")
	}

	if _, err := s.payments.InsertWebhookEvent(ctx, event.ID, event.Type, payload); err != nil {
		return err
	}

	switch event.Type {
	case "payment_intent.succeeded":
		intentID, _ := extractIntentID(event.Data)
		if intentID == "" {
			return nil
		}
		if _, err := s.payments.MarkSucceeded(ctx, intentID); err != nil {
			if errors.Is(err, apperrors.ErrNotFound) || apperrors.Is(err, apperrors.KindNotFound) {
				// Either the row doesn't exist yet (race with API path
				// that hasn't called AttachIntent) or it's already
				// terminal. Both are safe to ignore here.
				s.log.Warn("payments.webhook: succeeded no-op",
					zap.String("intent_id", intentID),
					zap.String("event_id", event.ID),
				)
				return nil
			}
			return err
		}
	case "payment_intent.payment_failed":
		intentID, reason := extractIntentID(event.Data)
		if intentID == "" {
			return nil
		}
		if _, err := s.payments.MarkFailed(ctx, intentID, reason); err != nil {
			if errors.Is(err, apperrors.ErrNotFound) || apperrors.Is(err, apperrors.KindNotFound) {
				s.log.Warn("payments.webhook: failed no-op",
					zap.String("intent_id", intentID),
					zap.String("event_id", event.ID),
				)
				return nil
			}
			return err
		}
	default:
		s.log.Info("payments.webhook: ignored event", zap.String("type", event.Type), zap.String("event_id", event.ID))
	}
	return nil
}

// ListPaymentsByUser returns the user's payments, paginated.
func (s *Service) ListPaymentsByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Payment, int, error) {
	return s.payments.ListByUser(ctx, userID, page)
}

// mapIntentStatus translates a Stripe PaymentIntent status into a domain
// payment status.
func mapIntentStatus(stripeStatus, failureReason string) (string, string) {
	switch stripeStatus {
	case "succeeded":
		return models.PaymentSucceeded, ""
	case "requires_action", "requires_confirmation", "processing":
		return models.PaymentPending, ""
	case "canceled":
		return models.PaymentFailed, "canceled"
	}
	reason := failureReason
	if reason == "" {
		reason = stripeStatus
	}
	return models.PaymentFailed, reason
}

// extractIntentID parses the data.object portion of a payment_intent.* event
// and returns the intent id and (for failures) the last_payment_error message.
func extractIntentID(raw json.RawMessage) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}
	var doc struct {
		ID               string `json:"id"`
		LastPaymentError struct {
			Message string `json:"message"`
		} `json:"last_payment_error"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", ""
	}
	return doc.ID, doc.LastPaymentError.Message
}
