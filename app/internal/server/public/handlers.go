package public

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
	maintsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/maintenance"
	paymentsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/payments"
	pricesvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/pricemodels"
	rentalsvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/rentals"
	scootersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/scooters"
	usersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/users"
	zonesvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/zones"
)

// ---- service interfaces consumed by the HTTP handlers.

// AuthService is the subset of the auth service used by HTTP handlers.
type AuthService interface {
	Register(ctx context.Context, email, firstName, lastName, password, phoneNumber string) (*models.User, string, error)
	Login(ctx context.Context, email, password string) (*models.User, string, error)
}

// OAuthService is the subset of the oauth service used by HTTP handlers.
type OAuthService interface {
	VerifyAndLogin(ctx context.Context, idToken string) (*models.User, string, error)
}

// UsersService is the subset of the users service used by HTTP handlers.
type UsersService interface {
	Get(ctx context.Context, id uuid.UUID) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, patch usersvc.UpdatePatch) (*models.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// ScootersService is the subset of the scooters service used by HTTP handlers.
type ScootersService interface {
	Create(ctx context.Context, qrCode, model string, zoneID *uuid.UUID, lat, lng *decimal.Decimal, batteryLevel int) (*models.Scooter, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Scooter, error)
	List(ctx context.Context, filter scootersvc.ListFilter) ([]models.Scooter, int, error)
	Nearby(ctx context.Context, lat, lng float64, radiusMeters, limit int) ([]models.Scooter, error)
	Update(ctx context.Context, id uuid.UUID, patch scootersvc.UpdatePatch) (*models.Scooter, error)
	Retire(ctx context.Context, id uuid.UUID) error
}

// ZonesService is the subset of the zones service used by HTTP handlers.
type ZonesService interface {
	Create(ctx context.Context, in zonesvc.CreateInput) (*models.Zone, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Zone, error)
	List(ctx context.Context, page models.Page) ([]models.Zone, int, error)
	Update(ctx context.Context, id uuid.UUID, patch zonesvc.UpdatePatch) (*models.Zone, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// PriceModelsService is the subset of the price-models service used by handlers.
type PriceModelsService interface {
	Create(ctx context.Context, in pricesvc.CreateInput) (*models.PriceModel, error)
	Get(ctx context.Context, id uuid.UUID) (*models.PriceModel, error)
	List(ctx context.Context, page models.Page) ([]models.PriceModel, int, error)
	Update(ctx context.Context, id uuid.UUID, patch pricesvc.UpdatePatch) (*models.PriceModel, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// RentalsService is the subset of the rentals service used by HTTP handlers.
type RentalsService interface {
	Start(ctx context.Context, userID, scooterID, priceModelID uuid.UUID, startLat, startLon *decimal.Decimal) (*models.Rental, error)
	End(ctx context.Context, rentalID, actorUserID uuid.UUID, endLat, endLon *decimal.Decimal) (*models.Rental, *rentalsvc.PaymentResult, error)
	AdminCancel(ctx context.Context, rentalID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Rental, int, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Rental, error)
}

// MaintenanceService is the subset of the maintenance service used by handlers.
type MaintenanceService interface {
	Create(ctx context.Context, in maintsvc.CreateInput) (*models.MaintenanceLog, error)
	Close(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error)
	ListByScooter(ctx context.Context, scooterID uuid.UUID, page models.Page) ([]models.MaintenanceLog, int, error)
	Get(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error)
}

// PaymentsService is the subset of the payments service used by handlers.
type PaymentsService interface {
	CreateSetupIntent(ctx context.Context, userID uuid.UUID) (clientSecret, setupIntentID string, err error)
	ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]paymentsvc.PaymentMethodView, error)
	DetachPaymentMethod(ctx context.Context, userID uuid.UUID, paymentMethodID string) error
	ListPaymentsByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Payment, int, error)
	HandleWebhookEvent(ctx context.Context, payload []byte, sigHeader string) error
}

// Handler is the fiber-based HTTP handler aggregate for all routes.
type Handler struct {
	version     string
	auth        AuthService
	oauth       OAuthService
	users       UsersService
	scooters    ScootersService
	zones       ZonesService
	priceModels PriceModelsService
	rentals     RentalsService
	maintenance MaintenanceService
	payments    PaymentsService
	log         *zap.Logger
}

// Deps groups the handler's construction dependencies.
type Deps struct {
	Version     string
	Auth        AuthService
	OAuth       OAuthService
	Users       UsersService
	Scooters    ScootersService
	Zones       ZonesService
	PriceModels PriceModelsService
	Rentals     RentalsService
	Maintenance MaintenanceService
	Payments    PaymentsService
	Log         *zap.Logger
}

// NewHandler constructs a new Handler from its dependencies.
func NewHandler(d Deps) *Handler {
	log := d.Log
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{
		version:     d.Version,
		auth:        d.Auth,
		oauth:       d.OAuth,
		users:       d.Users,
		scooters:    d.Scooters,
		zones:       d.Zones,
		priceModels: d.PriceModels,
		rentals:     d.Rentals,
		maintenance: d.Maintenance,
		payments:    d.Payments,
		log:         log,
	}
}

// listEnvelope is the body of list responses.
type listEnvelope struct {
	Items any `json:"items"`
	Total int `json:"total"`
}

// assertOwnerOrAdmin returns nil when the request identity owns targetID or
// has the admin role. Otherwise it returns an APIError suitable for WriteError.
func assertOwnerOrAdmin(c fiber.Ctx, targetID uuid.UUID) *resp.APIError {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return resp.ErrUnauthorized("")
	}
	if ident.Role == models.RoleAdmin {
		return nil
	}
	if ident.UserID == targetID {
		return nil
	}
	return resp.ErrForbidden("")
}

// ---- health / version

// Health returns a minimal ok JSON envelope for liveness checks.
func (h *Handler) Health(c fiber.Ctx) error {
	return WriteJSON(c, fiber.StatusOK, map[string]string{"status": "ok"})
}

// Version returns the configured version string.
func (h *Handler) Version(c fiber.Ctx) error {
	v := h.version
	if v == "" {
		v = "dev"
	}
	return WriteJSON(c, fiber.StatusOK, map[string]string{"version": v})
}

// ---- auth

type registerReq struct {
	Email       string `json:"email" validate:"required,email,max=255"`
	FirstName   string `json:"first_name" validate:"required,min=1,max=100"`
	LastName    string `json:"last_name" validate:"omitempty,max=100"`
	Password    string `json:"password" validate:"required,min=8,max=128"`
	PhoneNumber string `json:"phone_number" validate:"omitempty,max=32"`
}

type loginReq struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=1,max=128"`
}

type oauthGoogleReq struct {
	IDToken string `json:"id_token" validate:"required,min=10"`
}

type authResp struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// Register creates a new end-user account and returns a token.
func (h *Handler) Register(c fiber.Ctx) error {
	var body registerReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	user, token, err := h.auth.Register(c.Context(), body.Email, body.FirstName, body.LastName, body.Password, body.PhoneNumber)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteCreated(c, authResp{Token: token, User: user})
}

// Login authenticates an existing user and returns a token.
func (h *Handler) Login(c fiber.Ctx) error {
	var body loginReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	user, token, err := h.auth.Login(c.Context(), body.Email, body.Password)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, authResp{Token: token, User: user})
}

// OAuthGoogle verifies a Google id_token and returns a session token.
func (h *Handler) OAuthGoogle(c fiber.Ctx) error {
	var body oauthGoogleReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	user, token, err := h.oauth.VerifyAndLogin(c.Context(), body.IDToken)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, authResp{Token: token, User: user})
}

// ---- scooters (public)

// ListScooters returns the paginated list of scooters with optional filters.
func (h *Handler) ListScooters(c fiber.Ctx) error {
	var statusPtr *string
	if s := QueryString(c, "status", ""); s != "" {
		statusPtr = &s
	}
	var zonePtr *uuid.UUID
	if z := QueryString(c, "zone_id", ""); z != "" {
		id, err := uuid.Parse(z)
		if err != nil {
			return WriteError(c, resp.ErrValidation("invalid zone_id"))
		}
		zonePtr = &id
	}
	items, total, err := h.scooters.List(c.Context(), scootersvc.ListFilter{
		Status: statusPtr,
		ZoneID: zonePtr,
		Page:   PageFromCtx(c),
	})
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: total})
}

// NearbyScooters returns scooters close to the given lat/lng coordinates.
func (h *Handler) NearbyScooters(c fiber.Ctx) error {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	if latStr == "" || lngStr == "" {
		return WriteError(c, resp.ErrValidation("lat and lng are required"))
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || lat < -90 || lat > 90 {
		return WriteError(c, resp.ErrValidation("lat must be a valid number in [-90,90]"))
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil || lng < -180 || lng > 180 {
		return WriteError(c, resp.ErrValidation("lng must be a valid number in [-180,180]"))
	}
	radius := QueryInt(c, "radius", 0)
	if radius == 0 {
		radius = QueryInt(c, "radius_m", 0)
	}
	limit := QueryInt(c, "limit", 20)
	items, err := h.scooters.Nearby(c.Context(), lat, lng, radius, limit)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: len(items)})
}

// GetScooter returns a single scooter by id.
func (h *Handler) GetScooter(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	s, err := h.scooters.Get(c.Context(), id)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, s)
}

// ---- zones (public)

// ListZones returns the paginated list of zones.
func (h *Handler) ListZones(c fiber.Ctx) error {
	items, total, err := h.zones.List(c.Context(), PageFromCtx(c))
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: total})
}

// ---- price models (public)

// ListPriceModels returns the paginated list of price models.
func (h *Handler) ListPriceModels(c fiber.Ctx) error {
	items, total, err := h.priceModels.List(c.Context(), PageFromCtx(c))
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: total})
}

// ---- users

type updateUserReq struct {
	FirstName   *string `json:"first_name" validate:"omitempty,min=1,max=100"`
	LastName    *string `json:"last_name" validate:"omitempty,max=100"`
	PhoneNumber *string `json:"phone_number" validate:"omitempty,max=32"`
}

// GetUser returns a user profile (owner-or-admin only).
func (h *Handler) GetUser(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	u, err := h.users.Get(c.Context(), id)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, u)
}

// UpdateUser mutates user fields; owner-or-admin only.
func (h *Handler) UpdateUser(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	var body updateUserReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	u, err := h.users.Update(c.Context(), id, usersvc.UpdatePatch{
		FirstName:   body.FirstName,
		LastName:    body.LastName,
		PhoneNumber: body.PhoneNumber,
	})
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, u)
}

// DeleteUser soft-deletes a user; owner-or-admin only.
func (h *Handler) DeleteUser(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	if err := h.users.SoftDelete(c.Context(), id); err != nil {
		return WriteDomain(c, err)
	}
	return WriteNoContent(c)
}

// ListUserRentals returns the paginated list of a user's rentals.
func (h *Handler) ListUserRentals(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	items, total, err := h.rentals.ListByUser(c.Context(), id, PageFromCtx(c))
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: total})
}

// ListUserPayments returns the paginated list of a user's payments.
func (h *Handler) ListUserPayments(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	items, total, err := h.payments.ListPaymentsByUser(c.Context(), id, PageFromCtx(c))
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: total})
}

// ---- payments

type setupIntentResp struct {
	ClientSecret  string `json:"client_secret"`
	SetupIntentID string `json:"setup_intent_id"`
}

type paymentMethodsResp struct {
	Methods []paymentsvc.PaymentMethodView `json:"methods"`
}

// CreateSetupIntent issues a Stripe SetupIntent for the authenticated user.
func (h *Handler) CreateSetupIntent(c fiber.Ctx) error {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return WriteError(c, resp.ErrUnauthorized(""))
	}
	cs, siID, err := h.payments.CreateSetupIntent(c.Context(), ident.UserID)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, setupIntentResp{ClientSecret: cs, SetupIntentID: siID})
}

// ListPaymentMethods returns the authenticated user's saved cards.
func (h *Handler) ListPaymentMethods(c fiber.Ctx) error {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return WriteError(c, resp.ErrUnauthorized(""))
	}
	methods, err := h.payments.ListPaymentMethods(c.Context(), ident.UserID)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, paymentMethodsResp{Methods: methods})
}

// DetachPaymentMethod removes a card from the authenticated user's customer.
func (h *Handler) DetachPaymentMethod(c fiber.Ctx) error {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return WriteError(c, resp.ErrUnauthorized(""))
	}
	pmID := c.Params("pm_id")
	if pmID == "" {
		return WriteError(c, resp.ErrValidation("missing pm_id"))
	}
	if err := h.payments.DetachPaymentMethod(c.Context(), ident.UserID, pmID); err != nil {
		return WriteDomain(c, err)
	}
	return WriteNoContent(c)
}

// StripeWebhook receives raw Stripe events. Always returns 200 unless
// signature verification fails (which yields 400).
func (h *Handler) StripeWebhook(c fiber.Ctx) error {
	sig := c.Get("Stripe-Signature")
	body := c.Body()
	if err := h.payments.HandleWebhookEvent(c.Context(), body, sig); err != nil {
		if apperrors.Is(err, apperrors.KindInvalid) {
			return WriteError(c, resp.ErrValidation(err.Error()))
		}
		h.log.Error("stripe webhook handler error", zap.Error(err))
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, map[string]string{"status": "ok"})
}

// ---- rentals (authenticated)

type startRentalReq struct {
	ScooterID    uuid.UUID        `json:"scooter_id" validate:"required"`
	PriceModelID uuid.UUID        `json:"price_model_id" validate:"required"`
	StartLat     *decimal.Decimal `json:"start_lat"`
	StartLon     *decimal.Decimal `json:"start_lon"`
}

type endRentalReq struct {
	EndLat *decimal.Decimal `json:"end_lat"`
	EndLon *decimal.Decimal `json:"end_lon"`
}

type endRentalPaymentResp struct {
	ID            *uuid.UUID `json:"id,omitempty"`
	Status        string     `json:"status"`
	ClientSecret  *string    `json:"client_secret,omitempty"`
	FailureReason *string    `json:"failure_reason,omitempty"`
}

type endRentalResp struct {
	Rental  *models.Rental         `json:"rental"`
	Payment *endRentalPaymentResp  `json:"payment,omitempty"`
}

// StartRental begins a new rental for the authenticated user.
func (h *Handler) StartRental(c fiber.Ctx) error {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return WriteError(c, resp.ErrUnauthorized(""))
	}
	var body startRentalReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	rental, err := h.rentals.Start(c.Context(), ident.UserID, body.ScooterID, body.PriceModelID, body.StartLat, body.StartLon)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteCreated(c, rental)
}

// GetRental returns a rental; must be owner or admin.
func (h *Handler) GetRental(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	rental, err := h.rentals.Get(c.Context(), id)
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, rental.UserID); apiErr != nil {
		return WriteError(c, apiErr)
	}
	return WriteJSON(c, fiber.StatusOK, rental)
}

// EndRental ends an active rental for the authenticated user and reports the
// post-rental charge result.
func (h *Handler) EndRental(c fiber.Ctx) error {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return WriteError(c, resp.ErrUnauthorized(""))
	}
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	var body endRentalReq
	if len(c.Body()) > 0 {
		if err := DecodeBody(c, &body); err != nil {
			return WriteDomain(c, err)
		}
	}
	rental, payment, err := h.rentals.End(c.Context(), id, ident.UserID, body.EndLat, body.EndLon)
	if err != nil {
		return WriteDomain(c, err)
	}
	out := endRentalResp{Rental: rental}
	if payment != nil {
		p := &endRentalPaymentResp{
			Status:        payment.Status,
			ClientSecret:  payment.ClientSecret,
			FailureReason: payment.FailureReason,
		}
		if payment.ID != uuid.Nil {
			id := payment.ID
			p.ID = &id
		}
		out.Payment = p
	}
	return WriteJSON(c, fiber.StatusOK, out)
}

// ---- admin: scooters

type createScooterReq struct {
	QRCode       string           `json:"qr_code" validate:"required,min=1,max=64"`
	Model        string           `json:"model" validate:"omitempty,max=100"`
	ZoneID       *uuid.UUID       `json:"zone_id"`
	Lat          *decimal.Decimal `json:"lat"`
	Lng          *decimal.Decimal `json:"lng"`
	BatteryLevel int              `json:"battery_level" validate:"gte=0,lte=100"`
}

// CreateScooter creates a new scooter; admin only.
func (h *Handler) CreateScooter(c fiber.Ctx) error {
	var body createScooterReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	s, err := h.scooters.Create(c.Context(), body.QRCode, body.Model, body.ZoneID, body.Lat, body.Lng, body.BatteryLevel)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteCreated(c, s)
}

// UpdateScooter patches scooter fields; admin only.
func (h *Handler) UpdateScooter(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	raw := map[string]any{}
	if err := DecodeRawBody(c, &raw); err != nil {
		return WriteDomain(c, err)
	}
	patch := scootersvc.UpdatePatch{}
	if v, ok := raw["model"]; ok {
		if s, ok := v.(string); ok {
			patch.Model = &s
		}
	}
	if v, ok := raw["status"]; ok {
		if s, ok := v.(string); ok {
			patch.Status = &s
		}
	}
	if v, ok := raw["zone_id"]; ok {
		patch.ZoneIDSet = true
		if s, ok := v.(string); ok && s != "" {
			z, err := uuid.Parse(s)
			if err != nil {
				return WriteError(c, resp.ErrValidation("invalid zone_id"))
			}
			patch.ZoneID = &z
		}
	}
	if v, ok := raw["battery_level"]; ok {
		if f, ok := v.(float64); ok {
			n := int(f)
			patch.BatteryLevel = &n
		}
	}
	if v, ok := raw["lat"]; ok {
		patch.LatSet = true
		if d, ok := decimalFromAny(v); ok {
			patch.Lat = &d
		}
	}
	if v, ok := raw["lng"]; ok {
		patch.LngSet = true
		if d, ok := decimalFromAny(v); ok {
			patch.Lng = &d
		}
	}
	s, err := h.scooters.Update(c.Context(), id, patch)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, s)
}

// RetireScooter marks a scooter as retired; admin only.
func (h *Handler) RetireScooter(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if err := h.scooters.Retire(c.Context(), id); err != nil {
		return WriteDomain(c, err)
	}
	return WriteNoContent(c)
}

// ---- admin: zones

type createZoneReq struct {
	Name         string           `json:"name" validate:"required,min=1,max=100"`
	CenterLat    decimal.Decimal  `json:"center_lat" validate:"required"`
	CenterLon    decimal.Decimal  `json:"center_lon" validate:"required"`
	RadiusMeters int              `json:"radius_meters" validate:"required,gt=0"`
	ZoneType     string           `json:"zone_type" validate:"omitempty,max=32"`
}

// CreateZone creates a new zone; admin only.
func (h *Handler) CreateZone(c fiber.Ctx) error {
	var body createZoneReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	z, err := h.zones.Create(c.Context(), zonesvc.CreateInput{
		Name:         body.Name,
		CenterLat:    body.CenterLat,
		CenterLon:    body.CenterLon,
		RadiusMeters: body.RadiusMeters,
		ZoneType:     body.ZoneType,
	})
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteCreated(c, z)
}

// UpdateZone updates a zone; admin only.
func (h *Handler) UpdateZone(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	raw := map[string]any{}
	if err := DecodeRawBody(c, &raw); err != nil {
		return WriteDomain(c, err)
	}
	patch := zonesvc.UpdatePatch{}
	if v, ok := raw["name"]; ok {
		if s, ok := v.(string); ok {
			patch.Name = &s
		}
	}
	if v, ok := raw["center_lat"]; ok {
		if d, ok := decimalFromAny(v); ok {
			patch.CenterLat = &d
		}
	}
	if v, ok := raw["center_lon"]; ok {
		if d, ok := decimalFromAny(v); ok {
			patch.CenterLon = &d
		}
	}
	if v, ok := raw["radius_meters"]; ok {
		if f, ok := v.(float64); ok {
			n := int(f)
			patch.RadiusMeters = &n
		}
	}
	if v, ok := raw["zone_type"]; ok {
		if s, ok := v.(string); ok {
			patch.ZoneType = &s
		}
	}
	z, err := h.zones.Update(c.Context(), id, patch)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, z)
}

// DeleteZone deletes a zone; admin only.
func (h *Handler) DeleteZone(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if err := h.zones.Delete(c.Context(), id); err != nil {
		return WriteDomain(c, err)
	}
	return WriteNoContent(c)
}

// ---- admin: price models

type createPriceModelReq struct {
	Name           string           `json:"name" validate:"required,min=1,max=100"`
	PricePerMinute decimal.Decimal  `json:"price_per_minute" validate:"required"`
	UnlockFee      decimal.Decimal  `json:"unlock_fee"`
	DailyCap       *decimal.Decimal `json:"daily_cap"`
	Currency       string           `json:"currency" validate:"omitempty,len=3"`
}

// CreatePriceModel creates a new price model; admin only.
func (h *Handler) CreatePriceModel(c fiber.Ctx) error {
	var body createPriceModelReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	pm, err := h.priceModels.Create(c.Context(), pricesvc.CreateInput{
		Name:           body.Name,
		PricePerMinute: body.PricePerMinute,
		UnlockFee:      body.UnlockFee,
		DailyCap:       body.DailyCap,
		Currency:       body.Currency,
	})
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteCreated(c, pm)
}

// UpdatePriceModel patches a price model; admin only.
func (h *Handler) UpdatePriceModel(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	raw := map[string]any{}
	if err := DecodeRawBody(c, &raw); err != nil {
		return WriteDomain(c, err)
	}
	patch := pricesvc.UpdatePatch{}
	if v, ok := raw["name"]; ok {
		if s, ok := v.(string); ok {
			patch.Name = &s
		}
	}
	if v, ok := raw["price_per_minute"]; ok {
		if d, ok := decimalFromAny(v); ok {
			patch.PricePerMinute = &d
		}
	}
	if v, ok := raw["unlock_fee"]; ok {
		if d, ok := decimalFromAny(v); ok {
			patch.UnlockFee = &d
		}
	}
	if v, ok := raw["daily_cap"]; ok {
		patch.DailyCapSet = true
		if v != nil {
			if d, ok := decimalFromAny(v); ok {
				patch.DailyCap = &d
			}
		}
	}
	if v, ok := raw["currency"]; ok {
		if s, ok := v.(string); ok {
			patch.Currency = &s
		}
	}
	pm, err := h.priceModels.Update(c.Context(), id, patch)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, pm)
}

// DeletePriceModel deletes a price model; admin only.
func (h *Handler) DeletePriceModel(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if err := h.priceModels.Delete(c.Context(), id); err != nil {
		return WriteDomain(c, err)
	}
	return WriteNoContent(c)
}

// ---- admin: rentals

// CancelRental administratively cancels a rental.
func (h *Handler) CancelRental(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if err := h.rentals.AdminCancel(c.Context(), id); err != nil {
		return WriteDomain(c, err)
	}
	return WriteNoContent(c)
}

// ---- admin: maintenance

type openMaintReq struct {
	TechnicianName   string           `json:"technician_name" validate:"omitempty,max=100"`
	IssueDescription string           `json:"issue_description" validate:"required,min=1"`
	RepairCost       *decimal.Decimal `json:"repair_cost"`
}

// OpenMaintenance starts a maintenance record for the given scooter.
func (h *Handler) OpenMaintenance(c fiber.Ctx) error {
	scooterID, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	var body openMaintReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	m, err := h.maintenance.Create(c.Context(), maintsvc.CreateInput{
		ScooterID:        scooterID,
		TechnicianName:   body.TechnicianName,
		IssueDescription: body.IssueDescription,
		RepairCost:       body.RepairCost,
	})
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteCreated(c, m)
}

// ListMaintenance returns maintenance records for a scooter.
func (h *Handler) ListMaintenance(c fiber.Ctx) error {
	scooterID, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	items, total, err := h.maintenance.ListByScooter(c.Context(), scooterID, PageFromCtx(c))
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, listEnvelope{Items: items, Total: total})
}

// CloseMaintenance closes a maintenance record.
func (h *Handler) CloseMaintenance(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	m, err := h.maintenance.Close(c.Context(), id)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, m)
}

// decimalFromAny converts loosely-typed JSON values into a decimal.Decimal.
func decimalFromAny(v any) (decimal.Decimal, bool) {
	switch t := v.(type) {
	case float64:
		return decimal.NewFromFloat(t), true
	case string:
		d, err := decimal.NewFromString(t)
		if err != nil {
			return decimal.Zero, false
		}
		return d, true
	}
	return decimal.Zero, false
}
