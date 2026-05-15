package payments_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	stripeclient "github.com/uniscoot/scooter-renting-backend/app/clients/stripe"
	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
	"github.com/uniscoot/scooter-renting-backend/app/internal/events"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	paymentsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/payments"
	paymentsrepo "github.com/uniscoot/scooter-renting-backend/app/internal/storage/postgres/repo/payments"
)

// ---- fakes

type fakeStripe struct {
	defaultPM   string
	defaultErr  error
	listPM      []stripeclient.PaymentMethodView
	listErr     error
	chargeErr   error
	chargeRes   *stripeclient.ChargeResult
	verifyErr   error
	verifyEvent *stripeclient.Event
}

func (f *fakeStripe) CreateCustomer(_ context.Context, _, _ string) (string, error) {
	return "cus_test", nil
}
func (f *fakeStripe) CreateSetupIntent(_ context.Context, _ string) (string, string, error) {
	return "cs_test", "si_test", nil
}
func (f *fakeStripe) ListPaymentMethods(_ context.Context, _ string) ([]stripeclient.PaymentMethodView, error) {
	return f.listPM, f.listErr
}
func (f *fakeStripe) DetachPaymentMethod(_ context.Context, _ string) error { return nil }
func (f *fakeStripe) GetCustomerDefaultPaymentMethod(_ context.Context, _ string) (string, error) {
	return f.defaultPM, f.defaultErr
}
func (f *fakeStripe) CreateAndConfirmPaymentIntent(_ context.Context, _ stripeclient.ChargeParams) (*stripeclient.ChargeResult, error) {
	return f.chargeRes, f.chargeErr
}
func (f *fakeStripe) VerifySignature(_ []byte, _ string) (*stripeclient.Event, error) {
	return f.verifyEvent, f.verifyErr
}

type fakePaymentsRepo struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]*models.Payment
	byProv   map[string]*models.Payment
	byIdem   map[string]*models.Payment
	created  int
	offline  int
	failedID uuid.UUID
}

func newFakePaymentsRepo() *fakePaymentsRepo {
	return &fakePaymentsRepo{
		byID:   map[uuid.UUID]*models.Payment{},
		byProv: map[string]*models.Payment{},
		byIdem: map[string]*models.Payment{},
	}
}

func (r *fakePaymentsRepo) Create(_ context.Context, in paymentsrepo.CreateInput) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created++
	p := &models.Payment{
		ID:            uuid.New(),
		UserID:        in.UserID,
		RentalID:      in.RentalID,
		Amount:        in.Amount,
		Currency:      in.Currency,
		PaymentMethod: in.PaymentMethod,
		Status:        in.Status,
	}
	r.byID[p.ID] = p
	return p, nil
}
func (r *fakePaymentsRepo) CreateOffline(_ context.Context, in paymentsrepo.CreateOfflineInput) (*models.Payment, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.IdempotencyKey != "" {
		if existing, ok := r.byIdem[in.IdempotencyKey]; ok {
			return existing, true, nil
		}
	}
	r.offline++
	rentalID := in.RentalID
	approverID := in.ApproverID
	now := time.Now().UTC()
	idemPtr := (*string)(nil)
	if in.IdempotencyKey != "" {
		k := in.IdempotencyKey
		idemPtr = &k
	}
	p := &models.Payment{
		ID:                uuid.New(),
		UserID:            in.UserID,
		RentalID:          &rentalID,
		Amount:            in.Amount,
		Currency:          in.Currency,
		PaymentMethod:     models.PaymentMethodOffline,
		Status:            models.PaymentSucceeded,
		OfflineApprovedBy: &approverID,
		OfflineApprovedAt: &now,
		IdempotencyKey:    idemPtr,
	}
	r.byID[p.ID] = p
	if in.IdempotencyKey != "" {
		r.byIdem[in.IdempotencyKey] = p
	}
	return p, false, nil
}
func (r *fakePaymentsRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byID[id]; ok {
		return p, nil
	}
	return nil, apperrors.NotFound("payment")
}
func (r *fakePaymentsRepo) GetByProviderID(_ context.Context, id string) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byProv[id]; ok {
		return p, nil
	}
	return nil, apperrors.NotFound("payment")
}
func (r *fakePaymentsRepo) GetByIdempotencyKey(_ context.Context, key string) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.byIdem[key]; ok {
		return p, nil
	}
	return nil, apperrors.NotFound("payment")
}
func (r *fakePaymentsRepo) AttachIntent(_ context.Context, in paymentsrepo.AttachIntentInput) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[in.PaymentID]
	if !ok {
		return nil, apperrors.NotFound("payment")
	}
	pid := in.ProviderPaymentID
	p.ProviderPaymentID = &pid
	if in.Status != "" {
		p.Status = in.Status
	}
	if in.FailureReason != nil {
		fr := *in.FailureReason
		p.FailureReason = &fr
	}
	r.byProv[in.ProviderPaymentID] = p
	return p, nil
}
func (r *fakePaymentsRepo) MarkByIDFailed(_ context.Context, id uuid.UUID, reason string) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, apperrors.NotFound("payment")
	}
	p.Status = models.PaymentFailed
	if reason != "" {
		fr := reason
		p.FailureReason = &fr
	}
	r.failedID = id
	return p, nil
}
func (r *fakePaymentsRepo) MarkSucceeded(_ context.Context, provID string) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byProv[provID]
	if !ok || p.Status != models.PaymentPending {
		return nil, apperrors.NotFound("payment")
	}
	p.Status = models.PaymentSucceeded
	return p, nil
}
func (r *fakePaymentsRepo) MarkFailed(_ context.Context, provID, reason string) (*models.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byProv[provID]
	if !ok || p.Status != models.PaymentPending {
		return nil, apperrors.NotFound("payment")
	}
	p.Status = models.PaymentFailed
	if reason != "" {
		fr := reason
		p.FailureReason = &fr
	}
	return p, nil
}
func (r *fakePaymentsRepo) ListByUser(_ context.Context, _ uuid.UUID, _ models.Page) ([]models.Payment, int, error) {
	return nil, 0, nil
}
func (r *fakePaymentsRepo) ListByUserSince(_ context.Context, _ uuid.UUID, _ time.Time, _ models.Page) ([]models.Payment, int, error) {
	return nil, 0, nil
}
func (r *fakePaymentsRepo) HasUnpaidRentals(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}
func (r *fakePaymentsRepo) InsertWebhookEvent(_ context.Context, _, _ string, _ []byte) (bool, error) {
	return true, nil
}

type fakeUsersRepo struct {
	user *models.User
}

func (f *fakeUsersRepo) GetByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	if f.user != nil && f.user.ID == id {
		return f.user, nil
	}
	if f.user != nil {
		return f.user, nil
	}
	return nil, apperrors.NotFound("user")
}
func (f *fakeUsersRepo) SetStripeCustomerID(_ context.Context, _ uuid.UUID, _ string) (*models.User, error) {
	return f.user, nil
}

type fakeRentalsRepo struct {
	r *models.Rental
}

func (f *fakeRentalsRepo) Get(_ context.Context, id uuid.UUID) (*models.Rental, error) {
	if f.r != nil && f.r.ID == id {
		return f.r, nil
	}
	return nil, apperrors.NotFound("rental")
}

type capturedEvent struct {
	Type string
	Env  events.Envelope
}

type fakePublisher struct {
	mu       sync.Mutex
	captured []capturedEvent
}

func (p *fakePublisher) Publish(_ context.Context, env events.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.captured = append(p.captured, capturedEvent{Type: env.Type, Env: env})
	return nil
}

// ---- helpers

func newSvc(stripe *fakeStripe, repo *fakePaymentsRepo, users *fakeUsersRepo) (*paymentsvc.Service, *fakePublisher) {
	cfg := &config.Config{
		Stripe: config.StripeConfig{
			SecretKey:     "sk_test",
			WebhookSecret: "whsec",
			Currency:      "USD",
		},
	}
	pub := &fakePublisher{}
	s := paymentsvc.New(stripe, repo, users, nil, cfg, pub, zap.NewNop())
	return s, pub
}

func mkRental(amount string) *models.Rental {
	d, _ := decimal.NewFromString(amount)
	return &models.Rental{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ScooterID: uuid.New(),
		TotalCost: d,
		Status:    models.RentalCompleted,
		EndTime:   ptrTime(time.Now()),
		StartTime: time.Now().Add(-10 * time.Minute),
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// ---- tests

func TestChargeRental_NoCard_MarksFailed(t *testing.T) {
	stripe := &fakeStripe{}
	repo := newFakePaymentsRepo()
	users := &fakeUsersRepo{}
	s, pub := newSvc(stripe, repo, users)

	rental := mkRental("10.00")
	user := &models.User{ID: rental.UserID, Email: "u@x.com"}

	res, err := s.ChargeRental(context.Background(), rental, user)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, models.PaymentFailed, res.Status)
	if assert.NotNil(t, res.FailureReason) {
		assert.Equal(t, "add_card_required", *res.FailureReason)
	}
	// Failure event should be emitted.
	require.NotEmpty(t, pub.captured)
	assert.Equal(t, events.TypePaymentFailed, pub.captured[len(pub.captured)-1].Type)
}

func TestChargeRental_StripeNetworkError_MarksFailed(t *testing.T) {
	stripe := &fakeStripe{
		defaultPM: "pm_123",
		chargeErr: errors.New("network unreachable"),
	}
	repo := newFakePaymentsRepo()
	users := &fakeUsersRepo{}
	s, pub := newSvc(stripe, repo, users)

	rental := mkRental("5.00")
	cust := "cus_x"
	user := &models.User{ID: rental.UserID, Email: "u@x.com", StripeCustomerID: &cust}

	res, err := s.ChargeRental(context.Background(), rental, user)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, models.PaymentFailed, res.Status)
	require.NotNil(t, res.FailureReason)
	assert.Contains(t, *res.FailureReason, "network")
	// Verify a PaymentFailed event was published.
	foundFailed := false
	for _, e := range pub.captured {
		if e.Type == events.TypePaymentFailed {
			foundFailed = true
		}
	}
	assert.True(t, foundFailed, "expected PaymentFailed event")
}

func TestChargeRental_StripeDecline_MarksFailed(t *testing.T) {
	// REQUIRED negative scenario: Stripe returns a result with status=failed.
	stripe := &fakeStripe{
		defaultPM: "pm_123",
		chargeRes: &stripeclient.ChargeResult{
			IntentID:      "pi_decline",
			Status:        "failed",
			FailureReason: "card_declined",
		},
	}
	repo := newFakePaymentsRepo()
	users := &fakeUsersRepo{}
	s, pub := newSvc(stripe, repo, users)

	rental := mkRental("9.99")
	cust := "cus_x"
	user := &models.User{ID: rental.UserID, Email: "u@x.com", StripeCustomerID: &cust}

	res, err := s.ChargeRental(context.Background(), rental, user)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, models.PaymentFailed, res.Status)
	require.NotNil(t, res.FailureReason)
	assert.Equal(t, "card_declined", *res.FailureReason)

	// Event should fire.
	foundFailed := false
	for _, e := range pub.captured {
		if e.Type == events.TypePaymentFailed {
			foundFailed = true
		}
	}
	assert.True(t, foundFailed)
}

func TestChargeRental_HappyPath_EmitsSucceeded(t *testing.T) {
	stripe := &fakeStripe{
		defaultPM: "pm_123",
		chargeRes: &stripeclient.ChargeResult{IntentID: "pi_ok", Status: "succeeded"},
	}
	repo := newFakePaymentsRepo()
	users := &fakeUsersRepo{}
	s, pub := newSvc(stripe, repo, users)

	rental := mkRental("3.00")
	cust := "cus_x"
	user := &models.User{ID: rental.UserID, Email: "u@x.com", StripeCustomerID: &cust}

	res, err := s.ChargeRental(context.Background(), rental, user)
	require.NoError(t, err)
	assert.Equal(t, models.PaymentSucceeded, res.Status)
	foundOK := false
	for _, e := range pub.captured {
		if e.Type == events.TypePaymentSucceeded {
			foundOK = true
		}
	}
	assert.True(t, foundOK)
}

func TestHandleWebhookEvent_BadSignature(t *testing.T) {
	stripe := &fakeStripe{verifyErr: errors.New("bad sig")}
	repo := newFakePaymentsRepo()
	users := &fakeUsersRepo{}
	s, _ := newSvc(stripe, repo, users)
	err := s.HandleWebhookEvent(context.Background(), []byte("{}"), "")
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindInvalid))
}

func TestHandleWebhookEvent_SucceededAndFailed(t *testing.T) {
	repo := newFakePaymentsRepo()
	// Seed a pending row keyed by provider_payment_id.
	intentID := "pi_xyz"
	p := &models.Payment{
		ID:                uuid.New(),
		UserID:            uuid.New(),
		Status:            models.PaymentPending,
		ProviderPaymentID: &intentID,
	}
	repo.byID[p.ID] = p
	repo.byProv[intentID] = p
	users := &fakeUsersRepo{user: &models.User{ID: p.UserID, Email: "u@x.com"}}

	stripe := &fakeStripe{
		verifyEvent: &stripeclient.Event{ID: "evt_1", Type: "payment_intent.succeeded", Data: []byte(`{"id":"pi_xyz"}`)},
	}
	s, pub := newSvc(stripe, repo, users)
	require.NoError(t, s.HandleWebhookEvent(context.Background(), []byte("{}"), ""))
	assert.Equal(t, models.PaymentSucceeded, p.Status)

	// Now simulate a failed event on a brand new pending row.
	intentID2 := "pi_fail"
	p2 := &models.Payment{
		ID:                uuid.New(),
		UserID:            users.user.ID,
		Status:            models.PaymentPending,
		ProviderPaymentID: &intentID2,
	}
	repo.byID[p2.ID] = p2
	repo.byProv[intentID2] = p2
	stripe.verifyEvent = &stripeclient.Event{ID: "evt_2", Type: "payment_intent.payment_failed", Data: []byte(`{"id":"pi_fail","last_payment_error":{"message":"card_declined"}}`)}
	require.NoError(t, s.HandleWebhookEvent(context.Background(), []byte("{}"), ""))
	assert.Equal(t, models.PaymentFailed, p2.Status)

	// Two events captured: succeeded + failed.
	gotOK, gotFail := false, false
	for _, e := range pub.captured {
		if e.Type == events.TypePaymentSucceeded {
			gotOK = true
		}
		if e.Type == events.TypePaymentFailed {
			gotFail = true
		}
	}
	assert.True(t, gotOK)
	assert.True(t, gotFail)
}

func TestApproveOffline_HappyPath(t *testing.T) {
	repo := newFakePaymentsRepo()
	rental := &models.Rental{ID: uuid.New(), UserID: uuid.New(), Status: models.RentalCompleted}
	users := &fakeUsersRepo{user: &models.User{ID: rental.UserID, Email: "u@x.com"}}
	stripe := &fakeStripe{}
	s, pub := newSvc(stripe, repo, users)
	s.SetRentalsRepo(&fakeRentalsRepo{r: rental})

	row, err := s.ApproveOffline(context.Background(), paymentsvc.OfflineApprovalInput{
		RentalID:       rental.ID,
		ApproverID:     uuid.New(),
		Amount:         decimal.NewFromFloat(7.5),
		Currency:       "USD",
		Note:           "paid at kiosk",
		IdempotencyKey: "idem-1",
	})
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, models.PaymentMethodOffline, row.PaymentMethod)
	assert.Equal(t, models.PaymentSucceeded, row.Status)

	foundOffline := false
	for _, e := range pub.captured {
		if e.Type == events.TypeOfflinePaymentApproved {
			foundOffline = true
		}
	}
	assert.True(t, foundOffline)
}

func TestApproveOffline_IdempotentReplay(t *testing.T) {
	repo := newFakePaymentsRepo()
	rental := &models.Rental{ID: uuid.New(), UserID: uuid.New(), Status: models.RentalCompleted}
	users := &fakeUsersRepo{user: &models.User{ID: rental.UserID}}
	stripe := &fakeStripe{}
	s, _ := newSvc(stripe, repo, users)
	s.SetRentalsRepo(&fakeRentalsRepo{r: rental})

	in := paymentsvc.OfflineApprovalInput{
		RentalID:       rental.ID,
		ApproverID:     uuid.New(),
		Amount:         decimal.NewFromInt(5),
		Currency:       "USD",
		IdempotencyKey: "rep-1",
	}
	first, err := s.ApproveOffline(context.Background(), in)
	require.NoError(t, err)
	second, err := s.ApproveOffline(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "replay must return same row")
}

func TestApproveOffline_RejectsActiveRental(t *testing.T) {
	repo := newFakePaymentsRepo()
	rental := &models.Rental{ID: uuid.New(), UserID: uuid.New(), Status: models.RentalActive}
	users := &fakeUsersRepo{user: &models.User{ID: rental.UserID}}
	stripe := &fakeStripe{}
	s, _ := newSvc(stripe, repo, users)
	s.SetRentalsRepo(&fakeRentalsRepo{r: rental})

	_, err := s.ApproveOffline(context.Background(), paymentsvc.OfflineApprovalInput{
		RentalID:       rental.ID,
		ApproverID:     uuid.New(),
		Amount:         decimal.NewFromInt(1),
		Currency:       "USD",
		IdempotencyKey: "x",
	})
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindConflict))
}

func TestEnsureCustomer_ReusesExistingID(t *testing.T) {
	cust := "cus_abc"
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New(), StripeCustomerID: &cust, Email: "a@b.com"}}
	stripe := &fakeStripe{}
	repo := newFakePaymentsRepo()
	s, _ := newSvc(stripe, repo, users)
	cs, siID, err := s.CreateSetupIntent(context.Background(), users.user.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, cs)
	assert.NotEmpty(t, siID)
}

func TestListPaymentMethods_HappyPath(t *testing.T) {
	cust := "cus_abc"
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New(), StripeCustomerID: &cust}}
	stripe := &fakeStripe{listPM: []stripeclient.PaymentMethodView{{ID: "pm_1"}}}
	s, _ := newSvc(stripe, newFakePaymentsRepo(), users)
	out, err := s.ListPaymentMethods(context.Background(), users.user.ID)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestDetachPaymentMethod_OwnerCheck(t *testing.T) {
	cust := "cus_abc"
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New(), StripeCustomerID: &cust}}
	stripe := &fakeStripe{listPM: []stripeclient.PaymentMethodView{{ID: "pm_owned"}}}
	s, _ := newSvc(stripe, newFakePaymentsRepo(), users)
	require.NoError(t, s.DetachPaymentMethod(context.Background(), users.user.ID, "pm_owned"))
	err := s.DetachPaymentMethod(context.Background(), users.user.ID, "pm_other")
	require.Error(t, err)
}

func TestUserHasPaymentMethod_ReturnsFalseNoCustomer(t *testing.T) {
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New()}}
	stripe := &fakeStripe{}
	s, _ := newSvc(stripe, newFakePaymentsRepo(), users)
	has, err := s.UserHasPaymentMethod(context.Background(), users.user.ID)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestUserHasUnpaidRentals_PassesThrough(t *testing.T) {
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New()}}
	stripe := &fakeStripe{}
	s, _ := newSvc(stripe, newFakePaymentsRepo(), users)
	got, err := s.UserHasUnpaidRentals(context.Background(), users.user.ID)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestListPaymentsByUser_PassesThrough(t *testing.T) {
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New()}}
	s, _ := newSvc(&fakeStripe{}, newFakePaymentsRepo(), users)
	_, _, err := s.ListPaymentsByUser(context.Background(), users.user.ID, models.Page{})
	require.NoError(t, err)
}

func TestListPaymentsByUserSince_PassesThrough(t *testing.T) {
	users := &fakeUsersRepo{user: &models.User{ID: uuid.New()}}
	s, _ := newSvc(&fakeStripe{}, newFakePaymentsRepo(), users)
	_, _, err := s.ListPaymentsByUserSince(context.Background(), users.user.ID, time.Now(), models.Page{})
	require.NoError(t, err)
}

func TestChargeRental_ZeroAmountSucceeds(t *testing.T) {
	users := &fakeUsersRepo{}
	s, pub := newSvc(&fakeStripe{}, newFakePaymentsRepo(), users)
	r := mkRental("0.00")
	u := &models.User{ID: r.UserID, Email: "u@x.com"}
	res, err := s.ChargeRental(context.Background(), r, u)
	require.NoError(t, err)
	assert.Equal(t, models.PaymentSucceeded, res.Status)
	require.NotEmpty(t, pub.captured)
}

func TestApproveOffline_RejectsNonexistentRental(t *testing.T) {
	repo := newFakePaymentsRepo()
	users := &fakeUsersRepo{}
	stripe := &fakeStripe{}
	s, _ := newSvc(stripe, repo, users)
	s.SetRentalsRepo(&fakeRentalsRepo{})

	_, err := s.ApproveOffline(context.Background(), paymentsvc.OfflineApprovalInput{
		RentalID:       uuid.New(),
		ApproverID:     uuid.New(),
		Amount:         decimal.NewFromInt(1),
		IdempotencyKey: "x",
	})
	require.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.KindNotFound))
}
