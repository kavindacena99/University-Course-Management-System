package enrollment

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"enrollment/internal/config"
)

const maxAccessTokenLength = 16384

type AccessClaims struct {
	SessionID   string   `json:"sid"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"perms"`
	TokenType   string   `json:"typ"`
	jwt.RegisteredClaims
}

func (c *AccessClaims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

type TokenVerifier struct {
	keys     map[string]ed25519.PublicKey
	issuer   string
	audience string
	parser   *jwt.Parser
}

func NewTokenVerifier(
	keys map[string]string,
	issuer string,
	audience string,
	leeway time.Duration,
) (*TokenVerifier, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one JWT verification key is required")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, fmt.Errorf("JWT issuer is required")
	}
	if strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("JWT audience is required")
	}
	if leeway < 0 {
		return nil, fmt.Errorf("JWT clock skew must not be negative")
	}

	parsedKeys := make(map[string]ed25519.PublicKey, len(keys))
	for kid, pemValue := range keys {
		key, err := parseEd25519PublicKey(kid, pemValue)
		if err != nil {
			return nil, err
		}
		parsedKeys[kid] = key
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(leeway),
	)

	return &TokenVerifier{
		keys:     parsedKeys,
		issuer:   issuer,
		audience: audience,
		parser:   parser,
	}, nil
}

func NewTokenVerifierFromConfig(cfg *config.Config) (*TokenVerifier, error) {
	return NewTokenVerifier(
		cfg.AuthJWTPublicKeys,
		cfg.AuthJWTIssuer,
		cfg.AuthJWTAudience,
		cfg.AuthJWTClockSkew,
	)
}

func (v *TokenVerifier) Verify(rawToken string) (*AccessClaims, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) > maxAccessTokenLength {
		return nil, ErrInvalidToken
	}

	claims := &AccessClaims{}
	token, err := v.parser.ParseWithClaims(rawToken, claims, v.keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if !token.Valid || token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
		return nil, ErrInvalidToken
	}
	if claims.TokenType != "access" {
		return nil, fmt.Errorf("%w: unexpected token type", ErrInvalidToken)
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, fmt.Errorf("%w: invalid subject", ErrInvalidToken)
	}
	if strings.TrimSpace(claims.SessionID) == "" {
		return nil, fmt.Errorf("%w: missing session id", ErrInvalidToken)
	}
	if len(claims.Roles) == 0 {
		return nil, fmt.Errorf("%w: missing roles", ErrInvalidToken)
	}

	return claims, nil
}

func (v *TokenVerifier) keyFunc(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodEdDSA {
		return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
	}

	kid, ok := token.Header["kid"].(string)
	if !ok || strings.TrimSpace(kid) == "" {
		return nil, fmt.Errorf("missing key id")
	}

	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}

	return key, nil
}

func HasAnyRole(claims *AccessClaims, allowed ...string) bool {
	if claims == nil {
		return false
	}

	for _, granted := range claims.Roles {
		for _, required := range allowed {
			if strings.EqualFold(granted, required) {
				return true
			}
		}
	}

	return false
}

func HasAnyPermission(claims *AccessClaims, required ...string) bool {
	if claims == nil {
		return false
	}

	for _, granted := range claims.Permissions {
		for _, want := range required {
			if granted == want {
				return true
			}
		}
	}

	return false
}

func parseEd25519PublicKey(kid, pemValue string) (ed25519.PublicKey, error) {
	pemValue = strings.ReplaceAll(pemValue, `\n`, "\n")

	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("JWT key %q is not valid PEM", kid)
	}
	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("JWT key %q must be a PUBLIC KEY PEM block", kid)
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse JWT key %q: %w", kid, err)
	}

	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("JWT key %q is not an Ed25519 public key", kid)
	}

	return publicKey, nil
}
