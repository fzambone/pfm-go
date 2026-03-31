//go:build integration

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	pfmdb "github.com/zambone/pfm-go/db"
	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	pgadapter "github.com/zambone/pfm-go/internal/adapter/postgres"
	domainaccount "github.com/zambone/pfm-go/internal/domain/account"
	domaincreditcard "github.com/zambone/pfm-go/internal/domain/creditcard"
	domainhousehold "github.com/zambone/pfm-go/internal/domain/household"
	domainledger "github.com/zambone/pfm-go/internal/domain/ledger"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/database"
)

// e2eEnv holds the fully wired test environment for E2E tests.
type e2eEnv struct {
	mux *http.ServeMux
}

// newE2EEnv spins up a Postgres testcontainer, runs migrations, and wires the
// full application stack — repos, domain logic, middleware, and HTTP router.
func newE2EEnv(t *testing.T, ctx context.Context) *e2eEnv {
	t.Helper()

	pool := newE2EPool(t, ctx)
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
		RegisterSvc:       userLogic,
		GetUserSvc:        userLogic,
		UpdateProfileSvc:  userLogic,
		ChangePasswordSvc: userLogic,
		DeactivateUserSvc: userLogic,

		CreateHouseholdSvc:     householdLogic,
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

	return &e2eEnv{mux: mux}
}

// newE2EPool creates a testcontainers PostgreSQL instance with migrations applied.
func newE2EPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18-alpine3.23",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseHost:           host,
		DatabasePort:           mappedPort.Int(),
		DatabaseName:           "testdb",
		DatabaseUser:           "testuser",
		DatabasePassword:       "testpass",
		DatabaseSSLMode:        "disable",
		DBConnectTimeoutSec:    5,
		DBStartupRetries:       3,
		DBStartupRetryDelaySec: 1,
		DBMaxOpenConns:         5,
		DBMaxIdleConns:         2,
		DBConnMaxLifetimeSec:   60,
		DBConnMaxIdleTimeSec:   30,
	}

	sqlDB, err := database.Open(ctx, cfg)
	require.NoError(t, err)
	sub, err := fs.Sub(pfmdb.Migrations, "migrations")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(ctx, sqlDB, sub))
	require.NoError(t, sqlDB.Close())

	pool, err := database.NewPool(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
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

// TestE2E_FullWorkflow verifies AC5 and AC6: a full register → login → create
// household → create account → post transaction workflow against a real database.
func TestE2E_FullWorkflow(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// Step 1: Register a user.
	w := env.do(t, http.MethodPost, "/api/v1/users", map[string]string{
		"email":        "alice@example.com",
		"display_name": "Alice",
		"password":     "secret1234",
	}, "")
	require.Equal(t, http.StatusCreated, w.Code, "register failed: %s", w.Body.String())
	user := decodeJSON(t, w)
	userID := user["id"].(string)
	assert.NotEmpty(t, userID)

	// Step 2: Login.
	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email":    "alice@example.com",
		"password": "secret1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login failed: %s", w.Body.String())
	loginResp := decodeJSON(t, w)
	token := loginResp["token"].(string)
	assert.NotEmpty(t, token)

	// Step 3: Create a household.
	w = env.do(t, http.MethodPost, "/api/v1/households", map[string]string{
		"name": "Alice's Home",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create household failed: %s", w.Body.String())
	household := decodeJSON(t, w)
	householdID := household["id"].(string)
	assert.Equal(t, "Alice's Home", household["name"])

	// Step 4: Create a checking account.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Checking",
		"account_type":  "CHECKING",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create account failed: %s", w.Body.String())
	account1 := decodeJSON(t, w)
	account1ID := account1["id"].(string)
	assert.Equal(t, "Checking", account1["name"])

	// Step 5: Create a savings account (for balanced transaction).
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/accounts", map[string]string{
		"name":          "Savings",
		"account_type":  "SAVINGS",
		"currency_code": "BRL",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "create savings failed: %s", w.Body.String())
	account2 := decodeJSON(t, w)
	account2ID := account2["id"].(string)

	// Step 6: Post a balanced transaction.
	w = env.do(t, http.MethodPost, "/api/v1/households/"+householdID+"/transactions", map[string]any{
		"description":      "Transfer to savings",
		"transaction_date": "2026-03-15",
		"entries": []map[string]any{
			{"account_id": account1ID, "entry_type": "DEBIT", "amount": 10000},
			{"account_id": account2ID, "entry_type": "CREDIT", "amount": 10000},
		},
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, "post transaction failed: %s", w.Body.String())
	txn := decodeJSON(t, w)
	assert.Equal(t, "Transfer to savings", txn["description"])
	entries, ok := txn["entries"].([]any)
	require.True(t, ok)
	assert.Len(t, entries, 2)

	// Step 7: Verify account balances.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+account1ID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	bal1 := decodeJSON(t, w)
	assert.Equal(t, float64(-10000), bal1["balance"]) // debit reduces balance

	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/accounts/"+account2ID+"/balance", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	bal2 := decodeJSON(t, w)
	assert.Equal(t, float64(10000), bal2["balance"]) // credit increases balance

	// Step 8: Get transaction history.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID+"/transactions", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&history))
	assert.Len(t, history, 1)
}

// TestE2E_UnauthenticatedRequest_Returns401 verifies AC2 against the full stack.
func TestE2E_UnauthenticatedRequest_Returns401(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	w := env.do(t, http.MethodGet, "/api/v1/households", nil, "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestE2E_NonMemberAccess_Returns403 verifies AC3: a user who is not a member
// of the household gets 403 Forbidden.
func TestE2E_NonMemberAccess_Returns403(t *testing.T) {
	ctx := context.Background()
	env := newE2EEnv(t, ctx)

	// Register user A and get token.
	w := env.do(t, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "a@example.com", "display_name": "Alice", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusCreated, w.Code, "register A: %s", w.Body.String())

	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "a@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login A: %s", w.Body.String())
	loginA := decodeJSON(t, w)
	tokenA, ok := loginA["token"].(string)
	require.True(t, ok, "login A response missing token: %v", loginA)

	// User A creates a household.
	w = env.do(t, http.MethodPost, "/api/v1/households", map[string]string{
		"name": "A's House",
	}, tokenA)
	require.Equal(t, http.StatusCreated, w.Code, "create household: %s", w.Body.String())
	houseResp := decodeJSON(t, w)
	householdID, ok := houseResp["id"].(string)
	require.True(t, ok, "household response missing id: %v", houseResp)

	// Register user B and get token.
	w = env.do(t, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "b@example.com", "display_name": "Bob", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusCreated, w.Code, "register B: %s", w.Body.String())

	w = env.do(t, http.MethodPost, "/auth/login", map[string]string{
		"email": "b@example.com", "password": "secret1234",
	}, "")
	require.Equal(t, http.StatusOK, w.Code, "login B: %s", w.Body.String())
	loginB := decodeJSON(t, w)
	tokenB, ok := loginB["token"].(string)
	require.True(t, ok, "login B response missing token: %v", loginB)

	// User B tries to access A's household → 403.
	w = env.do(t, http.MethodGet, "/api/v1/households/"+householdID, nil, tokenB)
	require.Equal(t, http.StatusForbidden, w.Code)
}
