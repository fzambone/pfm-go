package http_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// TestHTTPMessageConstants_Exist verifies that all HTTP-layer message constants
// are defined in the message package. These constants provide user-facing error
// messages for the response infrastructure.
func TestHTTPMessageConstants_Exist(t *testing.T) {
	t.Parallel()

	// Each constant must be a non-empty string — the exact wording is tested
	// by the response helper tests that use them.
	constants := map[string]string{
		"MsgBadRequestBody":   message.MsgBadRequestBody,
		"MsgNotFound":         message.MsgNotFound,
		"MsgConflict":         message.MsgConflict,
		"MsgInternalError":    message.MsgInternalError,
		"MsgUnbalancedLedger": message.MsgUnbalancedLedger,
		"MsgBalanceNotZero":   message.MsgBalanceNotZero,
		"MsgNotCreditCard":    message.MsgNotCreditCard,
		"MsgLastAdmin":        message.MsgLastAdmin,
	}

	for name, val := range constants {
		assert.NotEmpty(t, val, "%s must not be empty", name)
	}
}

// --- WriteJSON tests ---

// TestWriteJSON_SerializesStruct verifies AC9: a success response helper
// serializes a struct to JSON with Content-Type application/json.
func TestWriteJSON_SerializesStruct(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	w := httptest.NewRecorder()
	pfmhttp.WriteJSON(w, http.StatusOK, payload{Name: "Alice", Age: 30})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got payload
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "Alice", got.Name)
	assert.Equal(t, 30, got.Age)
}

// TestWriteJSON_NilValue verifies edge case: nil value does not panic.
func TestWriteJSON_NilValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	pfmhttp.WriteJSON(w, http.StatusOK, nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

// --- WriteCreated tests ---

// TestWriteCreated_Returns201WithLocation verifies AC10: a 201 Created response
// includes the Location header and serialized JSON body.
func TestWriteCreated_Returns201WithLocation(t *testing.T) {
	t.Parallel()

	type resource struct {
		ID string `json:"id"`
	}

	w := httptest.NewRecorder()
	pfmhttp.WriteCreated(w, "/api/v1/users/abc-123", resource{ID: "abc-123"})

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "/api/v1/users/abc-123", w.Header().Get("Location"))
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got resource
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "abc-123", got.ID)
}

// --- WriteNoContent tests ---

// TestWriteNoContent_Returns204WithEmptyBody verifies edge case: 204 No Content
// has an empty body (e.g., deactivate, delete operations).
func TestWriteNoContent_Returns204WithEmptyBody(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	pfmhttp.WriteNoContent(w)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

// --- DecodeBody tests ---

// TestDecodeBody_ValidJSON verifies AC7 happy path: a well-formed JSON body
// is decoded into the target struct without error.
func TestDecodeBody_ValidJSON(t *testing.T) {
	t.Parallel()

	type input struct {
		Name string `json:"name"`
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Bob"}`))
	var got input
	err := pfmhttp.DecodeBody(r, &got)

	require.NoError(t, err)
	assert.Equal(t, "Bob", got.Name)
}

// TestDecodeBody_MalformedJSON verifies AC7: malformed JSON returns an error.
func TestDecodeBody_MalformedJSON(t *testing.T) {
	t.Parallel()

	type input struct {
		Name string `json:"name"`
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad json}"))
	var got input
	err := pfmhttp.DecodeBody(r, &got)

	require.Error(t, err)
}

// TestDecodeBody_EmptyBody verifies AC7 edge case: empty body returns an error.
func TestDecodeBody_EmptyBody(t *testing.T) {
	t.Parallel()

	type input struct {
		Name string `json:"name"`
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	var got input
	err := pfmhttp.DecodeBody(r, &got)

	require.Error(t, err)
}

// --- MapError tests ---

// TestMapError_DomainErrors verifies AC2–AC4 and AC8: each domain error sentinel
// maps to the correct HTTP status code and user-facing message.
func TestMapError_DomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		// AC2: not found errors → 404
		{"user not found", message.ErrUserNotFound, http.StatusNotFound, message.MsgNotFound},
		{"household not found", message.ErrHouseholdNotFound, http.StatusNotFound, message.MsgNotFound},
		{"account not found", message.ErrAccountNotFound, http.StatusNotFound, message.MsgNotFound},
		{"cc settings not found", message.ErrCreditCardSettingsNotFound, http.StatusNotFound, message.MsgNotFound},

		// AC3: version conflict errors → 409
		{"user version conflict", message.ErrUserVersionConflict, http.StatusConflict, message.MsgConflict},
		{"household version conflict", message.ErrHouseholdVersionConflict, http.StatusConflict, message.MsgConflict},
		{"account version conflict", message.ErrAccountVersionConflict, http.StatusConflict, message.MsgConflict},
		{"cc settings version conflict", message.ErrCreditCardSettingsVersionConflict, http.StatusConflict, message.MsgConflict},

		// AC4: already-exists / duplicate errors → 409
		{"email taken", message.ErrUserEmailTaken, http.StatusConflict, message.MsgConflict},
		{"member exists", message.ErrHouseholdMemberExists, http.StatusConflict, message.MsgConflict},
		{"account name taken", message.ErrAccountNameTaken, http.StatusConflict, message.MsgConflict},
		{"cc settings exists", message.ErrCreditCardSettingsExists, http.StatusConflict, message.MsgConflict},

		// Domain-specific errors with specific messages
		{"unbalanced ledger", message.ErrLedgerUnbalanced, http.StatusUnprocessableEntity, message.MsgUnbalancedLedger},
		{"balance not zero", message.ErrAccountBalanceNotZero, http.StatusConflict, message.MsgBalanceNotZero},
		{"not credit card", message.ErrCreditCardSettingsNotCreditCard, http.StatusConflict, message.MsgNotCreditCard},
		{"last admin", message.ErrHouseholdLastAdmin, http.StatusConflict, message.MsgLastAdmin},
		{"member not found", message.ErrHouseholdMemberNotFound, http.StatusNotFound, message.MsgNotFound},
		{"invalid credentials", message.ErrLoginInvalidCredentials, http.StatusUnauthorized, message.MsgLoginInvalidCredentials},

		// AC8: unrecognized error → 500
		{"unknown error", errors.New("something unexpected"), http.StatusInternalServerError, message.MsgInternalError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, msg := pfmhttp.MapError(tc.err)
			assert.Equal(t, tc.wantStatus, status)
			assert.Equal(t, tc.wantMsg, msg)
		})
	}
}

// TestMapError_WrappedErrors verifies that errors.Is unwrapping works — a domain
// sentinel wrapped with fmt.Errorf still maps correctly.
func TestMapError_WrappedErrors(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("user logic: find by id: %w", message.ErrUserNotFound)
	status, msg := pfmhttp.MapError(wrapped)

	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, message.MsgNotFound, msg)
}

// --- WriteError tests ---

// TestWriteError_DomainError verifies the full flow: WriteError maps a domain
// error to status + message and writes the structured JSON response.
func TestWriteError_DomainError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	pfmhttp.WriteError(w, message.ErrUserNotFound)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	body := decodeBody(t, w)
	assert.Equal(t, message.MsgNotFound, body["error"])
}

// TestWriteError_UnknownError verifies AC6: unexpected errors return 500 with
// a generic message — no internal details leaked.
func TestWriteError_UnknownError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	pfmhttp.WriteError(w, errors.New("database connection refused"))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := decodeBody(t, w)
	assert.Equal(t, message.MsgInternalError, body["error"])
	// Must NOT contain the original error message
	assert.NotContains(t, w.Body.String(), "database connection refused")
}

// --- WriteValidationError tests ---

// TestWriteValidationError_FieldViolations verifies AC5: validation errors return
// 400 with per-field violation details.
func TestWriteValidationError_FieldViolations(t *testing.T) {
	t.Parallel()

	ve := &validate.ValidationError{
		Violations: []validate.Violation{
			{Field: "name", Message: "is required"},
			{Field: "email", Message: "must be at least 3 characters"},
		},
	}

	w := httptest.NewRecorder()
	pfmhttp.WriteValidationError(w, ve)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	body := decodeBody(t, w)
	assert.Equal(t, message.MsgValidationFailed, body["error"])
	fields, ok := body["fields"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "is required", fields["name"])
	assert.Equal(t, "must be at least 3 characters", fields["email"])
}
