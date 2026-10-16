package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	DBHost    string
	DBPort    int
	DBName    string
	DBUser    string
	DBPass    string
	DBSSLMode string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env:%w", err)
	}

	port := getEnv("USER_SERVICE_PORT", "8082")
	if err := validatePort("USER_SERVICE_PORT", port); err != nil {
		return nil, err
	}

	dbPortValue := getEnv("DB_PORT", "5432")
	dbPort, err := strconv.Atoi(dbPortValue)
	if err != nil || dbPort < 1 || dbPort > 65535 {
		return nil, fmt.Errorf("invalid DB_PORT: must be an integer between 1 and 65535")
	}

	cfg := &Config{
		Port:      port,
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    dbPort,
		DBName:    getEnv("DB_NAME", ""),
		DBUser:    getEnv("DB_USER", ""),
		DBPass:    os.Getenv("DB_PASSWORD"),
		DBSSLMode: getEnv("DB_SSLMODE", "disable"),
	}

	required := map[string]string{
		"DB_NAME":     cfg.DBName,
		"DB_USER":     cfg.DBUser,
		"DB_PASSWORD": cfg.DBPass,
		"DB_HOST":     cfg.DBHost,
	}

	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%s must be configured", key)
		}
	}

	switch cfg.DBSSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		// Valid PostgreSQL SSL modes.
	default:
		return nil, fmt.Errorf("invalid DB_SSLMODE: unsupported SSL mode")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid %s: must be an integer between 1 and 65535", name)
	}

	return nil
}
