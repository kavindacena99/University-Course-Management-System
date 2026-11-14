package auth

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

func newTestTokenManager(
	t *testing.T,
	keyID string,
	accessTTL time.Duration,
) (*TokenManager, ed25519.PrivateKey) {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate token key: %v", err)
	}
	manager, err := NewTokenManager(
		privateKey,
		keyID,
		"university-auth",
		"university-api",
		accessTTL,
		0,
	)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}
	return manager, privateKey
}

func TestTokenManagerIssuesAndVerifiesAccessToken(t *testing.T) {
	manager, _ := newTestTokenManager(t, "test-key", time.Minute)
	userID := uuid.New()
	sessionID := uuid.New()

	raw, expiry, err := manager.IssueAccessToken(
		userID,
		sessionID,
		[]string{RoleStudent},
		[]string{PermUsersRead},
	)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	if !expiry.After(time.Now()) {
		t.Fatal("expected a future expiry")
	}

	claims, err := manager.VerifyAccessToken(raw)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	parsedUserID, err := claims.UserID()
	if err != nil {
		t.Fatalf("parse user id: %v", err)
	}
	if parsedUserID != userID {
		t.Fatalf("user id = %s, want %s", parsedUserID, userID)
	}
	if claims.SessionID != sessionID.String() {
		t.Fatalf("session id = %q", claims.SessionID)
	}
	if !HasAnyRole(claims, RoleStudent) {
		t.Fatal("expected student role")
	}
	if HasAnyRole(claims, RoleAdmin) {
		t.Fatal("did not expect admin role")
	}
	if !HasAnyPermission(claims, PermUsersRead) {
		t.Fatal("expected users:read permission")
	}
	if HasAnyPermission(claims, PermUsersManage) {
		t.Fatal("did not expect users:manage permission")
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager, _ := newTestTokenManager(t, "test-key", time.Millisecond)
	raw, _, err := manager.IssueAccessToken(
		uuid.New(),
		uuid.New(),
		[]string{RoleStudent},
		nil,
	)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if _, err := manager.VerifyAccessToken(raw); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestTokenManagerRejectsUnknownKeyID(t *testing.T) {
	manager, _ := newTestTokenManager(t, "test-key", time.Minute)
	raw, _, err := manager.IssueAccessToken(
		uuid.New(),
		uuid.New(),
		[]string{RoleStudent},
		nil,
	)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	other, _ := newTestTokenManager(t, "other-key", time.Minute)
	if _, err := other.VerifyAccessToken(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestTokenManagerRejectsInvalidSessionID(t *testing.T) {
	manager, privateKey := newTestTokenManager(t, "test-key", time.Minute)
	now := time.Now()
	claims := AccessClaims{
		SessionID: "not-a-uuid",
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
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "test-key"
	raw, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := manager.VerifyAccessToken(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestTokenManagerRejectsTamperedToken(t *testing.T) {
	manager, _ := newTestTokenManager(t, "test-key", time.Minute)
	raw, _, err := manager.IssueAccessToken(
		uuid.New(),
		uuid.New(),
		[]string{RoleStudent},
		nil,
	)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	parts := strings.Split(raw, ".")
	replacement := "a"
	if strings.HasPrefix(parts[2], "a") {
		replacement = "b"
	}
	parts[2] = replacement + parts[2][1:]

	if _, err := manager.VerifyAccessToken(strings.Join(parts, ".")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestTokenManagerPublicKeyPEM(t *testing.T) {
	manager, _ := newTestTokenManager(t, "test-key", time.Minute)

	publicPEM, err := manager.PublicKeyPEM()
	if err != nil {
		t.Fatalf("export public key: %v", err)
	}
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil || block.Type != "PUBLIC KEY" {
		t.Fatal("expected a PUBLIC KEY PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	if _, ok := parsed.(ed25519.PublicKey); !ok {
		t.Fatal("expected an Ed25519 public key")
	}
}
