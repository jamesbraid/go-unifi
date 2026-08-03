package unifi

import (
	"fmt"
	"time"
)

// RateLimitError indicates the controller rejected a request with HTTP 429
// (Too Many Requests). UniFi rate-limits POST /api/auth/login in particular, so
// a workflow that re-authenticates on every operation (username/password) can
// exhaust the limit. RetryAfter carries the controller's Retry-After hint when
// provided. Using API-key auth (UNIFI_API_KEY) avoids the per-request login
// entirely.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (err *RateLimitError) Error() string {
	if err.RetryAfter > 0 {
		return fmt.Sprintf(
			"rate limited by controller (retry after %s); consider API-key auth to avoid per-run login",
			err.RetryAfter,
		)
	}
	return "rate limited by controller; consider API-key auth to avoid per-run login"
}

type LoginRequiredError struct {
	APIKey bool // true when the rejection is for an API-key request
}

func (err *LoginRequiredError) Error() string {
	if err.APIKey {
		return "API key rejected (HTTP 401): check that the key is valid and has not been revoked"
	}
	return "login required"
}

type NotFoundError struct {
	Type  string
	Attr  string
	Value string
}

func (err *NotFoundError) Error() string {
	if err.Attr != "" && err.Value != "" {
		return fmt.Sprintf("not found: type=%s, attr=%s, value=%s", err.Type, err.Attr, err.Value)
	} else {
		return fmt.Sprintf("not found: type=%s", err.Type)
	}
}

// ValidationError is the controller's own account of why it rejected a
// field: which field, and the pattern the value had to match. The controller
// sends this alongside the error code, and the SDK used to drop it, leaving
// callers with a bare api.err.InvalidPayload and no way to tell which of
// eighty fields was at fault.
type ValidationError struct {
	Field   string `json:"field"`
	Pattern string `json:"pattern"`
}

// APIError is an error the controller reported in its response envelope. RC
// and Message carry the controller's own code and message; Validation is set
// only when the controller named a specific field.
type APIError struct {
	RC      string
	Message string
	// Validation is the controller's field-level detail, when it sent any.
	// Reach it with errors.As on *APIError to map the failure back onto the
	// field that caused it.
	Validation *ValidationError
}

func (err *APIError) Error() string {
	if err.Validation == nil || err.Validation.Field == "" {
		return err.Message
	}

	detail := err.Validation.Field
	if err.Validation.Pattern != "" {
		detail = fmt.Sprintf("%s must match %s", detail, err.Validation.Pattern)
	} else {
		detail = fmt.Sprintf("%s is invalid", detail)
	}
	if err.Message == "" {
		return detail
	}
	return fmt.Sprintf("%s: %s", err.Message, detail)
}
