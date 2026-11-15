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

	EnrollmentServiceInternalURL string
	EnrollmentServiceTimeout     time.Duration
	AnalyticsServiceInternalURL  string

	ObjectStorageEndpoint  string
	ObjectStorageAccessKey string
	ObjectStorageSecretKey string
	ObjectStorageBucket    string
	ObjectStorageUseSSL    bool
	MaxUploadBytes         int64

	CORSAllowedOrigins   []string
	CORSAllowCredentials bool
	TrustedProxies       []string

	HTTPReadTimeout   time.Duration
	HTTPWriteTimeout  time.Duration
	HTTPIdleTimeout   time.Duration
	HTTPHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
	MaxRequestBytes   int64
	LogLevel          string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	port := getEnv("ASSESSMENT_SERVICE_PORT", "8085")
	if err := validatePort("ASSESSMENT_SERVICE_PORT", port); err != nil {
		return nil, err
	}
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil || dbPort < 1 || dbPort > 65535 {
		return nil, fmt.Errorf("invalid DB_PORT: must be an integer between 1 and 65535")
	}

	cfg := &Config{
		Environment: getEnv("APP_ENV", "development"),
		ServiceName: "assessment-service",
		BindHost:    getEnv("BIND_HOST", "127.0.0.1"),
		Port:        port,
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      dbPort,
		DBName:      os.Getenv("DB_NAME"),
		DBUser:      os.Getenv("DB_USER"),
		DBPass:      os.Getenv("DB_PASSWORD"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),

		AuthJWTIssuer:         getEnv("AUTH_JWT_ISSUER", "university-auth"),
		AuthJWTAudience:       getEnv("AUTH_JWT_AUDIENCE", "university-api"),
		AuthCookieEnabled:     true,
		AccessTokenCookieName: getEnv("ACCESS_TOKEN_COOKIE_NAME", "access_token"),
		CSRFCookieName:        getEnv("CSRF_COOKIE_NAME", "csrf_token"),
		CSRFHeaderName:        getEnv("CSRF_HEADER_NAME", "X-CSRF-Token"),

		InternalServiceID:             getEnv("INTERNAL_SERVICE_ID", "assessment-service"),
		InternalServiceSecret:         os.Getenv("INTERNAL_SERVICE_SECRET"),
		InternalServiceSecretPrevious: os.Getenv("INTERNAL_SERVICE_SECRET_PREVIOUS"),
		InternalAllowedCallers:        splitCSV(getEnv("INTERNAL_ALLOWED_CALLERS", "")),
		EnrollmentServiceInternalURL:  getEnv("ENROLLMENT_SERVICE_INTERNAL_URL", "http://127.0.0.1:8084"),
		AnalyticsServiceInternalURL:   strings.TrimSpace(os.Getenv("ANALYTICS_SERVICE_INTERNAL_URL")),

		ObjectStorageEndpoint:  getEnv("OBJECT_STORAGE_ENDPOINT", "127.0.0.1:9000"),
		ObjectStorageAccessKey: os.Getenv("OBJECT_STORAGE_ACCESS_KEY"),
		ObjectStorageSecretKey: os.Getenv("OBJECT_STORAGE_SECRET_KEY"),
		ObjectStorageBucket:    getEnv("OBJECT_STORAGE_BUCKET", "submissions"),
		ObjectStorageUseSSL:    false,
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}
	if cfg.AuthCookieEnabled, err = getBool("AUTH_COOKIE_ENABLED", true); err != nil {
		return nil, err
	}
	if cfg.ObjectStorageUseSSL, err = getBool("OBJECT_STORAGE_USE_SSL", false); err != nil {
		return nil, err
	}
	if cfg.AuthJWTPublicKeys, err = parseJWTKeys(os.Getenv("AUTH_JWT_PUBLIC_KEYS_JSON")); err != nil {
		return nil, err
	}
	if cfg.AuthJWTClockSkew, err = getDuration("AUTH_JWT_CLOCK_SKEW", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.InternalAuthMaxSkew, err = getDuration("INTERNAL_AUTH_MAX_SKEW", 2*time.Minute); err != nil {
		return nil, err
	}
	if cfg.EnrollmentServiceTimeout, err = getDuration("ENROLLMENT_SERVICE_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPReadTimeout, err = getDuration("HTTP_READ_TIMEOUT", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPWriteTimeout, err = getDuration("HTTP_WRITE_TIMEOUT", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.HTTPIdleTimeout, err = getDuration("HTTP_IDLE_TIMEOUT", time.Minute); err != nil {
		return nil, err
	}
	if cfg.HTTPHeaderTimeout, err = getDuration("HTTP_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = getDuration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.MaxUploadBytes, err = getInt64("MAX_UPLOAD_BYTES", 20<<20, 1, 100<<20); err != nil {
		return nil, err
	}
	if cfg.MaxRequestBytes, err = getInt64("MAX_REQUEST_BYTES", cfg.MaxUploadBytes+(3<<20), 1, 110<<20); err != nil {
		return nil, err
	}
	cfg.CORSAllowedOrigins = splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if cfg.CORSAllowCredentials, err = getBool("CORS_ALLOW_CREDENTIALS", true); err != nil {
		return nil, err
	}
	cfg.TrustedProxies = splitCSV(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1"))

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validate(cfg *Config) error {
	for key, value := range map[string]string{
		"DB_NAME": cfg.DBName, "DB_USER": cfg.DBUser, "DB_PASSWORD": cfg.DBPass,
		"DB_HOST": cfg.DBHost, "OBJECT_STORAGE_ACCESS_KEY": cfg.ObjectStorageAccessKey,
		"OBJECT_STORAGE_SECRET_KEY": cfg.ObjectStorageSecretKey,
	} {
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
	if cfg.AuthJWTClockSkew < 0 {
		return fmt.Errorf("AUTH_JWT_CLOCK_SKEW must not be negative")
	}
	if strings.TrimSpace(cfg.AuthJWTIssuer) == "" || strings.TrimSpace(cfg.AuthJWTAudience) == "" {
		return fmt.Errorf("AUTH_JWT_ISSUER and AUTH_JWT_AUDIENCE must be configured")
	}
	if strings.TrimSpace(cfg.InternalServiceID) == "" {
		return fmt.Errorf("INTERNAL_SERVICE_ID must be configured")
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
	if cfg.EnrollmentServiceTimeout <= 0 {
		return fmt.Errorf("ENROLLMENT_SERVICE_TIMEOUT must be positive")
	}
	if cfg.ObjectStorageEndpoint == "" || strings.Contains(cfg.ObjectStorageEndpoint, "://") {
		return fmt.Errorf("OBJECT_STORAGE_ENDPOINT must be a host:port value")
	}
	if cfg.ObjectStorageBucket == "" {
		return fmt.Errorf("OBJECT_STORAGE_BUCKET must be configured")
	}
	if cfg.MaxRequestBytes <= cfg.MaxUploadBytes {
		return fmt.Errorf("MAX_REQUEST_BYTES must exceed MAX_UPLOAD_BYTES")
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
	parsed, err := url.Parse(cfg.EnrollmentServiceInternalURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("ENROLLMENT_SERVICE_INTERNAL_URL must be an absolute HTTP URL without a path")
	}
	if cfg.AnalyticsServiceInternalURL != "" {
		parsed, err := url.Parse(cfg.AnalyticsServiceInternalURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("ANALYTICS_SERVICE_INTERNAL_URL must be an absolute HTTP URL without credentials or a path")
		}
	}
	for _, origin := range cfg.CORSAllowedOrigins {
		if origin == "*" && cfg.CORSAllowCredentials {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS cannot contain * when credentials are enabled")
		}
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
	return nil
}

func parseJWTKeys(value string) (map[string]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON must be configured")
	}
	keys := map[string]string{}
	if err := json.Unmarshal([]byte(value), &keys); err != nil || len(keys) == 0 {
		return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON must be a non-empty JSON object")
	}
	for kid, pemValue := range keys {
		if strings.TrimSpace(kid) == "" {
			return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON contains an empty key id")
		}
		pemValue = strings.ReplaceAll(pemValue, `\n`, "\n")
		block, _ := pem.Decode([]byte(pemValue))
		if block == nil || block.Type != "PUBLIC KEY" {
			return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON key %q must be a PUBLIC KEY PEM", kid)
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON key %q is invalid", kid)
		}
		if _, ok := parsed.(ed25519.PublicKey); !ok {
			return nil, fmt.Errorf("AUTH_JWT_PUBLIC_KEYS_JSON key %q must be Ed25519", kid)
		}
	}
	return keys, nil
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
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid %s: must be a positive duration", key)
	}
	return duration, nil
}

func getInt64(key string, fallback, min, max int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < min || number > max {
		return 0, fmt.Errorf("invalid %s: must be between %d and %d", key, min, max)
	}
	return number, nil
}

func getBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s: must be a boolean", key)
	}
	return parsed, nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
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
