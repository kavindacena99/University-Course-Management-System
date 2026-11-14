package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"
)

func setBaseEnvironment(t *testing.T) {
	t.Helper()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
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
	keysJSON, err := json.Marshal(map[string]string{"test-key": pemValue})
	if err != nil {
		t.Fatalf("marshal keys: %v", err)
	}

	t.Setenv("APP_ENV", "test")
	t.Setenv("DB_NAME", "courses")
	t.Setenv("DB_USER", "course_app")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("AUTH_JWT_PUBLIC_KEYS_JSON", string(keysJSON))
	t.Setenv("INTERNAL_SERVICE_SECRET", "01234567890123456789012345678901")
	t.Setenv("COURSE_SERVICE_PORT", "8083")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")
}

func TestLoadValidConfiguration(t *testing.T) {
	setBaseEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ServiceName != "course-service" {
		t.Errorf("service name = %q", cfg.ServiceName)
	}
	if cfg.Port != "8083" || cfg.DBName != "courses" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.AuthJWTPublicKeys) != 1 {
		t.Errorf("public keys = %d", len(cfg.AuthJWTPublicKeys))
	}
	if cfg.UserServiceTimeout.Seconds() != 2 {
		t.Errorf("user timeout = %s", cfg.UserServiceTimeout)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "port", key: "COURSE_SERVICE_PORT", value: "70000"},
		{name: "db port", key: "DB_PORT", value: "abc"},
		{name: "missing database", key: "DB_NAME", value: ""},
		{name: "missing user", key: "DB_USER", value: ""},
		{name: "missing password", key: "DB_PASSWORD", value: ""},
		{name: "short internal secret", key: "INTERNAL_SERVICE_SECRET", value: "short"},
		{name: "missing jwt keys", key: "AUTH_JWT_PUBLIC_KEYS_JSON", value: ""},
		{name: "invalid jwt json", key: "AUTH_JWT_PUBLIC_KEYS_JSON", value: "[]"},
		{name: "invalid clock skew", key: "AUTH_JWT_CLOCK_SKEW", value: "0s"},
		{name: "invalid request limit", key: "MAX_REQUEST_BYTES", value: "0"},
		{name: "wildcard cors", key: "CORS_ALLOWED_ORIGINS", value: "*"},
		{name: "invalid user url", key: "USER_SERVICE_INTERNAL_URL", value: "relative"},
		{name: "invalid log level", key: "LOG_LEVEL", value: "trace"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setBaseEnvironment(t)
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
