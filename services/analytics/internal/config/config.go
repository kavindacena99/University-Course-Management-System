package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment string
	BindHost    string
	Port        string
	LogLevel    string

	DBHost    string
	DBPort    int
	DBName    string
	DBUser    string
	DBPass    string
	DBSSLMode string

	AuthJWTPublicKeys map[string]string
	AuthJWTIssuer     string
	AuthJWTAudience   string
	AuthJWTClockSkew  time.Duration

	AuthCookieEnabled     bool
	AccessTokenCookieName string
	CSRFCookieName        string
	CSRFHeaderName        string

	InternalServiceID             string
	InternalServiceSecret         string
	InternalServiceSecretPrevious string
	InternalAllowedCallers        []string
	InternalAuthMaxSkew           time.Duration

	CourseServiceInternalURL     string
	AssessmentServiceInternalURL string

	HTTPReadTimeout   time.Duration
	HTTPWriteTimeout  time.Duration
	HTTPIdleTimeout   time.Duration
	HTTPHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
	MaxRequestBytes   int64
	TrustedProxies    []string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}
	port := env("ANALYTICS_SERVICE_PORT", "8086")
	if err := validatePort(port); err != nil {
		return nil, err
	}
	dbPort, err := strconv.Atoi(env("DB_PORT", "5432"))
	if err != nil || dbPort < 1 || dbPort > 65535 {
		return nil, fmt.Errorf("DB_PORT must be between 1 and 65535")
	}
	maxRequest, err := strconv.ParseInt(env("MAX_REQUEST_BYTES", "1048576"), 10, 64)
	if err != nil || maxRequest < 1024 || maxRequest > 8<<20 {
		return nil, fmt.Errorf("MAX_REQUEST_BYTES must be between 1024 and 8388608")
	}
	cfg := &Config{
		Environment:               env("APP_ENV", "development"),
		BindHost:                  env("BIND_HOST", "127.0.0.1"),
		Port:                      port,
		LogLevel:                  env("LOG_LEVEL", "info"),
		DBHost:                    env("DB_HOST", "localhost"),
		DBPort:                    dbPort,
		DBName:                    strings.TrimSpace(os.Getenv("DB_NAME")),
		DBUser:                    strings.TrimSpace(os.Getenv("DB_USER")),
		DBPass:                    os.Getenv("DB_PASSWORD"),
		DBSSLMode:                 env("DB_SSLMODE", "disable"),
		AuthJWTIssuer:             env("AUTH_JWT_ISSUER", "university-auth"),
		AuthJWTAudience:           env("AUTH_JWT_AUDIENCE", "university-api"),
		AuthCookieEnabled:         true,
		AccessTokenCookieName:     env("ACCESS_TOKEN_COOKIE_NAME", "access_token"),
		CSRFCookieName:            env("CSRF_COOKIE_NAME", "csrf_token"),
		CSRFHeaderName:            env("CSRF_HEADER_NAME", "X-CSRF-Token"),
		InternalServiceID:             env("INTERNAL_SERVICE_ID", "analytics-service"),
		InternalServiceSecret:         os.Getenv("INTERNAL_SERVICE_SECRET"),
		InternalServiceSecretPrevious: os.Getenv("INTERNAL_SERVICE_SECRET_PREVIOUS"),
		InternalAllowedCallers:    split(os.Getenv("INTERNAL_ALLOWED_CALLERS")),
		CourseServiceInternalURL:  env("COURSE_SERVICE_INTERNAL_URL", "http://127.0.0.1:8083"),
		AssessmentServiceInternalURL: env("ASSESSMENT_SERVICE_INTERNAL_URL", "http://127.0.0.1:8085"),
		MaxRequestBytes:           maxRequest,
		TrustedProxies:            split(env("TRUSTED_PROXIES", "127.0.0.1,::1")),
	}
	cfg.AuthCookieEnabled, err = parseBool("AUTH_COOKIE_ENABLED", true)
	if err != nil {
		return nil, err
	}
	cfg.AuthJWTClockSkew, err = parseDuration("AUTH_JWT_CLOCK_SKEW", 30*time.Second)
	if err != nil {
		return nil, err
	}
	cfg.InternalAuthMaxSkew, err = parseDuration("INTERNAL_AUTH_MAX_SKEW", 2*time.Minute)
	if err != nil {
		return nil, err
	}
	cfg.HTTPReadTimeout, err = parseDuration("HTTP_READ_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	cfg.HTTPWriteTimeout, err = parseDuration("HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	cfg.HTTPIdleTimeout, err = parseDuration("HTTP_IDLE_TIMEOUT", time.Minute)
	if err != nil {
		return nil, err
	}
	cfg.HTTPHeaderTimeout, err = parseDuration("HTTP_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return nil, err
	}
	cfg.ShutdownTimeout, err = parseDuration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(os.Getenv("AUTH_JWT_PUBLIC_KEYS_JSON")), &cfg.AuthJWTPublicKeys); err != nil ||
		len(cfg.AuthJWTPublicKeys) == 0 {
		return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON must be a non-empty JSON object")
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validate(cfg *Config) error {
	for key, value := range map[string]string{
		"DB_NAME": cfg.DBName, "DB_USER": cfg.DBUser, "DB_PASSWORD": cfg.DBPass,
		"INTERNAL_SERVICE_SECRET": cfg.InternalServiceSecret,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must be configured", key)
		}
	}
	if len(cfg.InternalServiceSecret) < 32 {
		return fmt.Errorf("INTERNAL_SERVICE_SECRET must be at least 32 characters")
	}
	if cfg.InternalServiceSecretPrevious != "" && len(cfg.InternalServiceSecretPrevious) < 32 {
		return fmt.Errorf("INTERNAL_SERVICE_SECRET_PREVIOUS must be at least 32 characters")
	}
	if cfg.InternalAuthMaxSkew <= 0 || cfg.InternalAuthMaxSkew > 10*time.Minute {
		return fmt.Errorf("INTERNAL_AUTH_MAX_SKEW must be between 0 and 10 minutes")
	}
	if cfg.AuthJWTClockSkew < 0 || strings.TrimSpace(cfg.AuthJWTIssuer) == "" ||
		strings.TrimSpace(cfg.AuthJWTAudience) == "" {
		return fmt.Errorf("JWT issuer, audience, and non-negative clock skew are required")
	}
	for _, value := range []string{
		cfg.CourseServiceInternalURL, cfg.AssessmentServiceInternalURL,
	} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.User != nil || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("internal service URLs must be absolute HTTP URLs without paths or credentials")
		}
	}
	if cfg.DBSSLMode != "disable" && cfg.DBSSLMode != "require" &&
		cfg.DBSSLMode != "verify-ca" && cfg.DBSSLMode != "verify-full" {
		return fmt.Errorf("DB_SSLMODE must be disable, require, verify-ca, or verify-full")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	return nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func split(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return parsed, nil
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return parsed, nil
}

func validatePort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("ANALYTICS_SERVICE_PORT must be between 1 and 65535")
	}
	return nil
}
