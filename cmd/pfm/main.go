package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"

	pfmhttp "github.com/zambone/pfm-go/internal/adapter/http"
	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	pgadapter "github.com/zambone/pfm-go/internal/adapter/postgres"
	domainaccount "github.com/zambone/pfm-go/internal/domain/account"
	domaincreditcard "github.com/zambone/pfm-go/internal/domain/creditcard"
	domainhousehold "github.com/zambone/pfm-go/internal/domain/household"
	domainledger "github.com/zambone/pfm-go/internal/domain/ledger"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/config"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/platform/observe"
	pfmdb "github.com/zambone/pfm-go/db"
)

// Version information injected at build via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// @title PFM-Go API
// @version 1.0
// @description Personal Financial Manager REST API — double-entry accounting for households.
// @basePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer {token}" (obtained from POST /auth/login).
func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf(message.ErrRunLoadConfig, err)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return fmt.Errorf(message.ErrRunLogLevel, cfg.LogLevel, err)
	}
	logger := observe.NewLogger(logLevel, nil)

	slog.SetDefault(logger)
	slog.InfoContext(ctx, message.MsgLoggerReady)
	slog.InfoContext(ctx, message.MsgStartupInfo,
		"version", Version,
		"build_time", BuildTime,
		"git_commit", GitCommit,
		"config", cfg.String(),
		"log_level", logLevel.String(),
	)

	tp, tracerShutdown, err := observe.NewTracerProvider(ctx, cfg, "pfm-go", Version)
	if err != nil {
		return fmt.Errorf(message.ErrRunTracerInit, err)
	}
	_ = tp
	slog.InfoContext(ctx, message.MsgTracerReady)

	db, err := database.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf(message.ErrRunOpenDB, err)
	}

	migrationsFS, err := fs.Sub(pfmdb.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf(message.ErrMigrateSubFS, err)
	}

	if err := database.Migrate(ctx, db, migrationsFS); err != nil {
		return fmt.Errorf(message.ErrRunMigrate, err)
	}

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf(message.ErrDBNewPool, err)
	}

	// --- Dependency wiring ---
	realClock := clock.NewRealClock()
	hasher := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	tokenSvc, err := authadapter.NewPasetoTokenService(cfg.TokenSecretKey, realClock)
	if err != nil {
		return fmt.Errorf(message.ErrRunTokenKey, err)
	}
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
		time.Duration(cfg.TokenExpirationSec)*time.Second,
	)
	householdLogic := domainhousehold.NewHouseholdLogic(householdRepo, transactor, realClock)
	householdMemberLogic := domainhousehold.NewHouseholdMemberLogic(
		householdRepo,
		newHouseholdUserCreator(userLogic),
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

	// Thin adapters for middleware.
	membershipFinder := pfmhttp.NewMembershipRoleFinder(householdRepo)

	var shuttingDown atomic.Bool

	mux := http.NewServeMux()
	pfmhttp.RegisterRoutes(mux, pfmhttp.RouteDeps{
		Version:        Version,
		ShuttingDown:   &shuttingDown,
		TokenValidator: tokenSvc,
		MembershipFinder: membershipFinder,

		// User
		LoginSvc:          loginLogic,
		GetUserSvc:        userLogic,
		UpdateProfileSvc:  userLogic,
		ChangePasswordSvc: userLogic,
		DeactivateUserSvc: userLogic,

		// Household
		CreateHouseholdSvc:     householdLogic,
		CreateHouseholdUserSvc: householdMemberLogic,
		GetHouseholdSvc:        householdLogic,
		ListHouseholdsSvc:      householdLogic,
		AddMemberSvc:           householdLogic,
		RemoveMemberSvc:        householdLogic,
		UpdateHouseholdNameSvc: householdLogic,
		DeactivateHouseholdSvc: householdLogic,

		// Account
		CreateAccountSvc:        accountLogic,
		GetAccountSvc:           accountLogic,
		ListAccountsSvc:         accountLogic,
		UpdateAccountNameSvc:    accountLogic,
		UpdateAccountBalanceSvc: accountLogic,
		DeactivateAccountSvc:    accountLogic,

		// Credit Card Settings
		CreateCCSettingsSvc:  ccSettingsLogic,
		GetCCSettingsSvc:     ccSettingsLogic,
		UpdateClosingDaySvc:  ccSettingsLogic,
		UpdateDueDaySvc:      ccSettingsLogic,
		UpdateCreditLimitSvc: ccSettingsLogic,
		DeleteCCSettingsSvc:  ccSettingsLogic,

		// Ledger
		PostTransactionSvc:       ledgerLogic,
		GetBalanceSvc:            ledgerLogic,
		GetTransactionHistorySvc: ledgerLogic,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, message.MsgServerError, "error", err)
		}
	}()
	slog.InfoContext(ctx, message.MsgServerStarting, "port", cfg.HTTPPort)

	<-ctx.Done()
	shuttingDown.Store(true)
	slog.InfoContext(context.Background(), message.MsgShuttingDown)

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.ShutdownTimeoutSec)*time.Second,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, message.MsgServerShutdownError, "error", err)
	}

	if err := db.Close(); err != nil {
		slog.ErrorContext(shutdownCtx, message.MsgDBCloseError, "error", err)
	}
	slog.InfoContext(context.Background(), message.MsgDBClosed)

	slog.InfoContext(context.Background(), message.MsgServerStopped)

	if err := tracerShutdown(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, message.MsgTracerShutdownError, "error", err)
	}

	return nil
}

// householdUserCreator adapts UserLogic to the household.userCreator interface.
// It lives here — in the composition root — because it bridges two domain packages
// that must not import each other.
type householdUserCreator struct {
	logic *domainuser.UserLogic
}

// newHouseholdUserCreator returns a householdUserCreator wrapping the given UserLogic.
func newHouseholdUserCreator(logic *domainuser.UserLogic) *householdUserCreator {
	return &householdUserCreator{logic: logic}
}

// Create implements the household.userCreator interface. It delegates to UserLogic.Register,
// which handles validation and password hashing, then maps the result to the household
// domain's minimal CreatedUser type.
func (a *householdUserCreator) Create(ctx context.Context, input domainhousehold.NewUserInput, callerID uuid.UUID) (domainhousehold.CreatedUser, error) {
	u, err := a.logic.Register(ctx, domainuser.RegisterInput{
		Email:       input.Email,
		DisplayName: input.DisplayName,
		Password:    input.Password,
	}, callerID)
	if err != nil {
		return domainhousehold.CreatedUser{}, err
	}
	return domainhousehold.CreatedUser{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
	}, nil
}
