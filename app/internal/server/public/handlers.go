package public

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/resp"
	pricesvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/pricemodels"
	scootersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/scooters"
	usersvc "github.com/uniscoot/scooter-renting-backend/app/internal/services/users"
)

// ---- service interfaces consumed by the HTTP handlers.

// AuthService is the subset of the auth service used by HTTP handlers.
type AuthService interface {
	Register(ctx context.Context, email, name, password, phone string) (*models.User, string, error)
	Login(ctx context.Context, email, password string) (*models.User, string, error)
}

// UsersService is the subset of the users service used by HTTP handlers.
type UsersService interface {
	Get(ctx context.Context, id uuid.UUID) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, patch usersvc.UpdatePatch) (*models.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	GetWallet(ctx context.Context, id uuid.UUID) (decimal.Decimal, error)
	TopUp(ctx context.Context, id uuid.UUID, amount decimal.Decimal) (decimal.Decimal, error)
}

// ScootersService is the subset of the scooters service used by HTTP handlers.
type ScootersService interface {
	Create(ctx context.Context, code, model string, zoneID *uuid.UUID, lat, lng *decimal.Decimal, batteryPct int) (*models.Scooter, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Scooter, error)
	List(ctx context.Context, filter scootersvc.ListFilter) ([]models.Scooter, int, error)
	Nearby(ctx context.Context, lat, lng float64, radiusMeters, limit int) ([]models.Scooter, error)
	Update(ctx context.Context, id uuid.UUID, patch scootersvc.UpdatePatch) (*models.Scooter, error)
	Retire(ctx context.Context, id uuid.UUID) error
}

// ZonesService is the subset of the zones service used by HTTP handlers.
type ZonesService interface {
	Create(ctx context.Context, name string, boundary *string) (*models.Zone, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Zone, error)
	List(ctx context.Context, page models.Page) ([]models.Zone, int, error)
	Update(ctx context.Context, id uuid.UUID, name *string, boundary *string) (*models.Zone, error)
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
	Start(ctx context.Context, userID, scooterID, priceModelID uuid.UUID) (*models.Rental, error)
	End(ctx context.Context, rentalID, actorUserID uuid.UUID) (*models.Rental, error)
	AdminCancel(ctx context.Context, rentalID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, page models.Page) ([]models.Rental, int, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Rental, error)
}

// MaintenanceService is the subset of the maintenance service used by handlers.
type MaintenanceService interface {
	Create(ctx context.Context, scooterID uuid.UUID, description string, technicianID *uuid.UUID) (*models.MaintenanceLog, error)
	Close(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error)
	ListByScooter(ctx context.Context, scooterID uuid.UUID, page models.Page) ([]models.MaintenanceLog, int, error)
	Get(ctx context.Context, id uuid.UUID) (*models.MaintenanceLog, error)
}

// Handler is the fiber-based HTTP handler aggregate for all routes.
type Handler struct {
	version     string
	auth        AuthService
	users       UsersService
	scooters    ScootersService
	zones       ZonesService
	priceModels PriceModelsService
	rentals     RentalsService
	maintenance MaintenanceService
	log         *zap.Logger
}

// Deps groups the handler's construction dependencies.
type Deps struct {
	Version     string
	Auth        AuthService
	Users       UsersService
	Scooters    ScootersService
	Zones       ZonesService
	PriceModels PriceModelsService
	Rentals     RentalsService
	Maintenance MaintenanceService
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
		users:       d.Users,
		scooters:    d.Scooters,
		zones:       d.Zones,
		priceModels: d.PriceModels,
		rentals:     d.Rentals,
		maintenance: d.Maintenance,
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
	Email    string `json:"email" validate:"required,email,max=255"`
	Name     string `json:"name" validate:"required,min=1,max=100"`
	Password string `json:"password" validate:"required,min=8,max=128"`
	Phone    string `json:"phone" validate:"omitempty,max=20"`
}

type loginReq struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=1,max=128"`
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
	user, token, err := h.auth.Register(c.Context(), body.Email, body.Name, body.Password, body.Phone)
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
	Name  *string `json:"name" validate:"omitempty,min=1,max=100"`
	Phone *string `json:"phone" validate:"omitempty,max=20"`
}

type topupReq struct {
	Amount decimal.Decimal `json:"amount" validate:"required"`
}

type walletResp struct {
	Balance decimal.Decimal `json:"balance"`
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
	u, err := h.users.Update(c.Context(), id, usersvc.UpdatePatch{Name: body.Name, Phone: body.Phone})
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

// GetWallet returns the user's wallet balance; owner-or-admin only.
func (h *Handler) GetWallet(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	bal, err := h.users.GetWallet(c.Context(), id)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, walletResp{Balance: bal})
}

// TopUpWallet adds funds to the user's wallet; owner-or-admin only.
func (h *Handler) TopUpWallet(c fiber.Ctx) error {
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	if apiErr := assertOwnerOrAdmin(c, id); apiErr != nil {
		return WriteError(c, apiErr)
	}
	var body topupReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	bal, err := h.users.TopUp(c.Context(), id, body.Amount)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, walletResp{Balance: bal})
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

// ---- rentals (authenticated)

type startRentalReq struct {
	ScooterID    uuid.UUID `json:"scooter_id" validate:"required"`
	PriceModelID uuid.UUID `json:"price_model_id" validate:"required"`
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
	rental, err := h.rentals.Start(c.Context(), ident.UserID, body.ScooterID, body.PriceModelID)
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

// EndRental ends an active rental for the authenticated user.
func (h *Handler) EndRental(c fiber.Ctx) error {
	ident := IdentityFromCtx(c)
	if ident == nil {
		return WriteError(c, resp.ErrUnauthorized(""))
	}
	id, err := URLParamUUID(c, "id")
	if err != nil {
		return WriteDomain(c, err)
	}
	rental, err := h.rentals.End(c.Context(), id, ident.UserID)
	if err != nil {
		return WriteDomain(c, err)
	}
	return WriteJSON(c, fiber.StatusOK, rental)
}

// ---- admin: scooters

type createScooterReq struct {
	Code       string           `json:"code" validate:"required,min=1,max=20"`
	Model      string           `json:"model" validate:"required,min=1,max=100"`
	ZoneID     *uuid.UUID       `json:"zone_id"`
	Lat        *decimal.Decimal `json:"lat"`
	Lng        *decimal.Decimal `json:"lng"`
	BatteryPct int              `json:"battery_pct" validate:"gte=0,lte=100"`
}

// CreateScooter creates a new scooter; admin only.
func (h *Handler) CreateScooter(c fiber.Ctx) error {
	var body createScooterReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	s, err := h.scooters.Create(c.Context(), body.Code, body.Model, body.ZoneID, body.Lat, body.Lng, body.BatteryPct)
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
	if v, ok := raw["battery_pct"]; ok {
		if f, ok := v.(float64); ok {
			n := int(f)
			patch.BatteryPct = &n
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
	Name     string  `json:"name" validate:"required,min=1,max=100"`
	Boundary *string `json:"boundary"`
}

type updateZoneReq struct {
	Name     *string `json:"name" validate:"omitempty,min=1,max=100"`
	Boundary *string `json:"boundary"`
}

// CreateZone creates a new zone; admin only.
func (h *Handler) CreateZone(c fiber.Ctx) error {
	var body createZoneReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	z, err := h.zones.Create(c.Context(), body.Name, body.Boundary)
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
	var body updateZoneReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	z, err := h.zones.Update(c.Context(), id, body.Name, body.Boundary)
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
	Name          string           `json:"name" validate:"required,min=1,max=100"`
	PerMinuteRate decimal.Decimal  `json:"per_minute_rate" validate:"required"`
	UnlockFee     decimal.Decimal  `json:"unlock_fee"`
	DailyCap      *decimal.Decimal `json:"daily_cap"`
	Currency      string           `json:"currency" validate:"omitempty,len=3"`
}

// CreatePriceModel creates a new price model; admin only.
func (h *Handler) CreatePriceModel(c fiber.Ctx) error {
	var body createPriceModelReq
	if err := DecodeBody(c, &body); err != nil {
		return WriteDomain(c, err)
	}
	pm, err := h.priceModels.Create(c.Context(), pricesvc.CreateInput{
		Name:          body.Name,
		PerMinuteRate: body.PerMinuteRate,
		UnlockFee:     body.UnlockFee,
		DailyCap:      body.DailyCap,
		Currency:      body.Currency,
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
	if v, ok := raw["per_minute_rate"]; ok {
		if d, ok := decimalFromAny(v); ok {
			patch.PerMinuteRate = &d
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
	Description  string     `json:"description" validate:"required,min=1"`
	TechnicianID *uuid.UUID `json:"technician_id"`
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
	m, err := h.maintenance.Create(c.Context(), scooterID, body.Description, body.TechnicianID)
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
