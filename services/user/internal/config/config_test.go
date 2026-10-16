package config

import (
	"testing"
)

func setValidConfig(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"USER_SERVICE_PORT": "8082",
		"DB_HOST":           "localhost",
		"DB_PORT":           "5432",
		"DB_NAME":           "users",
		"DB_USER":           "user_app",
		"DB_PASSWORD":       "test-password",
		"DB_SSLMODE":        "disable",
	}

	for key, value := range values {
		t.Setenv(key, value)
	}

	// Prevent a local .env file from overriding these test settings.
	t.Setenv("GODOTENV_DISABLE_LOOKUP", "true")
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
