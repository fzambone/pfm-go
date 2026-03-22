package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
)

// PasetoTokenService implements port/auth.TokenService using PASETO v4.local
// (symmetric encryption). Tokens are self-contained: they encode the user ID,
// issued-at, not-before, and expiration claims. The symmetric key and current
// time are injected so the service is fully testable without a real clock.
type PasetoTokenService struct {
	key paseto.V4SymmetricKey
	clk clock.Clock
}

// NewPasetoTokenService creates a PasetoTokenService from a hex-encoded 32-byte key.
// Returns an error if keyHex is empty, not valid hex, or not exactly 32 bytes.
// Panics if clk is nil to catch wiring mistakes at startup.
func NewPasetoTokenService(keyHex string, clk clock.Clock) (*PasetoTokenService, error) {
	if clk == nil {
		panic("auth: NewPasetoTokenService requires non-nil clock")
	}
	if keyHex == "" {
		return nil, fmt.Errorf(message.ErrTokenServiceKeyDecode, fmt.Errorf("key is empty"))
	}
	raw, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf(message.ErrTokenServiceKeyDecode, err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf(message.ErrTokenServiceKeyLength, len(raw))
	}
	key, err := paseto.V4SymmetricKeyFromBytes(raw)
	if err != nil {
		return nil, fmt.Errorf(message.ErrTokenServiceKeyDecode, err)
	}
	return &PasetoTokenService{key: key, clk: clk}, nil
}

// Issue encrypts a new PASETO v4.local token containing the userID, iat, nbf,
// and exp claims. Each call uses a fresh random nonce so tokens are unique.
func (s *PasetoTokenService) Issue(ctx context.Context, userID uuid.UUID, expiresIn time.Duration) (string, error) {
	_, span := otel.Tracer("auth").Start(ctx, "PasetoTokenService.Issue")
	defer span.End()

	now := s.clk.Now()
	token := paseto.NewToken()
	token.SetSubject(userID.String())
	token.SetIssuedAt(now)
	token.SetNotBefore(now)
	token.SetExpiration(now.Add(expiresIn))

	encrypted := token.V4Encrypt(s.key, nil)
	span.SetStatus(codes.Ok, "")
	return encrypted, nil
}

// Validate decrypts and verifies the token, returning the embedded userID.
// Returns a wrapped ErrTokenExpired if the token has passed its expiry, or
// a wrapped ErrTokenInvalid for any other failure (tampered, malformed, wrong key).
func (s *PasetoTokenService) Validate(ctx context.Context, token string) (uuid.UUID, error) {
	_, span := otel.Tracer("auth").Start(ctx, "PasetoTokenService.Validate")
	defer span.End()

	// NewParserWithoutExpiryCheck avoids the default NotExpired() rule which
	// uses time.Now() — we enforce expiry ourselves via ValidAt(s.clk.Now()).
	parser := paseto.NewParserWithoutExpiryCheck()
	parser.AddRule(paseto.ValidAt(s.clk.Now()))

	parsed, err := parser.ParseV4Local(s.key, token, nil)
	if err != nil {
		// Distinguish expiry from other failures using the error type from the library.
		if errors.Is(err, paseto.RuleError{}) {
			// Rule errors from ValidAt() indicate the token is structurally valid
			// but has passed its expiration.
			wrappedExp := fmt.Errorf(message.ErrTokenServiceValidate, message.ErrTokenExpired)
			span.RecordError(wrappedExp)
			span.SetStatus(codes.Error, wrappedExp.Error())
			return uuid.Nil, wrappedExp
		}
		wrappedInvalid := fmt.Errorf(message.ErrTokenServiceValidate, message.ErrTokenInvalid)
		span.RecordError(wrappedInvalid)
		span.SetStatus(codes.Error, wrappedInvalid.Error())
		return uuid.Nil, wrappedInvalid
	}

	sub, err := parsed.GetSubject()
	if err != nil {
		wrappedInvalid := fmt.Errorf(message.ErrTokenServiceValidate, message.ErrTokenInvalid)
		span.RecordError(wrappedInvalid)
		span.SetStatus(codes.Error, wrappedInvalid.Error())
		return uuid.Nil, wrappedInvalid
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		wrappedInvalid := fmt.Errorf(message.ErrTokenServiceValidate, message.ErrTokenInvalid)
		span.RecordError(wrappedInvalid)
		span.SetStatus(codes.Error, wrappedInvalid.Error())
		return uuid.Nil, wrappedInvalid
	}

	span.SetStatus(codes.Ok, "")
	return userID, nil
}
