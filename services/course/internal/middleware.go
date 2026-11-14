package course

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const claimsContextKey = "auth_claims"

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}

		SetRequestID(c, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func AccessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		logger.Info("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(startedAt).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", RequestID(c),
		)
	}
}

func RecoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logger.Error("panic_recovered",
				"error", fmt.Sprint(recovered),
				"path", c.Request.URL.Path,
				"request_id", RequestID(c),
			)

			if c.Writer.Written() {
				c.Abort()
				return
			}

			AbortWithError(c, http.StatusInternalServerError, CodeInternal,
				"An internal error occurred.")
		}()

		c.Next()
	}
}

func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		header.Set("Cross-Origin-Resource-Policy", "same-site")
		header.Set("Cache-Control", "no-store")
		c.Next()
	}
}

func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}

		if c.Request.ContentLength > maxBytes {
			AbortWithError(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				"The request body is too large.")
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func CORSMiddleware(allowedOrigins []string, allowCredentials bool) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			allowAll = true
			continue
		}
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		_, isAllowed := allowed[origin]
		if !isAllowed && !allowAll {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		if allowAll && !allowCredentials {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		if allowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")

		if c.Request.Method != http.MethodOptions {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers",
			"Authorization, Content-Type, X-CSRF-Token, X-Request-ID")
		c.Header("Access-Control-Max-Age", "600")
		c.AbortWithStatus(http.StatusNoContent)
	}
}

func AuthMiddleware(
	verifier *TokenVerifier,
	cookieEnabled bool,
	accessTokenCookieName string,
	csrfCookieName string,
	csrfHeaderName string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, fromCookie, ok := extractAccessToken(
			c,
			cookieEnabled,
			accessTokenCookieName,
		)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
				"Authentication is required.")
			return
		}

		claims, err := verifier.Verify(rawToken)
		if err != nil {
			if errors.Is(err, ErrTokenExpired) {
				AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
					"The access token has expired.")
				return
			}
			AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
				"The access token is invalid.")
			return
		}

		if fromCookie && isUnsafeMethod(c.Request.Method) {
			csrfCookie, err := c.Cookie(csrfCookieName)
			csrfHeader := c.GetHeader(csrfHeaderName)
			if err != nil || csrfCookie == "" || csrfHeader == "" ||
				subtle.ConstantTimeCompare([]byte(csrfCookie), []byte(csrfHeader)) != 1 {
				AbortWithError(c, http.StatusForbidden, CodeCSRFFailed,
					"CSRF validation failed.")
				return
			}
		}

		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
				"Authentication is required.")
			return
		}

		if !HasAnyRole(claims, roles...) {
			AbortWithError(c, http.StatusForbidden, CodeForbidden,
				"You do not have permission to perform this action.")
			return
		}

		c.Next()
	}
}

func RequirePermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
				"Authentication is required.")
			return
		}

		if !HasAnyPermission(claims, permissions...) {
			AbortWithError(c, http.StatusForbidden, CodeForbidden,
				"You do not have permission to perform this action.")
			return
		}

		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) (*AccessClaims, bool) {
	value, exists := c.Get(claimsContextKey)
	if !exists {
		return nil, false
	}
	claims, ok := value.(*AccessClaims)
	return claims, ok
}

func extractAccessToken(
	c *gin.Context,
	cookieEnabled bool,
	cookieName string,
) (string, bool, bool) {
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	if authorization != "" {
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return "", false, false
		}
		token := strings.TrimSpace(parts[1])
		if token == "" {
			return "", false, false
		}
		return token, false, true
	}

	if !cookieEnabled {
		return "", false, false
	}

	token, err := c.Cookie(cookieName)
	if err != nil || strings.TrimSpace(token) == "" {
		return "", false, false
	}
	return token, true, true
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
