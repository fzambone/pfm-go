//go:build integration

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	pgadapter "github.com/zambone/pfm-go/internal/adapter/postgres"
	domainaccount "github.com/zambone/pfm-go/internal/domain/account"
	domaincreditcard "github.com/zambone/pfm-go/internal/domain/creditcard"
	domainhousehold "github.com/zambone/pfm-go/internal/domain/household"
	domainledger "github.com/zambone/pfm-go/internal/domain/ledger"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// e2eEnv holds the fully wired test environment for E2E tests.
type e2eEnv struct {
	mux    *http.ServeMux
	pool   *pgxpool.Pool // direct DB access for bootstrapAdmin
	hasher *authadapter.Argon2idHasher
}

// newE2EEnv spins up a Postgres testcontainer, runs migrations, and wires the
// full application stack — repos, domain logic, middleware, and HTTP router.
func newE2EEnv(t *testing.T, ctx context.Context) *e2eEnv {
	t.Helper()

	pool := sharedDB.NewPool(t, ctx)
	realClock := clock.NewRealClock()
	hasher := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	// 32-byte hex-encoded key for PASETO token service (decodes to 32 bytes).
	tokenKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tokenSvc, err := authadapter.NewPasetoTokenService(tokenKey, realClock)
	require.NoError(t, err)
	transactor := database.NewPostgresTransactor(pool)

	// Repositories.
	userRepo := pgadapter.NewUserRepo(pool)
	householdRepo := pgadapter.NewHouseholdRepo(pool)
	accountRepo := pgadapter.NewAccountRepo(pool)
	ccSettingsRepo := pgadapter.NewCreditCardSettingsRepo(pool)
	ledgerRepo := pgadapter.NewLedgerRepo(pool)

	// Domain logic.
	userLogic := domainuser.NewUserLogic(userRepo, hasher, realClock)
	loginLogic := domainuser.NewLoginLogic(
		userRepo, hasher, tokenSvc, realClock,
		15*time.Minute,
	)
	householdLogic := domainhousehold.NewHouseholdLogic(householdRepo, transactor, realClock)
	householdMemberLogic := domainhousehold.NewHouseholdMemberLogic(
		householdRepo,
		newE2EHouseholdUserCreator(userLogic),
		transactor,
		realClock,
	)
	accountLogic := domainaccount.NewAccountLogic(accountRepo, realClock)
	ccSettingsLogic := domaincreditcard.NewSettingsLogic(
		ccSettingsRepo,
		pfmhttp.NewAccountTypeFinder(accountRepo),
		realClock,
	)
	ledgerLogic := domainledger.NewLedgerLogic(ledgerRepo, transactor, realClock)

	membershipFinder := pfmhttp.NewMembershipRoleFinder(householdRepo)

	var shuttingDown atomic.Bool
	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, pfmhttp.RouteDeps{
		Version:      "test",
		ShuttingDown: &shuttingDown,

		TokenValidator:   tokenSvc,
		MembershipFinder: membershipFinder,

		LoginSvc:          loginLogic,
		GetUserSvc:        userLogic,
		UpdateProfileSvc:  userLogic,
		ChangePasswordSvc: userLogic,
		DeactivateUserSvc: userLogic,

		CreateHouseholdSvc:     householdLogic,
		CreateHouseholdUserSvc: householdMemberLogic,
		GetHouseholdSvc:        householdLogic,
		ListHouseholdsSvc:      householdLogic,
		AddMemberSvc:           householdLogic,
		RemoveMemberSvc:        householdLogic,
		UpdateHouseholdNameSvc: householdLogic,
		DeactivateHouseholdSvc: householdLogic,

		CreateAccountSvc:        accountLogic,
		GetAccountSvc:           accountLogic,
		ListAccountsSvc:         accountLogic,
		UpdateAccountNameSvc:    accountLogic,
		UpdateAccountBalanceSvc: accountLogic,
		DeactivateAccountSvc:    accountLogic,

		CreateCCSettingsSvc:  ccSettingsLogic,
		GetCCSettingsSvc:     ccSettingsLogic,
		UpdateClosingDaySvc:  ccSettingsLogic,
		UpdateDueDaySvc:      ccSettingsLogic,
		UpdateCreditLimitSvc: ccSettingsLogic,
		DeleteCCSettingsSvc:  ccSettingsLogic,

		PostTransactionSvc:       ledgerLogic,
		GetBalanceSvc:            ledgerLogic,
		GetTransactionHistorySvc: ledgerLogic,
	})

	return &e2eEnv{mux: mux, pool: pool, hasher: hasher}
}

// e2eHouseholdUserCreator adapts UserLogic to the household.userCreator interface
// for E2E tests. This mirrors the production adapter in cmd/pfm/main.go.
type e2eHouseholdUserCreator struct {
	logic *domainuser.UserLogic
}

func newE2EHouseholdUserCreator(logic *domainuser.UserLogic) *e2eHouseholdUserCreator {
	return &e2eHouseholdUserCreator{logic: logic}
}

func (a *e2eHouseholdUserCreator) Create(ctx context.Context, input domainhousehold.NewUserInput, callerID uuid.UUID) (domainhousehold.CreatedUser, error) {
	u, err := a.logic.Register(ctx, domainuser.RegisterInput{
		Email:       input.Email,
		DisplayName: input.DisplayName,
		Password:    input.Password,
	}, callerID)
	if err != nil {
		return domainhousehold.CreatedUser{}, err
	}
	return domainhousehold.CreatedUser{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName}, nil
}

// bootstrapAdmin inserts the first user + household + membership directly via SQL
// (bypassing the HTTP layer, which requires auth for user creation), then logs in
// via HTTP to obtain a real PASETO token.
//
// This is the only way to create the "seed" admin user for E2E tests, mirroring
// what the cmd/pfm-seed binary will do in production (#182).
func (e *e2eEnv) bootstrapAdmin(t *testing.T, ctx context.Context, email, displayName, password string) (token, userID, householdID string) {
	t.Helper()

	// Hash the password using the same hasher as the application.
	hash, err := e.hasher.Hash(ctx, password)
	require.NoError(t, err, "bootstrap: hash password")

	// Insert admin user directly into the DB.
	var uid string
	err = e.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, created_by, updated_by)
		VALUES ($1, $2, $3, gen_random_uuid(), gen_random_uuid())
		RETURNING id::text
	`, email, displayName, hash).Scan(&uid)
	require.NoError(t, err, "bootstrap: insert user")

	// Insert household.
	var hid string
	err = e.pool.QueryRow(ctx, `
		INSERT INTO households (name, created_by, updated_by)
		VALUES ('Bootstrap Household', $1::uuid, $1::uuid)
		RETURNING id::text
	`, uid).Scan(&hid)
	require.NoError(t, err, "bootstrap: insert household")

	// Insert admin membership.
	_, err = e.pool.Exec(ctx, `
		INSERT INTO household_members (household_id, user_id, role, invited_by)
		VALUES ($1::uuid, $2::uuid, 'ADMIN', $2::uuid)
	`, hid, uid)
	require.NoError(t, err, "bootstrap: insert membership")

	// Login via HTTP to get a real PASETO token.
	w := e.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": email, "password": password,
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "bootstrap: login failed: %s", w.Body.String())
	login := decodeJSON(t, w)
	token = login["token"].(string)
	require.NotEmpty(t, token, "bootstrap: empty token")

	return token, uid, hid
}

// do sends an HTTP request to the mux and returns the recorder.
func (e *e2eEnv) do(t *testing.T, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	r := httptest.NewRequest(method, path, reqBody)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

// decodeJSON decodes a JSON response into a map.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&m))
	return m
}

// TestE2E_FullWorkflow verifies AC5 and AC6: a full bootstrap → login → create
// household user → create account → post transaction workflow against a real database.
func TestE2E_FullWorkflow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// Step 1: Bootstrap an admin user (direct DB insert + login).
	adminToken, _, householdID := env.bootstrapAdmin(t, ctx, "alice@example.com", "Alice", "secret1234")

	// Step 2: Create a second user via the new household-scoped endpoint.
	w := env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/users", map[string]string{
		"email":        "bob@example.com",
		"display_name": "Bob",
		"password":     "secret1234",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, "create household user failed: %s", w.Body.String())
	newUser := decodeJSON(t, w)
	assert.NotEmpty(t, newUser["id"])

	// Step 3: Create a checking account.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Checking",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, "create account failed: %s", w.Body.String())
	account1 := decodeJSON(t, w)
	account1ID := account1["id"].(string)
	assert.Equal(t, "Checking", account1["name"])

	// Step 4: Create a savings account (for balanced transaction).
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Savings",
		"account_type":  "SAVINGS",
		"currency_code": "BRL",
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, "create savings failed: %s", w.Body.String())
	account2 := decodeJSON(t, w)
	account2ID := account2["id"].(string)

	// Step 5: Post a balanced transaction.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "Transfer to savings",
		"transaction_date": "2026-03-15",
		"entries": []map[string]any{
			{"account_id": account1ID, "entry_type": "DEBIT", "amount": 10000},
			{"account_id": account2ID, "entry_type": "CREDIT", "amount": 10000},
		},
	}, adminToken)
	require.Equal(t, http.StatusCreated, w.Code, "post transaction failed: %s", w.Body.String())
	txn := decodeJSON(t, w)
	assert.Equal(t, "Transfer to savings", txn["description"])
	entries, ok := txn["entries"].([]any)
	require.True(t, ok)
	assert.Len(t, entries, 2)

	// Step 6: Verify account balances.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+account1ID+"/balance", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code)
	bal1 := decodeJSON(t, w)
	assert.Equal(t, float64(-10000), bal1["balance"])

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+account2ID+"/balance", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code)
	bal2 := decodeJSON(t, w)
	assert.Equal(t, float64(10000), bal2["balance"])

	// Step 7: Get transaction history.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/transactions", nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&history))
	assert.Len(t, history, 1)
}

// TestE2E_UnauthenticatedRequest_Returns401 verifies AC2 against the full stack.
func TestE2E_UnauthenticatedRequest_Returns401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	w := env.do(t, http.MethodGet, "/api/v1/households", nil, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_NonMemberAccess_Returns403 verifies AC3: a user who is not a member
// of the household gets 403 Forbidden.
func TestE2E_NonMemberAccess_Returns403(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// Bootstrap admin user A with their own household.
	tokenA, _, householdID := env.bootstrapAdmin(t, ctx, fmt.Sprintf("a-%d@example.com", time.Now().UnixNano()), "User A", "secret1234")

	// Bootstrap a second independent admin user B (different household).
	tokenB, _, _ := env.bootstrapAdmin(t, ctx, fmt.Sprintf("b-%d@example.com", time.Now().UnixNano()), "User B", "secret1234")

	// User B tries to access A's household — 403.
	w := env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)

	_ = tokenA
}
