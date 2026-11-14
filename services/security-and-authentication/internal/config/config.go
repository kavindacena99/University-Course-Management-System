package config

import (
	"crypto/ed25519"
	"crypto/x509"
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

	JWTPrivateKey      ed25519.PrivateKey
	JWTKeyID           string
	JWTIssuer          string
	JWTAudience        string
	JWTClockSkew       time.Duration
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	SessionAbsoluteTTL time.Duration

	CookieEnabled          bool
	CookieSecure           bool
	CookieSameSite         string
	CookieDomain           string
	AccessTokenCookieName  string
	RefreshTokenCookieName string
	CSRFCookieName         string
	CSRFHeaderName         string
	TokenDelivery          string

	Argon2MemoryKiB   uint32
	Argon2Iterations  uint32
	Argon2Parallelism uint8
	Argon2SaltLength  uint32
	Argon2KeyLength   uint32

	MaxFailedLoginAttempts   int
	AccountLockoutDuration   time.Duration
	LoginRateLimitPerMinute  int
	ForgotRateLimitPerMinute int

	NotificationMode      string
	NotificationFilePath  string
	PublicBaseURL         string
	ActivationTokenTTL    time.Duration
	PasswordResetTokenTTL time.Duration

	InternalServiceID             string
	InternalServiceSecret         string
	InternalServiceSecretPrevious string
	AllowedInternalCallers        []string
	InternalAuthMaxSkew           time.Duration

	UserServiceInternalURL string
	EligibilityTimeout     time.Duration

	CORSAllowedOrigins   []string
	CORSAllowCredentials bool
	TrustedProxies       []string

	HTTPReadTimeout   time.Duration
	HTTPWriteTimeout  time.Duration
	HTTPIdleTimeout   time.Duration
	HTTPHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration

	MaxRequestBytes int64
	LogLevel        string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	port := getEnv("AUTH_SERVICE_PORT", "8081")
	if err := validatePort("AUTH_SERVICE_PORT", port); err != nil {
		return nil, err
	}

	dbPortValue := getEnv("DB_PORT", "5432")
	dbPort, err := strconv.Atoi(dbPortValue)
	if err != nil || dbPort < 1 || dbPort > 65535 {
		return nil, fmt.Errorf("invalid DB_PORT: must be an integer between 1 and 65535")
	}

	environment := getEnv("APP_ENV", "development")

	cfg := &Config{
		Environment: environment,
		ServiceName: "auth-service",
		BindHost:    getEnv("BIND_HOST", "127.0.0.1"),
		Port:        port,

		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    dbPort,
		DBName:    getEnv("DB_NAME", ""),
		DBUser:    getEnv("DB_USER", ""),
		DBPass:    os.Getenv("DB_PASSWORD"),
		DBSSLMode: getEnv("DB_SSLMODE", "disable"),

		JWTKeyID:    getEnv("AUTH_JWT_KEY_ID", ""),
		JWTIssuer:   getEnv("AUTH_JWT_ISSUER", "university-auth"),
		JWTAudience: getEnv("AUTH_JWT_AUDIENCE", "university-api"),

		CookieSameSite:         getEnv("AUTH_COOKIE_SAMESITE", "Lax"),
		CookieDomain:           os.Getenv("AUTH_COOKIE_DOMAIN"),
		AccessTokenCookieName:  getEnv("ACCESS_TOKEN_COOKIE_NAME", "access_token"),
		RefreshTokenCookieName: getEnv("REFRESH_TOKEN_COOKIE_NAME", "refresh_token"),
		CSRFCookieName:         getEnv("CSRF_COOKIE_NAME", "csrf_token"),
		CSRFHeaderName:         getEnv("CSRF_HEADER_NAME", "X-CSRF-Token"),
		TokenDelivery:          strings.ToLower(getEnv("AUTH_TOKEN_DELIVERY", "both")),

		NotificationMode:     strings.ToLower(getEnv("NOTIFICATIONS_MODE", "file")),
		NotificationFilePath: getEnv("NOTIFICATIONS_FILE_PATH", "./var/notifications.log"),
		PublicBaseURL:        getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),

		InternalServiceID:             getEnv("INTERNAL_SERVICE_ID", "auth-service"),
		InternalServiceSecret:         os.Getenv("INTERNAL_SERVICE_SECRET"),
		InternalServiceSecretPrevious: os.Getenv("INTERNAL_SERVICE_SECRET_PREVIOUS"),

		UserServiceInternalURL: getEnv("USER_SERVICE_INTERNAL_URL", "http://127.0.0.1:8082"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}

	cfg.AllowedInternalCallers = splitCSV(getEnv("INTERNAL_ALLOWED_CALLERS", "user-service,course-service,enrollment-service"))
	cfg.CORSAllowedOrigins = splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	cfg.TrustedProxies = splitCSV(getEnv("TRUSTED_PROXIES", "127.0.0.1,::1"))

	if cfg.CookieEnabled, err = getBool("AUTH_COOKIE_ENABLED", true); err != nil {
		return nil, err
	}
	if cfg.CookieSecure, err = getBool("AUTH_COOKIE_SECURE", environment == "production"); err != nil {
		return nil, err
	}
	if cfg.CORSAllowCredentials, err = getBool("CORS_ALLOW_CREDENTIALS", true); err != nil {
		return nil, err
	}

	if cfg.JWTPrivateKey, err = loadPrivateKey(); err != nil {
		return nil, err
	}

	if cfg.JWTClockSkew, err = getDuration("AUTH_JWT_CLOCK_SKEW", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.AccessTokenTTL, err = getDuration("AUTH_ACCESS_TOKEN_TTL", 10*time.Minute); err != nil {
		return nil, err
	}
	if cfg.RefreshTokenTTL, err = getDuration("AUTH_REFRESH_TOKEN_TTL", 24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.SessionAbsoluteTTL, err = getDuration("AUTH_SESSION_ABSOLUTE_TTL", 7*24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.AccountLockoutDuration, err = getDuration("AUTH_ACCOUNT_LOCKOUT_DURATION", 15*time.Minute); err != nil {
		return nil, err
	}
	if cfg.ActivationTokenTTL, err = getDuration("AUTH_ACTIVATION_TOKEN_TTL", 24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.PasswordResetTokenTTL, err = getDuration("AUTH_PASSWORD_RESET_TOKEN_TTL", 30*time.Minute); err != nil {
		return nil, err
	}
	if cfg.EligibilityTimeout, err = getDuration("USER_ELIGIBILITY_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}
	if cfg.InternalAuthMaxSkew, err = getDuration("INTERNAL_AUTH_MAX_SKEW", 2*time.Minute); err != nil {
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

	if cfg.Argon2MemoryKiB, err = getUint32("ARGON2_MEMORY_KIB", 65536, 8192, 1048576); err != nil {
		return nil, err
	}
	if cfg.Argon2Iterations, err = getUint32("ARGON2_ITERATIONS", 3, 1, 10); err != nil {
		return nil, err
	}
	parallelism, err := getInt("ARGON2_PARALLELISM", 2, 1, 16)
	if err != nil {
		return nil, err
	}
	cfg.Argon2Parallelism = uint8(parallelism)
	if cfg.Argon2SaltLength, err = getUint32("ARGON2_SALT_LENGTH", 16, 16, 64); err != nil {
		return nil, err
	}
	if cfg.Argon2KeyLength, err = getUint32("ARGON2_KEY_LENGTH", 32, 16, 64); err != nil {
		return nil, err
	}

	if cfg.MaxFailedLoginAttempts, err = getInt("AUTH_MAX_FAILED_LOGIN_ATTEMPTS", 5, 1, 100); err != nil {
		return nil, err
	}
	if cfg.LoginRateLimitPerMinute, err = getInt("LOGIN_RATE_LIMIT_PER_MINUTE", 10, 0, 10000); err != nil {
		return nil, err
	}
	if cfg.ForgotRateLimitPerMinute, err = getInt("FORGOT_PASSWORD_RATE_LIMIT_PER_MINUTE", 5, 0, 10000); err != nil {
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
	if _, ok := safeToken(cfg.JWTKeyID); !ok {
		return fmt.Errorf("AUTH_JWT_KEY_ID must be configured and contain only letters, digits, '-', '_', and '.'")
	}
	if strings.TrimSpace(cfg.JWTIssuer) == "" {
		return fmt.Errorf("AUTH_JWT_ISSUER must be configured")
	}
	if strings.TrimSpace(cfg.JWTAudience) == "" {
		return fmt.Errorf("AUTH_JWT_AUDIENCE must be configured")
	}
	if cfg.JWTClockSkew > 5*time.Minute {
		return fmt.Errorf("AUTH_JWT_CLOCK_SKEW must not exceed 5 minutes")
	}
	if cfg.AccessTokenTTL >= cfg.RefreshTokenTTL {
		return fmt.Errorf("AUTH_ACCESS_TOKEN_TTL must be shorter than AUTH_REFRESH_TOKEN_TTL")
	}
	if cfg.RefreshTokenTTL > cfg.SessionAbsoluteTTL {
		return fmt.Errorf("AUTH_REFRESH_TOKEN_TTL must not exceed AUTH_SESSION_ABSOLUTE_TTL")
	}

	switch cfg.TokenDelivery {
	case "body", "cookie", "both":
	default:
		return fmt.Errorf("AUTH_TOKEN_DELIVERY must be body, cookie, or both")
	}
	if cfg.TokenDelivery != "body" && !cfg.CookieEnabled {
		return fmt.Errorf("AUTH_COOKIE_ENABLED must be true when cookie token delivery is used")
	}

	if cfg.CookieEnabled {
		for key, value := range map[string]string{
			"ACCESS_TOKEN_COOKIE_NAME":  cfg.AccessTokenCookieName,
			"REFRESH_TOKEN_COOKIE_NAME": cfg.RefreshTokenCookieName,
			"CSRF_COOKIE_NAME":          cfg.CSRFCookieName,
		} {
			if !isValidHTTPToken(value) {
				return fmt.Errorf("%s must be a valid cookie name", key)
			}
		}
		if !isValidHTTPToken(cfg.CSRFHeaderName) {
			return fmt.Errorf("CSRF_HEADER_NAME must be a valid HTTP header name")
		}
	}

	switch strings.ToLower(cfg.CookieSameSite) {
	case "lax", "strict", "none":
	default:
		return fmt.Errorf("AUTH_COOKIE_SAMESITE must be Lax, Strict, or None")
	}
	if strings.EqualFold(cfg.CookieSameSite, "none") && !cfg.CookieSecure {
		return fmt.Errorf("AUTH_COOKIE_SECURE must be true when SameSite=None")
	}
	if cfg.CookieDomain != "" && strings.ContainsAny(cfg.CookieDomain, " \t\r\n/") {
		return fmt.Errorf("AUTH_COOKIE_DOMAIN must be a valid domain")
	}

	switch cfg.NotificationMode {
	case "file", "log", "none":
	default:
		return fmt.Errorf("NOTIFICATIONS_MODE must be file, log, or none")
	}
	if cfg.Environment == "production" && cfg.NotificationMode != "none" {
		return fmt.Errorf("production requires an external notification provider; " +
			"only NOTIFICATIONS_MODE=none is supported without one")
	}
	if cfg.NotificationMode == "file" && strings.TrimSpace(cfg.NotificationFilePath) == "" {
		return fmt.Errorf("NOTIFICATIONS_FILE_PATH must be configured for file notifications")
	}

	publicURL, err := url.Parse(cfg.PublicBaseURL)
	if err != nil || publicURL.Scheme == "" || publicURL.Host == "" {
		return fmt.Errorf("PUBLIC_BASE_URL must be an absolute URL")
	}
	if publicURL.Scheme != "http" && publicURL.Scheme != "https" {
		return fmt.Errorf("PUBLIC_BASE_URL must use http or https")
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

	userServiceURL, err := url.Parse(cfg.UserServiceInternalURL)
	if err != nil || userServiceURL.Scheme == "" || userServiceURL.Host == "" {
		return fmt.Errorf("USER_SERVICE_INTERNAL_URL must be an absolute URL")
	}
	if userServiceURL.Scheme != "http" && userServiceURL.Scheme != "https" {
		return fmt.Errorf("USER_SERVICE_INTERNAL_URL must use http or https")
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

func loadPrivateKey() (ed25519.PrivateKey, error) {
	filePath := strings.TrimSpace(os.Getenv("AUTH_JWT_PRIVATE_KEY_FILE"))
	pemValue := os.Getenv("AUTH_JWT_PRIVATE_KEY_PEM")

	if filePath != "" && pemValue != "" {
		return nil, fmt.Errorf("configure either AUTH_JWT_PRIVATE_KEY_FILE or AUTH_JWT_PRIVATE_KEY_PEM, not both")
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read AUTH_JWT_PRIVATE_KEY_FILE: %w", err)
		}
		pemValue = string(data)
	}
	if strings.TrimSpace(pemValue) == "" {
		return nil, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_FILE or AUTH_JWT_PRIVATE_KEY_PEM must be configured")
	}

	pemValue = strings.ReplaceAll(pemValue, `\n`, "\n")
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_PEM is not valid PEM")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_PEM must be a PRIVATE KEY PEM block")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse JWT private key: %w", err)
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_PEM is not an Ed25519 private key")
	}

	return privateKey, nil
}

func safeToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return "", false
		}
	}
	return value, true
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

func getUint32(key string, fallback, min, max uint32) (uint32, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil || uint32(n) < min || uint32(n) > max {
		return 0, fmt.Errorf("invalid %s: must be an integer between %d and %d", key, min, max)
	}
	return uint32(n), nil
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

func getBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s: must be true or false", key)
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
		part = strings.TrimSpace(part)
		if part != "" {
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
