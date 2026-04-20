package public

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
	"github.com/uniscoot/scooter-renting-backend/app/internal/models"
	"github.com/uniscoot/scooter-renting-backend/app/internal/server/middleware"
)

var (
	validateOnce sync.Once
	validate     *validator.Validate
)

func getValidator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())
	})
	return validate
}

// ValidateStruct runs struct validation on v, returning an apperrors.Invalid
// error describing the first batch of failures.
func ValidateStruct(v any) error {
	if err := getValidator().Struct(v); err != nil {
		var invalid *validator.InvalidValidationError
		if errors.As(err, &invalid) {
			return apperrors.Invalid("validation error: " + err.Error())
		}

		var ves validator.ValidationErrors
		if errors.As(err, &ves) {
			parts := make([]string, 0, len(ves))
			for _, fe := range ves {
				parts = append(parts, fe.Field()+" failed "+fe.Tag())
			}
			return apperrors.Invalid("validation failed: " + strings.Join(parts, ", "))
		}
		return apperrors.Invalid("validation failed: " + err.Error())
	}
	return nil
}

// DecodeBody parses the request body into v (rejecting unknown fields and
// empty payloads) and then runs validator rules on it.
func DecodeBody(c fiber.Ctx, v any) error {
	body := c.Body()
	if len(body) == 0 {
		return apperrors.Invalid("empty request body")
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return apperrors.Invalid("empty request body")
		}
		return apperrors.Invalid("invalid json: " + err.Error())
	}
	return ValidateStruct(v)
}

// DecodeRawBody parses the request body into v without validation.
// Useful for PATCH-style endpoints that read into map[string]any to detect
// presence/absence of fields.
func DecodeRawBody(c fiber.Ctx, v any) error {
	body := c.Body()
	if len(body) == 0 {
		return apperrors.Invalid("empty request body")
	}
	if err := json.Unmarshal(body, v); err != nil {
		return apperrors.Invalid("invalid json: " + err.Error())
	}
	return nil
}

// URLParamUUID reads a URL parameter and parses it as uuid.UUID.
func URLParamUUID(c fiber.Ctx, key string) (uuid.UUID, error) {
	raw := c.Params(key)
	if raw == "" {
		return uuid.Nil, apperrors.Invalid("missing url param " + key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Invalid("invalid uuid for " + key + ": " + err.Error())
	}
	return id, nil
}

// QueryString returns the query-string value for key or def if absent.
func QueryString(c fiber.Ctx, key, def string) string {
	v := c.Query(key)
	if v == "" {
		return def
	}
	return v
}

// QueryInt parses the query-string value for key as int or returns def.
func QueryInt(c fiber.Ctx, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// QueryFloat parses the query-string value for key as float64 or returns def.
func QueryFloat(c fiber.Ctx, key string, def float64) float64 {
	v := c.Query(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// PageFromCtx reads limit/offset query params and returns a clamped models.Page.
func PageFromCtx(c fiber.Ctx) models.Page {
	p := models.Page{
		Limit:  QueryInt(c, "limit", models.DefaultPageLimit),
		Offset: QueryInt(c, "offset", 0),
	}
	return p.Clamp()
}

// IdentityFromCtx returns the authenticated *Identity from fiber.Ctx locals
// or nil when absent.
func IdentityFromCtx(c fiber.Ctx) *middleware.Identity {
	v, _ := c.Locals(middleware.KeyIdentity).(*middleware.Identity)
	return v
}
