package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal test key: %v", err)
	}
	path := filepath.Join(t.TempDir(), "jwt-private.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}), 0o600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	return path
}

func setValidConfig(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"APP_ENV":                   "development",
		"AUTH_SERVICE_PORT":         "8081",
		"DB_HOST":                   "localhost",
		"DB_PORT":                   "5432",
		"DB_NAME":                   "authentication_and_security",
		"DB_USER":                   "auth_app",
		"DB_PASSWORD":               "test-password",
		"DB_SSLMODE":                "disable",
		"AUTH_JWT_KEY_ID":           "test-key",
		"AUTH_JWT_PRIVATE_KEY_FILE": writeTestPrivateKey(t),
		"AUTH_JWT_PRIVATE_KEY_PEM":  "",
		"INTERNAL_SERVICE_SECRET":   strings.Repeat("s", 32),
		"NOTIFICATIONS_MODE":        "none",
		"LOG_LEVEL":                 "info",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}

func TestLoadValidConfiguration(t *testing.T) {
	setValidConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort = %d, want 5432", cfg.DBPort)
	}
	if cfg.DBName != "authentication_and_security" {
		t.Errorf("DBName = %q", cfg.DBName)
	}
	if cfg.JWTKeyID != "test-key" {
		t.Errorf("JWTKeyID = %q", cfg.JWTKeyID)
	}
	if len(cfg.JWTPrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("JWTPrivateKey length = %d", len(cfg.JWTPrivateKey))
	}
	if cfg.JWTClockSkew != 30*time.Second {
		t.Errorf("JWTClockSkew = %v", cfg.JWTClockSkew)
	}
	if cfg.AccessTokenTTL != 10*time.Minute {
		t.Errorf("AccessTokenTTL = %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 24*time.Hour {
		t.Errorf("RefreshTokenTTL = %v", cfg.RefreshTokenTTL)
	}
	if cfg.SessionAbsoluteTTL != 7*24*time.Hour {
		t.Errorf("SessionAbsoluteTTL = %v", cfg.SessionAbsoluteTTL)
	}
	if !cfg.CookieEnabled {
		t.Error("CookieEnabled = false, want true")
	}
	if cfg.TokenDelivery != "both" {
		t.Errorf("TokenDelivery = %q", cfg.TokenDelivery)
	}
	if len(cfg.AllowedInternalCallers) != 1 ||
		cfg.AllowedInternalCallers[0] != "user-service" {
		t.Errorf("AllowedInternalCallers = %v", cfg.AllowedInternalCallers)
	}
	if cfg.Argon2MemoryKiB != 65536 {
		t.Errorf("Argon2MemoryKiB = %d", cfg.Argon2MemoryKiB)
	}
}

func TestLoadRequiresPrivateKey(t *testing.T) {
	setValidConfig(t)
	t.Setenv("AUTH_JWT_PRIVATE_KEY_FILE", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a missing JWT private key")
	}
}

func TestLoadRejectsBothPrivateKeySources(t *testing.T) {
	setValidConfig(t)
	privateKey, err := os.ReadFile(os.Getenv("AUTH_JWT_PRIVATE_KEY_FILE"))
	if err != nil {
		t.Fatalf("read test key: %v", err)
	}
	t.Setenv("AUTH_JWT_PRIVATE_KEY_PEM", string(privateKey))

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted both private key sources")
	}
}

func TestLoadRejectsInvalidKeyID(t *testing.T) {
	setValidConfig(t)
	t.Setenv("AUTH_JWT_KEY_ID", "bad key!")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid JWT key id")
	}
}

func TestLoadRejectsShortInternalSecret(t *testing.T) {
	setValidConfig(t)
	t.Setenv("INTERNAL_SERVICE_SECRET", "short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a short internal service secret")
	}
}

func TestLoadRejectsInvalidTokenDelivery(t *testing.T) {
	setValidConfig(t)
	t.Setenv("AUTH_TOKEN_DELIVERY", "header")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid token delivery mode")
	}
}

func TestLoadRejectsCookieDeliveryWhenCookiesDisabled(t *testing.T) {
	setValidConfig(t)
	t.Setenv("AUTH_TOKEN_DELIVERY", "cookie")
	t.Setenv("AUTH_COOKIE_ENABLED", "false")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted cookie delivery with cookies disabled")
	}
}

func TestLoadRejectsInvalidTokenLifetimes(t *testing.T) {
	setValidConfig(t)
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "2h")
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "1h")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted access TTL longer than refresh TTL")
	}

	setValidConfig(t)
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "200h")
	t.Setenv("AUTH_SESSION_ABSOLUTE_TTL", "100h")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted refresh TTL longer than session TTL")
	}
}

func TestLoadRejectsProductionFileNotifications(t *testing.T) {
	setValidConfig(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("NOTIFICATIONS_MODE", "file")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted file notifications in production")
	}
}

func TestLoadRejectsInvalidArgon2Parameters(t *testing.T) {
	setValidConfig(t)
	t.Setenv("ARGON2_MEMORY_KIB", "1024")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted invalid Argon2 memory")
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	setValidConfig(t)
	t.Setenv("LOG_LEVEL", "verbose")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid log level")
	}
}
