package http

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/google/uuid"

	domainacct "github.com/zambone/pfm-go/internal/domain/account"
	domainhouse "github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/middleware"
	"github.com/zambone/pfm-go/internal/types"
)

// tokenValidator abstracts token validation for the authn middleware.
// Structurally satisfied by adapter/auth.PasetoTokenService.
type tokenValidator interface {
	Validate(ctx context.Context, token string) (uuid.UUID, error)
}

// roleFinder abstracts membership role lookup for the authz middleware.
// Structurally satisfied by MembershipRoleFinder.
type roleFinder interface {
	FindRole(ctx context.Context, householdID, userID uuid.UUID) (types.Role, error)
}

// RouteDeps carries all dependencies needed to register HTTP routes.
// Each field corresponds to a narrow service interface consumed by a single handler.
type RouteDeps struct {
	Version      string
	ShuttingDown *atomic.Bool

	// Auth
	TokenValidator tokenValidator
	MembershipFinder roleFinder

	// User handlers
	LoginSvc          loginService
	GetUserSvc        getUserService
	UpdateProfileSvc  updateProfileService
	ChangePasswordSvc changePasswordService
	DeactivateUserSvc deactivateUserService

	// Household handlers
	CreateHouseholdSvc      createHouseholdService
	CreateHouseholdUserSvc  createHouseholdUserService
	GetHouseholdSvc        getHouseholdService
	ListHouseholdsSvc      listHouseholdsService
	AddMemberSvc           addMemberService
	RemoveMemberSvc        removeMemberService
	UpdateHouseholdNameSvc updateHouseholdNameService
	DeactivateHouseholdSvc deactivateHouseholdService

	// Account handlers
	CreateAccountSvc        createAccountService
	GetAccountSvc           getAccountService
	ListAccountsSvc         listAccountsService
	UpdateAccountNameSvc    updateAccountNameService
	UpdateAccountBalanceSvc updateAccountBalanceService
	DeactivateAccountSvc    deactivateAccountService

	// Credit card settings handlers
	CreateCCSettingsSvc createCCSettingsService
	GetCCSettingsSvc    getCCSettingsService
	UpdateClosingDaySvc updateClosingDayService
	UpdateDueDaySvc     updateDueDayService
	UpdateCreditLimitSvc updateCreditLimitService
	DeleteCCSettingsSvc  deleteCCSettingsService

	// Ledger handlers
	PostTransactionSvc       postTransactionService
	GetBalanceSvc            getBalanceService
	GetTransactionHistorySvc getTransactionHistoryService
}

// RegisterRoutes registers all HTTP endpoints on the given mux with the appropriate
// middleware chains. Routes follow RESTful conventions under /api/v1/ with household
// scoping for resource endpoints.
func RegisterRoutes(mux *http.ServeMux, d RouteDeps) {
	authn := middleware.Authn(d.TokenValidator)
	guard := middleware.HouseholdGuard(d.MembershipFinder, "household_id")
	adminGuard := middleware.HouseholdAdminGuard(d.MembershipFinder, "household_id")

	// --- Health (public) ---
	mux.Handle("GET /healthz", HealthHandler(d.Version))
	mux.Handle("GET /health/live", LiveHandler())
	mux.Handle("GET /health/ready", ReadyHandler(d.ShuttingDown))

	// --- Docs (public) ---
	mux.Handle("GET /docs", DocsHandler())
	mux.Handle("GET /api/v1/openapi.yaml", OpenAPIHandler())

	// --- Auth (public) ---
	mux.Handle("POST /auth/login", LoginHandler(d.LoginSvc))

	// --- Users (all require authentication) ---
	mux.Handle("GET /api/v1/users/{id}", authn(GetUserHandler(d.GetUserSvc)))
	mux.Handle("PUT /api/v1/users/{id}", authn(UpdateProfileHandler(d.UpdateProfileSvc)))
	mux.Handle("PUT /api/v1/users/{id}/password", authn(ChangePasswordHandler(d.ChangePasswordSvc)))
	mux.Handle("DELETE /api/v1/users/{id}", authn(DeactivateUserHandler(d.DeactivateUserSvc)))

	// --- Households ---
	// Create and list require authentication but no household guard (user doesn't belong yet).
	mux.Handle("POST /api/v1/households", authn(CreateHouseholdHandler(d.CreateHouseholdSvc)))
	mux.Handle("GET /api/v1/households", authn(ListHouseholdsHandler(d.ListHouseholdsSvc)))
	// Household-scoped: require membership via guard.
	mux.Handle("GET /api/v1/households/{household_id}", authn(guard(GetHouseholdHandler(d.GetHouseholdSvc))))
	mux.Handle("PUT /api/v1/households/{household_id}", authn(adminGuard(UpdateHouseholdNameHandler(d.UpdateHouseholdNameSvc))))
	mux.Handle("DELETE /api/v1/households/{household_id}", authn(adminGuard(DeactivateHouseholdHandler(d.DeactivateHouseholdSvc))))

	// --- Members and household-scoped user creation ---
	// Creating a user within a household requires auth + admin role.
	mux.Handle("POST /api/v1/households/{household_id}/users", authn(adminGuard(CreateHouseholdUserHandler(d.CreateHouseholdUserSvc))))
	mux.Handle("POST /api/v1/households/{household_id}/members", authn(adminGuard(AddMemberHandler(d.AddMemberSvc))))
	mux.Handle("DELETE /api/v1/households/{household_id}/members/{user_id}", authn(adminGuard(RemoveMemberHandler(d.RemoveMemberSvc))))

	// --- Accounts ---
	mux.Handle("POST /api/v1/households/{household_id}/accounts", authn(guard(CreateAccountHandler(d.CreateAccountSvc))))
	mux.Handle("GET /api/v1/households/{household_id}/accounts", authn(guard(ListAccountsHandler(d.ListAccountsSvc))))
	mux.Handle("GET /api/v1/households/{household_id}/accounts/{id}", authn(guard(GetAccountHandler(d.GetAccountSvc))))
	mux.Handle("PUT /api/v1/households/{household_id}/accounts/{id}/name", authn(guard(UpdateAccountNameHandler(d.UpdateAccountNameSvc))))
	mux.Handle("PUT /api/v1/households/{household_id}/accounts/{id}/balance", authn(guard(UpdateAccountBalanceHandler(d.UpdateAccountBalanceSvc))))
	mux.Handle("DELETE /api/v1/households/{household_id}/accounts/{id}", authn(guard(DeactivateAccountHandler(d.DeactivateAccountSvc))))

	// --- Credit Card Settings ---
	mux.Handle("POST /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings", authn(guard(CreateCreditCardSettingsHandler(d.CreateCCSettingsSvc))))
	mux.Handle("GET /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings", authn(guard(GetCreditCardSettingsHandler(d.GetCCSettingsSvc))))
	mux.Handle("PUT /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/closing-day", authn(guard(UpdateClosingDayHandler(d.UpdateClosingDaySvc))))
	mux.Handle("PUT /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/due-day", authn(guard(UpdateDueDayHandler(d.UpdateDueDaySvc))))
	mux.Handle("PUT /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings/limit", authn(guard(UpdateCreditLimitHandler(d.UpdateCreditLimitSvc))))
	mux.Handle("DELETE /api/v1/households/{household_id}/accounts/{account_id}/credit-card-settings", authn(guard(DeleteCreditCardSettingsHandler(d.DeleteCCSettingsSvc))))

	// --- Ledger ---
	mux.Handle("POST /api/v1/households/{household_id}/transactions", authn(guard(PostTransactionHandler(d.PostTransactionSvc))))
	mux.Handle("GET /api/v1/households/{household_id}/transactions", authn(guard(GetTransactionHistoryHandler(d.GetTransactionHistorySvc))))
	mux.Handle("GET /api/v1/households/{household_id}/accounts/{account_id}/balance", authn(guard(GetBalanceHandler(d.GetBalanceSvc))))
}

// MembershipRoleFinder adapts a household membership lookup to the narrow FindRole
// interface required by the authz middleware. It wraps FindMembership and extracts the role.
type MembershipRoleFinder struct {
	finder membershipLookup
}

// membershipLookup is the subset of HouseholdRepo required by MembershipRoleFinder.
type membershipLookup interface {
	FindMembership(ctx context.Context, householdID, userID uuid.UUID) (domainhouse.Membership, error)
}

// NewMembershipRoleFinder creates a MembershipRoleFinder. Panics if finder is nil.
func NewMembershipRoleFinder(finder membershipLookup) *MembershipRoleFinder {
	if finder == nil {
		panic("http: NewMembershipRoleFinder requires non-nil membershipLookup")
	}
	return &MembershipRoleFinder{finder: finder}
}

// FindRole returns the role of a user within a household.
func (m *MembershipRoleFinder) FindRole(ctx context.Context, householdID, userID uuid.UUID) (types.Role, error) {
	membership, err := m.finder.FindMembership(ctx, householdID, userID)
	if err != nil {
		return "", err
	}
	return membership.Role, nil
}

// AccountTypeFinder adapts an account lookup to the narrow FindAccountType
// interface required by the creditcard domain. It wraps FindByID and extracts the type.
type AccountTypeFinder struct {
	finder accountLookup
}

// accountLookup is the subset of AccountRepo required by AccountTypeFinder.
type accountLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (domainacct.Account, error)
}

// NewAccountTypeFinder creates an AccountTypeFinder. Panics if finder is nil.
func NewAccountTypeFinder(finder accountLookup) *AccountTypeFinder {
	if finder == nil {
		panic("http: NewAccountTypeFinder requires non-nil accountLookup")
	}
	return &AccountTypeFinder{finder: finder}
}

// FindAccountType returns the account type for the given account ID.
func (a *AccountTypeFinder) FindAccountType(ctx context.Context, accountID uuid.UUID) (types.AccountType, error) {
	acct, err := a.finder.FindByID(ctx, accountID)
	if err != nil {
		return "", err
	}
	return acct.AccountType, nil
}
