package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	ServiceName string
	BindHost    string
	Port        string

	DBHost    string
	DBPort    int
	DBName    string
	DBUser    string
	DBPass    string
	DBSSLMode string

	AuthJWTPublicKeysJSON string
	AuthJWTPublicKeys     map[string]string
	AuthJWTIssuer         string
	AuthJWTAudience       string
	AuthJWTClockSkew      time.Duration

	AuthCookieEnabled     bool
	AccessTokenCookieName string
	CSRFCookieName        string
	CSRFHeaderName        string

	InternalServiceID             string
	InternalServiceSecret         string
	InternalServiceSecretPrevious string
	AllowedInternalCallers        []string
	InternalAuthMaxSkew           time.Duration

	AdminBootstrapToken string

	AuthServiceInternalURL string
	EligibilityTimeout     time.Duration

	OutboxPollInterval time.Duration
	OutboxBatchSize    int
	OutboxMaxAttempts  int
	OutboxHTTPTimeout  time.Duration

	CORSAllowedOrigins   []string
	CORSAllowCredentials bool

	TrustedProxies []string

	HTTPReadTimeout   time.Duration
	HTTPWriteTimeout  time.Duration
	HTTPIdleTimeout   time.Duration
	HTTPHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration

	MaxRequestBytes int64

	RegistrationRateLimitPerMinute int

	LogLevel string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
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
		Environment: getEnv("APP_ENV", "development"),
		ServiceName: "user-service",
		BindHost:    getEnv("BIND_HOST", "127.0.0.1"),
		Port:        port,

		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    dbPort,
		DBName:    getEnv("DB_NAME", ""),
		DBUser:    getEnv("DB_USER", ""),
		DBPass:    os.Getenv("DB_PASSWORD"),
		DBSSLMode: getEnv("DB_SSLMODE", "disable"),

		AuthJWTPublicKeysJSON: os.Getenv("AUTH_JWT_PUBLIC_KEYS_JSON"),
		AuthJWTIssuer:         getEnv("AUTH_JWT_ISSUER", "university-auth"),
		AuthJWTAudience:       getEnv("AUTH_JWT_AUDIENCE", "university-api"),

		AuthCookieEnabled:     getBool("AUTH_COOKIE_ENABLED", true),
		AccessTokenCookieName: getEnv("ACCESS_TOKEN_COOKIE_NAME", "access_token"),
		CSRFCookieName:        getEnv("CSRF_COOKIE_NAME", "csrf_token"),
		CSRFHeaderName:        getEnv("CSRF_HEADER_NAME", "X-CSRF-Token"),

		InternalServiceID:             getEnv("INTERNAL_SERVICE_ID", "user-service"),
		InternalServiceSecret:         os.Getenv("INTERNAL_SERVICE_SECRET"),
		InternalServiceSecretPrevious: os.Getenv("INTERNAL_SERVICE_SECRET_PREVIOUS"),

		AdminBootstrapToken: os.Getenv("ADMIN_BOOTSTRAP_TOKEN"),

		AuthServiceInternalURL: getEnv("AUTH_SERVICE_INTERNAL_URL", "http://127.0.0.1:8081"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}

	cfg.AllowedInternalCallers = splitCSV(getEnv("INTERNAL_ALLOWED_CALLERS", "auth-service,course-service,enrollment-service"))
	cfg.CORSAllowedOrigins = splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	cfg.TrustedProxies = splitCSV(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1"))
	cfg.CORSAllowCredentials = getBool("CORS_ALLOW_CREDENTIALS", true)

	if cfg.AuthJWTClockSkew, err = getDuration("AUTH_JWT_CLOCK_SKEW", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.EligibilityTimeout, err = getDuration("AUTH_ELIGIBILITY_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}
	if cfg.InternalAuthMaxSkew, err = getDuration("INTERNAL_AUTH_MAX_SKEW", 2*time.Minute); err != nil {
		return nil, err
	}
	if cfg.OutboxPollInterval, err = getDuration("OUTBOX_POLL_INTERVAL", 2*time.Second); err != nil {
		return nil, err
	}
	if cfg.OutboxHTTPTimeout, err = getDuration("OUTBOX_HTTP_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPReadTimeout, err = getDuration("HTTP_READ_TIMEOUT", 15*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPWriteTimeout, err = getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPIdleTimeout, err = getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPHeaderTimeout, err = getDuration("HTTP_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = getDuration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}

	if cfg.OutboxBatchSize, err = getInt("OUTBOX_BATCH_SIZE", 20, 1, 100); err != nil {
		return nil, err
	}
	if cfg.OutboxMaxAttempts, err = getInt("OUTBOX_MAX_ATTEMPTS", 10, 1, 100); err != nil {
		return nil, err
	}
	if cfg.RegistrationRateLimitPerMinute, err = getInt("REGISTRATION_RATE_LIMIT_PER_MINUTE", 30, 0, 10000); err != nil {
		return nil, err
	}
	maxRequestBytes, err := getInt64("MAX_REQUEST_BYTES", 1<<20, 1, 10<<20)
	if err != nil {
		return nil, err
	}
	cfg.MaxRequestBytes = maxRequestBytes

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	required := map[string]string{
		"DB_NAME":     cfg.DBName,
		"DB_USER":     cfg.DBUser,
		"DB_PASSWORD": cfg.DBPass,
		"DB_HOST":     cfg.DBHost,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must be configured", key)
		}
	}

	switch cfg.DBSSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("invalid DB_SSLMODE: unsupported SSL mode")
	}

	if strings.TrimSpace(cfg.BindHost) == "" {
		return fmt.Errorf("BIND_HOST must be configured")
	}

	if cfg.AuthJWTPublicKeysJSON == "" {
		return fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON must be configured")
	}
	keys := map[string]string{}
	if err := json.Unmarshal([]byte(cfg.AuthJWTPublicKeysJSON), &keys); err != nil {
		return fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON must be a JSON object: %w", err)
	}
	if len(keys) == 0 {
		return fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON must contain at least one key")
	}
	for kid, pemValue := range keys {
		if strings.TrimSpace(kid) == "" || strings.TrimSpace(pemValue) == "" {
			return fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON contains an empty key id or key")
		}
		if err := validateEd25519PublicKeyPEM(pemValue); err != nil {
			return fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON key %q: %w", kid, err)
		}
	}
	cfg.AuthJWTPublicKeys = keys

	if strings.TrimSpace(cfg.AuthJWTIssuer) == "" {
		return fmt.Errorf("AUTH_JWT_ISSUER must be configured")
	}
	if strings.TrimSpace(cfg.AuthJWTAudience) == "" {
		return fmt.Errorf("AUTH_JWT_AUDIENCE must be configured")
	}

	if len(cfg.InternalServiceSecret) < 32 {
		return fmt.Errorf("INTERNAL_SERVICE_SECRET must be at least 32 characters")
	}
	if cfg.InternalServiceSecretPrevious != "" && len(cfg.InternalServiceSecretPrevious) < 32 {
		return fmt.Errorf("INTERNAL_SERVICE_SECRET_PREVIOUS must be at least 32 characters when set")
	}
	if len(cfg.AllowedInternalCallers) == 0 {
		return fmt.Errorf("INTERNAL_ALLOWED_CALLERS must contain at least one service identity")
	}
	if cfg.InternalAuthMaxSkew > 10*time.Minute {
		return fmt.Errorf("INTERNAL_AUTH_MAX_SKEW must not exceed 10 minutes")
	}
	if cfg.AdminBootstrapToken != "" && len(cfg.AdminBootstrapToken) < 32 {
		return fmt.Errorf("ADMIN_BOOTSTRAP_TOKEN must be at least 32 characters when set")
	}

	if cfg.AuthCookieEnabled {
		for key, value := range map[string]string{
			"ACCESS_TOKEN_COOKIE_NAME": cfg.AccessTokenCookieName,
			"CSRF_COOKIE_NAME":         cfg.CSRFCookieName,
		} {
			if !isValidHTTPToken(value) {
				return fmt.Errorf("%s must be a valid cookie name", key)
			}
		}
		if !isValidHTTPToken(cfg.CSRFHeaderName) {
			return fmt.Errorf("CSRF_HEADER_NAME must be a valid HTTP header name")
		}
	}

	parsed, err := url.Parse(cfg.AuthServiceInternalURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("AUTH_SERVICE_INTERNAL_URL must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("AUTH_SERVICE_INTERNAL_URL must use http or https")
	}

	for _, origin := range cfg.CORSAllowedOrigins {
		if origin == "*" && cfg.CORSAllowCredentials {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS cannot contain * when CORS_ALLOW_CREDENTIALS is true")
		}
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid LOG_LEVEL: must be debug, info, warn, or error")
	}

	return nil
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

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive duration", key)
	}
	return d, nil
}

func getInt(key string, fallback, min, max int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("invalid %s: must be an integer between %d and %d", key, min, max)
	}
	return n, nil
}

func getInt64(key string, fallback, min, max int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("invalid %s: must be an integer between %d and %d", key, min, max)
	}
	return n, nil
}

func getBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func validateEd25519PublicKeyPEM(value string) error {
	value = strings.ReplaceAll(value, `\n`, "\n")

	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return fmt.Errorf("must be valid PEM")
	}
	if block.Type != "PUBLIC KEY" {
		return fmt.Errorf("must be a PUBLIC KEY PEM block")
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	if _, ok := parsed.(ed25519.PublicKey); !ok {
		return fmt.Errorf("must be an Ed25519 public key")
	}

	return nil
}

func isValidHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r <= 0x20 || r >= 0x7f {
			return false
		}
		switch r {
		case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}':
			return false
		}
	}
	return true
}
