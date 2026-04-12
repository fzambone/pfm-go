package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	domainacct "github.com/zambone/pfm-go/internal/domain/account"
	domaincc "github.com/zambone/pfm-go/internal/domain/creditcard"
	domainhouse "github.com/zambone/pfm-go/internal/domain/household"
	domainledger "github.com/zambone/pfm-go/internal/domain/ledger"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/types"
)

// --- Stub that satisfies all handler service interfaces for route registration ---

// allServicesStub implements every service interface used by every handler,
// allowing RegisterRoutes to wire without real dependencies.
type allServicesStub struct{}

func (s *allServicesStub) Login(_ context.Context, _, _ string) (domainuser.LoginResult, error) {
	return domainuser.LoginResult{}, nil
}
func (s *allServicesStub) FindByID(_ context.Context, _ uuid.UUID) (domainuser.User, error) {
	return domainuser.User{}, nil
}
func (s *allServicesStub) UpdateProfile(_ context.Context, _ uuid.UUID, _ domainuser.UpdateProfileInput, _ int, _ uuid.UUID) (domainuser.User, error) {
	return domainuser.User{}, nil
}
func (s *allServicesStub) ChangePassword(_ context.Context, _ uuid.UUID, _ domainuser.ChangePasswordInput, _ int, _ uuid.UUID) (domainuser.User, error) {
	return domainuser.User{}, nil
}
func (s *allServicesStub) Deactivate(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

// tokenValidatorStub satisfies the tokenValidator interface for authn middleware.
type tokenValidatorStub struct{}

func (s *tokenValidatorStub) Validate(_ context.Context, _ string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

// membershipFinderStub satisfies the membershipFinder interface for authz middleware.
type membershipFinderStub struct{}

func (s *membershipFinderStub) FindRole(_ context.Context, _, _ uuid.UUID) (types.Role, error) {
	return types.RoleAdmin, nil
}

// householdServiceStub satisfies all household handler interfaces.
type householdServiceStub struct{}

func (s *householdServiceStub) Create(_ context.Context, _ domainhouse.CreateInput, _ uuid.UUID) (domainhouse.Household, error) {
	return domainhouse.Household{}, nil
}
func (s *householdServiceStub) CreateHouseholdUser(_ context.Context, _ uuid.UUID, _ domainhouse.NewUserInput, _ uuid.UUID) (domainhouse.CreatedMember, error) {
	return domainhouse.CreatedMember{}, nil
}
func (s *householdServiceStub) FindByID(_ context.Context, _ uuid.UUID) (domainhouse.Household, error) {
	return domainhouse.Household{}, nil
}
func (s *householdServiceStub) ListForUser(_ context.Context, _ uuid.UUID) ([]domainhouse.Household, error) {
	return []domainhouse.Household{}, nil
}
func (s *householdServiceStub) AddMember(_ context.Context, _ uuid.UUID, _ domainhouse.AddMemberInput, _ uuid.UUID) (domainhouse.Membership, error) {
	return domainhouse.Membership{}, nil
}
func (s *householdServiceStub) RemoveMember(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (s *householdServiceStub) UpdateName(_ context.Context, _ uuid.UUID, _ domainhouse.UpdateNameInput, _ int, _ uuid.UUID) (domainhouse.Household, error) {
	return domainhouse.Household{}, nil
}

func (s *householdServiceStub) Deactivate(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

// accountServiceStub satisfies all account handler interfaces.
type accountServiceStub struct{}

func (s *accountServiceStub) Create(_ context.Context, _ uuid.UUID, _ domainacct.CreateInput, _ uuid.UUID) (domainacct.Account, error) {
	return domainacct.Account{}, nil
}
func (s *accountServiceStub) FindByID(_ context.Context, _ uuid.UUID) (domainacct.Account, error) {
	return domainacct.Account{}, nil
}
func (s *accountServiceStub) ListForHousehold(_ context.Context, _ uuid.UUID) ([]domainacct.Account, error) {
	return []domainacct.Account{}, nil
}
func (s *accountServiceStub) UpdateName(_ context.Context, _ uuid.UUID, _ domainacct.UpdateNameInput, _ int, _ uuid.UUID) (domainacct.Account, error) {
	return domainacct.Account{}, nil
}
func (s *accountServiceStub) UpdateBalance(_ context.Context, _ uuid.UUID, _ domainacct.UpdateBalanceInput, _ int, _ uuid.UUID) (domainacct.Account, error) {
	return domainacct.Account{}, nil
}

func (s *accountServiceStub) Deactivate(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

// ccSettingsServiceStub satisfies all credit card settings handler interfaces.
type ccSettingsServiceStub struct{}

func (s *ccSettingsServiceStub) Create(_ context.Context, _ uuid.UUID, _ domaincc.CreateInput, _ uuid.UUID) (domaincc.Settings, error) {
	return domaincc.Settings{}, nil
}
func (s *ccSettingsServiceStub) FindByAccountID(_ context.Context, _ uuid.UUID) (domaincc.Settings, error) {
	return domaincc.Settings{}, nil
}
func (s *ccSettingsServiceStub) UpdateClosingDay(_ context.Context, _ uuid.UUID, _ domaincc.UpdateClosingDayInput, _ int, _ uuid.UUID) (domaincc.Settings, error) {
	return domaincc.Settings{}, nil
}
func (s *ccSettingsServiceStub) UpdateDueDay(_ context.Context, _ uuid.UUID, _ domaincc.UpdateDueDayInput, _ int, _ uuid.UUID) (domaincc.Settings, error) {
	return domaincc.Settings{}, nil
}
func (s *ccSettingsServiceStub) UpdateLimit(_ context.Context, _ uuid.UUID, _ domaincc.UpdateLimitInput, _ int, _ uuid.UUID) (domaincc.Settings, error) {
	return domaincc.Settings{}, nil
}
func (s *ccSettingsServiceStub) Delete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

// ledgerServiceStub satisfies all ledger handler interfaces.
type ledgerServiceStub struct{}

func (s *ledgerServiceStub) PostTransaction(_ context.Context, _ uuid.UUID, _ domainledger.PostTransactionInput, _ uuid.UUID) (domainledger.Transaction, []domainledger.Entry, error) {
	return domainledger.Transaction{}, nil, nil
}
func (s *ledgerServiceStub) GetBalance(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (s *ledgerServiceStub) GetTransactionHistory(_ context.Context, _ uuid.UUID, _ domainledger.HistoryQuery) ([]domainledger.TransactionWithEntries, error) {
	return []domainledger.TransactionWithEntries{}, nil
}

// buildTestDeps creates a RouteDeps with all stubs for route registration testing.
func buildTestDeps() pfmhttp.RouteDeps {
	var shuttingDown atomic.Bool
	userSvc := &allServicesStub{}
	houseSvc := &householdServiceStub{}
	acctSvc := &accountServiceStub{}
	ccSvc := &ccSettingsServiceStub{}
	ledgerSvc := &ledgerServiceStub{}

	return pfmhttp.RouteDeps{
		Version:        "test",
		ShuttingDown:   &shuttingDown,
		TokenValidator: &tokenValidatorStub{},
		MembershipFinder: &membershipFinderStub{},
		LoginSvc:            userSvc,
		GetUserSvc:          userSvc,
		UpdateProfileSvc:    userSvc,
		ChangePasswordSvc:   userSvc,
		DeactivateUserSvc:   userSvc,
		CreateHouseholdSvc:     houseSvc,
		CreateHouseholdUserSvc: houseSvc,
		GetHouseholdSvc:        houseSvc,
		ListHouseholdsSvc:      houseSvc,
		AddMemberSvc:           houseSvc,
		RemoveMemberSvc:        houseSvc,
		UpdateHouseholdNameSvc:    houseSvc,
		DeactivateHouseholdSvc:    houseSvc,
		CreateAccountSvc:          acctSvc,
		GetAccountSvc:             acctSvc,
		ListAccountsSvc:           acctSvc,
		UpdateAccountNameSvc:      acctSvc,
		UpdateAccountBalanceSvc:   acctSvc,
		DeactivateAccountSvc:      acctSvc,
		CreateCCSettingsSvc:       ccSvc,
		GetCCSettingsSvc:          ccSvc,
		UpdateClosingDaySvc:       ccSvc,
		UpdateDueDaySvc:           ccSvc,
		UpdateCreditLimitSvc:      ccSvc,
		DeleteCCSettingsSvc:       ccSvc,
		PostTransactionSvc:        ledgerSvc,
		GetBalanceSvc:             ledgerSvc,
		GetTransactionHistorySvc:  ledgerSvc,
	}
}

// TestRegisterRoutes_EndpointsReachable verifies AC1: all domain endpoints are
// registered under /api/v1/ with appropriate HTTP methods.
func TestRegisterRoutes_EndpointsReachable(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, buildTestDeps())

	hid := uuid.New().String()
	aid := uuid.New().String()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int // just check it's NOT 404/405
	}{
		// Health
		{"healthz", http.MethodGet, "/healthz", http.StatusOK},
		{"liveness", http.MethodGet, "/health/live", http.StatusOK},
		{"readiness", http.MethodGet, "/health/ready", http.StatusOK},

		// Auth (public)
		{"login", http.MethodPost, "/auth/login", http.StatusBadRequest}, // no body → 400, but route matched

		// Users (authn required → 401 without token)
		{"get user", http.MethodGet, "/api/v1/users/" + uuid.New().String(), http.StatusUnauthorized},
		{"update profile", http.MethodPut, "/api/v1/users/" + uuid.New().String(), http.StatusUnauthorized},
		{"change password", http.MethodPut, "/api/v1/users/" + uuid.New().String() + "/password", http.StatusUnauthorized},
		{"deactivate user", http.MethodDelete, "/api/v1/users/" + uuid.New().String(), http.StatusUnauthorized},

		// Households (authn required)
		{"create household", http.MethodPost, "/api/v1/households", http.StatusUnauthorized},
		{"list households", http.MethodGet, "/api/v1/households", http.StatusUnauthorized},

		// Household-scoped user creation (authn + admin guard)
		{"create household user", http.MethodPost, "/api/v1/households/" + hid + "/users", http.StatusUnauthorized},

		// Household-scoped (authn + guard)
		{"get household", http.MethodGet, "/api/v1/households/" + hid, http.StatusUnauthorized},
		{"update household name", http.MethodPut, "/api/v1/households/" + hid, http.StatusUnauthorized},
		{"deactivate household", http.MethodDelete, "/api/v1/households/" + hid, http.StatusUnauthorized},
		{"add member", http.MethodPost, "/api/v1/households/" + hid + "/members", http.StatusUnauthorized},
		{"remove member", http.MethodDelete, "/api/v1/households/" + hid + "/members/" + uuid.New().String(), http.StatusUnauthorized},

		// Accounts
		{"create account", http.MethodPost, "/api/v1/households/" + hid + "/accounts", http.StatusUnauthorized},
		{"list accounts", http.MethodGet, "/api/v1/households/" + hid + "/accounts", http.StatusUnauthorized},
		{"get account", http.MethodGet, "/api/v1/households/" + hid + "/accounts/" + aid, http.StatusUnauthorized},
		{"update account name", http.MethodPut, "/api/v1/households/" + hid + "/accounts/" + aid + "/name", http.StatusUnauthorized},
		{"update account balance", http.MethodPut, "/api/v1/households/" + hid + "/accounts/" + aid + "/balance", http.StatusUnauthorized},
		{"deactivate account", http.MethodDelete, "/api/v1/households/" + hid + "/accounts/" + aid, http.StatusUnauthorized},

		// Credit card settings
		{"create cc settings", http.MethodPost, "/api/v1/households/" + hid + "/accounts/" + aid + "/credit-card-settings", http.StatusUnauthorized},
		{"get cc settings", http.MethodGet, "/api/v1/households/" + hid + "/accounts/" + aid + "/credit-card-settings", http.StatusUnauthorized},
		{"update closing day", http.MethodPut, "/api/v1/households/" + hid + "/accounts/" + aid + "/credit-card-settings/closing-day", http.StatusUnauthorized},
		{"update due day", http.MethodPut, "/api/v1/households/" + hid + "/accounts/" + aid + "/credit-card-settings/due-day", http.StatusUnauthorized},
		{"update credit limit", http.MethodPut, "/api/v1/households/" + hid + "/accounts/" + aid + "/credit-card-settings/limit", http.StatusUnauthorized},
		{"delete cc settings", http.MethodDelete, "/api/v1/households/" + hid + "/accounts/" + aid + "/credit-card-settings", http.StatusUnauthorized},

		// Ledger
		{"post transaction", http.MethodPost, "/api/v1/households/" + hid + "/transactions", http.StatusUnauthorized},
		{"get history", http.MethodGet, "/api/v1/households/" + hid + "/transactions", http.StatusUnauthorized},
		{"get balance", http.MethodGet, "/api/v1/households/" + hid + "/accounts/" + aid + "/balance", http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			// The route is reachable — NOT a 404 (unmatched route) or 405 (wrong method)
			assert.Equal(t, tc.wantStatus, w.Code,
				"route %s %s returned unexpected status", tc.method, tc.path)
		})
	}
}

// TestRegisterRoutes_UnauthenticatedProtectedEndpoint_Returns401 verifies AC2:
// unauthenticated requests to protected endpoints get 401.
func TestRegisterRoutes_UnauthenticatedProtectedEndpoint_Returns401(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, buildTestDeps())

	r := httptest.NewRequest(http.MethodGet, "/api/v1/households", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRegisterRoutes_NonExistentRoute_Returns404 verifies edge case.
func TestRegisterRoutes_NonExistentRoute_Returns404(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, buildTestDeps())

	r := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestRegisterRoutes_WrongMethod_Returns405 verifies AC8.
func TestRegisterRoutes_WrongMethod_Returns405(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, buildTestDeps())

	// GET on a POST-only endpoint
	r := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// placeholder to use time import (used in stubs)
var _ = time.Now
