package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessClaims struct {
	SessionID string   `json:"sid"`
	Roles     []string `json:"roles"`
	TokenType string   `json:"typ"`
	jwt.RegisteredClaims
}

func (c *AccessClaims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

type TokenManager struct {
	private   ed25519.PrivateKey
	keyID     string
	issuer    string
	audience  string
	accessTTL time.Duration
	now       func() time.Time
	parser    *jwt.Parser
}

func NewTokenManager(
	privateKey ed25519.PrivateKey,
	keyID string,
	issuer string,
	audience string,
	accessTTL time.Duration,
	leeway time.Duration,
) (*TokenManager, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid Ed25519 private key")
	}
	if strings.TrimSpace(keyID) == "" {
		return nil, fmt.Errorf("JWT key id is required")
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("JWT issuer and audience are required")
	}
	if accessTTL <= 0 {
		return nil, fmt.Errorf("access token TTL must be positive")
	}
	if leeway < 0 {
		return nil, fmt.Errorf("JWT clock skew must not be negative")
	}

	return &TokenManager{
		private:   privateKey,
		keyID:     keyID,
		issuer:    issuer,
		audience:  audience,
		accessTTL: accessTTL,
		now:       time.Now,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(leeway),
		),
	}, nil
}

func (m *TokenManager) IssueAccessToken(
	userID uuid.UUID,
	sessionID uuid.UUID,
	roles []string,
) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(m.accessTTL)

	claims := AccessClaims{
		SessionID: sessionID.String(),
		Roles:     append([]string(nil), roles...),
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = m.keyID

	signed, err := token.SignedString(m.private)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signed, expiresAt, nil
}

func (m *TokenManager) VerifyAccessToken(rawToken string) (*AccessClaims, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) > 16384 {
		return nil, ErrInvalidToken
	}

	claims := &AccessClaims{}
	token, err := m.parser.ParseWithClaims(rawToken, claims, m.keyFunc)
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
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.SessionID); err != nil {
		return nil, ErrInvalidToken
	}
	if len(claims.Roles) == 0 {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (m *TokenManager) PublicKeyPEM() (string, error) {
	publicKey, ok := m.private.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("derive JWT public key")
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("marshal JWT public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})), nil
}

func (m *TokenManager) keyFunc(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodEdDSA {
		return nil, fmt.Errorf("unexpected signing method")
	}
	if kid, _ := token.Header["kid"].(string); kid != m.keyID {
		return nil, fmt.Errorf("unknown key id")
	}
	return m.private.Public(), nil
}

func GenerateOpaqueToken() (string, []byte, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", nil, fmt.Errorf("generate random token: %w", err)
	}

	raw := base64.RawURLEncoding.EncodeToString(random)
	return raw, HashOpaqueToken(raw), nil
}

func HashOpaqueToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func GenerateCSRFToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate CSRF token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
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
