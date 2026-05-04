package resp

import (
	"net/http"
	"strings"

	"github.com/uniscoot/scooter-renting-backend/app/internal/apperrors"
)

// Error codes returned to clients.
const (
	CodeBadRequest         = "BAD_REQUEST"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeConflict           = "CONFLICT"
	CodeInternal           = "INTERNAL"
	CodeValidation         = "VALIDATION_FAILED"
	CodeScooterUnavailable = "SCOOTER_UNAVAILABLE"
	CodeEmailExists        = "EMAIL_ALREADY_EXISTS"
	CodeInsufficientFunds  = "INSUFFICIENT_FUNDS"
	CodeRentalAlreadyEnded = "RENTAL_ALREADY_ENDED"
)

// Default machine-readable kinds for the standard error codes. Business
// flows that need a more specific kind (e.g. "add_card_required") set it
// via the apperrors message which FromDomain copies onto APIError.Kind.
const (
	KindBadRequest         = "bad_request"
	KindUnauthorized       = "unauthorized"
	KindForbidden          = "forbidden"
	KindNotFound           = "not_found"
	KindConflict           = "conflict"
	KindInternal           = "internal"
	KindValidation         = "validation_failed"
	KindScooterUnavailable = "scooter_unavailable"
	KindEmailExists        = "email_already_exists"
	KindInsufficientFunds  = "insufficient_funds"
	KindRentalAlreadyEnded = "rental_already_ended"
)

// businessConflictKinds is the closed set of conflict messages whose value
// is meant to flow through to the JSON envelope's `kind` field. Anything
// outside this set falls back to KindConflict so we never leak internal
// human-readable strings as machine identifiers.
var businessConflictKinds = map[string]struct{}{
	"add_card_required":     {},
	"outstanding_balance":   {},
	"account_already_linked": {},
}

// ErrBadRequest builds a 400 APIError with CodeBadRequest by default.
func ErrBadRequest(code, msg string) *APIError {
	if code == "" {
		code = CodeBadRequest
	}
	return &APIError{HTTPCode: http.StatusBadRequest, Code: code, Kind: KindBadRequest, Message: msg}
}

// ErrUnauthorized builds a 401 APIError.
func ErrUnauthorized(msg string) *APIError {
	if msg == "" {
		msg = "unauthorized"
	}
	return &APIError{HTTPCode: http.StatusUnauthorized, Code: CodeUnauthorized, Kind: KindUnauthorized, Message: msg}
}

// ErrForbidden builds a 403 APIError.
func ErrForbidden(msg string) *APIError {
	if msg == "" {
		msg = "forbidden"
	}
	return &APIError{HTTPCode: http.StatusForbidden, Code: CodeForbidden, Kind: KindForbidden, Message: msg}
}

// ErrNotFound builds a 404 APIError.
func ErrNotFound(resource string) *APIError {
	msg := "not found"
	if resource != "" {
		msg = resource + " not found"
	}
	return &APIError{HTTPCode: http.StatusNotFound, Code: CodeNotFound, Kind: KindNotFound, Message: msg}
}

// ErrConflict builds a 409 APIError.
func ErrConflict(code, msg string) *APIError {
	if code == "" {
		code = CodeConflict
	}
	return &APIError{HTTPCode: http.StatusConflict, Code: code, Kind: KindConflict, Message: msg}
}

// ErrValidation builds a 400 APIError with the validation code.
func ErrValidation(msg string) *APIError {
	if msg == "" {
		msg = "validation failed"
	}
	return &APIError{HTTPCode: http.StatusBadRequest, Code: CodeValidation, Kind: KindValidation, Message: msg}
}

// ErrInternal builds a 500 APIError.
func ErrInternal() *APIError {
	return &APIError{HTTPCode: http.StatusInternalServerError, Code: CodeInternal, Kind: KindInternal, Message: "internal server error"}
}

// FromDomain maps an apperrors.Kind (or sentinel) error onto an *APIError.
func FromDomain(err error) *APIError {
	if err == nil {
		return nil
	}

	kind := apperrors.KindOf(err)
	msg := err.Error()

	switch kind {
	case apperrors.KindNotFound:
		return &APIError{HTTPCode: http.StatusNotFound, Code: CodeNotFound, Kind: KindNotFound, Message: msg}
	case apperrors.KindConflict:
		code := CodeConflict
		respKind := KindConflict
		lowered := strings.ToLower(msg)
		if strings.Contains(lowered, "email") {
			code = CodeEmailExists
			respKind = KindEmailExists
		}
		// Business conflicts carry their machine-readable kind in the
		// message itself (e.g. "add_card_required"); copy it through
		// to the envelope's kind so the FE can branch on it.
		if _, ok := businessConflictKinds[msg]; ok {
			respKind = msg
		}
		return &APIError{HTTPCode: http.StatusConflict, Code: code, Kind: respKind, Message: msg}
	case apperrors.KindInvalid:
		return &APIError{HTTPCode: http.StatusBadRequest, Code: CodeValidation, Kind: KindValidation, Message: msg}
	case apperrors.KindUnauthorized:
		return &APIError{HTTPCode: http.StatusUnauthorized, Code: CodeUnauthorized, Kind: KindUnauthorized, Message: msg}
	case apperrors.KindForbidden:
		return &APIError{HTTPCode: http.StatusForbidden, Code: CodeForbidden, Kind: KindForbidden, Message: msg}
	case apperrors.KindInsufficient:
		return &APIError{HTTPCode: http.StatusConflict, Code: CodeInsufficientFunds, Kind: KindInsufficientFunds, Message: msg}
	case apperrors.KindScooterUnavailable:
		return &APIError{HTTPCode: http.StatusConflict, Code: CodeScooterUnavailable, Kind: KindScooterUnavailable, Message: msg}
	case apperrors.KindRentalAlreadyEnded:
		return &APIError{HTTPCode: http.StatusConflict, Code: CodeRentalAlreadyEnded, Kind: KindRentalAlreadyEnded, Message: msg}
	}
	return ErrInternal()
}
