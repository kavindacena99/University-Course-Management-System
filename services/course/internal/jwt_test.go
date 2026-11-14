package course

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

type jwtTestKeys struct {
	private ed25519.PrivateKey
	public  map[string]string
}

func newJWTTestKeys(t *testing.T) jwtTestKeys {
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

	return jwtTestKeys{
		private: privateKey,
		public:  map[string]string{"test-key": pemValue},
	}
}

func newJWTTestVerifier(t *testing.T, keys map[string]string) *TokenVerifier {
	t.Helper()

	verifier, err := NewTokenVerifier(
		keys,
		"university-auth",
		"university-api",
		0,
	)
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	return verifier
}

func signJWTTestToken(
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

func jwtTestClaims() AccessClaims {
	now := time.Now()
	return AccessClaims{
		SessionID:   uuid.NewString(),
		Roles:       []string{RoleLecturer},
		Permissions: []string{PermOfferingsManage},
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "university-auth",
			Audience:  jwt.ClaimStrings{"university-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
}

func TestTokenVerifierAcceptsValidAccessToken(t *testing.T) {
	keys := newJWTTestKeys(t)
	verifier := newJWTTestVerifier(t, keys.public)
	claims := jwtTestClaims()

	verified, err := verifier.Verify(
		signJWTTestToken(t, keys.private, "test-key", claims),
	)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if verified.Subject != claims.Subject || verified.SessionID != claims.SessionID {
		t.Fatal("token identity claims did not round-trip")
	}
	if !HasAnyRole(verified, RoleLecturer) || HasAnyRole(verified, RoleAdmin) {
		t.Fatal("unexpected role evaluation")
	}
	if !HasAnyPermission(verified, PermOfferingsManage) ||
		HasAnyPermission(verified, PermCoursesManage) {
		t.Fatal("unexpected permission evaluation")
	}
}

func TestTokenVerifierRejectsInvalidTokens(t *testing.T) {
	keys := newJWTTestKeys(t)
	verifier := newJWTTestVerifier(t, keys.public)

	tests := []struct {
		name    string
		mutate  func(*AccessClaims)
		kid     string
		wantErr error
	}{
		{
			name: "expired",
			mutate: func(claims *AccessClaims) {
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			},
			kid:     "test-key",
			wantErr: ErrTokenExpired,
		},
		{
			name: "wrong issuer",
			mutate: func(claims *AccessClaims) {
				claims.Issuer = "other-issuer"
			},
			kid:     "test-key",
			wantErr: ErrInvalidToken,
		},
		{
			name: "wrong audience",
			mutate: func(claims *AccessClaims) {
				claims.Audience = jwt.ClaimStrings{"other-api"}
			},
			kid:     "test-key",
			wantErr: ErrInvalidToken,
		},
		{
			name: "refresh token",
			mutate: func(claims *AccessClaims) {
				claims.TokenType = "refresh"
			},
			kid:     "test-key",
			wantErr: ErrInvalidToken,
		},
		{
			name: "unknown key",
			mutate: func(*AccessClaims) {
			},
			kid:     "other-key",
			wantErr: ErrInvalidToken,
		},
		{
			name: "missing subject",
			mutate: func(claims *AccessClaims) {
				claims.Subject = ""
			},
			kid:     "test-key",
			wantErr: ErrInvalidToken,
		},
		{
			name: "missing session",
			mutate: func(claims *AccessClaims) {
				claims.SessionID = ""
			},
			kid:     "test-key",
			wantErr: ErrInvalidToken,
		},
		{
			name: "missing roles",
			mutate: func(claims *AccessClaims) {
				claims.Roles = nil
			},
			kid:     "test-key",
			wantErr: ErrInvalidToken,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := jwtTestClaims()
			test.mutate(&claims)
			_, err := verifier.Verify(
				signJWTTestToken(t, keys.private, test.kid, claims),
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestTokenVerifierRejectsTamperedTokenAndRejectsHS256(t *testing.T) {
	keys := newJWTTestKeys(t)
	verifier := newJWTTestVerifier(t, keys.public)
	raw := signJWTTestToken(t, keys.private, "test-key", jwtTestClaims())

	parts := strings.Split(raw, ".")
	replacement := "a"
	if strings.HasPrefix(parts[2], "a") {
		replacement = "b"
	}
	parts[2] = replacement + parts[2][1:]
	if _, err := verifier.Verify(strings.Join(parts, ".")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected tampered token rejection, got %v", err)
	}

	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtTestClaims())
	hsToken.Header["kid"] = "test-key"
	signed, err := hsToken.SignedString([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("sign HS256 token: %v", err)
	}
	if _, err := verifier.Verify(signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected HS256 rejection, got %v", err)
	}
}

func TestTokenVerifierParsesEscapedPEMNewlines(t *testing.T) {
	keys := newJWTTestKeys(t)
	escaped := map[string]string{
		"test-key": strings.ReplaceAll(keys.public["test-key"], "\n", `\n`),
	}
	verifier := newJWTTestVerifier(t, escaped)

	if _, err := verifier.Verify(
		signJWTTestToken(t, keys.private, "test-key", jwtTestClaims()),
	); err != nil {
		t.Fatalf("verify token: %v", err)
	}
}

func TestTokenVerifierConfigurationValidation(t *testing.T) {
	keys := newJWTTestKeys(t)

	if _, err := NewTokenVerifier(nil, "issuer", "audience", 0); err == nil {
		t.Fatal("expected missing keys error")
	}
	if _, err := NewTokenVerifier(keys.public, "", "audience", 0); err == nil {
		t.Fatal("expected missing issuer error")
	}
	if _, err := NewTokenVerifier(keys.public, "issuer", "", 0); err == nil {
		t.Fatal("expected missing audience error")
	}
	if _, err := NewTokenVerifier(keys.public, "issuer", "audience", -time.Second); err == nil {
		t.Fatal("expected negative leeway error")
	}
	if _, err := NewTokenVerifier(
		map[string]string{"test-key": "not-a-pem"},
		"issuer",
		"audience",
		0,
	); err == nil {
		t.Fatal("expected invalid PEM error")
	}
}
