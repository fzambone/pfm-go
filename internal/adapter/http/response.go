package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

const headerLocation = "Location"

// WriteJSON serializes v as JSON and writes it to w with the given HTTP status code.
// Content-Type is set to application/json. If v is nil, the body is "null".
// Encoding errors are best-effort — the status code has already been sent.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // best-effort
}

// WriteCreated writes a 201 Created response with a Location header pointing to
// the newly created resource and the resource serialized as JSON in the body.
func WriteCreated(w http.ResponseWriter, location string, v any) {
	w.Header().Set(headerLocation, location)
	WriteJSON(w, http.StatusCreated, v)
}

// WriteNoContent writes a 204 No Content response with an empty body.
// Used for delete and deactivate operations where no response payload is needed.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// DecodeBody reads the request body and decodes it as JSON into dst.
// Returns an error if the body is empty or contains malformed JSON.
func DecodeBody(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

// errorMapping pairs a domain error sentinel with the HTTP status code and
// user-facing message it should produce. Order matters: the first match wins,
// so more specific errors should appear before generic ones.
type errorMapping struct {
	sentinel error
	status   int
	message  string
}

// errMappings is the single source of truth for domain-error-to-HTTP translation.
// Each entry uses errors.Is for matching, so wrapped errors are handled correctly.
var errMappings = []errorMapping{
	// Authentication
	{message.ErrLoginInvalidCredentials, http.StatusUnauthorized, message.MsgLoginInvalidCredentials},

	// Not found → 404
	{message.ErrUserNotFound, http.StatusNotFound, message.MsgNotFound},
	{message.ErrHouseholdNotFound, http.StatusNotFound, message.MsgNotFound},
	{message.ErrHouseholdMemberNotFound, http.StatusNotFound, message.MsgNotFound},
	{message.ErrAccountNotFound, http.StatusNotFound, message.MsgNotFound},
	{message.ErrCreditCardSettingsNotFound, http.StatusNotFound, message.MsgNotFound},

	// Version conflict → 409
	{message.ErrUserVersionConflict, http.StatusConflict, message.MsgConflict},
	{message.ErrHouseholdVersionConflict, http.StatusConflict, message.MsgConflict},
	{message.ErrAccountVersionConflict, http.StatusConflict, message.MsgConflict},
	{message.ErrCreditCardSettingsVersionConflict, http.StatusConflict, message.MsgConflict},

	// Already exists / duplicates → 409
	{message.ErrUserEmailTaken, http.StatusConflict, message.MsgConflict},
	{message.ErrHouseholdMemberExists, http.StatusConflict, message.MsgConflict},
	{message.ErrAccountNameTaken, http.StatusConflict, message.MsgConflict},
	{message.ErrCreditCardSettingsExists, http.StatusConflict, message.MsgConflict},

	// Domain-specific business rule violations
	{message.ErrLedgerUnbalanced, http.StatusUnprocessableEntity, message.MsgUnbalancedLedger},
	{message.ErrAccountBalanceNotZero, http.StatusConflict, message.MsgBalanceNotZero},
	{message.ErrCreditCardSettingsNotCreditCard, http.StatusConflict, message.MsgNotCreditCard},
	{message.ErrHouseholdLastAdmin, http.StatusConflict, message.MsgLastAdmin},
}

// MapError translates a domain error into an HTTP status code and user-facing
// message. It uses errors.Is to match wrapped errors against known sentinels.
// Unrecognized errors default to 500 Internal Server Error with a generic message
// so that internal details are never leaked to the client.
func MapError(err error) (status int, msg string) {
	for _, m := range errMappings {
		if errors.Is(err, m.sentinel) {
			return m.status, m.message
		}
	}
	return http.StatusInternalServerError, message.MsgInternalError
}

// WriteError maps a domain error to an HTTP status code and writes a structured
// JSON error response. Unrecognized errors produce a 500 with a generic message.
func WriteError(w http.ResponseWriter, err error) {
	status, msg := MapError(err)
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg}) // best-effort
}

// WriteValidationError writes a 400 Bad Request response with per-field violation
// details. The response body contains an "error" key with a summary message and
// a "fields" object mapping field names to their validation failure messages.
func WriteValidationError(w http.ResponseWriter, ve *validate.ValidationError) {
	fields := make(map[string]string, len(ve.Violations))
	for _, v := range ve.Violations {
		fields[v.Field] = v.Message
	}
	w.Header().Set(headerContentType, mimeJSON)
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{ // best-effort
		"error":  message.MsgValidationFailed,
		"fields": fields,
	})
}
