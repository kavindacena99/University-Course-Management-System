package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
)

func validTestConfig(t *testing.T) {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := json.Marshal(map[string]string{
		"test": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})),
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"DB_HOST": "127.0.0.1", "DB_PORT": "5432",
		"DB_NAME": "enrollments", "DB_USER": "enrollment_app",
		"DB_PASSWORD": "test-only", "DB_SSLMODE": "disable",
		"AUTH_JWT_PUBLIC_KEYS_JSON":   string(keys),
		"INTERNAL_SERVICE_SECRET":     strings.Repeat("s", 32),
		"INTERNAL_ALLOWED_CALLERS":    "assessment-service",
		"COURSE_SERVICE_INTERNAL_URL": "http://course:8083",
	} {
		t.Setenv(key, value)
	}
}

func TestLoadValidEnrollmentConfig(t *testing.T) {
	validTestConfig(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8084" || cfg.ServiceName != "enrollment-service" ||
		cfg.CourseServiceInternalURL != "http://course:8083" ||
		len(cfg.AllowedInternalCallers) != 1 ||
		cfg.AllowedInternalCallers[0] != "assessment-service" {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}

func TestLoadRejectsMissingSecretAndInvalidCourseURL(t *testing.T) {
	for _, tc := range []struct {
		key, value string
	}{
		{"INTERNAL_SERVICE_SECRET", "short"},
		{"COURSE_SERVICE_INTERNAL_URL", "relative"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			validTestConfig(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatalf("expected %s validation failure", tc.key)
			}
		})
	}
}
