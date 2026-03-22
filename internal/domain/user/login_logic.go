package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zambone/pfm-go/internal/message"
)

// LoginLogic orchestrates credential validation and token issuance for user login.
type LoginLogic struct {
	repo     Repository
	hasher   passwordVerifier
	tokens   tokenIssuer
	clk      clocker
	tokenTTL time.Duration
}

// NewLoginLogic constructs a LoginLogic. Panics if any dependency is nil or tokenTTL is zero.
func NewLoginLogic(
	repo Repository,
	hasher passwordVerifier,
	tokens tokenIssuer,
	clk clocker,
	tokenTTL time.Duration,
) *LoginLogic {
	if repo == nil {
		panic("user: NewLoginLogic requires non-nil repo")
	}
	if hasher == nil {
		panic("user: NewLoginLogic requires non-nil hasher")
	}
	if tokens == nil {
		panic("user: NewLoginLogic requires non-nil tokens")
	}
	if clk == nil {
		panic("user: NewLoginLogic requires non-nil clk")
	}
	if tokenTTL <= 0 {
		panic("user: NewLoginLogic requires positive tokenTTL")
	}
	return &LoginLogic{
		repo:     repo,
		hasher:   hasher,
		tokens:   tokens,
		clk:      clk,
		tokenTTL: tokenTTL,
	}
}

// Login authenticates a user by email and password and returns a token on success.
// Callers are responsible for validating that email and password are non-empty before
// calling Login; this method trusts that pre-conditions have been checked.
//
// Returns an error wrapping message.ErrLoginInvalidCredentials for all auth failures
// (user not found, soft-deleted, wrong password) — the caller must never reveal which.
func (l *LoginLogic) Login(ctx context.Context, email, password string) (LoginResult, error) {
	u, err := l.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, message.ErrLoginInvalidCredentials) {
			return LoginResult{}, fmt.Errorf(message.ErrLoginFindUser, message.ErrLoginInvalidCredentials)
		}
		return LoginResult{}, fmt.Errorf(message.ErrLoginFindUser, err)
	}

	ok, err := l.hasher.Verify(ctx, password, u.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf(message.ErrLoginVerifyPassword, err)
	}
	if !ok {
		return LoginResult{}, fmt.Errorf(message.ErrLoginVerifyPassword, message.ErrLoginInvalidCredentials)
	}

	token, err := l.tokens.Issue(ctx, u.ID, l.tokenTTL)
	if err != nil {
		return LoginResult{}, fmt.Errorf(message.ErrLoginIssueToken, err)
	}

	return LoginResult{
		Token:     token,
		ExpiresAt: l.clk.Now().Add(l.tokenTTL),
	}, nil
}
