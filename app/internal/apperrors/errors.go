package apperrors

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInternal Kind = iota
	KindNotFound
	KindConflict
	KindInvalid
	KindUnauthorized
	KindForbidden
	KindInsufficient
	KindScooterUnavailable
	KindRentalAlreadyEnded
)

type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string {
	return e.Msg
}

// Wrap builds an *Error carrying the given kind and a formatted message.
func Wrap(kind Kind, format string, args ...any) error {
	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...)}
}

// Is reports whether err (or anything it wraps) is an apperrors.Error of the given kind.
func Is(err error, kind Kind) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind == kind
	}
	switch kind {
	case KindNotFound:
		return errors.Is(err, ErrNotFound)
	case KindConflict:
		return errors.Is(err, ErrConflict)
	case KindInvalid:
		return errors.Is(err, ErrInvalidInput)
	case KindUnauthorized:
		return errors.Is(err, ErrUnauthorized)
	case KindForbidden:
		return errors.Is(err, ErrForbidden)
	case KindInsufficient:
		return errors.Is(err, ErrInsufficientFunds)
	case KindScooterUnavailable:
		return errors.Is(err, ErrScooterUnavailable)
	case KindRentalAlreadyEnded:
		return errors.Is(err, ErrRentalAlreadyEnded)
	}
	return false
}

// KindOf returns the kind embedded in err, or KindInternal if none.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return KindNotFound
	case errors.Is(err, ErrConflict):
		return KindConflict
	case errors.Is(err, ErrInvalidInput):
		return KindInvalid
	case errors.Is(err, ErrUnauthorized):
		return KindUnauthorized
	case errors.Is(err, ErrForbidden):
		return KindForbidden
	case errors.Is(err, ErrInsufficientFunds):
		return KindInsufficient
	case errors.Is(err, ErrScooterUnavailable):
		return KindScooterUnavailable
	case errors.Is(err, ErrRentalAlreadyEnded):
		return KindRentalAlreadyEnded
	}
	return KindInternal
}

var (
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("conflict")
	ErrInvalidInput         = errors.New("invalid input")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrScooterUnavailable   = errors.New("scooter unavailable")
	ErrRentalAlreadyEnded   = errors.New("rental already ended")
)

func NotFound(resource string) error {
	return &Error{Kind: KindNotFound, Msg: resource + " not found"}
}

func Conflict(msg string) error {
	return &Error{Kind: KindConflict, Msg: msg}
}

func Invalid(msg string) error {
	return &Error{Kind: KindInvalid, Msg: msg}
}

func Unauthorized(msg string) error {
	return &Error{Kind: KindUnauthorized, Msg: msg}
}

func Forbidden(msg string) error {
	return &Error{Kind: KindForbidden, Msg: msg}
}

func Insufficient(msg string) error {
	if msg == "" {
		msg = "insufficient funds"
	}
	return &Error{Kind: KindInsufficient, Msg: msg}
}

func ScooterUnavailable(msg string) error {
	if msg == "" {
		msg = "scooter unavailable"
	}
	return &Error{Kind: KindScooterUnavailable, Msg: msg}
}

func RentalAlreadyEnded(msg string) error {
	if msg == "" {
		msg = "rental already ended"
	}
	return &Error{Kind: KindRentalAlreadyEnded, Msg: msg}
}

func Internal(msg string) error {
	if msg == "" {
		msg = "internal error"
	}
	return &Error{Kind: KindInternal, Msg: msg}
}
