package user

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type testKeyPair struct {
	private ed25519.PrivateKey
	public  map[string]string
}

func newTestKeyPair(t *testing.T) testKeyPair {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pemValue := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	}))

	return testKeyPair{
		private: privateKey,
		public:  map[string]string{"test-key": pemValue},
	}
}

func newTestVerifier(t *testing.T, keys map[string]string, leeway time.Duration) *TokenVerifier {
	t.Helper()

	verifier, err := NewTokenVerifier(
		keys,
		"university-auth",
		"university-api",
		leeway,
	)
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	return verifier
}

func signedAccessToken(
	t *testing.T,
	private ed25519.PrivateKey,
	kid string,
	claims AccessClaims,
) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(private)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func validClaims() AccessClaims {
	now := time.Now()
	return AccessClaims{
		SessionID: uuid.NewString(),
		Roles:     []string{RoleStudent},
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "university-auth",
			Audience:  jwt.ClaimStrings{"university-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
}

func TestTokenVerifierAcceptsValidToken(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)
	claims := validClaims()

	raw := signedAccessToken(t, keys.private, "test-key", claims)
	verified, err := verifier.Verify(raw)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if verified.Subject != claims.Subject {
		t.Fatalf("subject = %q", verified.Subject)
	}
	if verified.SessionID != claims.SessionID {
		t.Fatalf("session = %q", verified.SessionID)
	}
	if _, err := verified.UserID(); err != nil {
		t.Fatalf("parse user ID: %v", err)
	}
	if !HasAnyRole(verified, RoleStudent) {
		t.Fatal("expected student role")
	}
	if HasAnyRole(verified, RoleAdmin) {
		t.Fatal("did not expect admin role")
	}
}

func TestTokenVerifierRejectsExpiredToken(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)
	claims := validClaims()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	_, err := verifier.Verify(signedAccessToken(t, keys.private, "test-key", claims))
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestTokenVerifierRejectsWrongIssuerOrAudience(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)

	claims := validClaims()
	claims.Issuer = "other-issuer"
	if _, err := verifier.Verify(signedAccessToken(t, keys.private, "test-key", claims)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token for issuer, got %v", err)
	}

	claims = validClaims()
	claims.Audience = jwt.ClaimStrings{"other-api"}
	if _, err := verifier.Verify(signedAccessToken(t, keys.private, "test-key", claims)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token for audience, got %v", err)
	}
}

func TestTokenVerifierRejectsRefreshTokenType(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)
	claims := validClaims()
	claims.TokenType = "refresh"

	if _, err := verifier.Verify(signedAccessToken(t, keys.private, "test-key", claims)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token, got %v", err)
	}
}

func TestTokenVerifierRejectsUnknownKeyID(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)

	_, err := verifier.Verify(signedAccessToken(t, keys.private, "other-key", validClaims()))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token, got %v", err)
	}
}

func TestTokenVerifierRejectsMissingSubject(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)
	claims := validClaims()
	claims.Subject = ""

	_, err := verifier.Verify(signedAccessToken(t, keys.private, "test-key", claims))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token, got %v", err)
	}
}

func TestTokenVerifierRejectsTamperedToken(t *testing.T) {
	keys := newTestKeyPair(t)
	verifier := newTestVerifier(t, keys.public, 0)
	raw := signedAccessToken(t, keys.private, "test-key", validClaims())

	parts := strings.Split(raw, ".")
	replacement := "a"
	if strings.HasPrefix(parts[2], "a") {
		replacement = "b"
	}
	parts[2] = replacement + parts[2][1:]
	tampered := strings.Join(parts, ".")

	_, err := verifier.Verify(tampered)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token, got %v", err)
	}
}

func TestTokenVerifierParsesEscapedNewlinesInPEM(t *testing.T) {
	keys := newTestKeyPair(t)
	escaped := map[string]string{
		"test-key": replaceNewlines(keys.public["test-key"]),
	}
	verifier := newTestVerifier(t, escaped, 0)

	raw := signedAccessToken(t, keys.private, "test-key", validClaims())
	if _, err := verifier.Verify(raw); err != nil {
		t.Fatalf("verify token: %v", err)
	}
}

func replaceNewlines(value string) string {
	result := ""
	for _, r := range value {
		if r == '\n' {
			result += `\n`
		} else {
			result += string(r)
		}
	}
	return result
}
