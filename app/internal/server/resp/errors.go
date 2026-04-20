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

// ErrBadRequest builds a 400 APIError with CodeBadRequest by default.
func ErrBadRequest(code, msg string) *APIError {
	if code == "" {
		code = CodeBadRequest
	}
	return &APIError{HTTPCode: http.StatusBadRequest, Code: code, Message: msg}
}

// ErrUnauthorized builds a 401 APIError.
func ErrUnauthorized(msg string) *APIError {
	if msg == "" {
		msg = "unauthorized"
	}
	return &APIError{HTTPCode: http.StatusUnauthorized, Code: CodeUnauthorized, Message: msg}
}

// ErrForbidden builds a 403 APIError.
func ErrForbidden(msg string) *APIError {
	if msg == "" {
		msg = "forbidden"
	}
	return &APIError{HTTPCode: http.StatusForbidden, Code: CodeForbidden, Message: msg}
}

// ErrNotFound builds a 404 APIError.
func ErrNotFound(resource string) *APIError {
	msg := "not found"
	if resource != "" {
		msg = resource + " not found"
	}
	return &APIError{HTTPCode: http.StatusNotFound, Code: CodeNotFound, Message: msg}
}

// ErrConflict builds a 409 APIError.
func ErrConflict(code, msg string) *APIError {
	if code == "" {
		code = CodeConflict
	}
	return &APIError{HTTPCode: http.StatusConflict, Code: code, Message: msg}
}

// ErrValidation builds a 400 APIError with the validation code.
func ErrValidation(msg string) *APIError {
	if msg == "" {
		msg = "validation failed"
	}
	return &APIError{HTTPCode: http.StatusBadRequest, Code: CodeValidation, Message: msg}
}

// ErrInternal builds a 500 APIError.
func ErrInternal() *APIError {
	return &APIError{HTTPCode: http.StatusInternalServerError, Code: CodeInternal, Message: "internal server error"}
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
		return &APIError{HTTPCode: http.StatusNotFound, Code: CodeNotFound, Message: msg}
	case apperrors.KindConflict:
		code := CodeConflict
		if strings.Contains(strings.ToLower(msg), "email") {
			code = CodeEmailExists
		}
		return &APIError{HTTPCode: http.StatusConflict, Code: code, Message: msg}
	case apperrors.KindInvalid:
		return &APIError{HTTPCode: http.StatusBadRequest, Code: CodeValidation, Message: msg}
	case apperrors.KindUnauthorized:
		return &APIError{HTTPCode: http.StatusUnauthorized, Code: CodeUnauthorized, Message: msg}
	case apperrors.KindForbidden:
		return &APIError{HTTPCode: http.StatusForbidden, Code: CodeForbidden, Message: msg}
	case apperrors.KindInsufficient:
		return &APIError{HTTPCode: http.StatusConflict, Code: CodeInsufficientFunds, Message: msg}
	case apperrors.KindScooterUnavailable:
		return &APIError{HTTPCode: http.StatusConflict, Code: CodeScooterUnavailable, Message: msg}
	case apperrors.KindRentalAlreadyEnded:
		return &APIError{HTTPCode: http.StatusConflict, Code: CodeRentalAlreadyEnded, Message: msg}
	}
	return ErrInternal()
}
