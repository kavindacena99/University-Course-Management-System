package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func testPublicKeysJSON(t *testing.T) string {
	t.Helper()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal test key: %v", err)
	}
	pemValue := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	payload, err := json.Marshal(map[string]string{
		"test-key": string(pemValue),
	})
	if err != nil {
		t.Fatalf("marshal test key map: %v", err)
	}
	return string(payload)
}

func setValidConfig(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"USER_SERVICE_PORT":                  "8082",
		"DB_HOST":                            "localhost",
		"DB_PORT":                            "5432",
		"DB_NAME":                            "users",
		"DB_USER":                            "user_app",
		"DB_PASSWORD":                        "test-password",
		"DB_SSLMODE":                         "disable",
		"AUTH_JWT_PUBLIC_KEYS_JSON":          testPublicKeysJSON(t),
		"INTERNAL_SERVICE_SECRET":            strings.Repeat("a", 32),
		"ADMIN_BOOTSTRAP_TOKEN":              strings.Repeat("b", 32),
		"AUTH_COOKIE_ENABLED":                "true",
		"ACCESS_TOKEN_COOKIE_NAME":           "access_token",
		"CSRF_COOKIE_NAME":                   "csrf_token",
		"CSRF_HEADER_NAME":                   "X-CSRF-Token",
		"INTERNAL_AUTH_MAX_SKEW":             "2m",
		"AUTH_JWT_CLOCK_SKEW":                "30s",
		"AUTH_ELIGIBILITY_TIMEOUT":           "2s",
		"OUTBOX_POLL_INTERVAL":               "2s",
		"OUTBOX_HTTP_TIMEOUT":                "5s",
		"OUTBOX_BATCH_SIZE":                  "20",
		"OUTBOX_MAX_ATTEMPTS":                "10",
		"HTTP_READ_TIMEOUT":                  "15s",
		"HTTP_WRITE_TIMEOUT":                 "15s",
		"HTTP_IDLE_TIMEOUT":                  "60s",
		"HTTP_HEADER_TIMEOUT":                "5s",
		"SHUTDOWN_TIMEOUT":                   "10s",
		"MAX_REQUEST_BYTES":                  "1048576",
		"REGISTRATION_RATE_LIMIT_PER_MINUTE": "30",
		"LOG_LEVEL":                          "info",
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

	if cfg.Port != "8082" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8082")
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort = %d, want %d", cfg.DBPort, 5432)
	}
	if cfg.DBName != "users" {
		t.Errorf("DBName = %q, want %q", cfg.DBName, "users")
	}
	if !cfg.AuthCookieEnabled {
		t.Error("AuthCookieEnabled = false, want true")
	}
	if cfg.AccessTokenCookieName != "access_token" {
		t.Errorf("AccessTokenCookieName = %q", cfg.AccessTokenCookieName)
	}
	if cfg.CSRFCookieName != "csrf_token" {
		t.Errorf("CSRFCookieName = %q", cfg.CSRFCookieName)
	}
	if cfg.CSRFHeaderName != "X-CSRF-Token" {
		t.Errorf("CSRFHeaderName = %q", cfg.CSRFHeaderName)
	}
	if cfg.InternalAuthMaxSkew != 2*time.Minute {
		t.Errorf("InternalAuthMaxSkew = %v, want 2m", cfg.InternalAuthMaxSkew)
	}
	if len(cfg.AuthJWTPublicKeys) != 1 {
		t.Errorf("AuthJWTPublicKeys length = %d, want 1", len(cfg.AuthJWTPublicKeys))
	}
	if len(cfg.TrustedProxies) == 0 {
		t.Error("TrustedProxies is empty")
	}
}

func TestLoadRejectsInvalidServicePort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536"} {
		t.Run(port, func(t *testing.T) {
			setValidConfig(t)
			t.Setenv("USER_SERVICE_PORT", port)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() accepted invalid service port %q", port)
			}
		})
	}
}

func TestLoadRejectsInvalidDatabasePort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536"} {
		t.Run(port, func(t *testing.T) {
			setValidConfig(t)
			t.Setenv("DB_PORT", port)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() accepted invalid database port %q", port)
			}
		})
	}
}

func TestLoadRejectsMissingPassword(t *testing.T) {
	setValidConfig(t)
	t.Setenv("DB_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a missing database password")
	}
}

func TestLoadRejectsWhitespaceOnlyDatabaseName(t *testing.T) {
	setValidConfig(t)
	t.Setenv("DB_NAME", "   ")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a whitespace-only database name")
	}
}

func TestLoadRejectsUnsupportedSSLMode(t *testing.T) {
	setValidConfig(t)
	t.Setenv("DB_SSLMODE", "unsafe")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted an unsupported SSL mode")
	}
}

func TestGetEnvUsesFallbackForEmptyValue(t *testing.T) {
	t.Setenv("TEST_EMPTY_CONFIG_VALUE", "")

	got := getEnv("TEST_EMPTY_CONFIG_VALUE", "fallback")
	if got != "fallback" {
		t.Fatalf("getEnv() = %q, want %q", got, "fallback")
	}
}

func TestLoadRequiresDatabaseCredentials(t *testing.T) {
	setValidConfig(t)

	for _, key := range []string{"DB_NAME", "DB_USER", "DB_PASSWORD"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() accepted missing %s", key)
			}
		})
	}
}

func TestLoadRequiresJWTKeys(t *testing.T) {
	setValidConfig(t)
	t.Setenv("AUTH_JWT_PUBLIC_KEYS_JSON", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted missing JWT public keys")
	}
}

func TestLoadRejectsMalformedJWTKeys(t *testing.T) {
	setValidConfig(t)

	for _, value := range []string{
		"not-json",
		"{}",
		`{"kid":""}`,
		`{"kid":"not-a-pem"}`,
	} {
		t.Run(base64.StdEncoding.EncodeToString([]byte(value)), func(t *testing.T) {
			setValidConfig(t)
			t.Setenv("AUTH_JWT_PUBLIC_KEYS_JSON", value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() accepted malformed JWT keys %q", value)
			}
		})
	}
}

func TestLoadRejectsShortInternalSecret(t *testing.T) {
	setValidConfig(t)
	t.Setenv("INTERNAL_SERVICE_SECRET", "short")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a short internal service secret")
	}
}

func TestLoadRejectsInvalidCookieConfiguration(t *testing.T) {
	cases := map[string]string{
		"ACCESS_TOKEN_COOKIE_NAME": "bad name",
		"CSRF_COOKIE_NAME":         "bad;name",
		"CSRF_HEADER_NAME":         "bad header",
	}

	for key, value := range cases {
		t.Run(key, func(t *testing.T) {
			setValidConfig(t)
			t.Setenv(key, value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() accepted invalid %s", key)
			}
		})
	}
}

func TestLoadRejectsWildcardCORSWithCredentials(t *testing.T) {
	setValidConfig(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted wildcard CORS with credentials")
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	setValidConfig(t)
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted an invalid log level")
	}
}
