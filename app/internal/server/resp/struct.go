package resp

import "net/http"

// Meta carries envelope metadata.
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// Envelope is the standard successful response wrapper.
type Envelope struct {
	Data any  `json:"data"`
	Meta Meta `json:"meta"`
}

// ErrorEnvelope is the standard error response wrapper.
type ErrorEnvelope struct {
	Error *APIError `json:"error"`
}

// APIError describes a domain/http error returned to clients.
type APIError struct {
	HTTPCode int    `json:"-"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

// StatusOr returns the APIError HTTPCode, defaulting to 500 when unset.
func (e *APIError) StatusOr(def int) int {
	if e == nil || e.HTTPCode == 0 {
		if def == 0 {
			return http.StatusInternalServerError
		}
		return def
	}
	return e.HTTPCode
}
