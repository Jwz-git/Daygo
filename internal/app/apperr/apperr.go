// Package apperr defines the only error shape allowed to cross the Wails
// boundary. Internal causes stay in the unwrap chain; the frontend receives only
// the stable code and a sanitized, non-localized message.
package apperr

import "fmt"

const prefix = "daygo:"

// Code is one member of the closed application error-code set in docs/05 §5.4.
type Code string

const (
	InvalidArgument       Code = "invalid_argument"
	NotFound              Code = "not_found"
	NotCaptureOwner       Code = "not_capture_owner"
	PermissionDenied      Code = "permission_denied"
	ProviderNotConfigured Code = "provider_not_configured"
	ProviderFailed        Code = "provider_failed"
	NativeUnavailable     Code = "native_unavailable"
	MediaDecodeFailed     Code = "media_decode_failed"
	Conflict              Code = "conflict"
	DatabaseError         Code = "database_error"
	Canceled              Code = "canceled"
	Internal              Code = "internal"
)

var codes = [...]Code{
	InvalidArgument,
	NotFound,
	NotCaptureOwner,
	PermissionDenied,
	ProviderNotConfigured,
	ProviderFailed,
	NativeUnavailable,
	MediaDecodeFailed,
	Conflict,
	DatabaseError,
	Canceled,
	Internal,
}

// Valid reports whether c belongs to the closed error-code set.
func (c Code) Valid() bool {
	switch c {
	case InvalidArgument, NotFound, NotCaptureOwner, PermissionDenied,
		ProviderNotConfigured, ProviderFailed, NativeUnavailable,
		MediaDecodeFailed, Conflict, DatabaseError, Canceled, Internal:
		return true
	default:
		return false
	}
}

// Codes returns a defensive copy of the closed set.
func Codes() []Code {
	result := make([]Code, len(codes))
	copy(result, codes[:])
	return result
}

// Error is the only error type a binding method may return to Wails.
type Error struct {
	Code    Code
	Message string
	err     error
}

// E wraps an internal cause with a stable public code and sanitized message.
// The caller owns sanitization: msg must never contain screen content, window
// titles, paths, credentials, or LLM payloads (docs/05 §5.4).
func E(code Code, msg string, err error) *Error {
	if !code.Valid() {
		code = Internal
	}
	return &Error{Code: code, Message: msg, err: err}
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s%s: %s", prefix, e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Retryable is the static retry policy from docs/05 §5.4.1.
func (c Code) Retryable() bool {
	return c == ProviderFailed || c == NativeUnavailable || c == Conflict
}

// Reportable is the static reporting policy. Sampled codes return true here;
// sampling itself belongs to the diagnostics layer.
func (c Code) Reportable() bool {
	switch c {
	case ProviderFailed, NativeUnavailable, MediaDecodeFailed, Conflict, DatabaseError, Internal:
		return true
	default:
		return false
	}
}
