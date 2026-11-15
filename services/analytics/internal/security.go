package analytics

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"analytics/internal/config"
)

const (
	claimsContextKey         = "auth_claims"
	internalCallerContextKey = "internal_caller"
	HeaderServiceID          = "X-Service-Id"
	HeaderServiceTimestamp   = "X-Service-Timestamp"
	HeaderServiceNonce       = "X-Service-Nonce"
	HeaderServiceSignature   = "X-Service-Signature"
)

type AccessClaims struct {
	SessionID   string   `json:"sid"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"perms"`
	TokenType   string   `json:"typ"`
	jwt.RegisteredClaims
}

func (c *AccessClaims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

type TokenVerifier struct {
	keys   map[string]ed25519.PublicKey
	parser *jwt.Parser
}

func NewTokenVerifier(cfg *config.Config) (*TokenVerifier, error) {
	keys := make(map[string]ed25519.PublicKey, len(cfg.AuthJWTPublicKeys))
	for keyID, encoded := range cfg.AuthJWTPublicKeys {
		encoded = strings.ReplaceAll(encoded, `\n`, "\n")
		block, _ := pem.Decode([]byte(encoded))
		if block == nil || block.Type != "PUBLIC KEY" {
			return nil, fmt.Errorf("invalid public key %q", keyID)
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse public key %q: %w", keyID, err)
		}
		key, ok := parsed.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key %q is not Ed25519", keyID)
		}
		keys[keyID] = key
	}
	return &TokenVerifier{
		keys: keys,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
			jwt.WithIssuer(cfg.AuthJWTIssuer),
			jwt.WithAudience(cfg.AuthJWTAudience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(cfg.AuthJWTClockSkew),
		),
	}, nil
}

func (v *TokenVerifier) Verify(value string) (*AccessClaims, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 16384 {
		return nil, errors.New("invalid access token")
	}
	claims := &AccessClaims{}
	token, err := v.parser.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		keyID, ok := token.Header["kid"].(string)
		if !ok || keyID == "" || token.Method != jwt.SigningMethodEdDSA {
			return nil, errors.New("invalid access token")
		}
		key, ok := v.keys[keyID]
		if !ok {
			return nil, errors.New("unknown signing key")
		}
		return key, nil
	})
	if err != nil || !token.Valid || claims.TokenType != "access" ||
		claims.SessionID == "" || len(claims.Roles) == 0 {
		return nil, errors.New("invalid access token")
	}
	if _, err := claims.UserID(); err != nil {
		return nil, errors.New("invalid access token subject")
	}
	return claims, nil
}

func AuthMiddleware(
	verifier *TokenVerifier,
	cookieEnabled bool,
	accessCookie, csrfCookie, csrfHeader string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("Authorization"))
		fromCookie := false
		if token != "" {
			parts := strings.SplitN(token, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") ||
				strings.TrimSpace(parts[1]) == "" {
				writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
				return
			}
			token = strings.TrimSpace(parts[1])
		} else if cookieEnabled {
			value, err := c.Cookie(accessCookie)
			if err != nil || value == "" {
				writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
				return
			}
			token, fromCookie = value, true
		} else {
			writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		claims, err := verifier.Verify(token)
		if err != nil {
			writeError(c, http.StatusUnauthorized, "unauthorized", "The access token is invalid or expired.")
			return
		}
		if fromCookie && c.Request.Method != http.MethodGet &&
			c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			cookieValue, cookieErr := c.Cookie(csrfCookie)
			headerValue := c.GetHeader(csrfHeader)
			if cookieErr != nil || cookieValue == "" || headerValue == "" ||
				subtle.ConstantTimeCompare([]byte(cookieValue), []byte(headerValue)) != 1 {
				writeError(c, http.StatusForbidden, "csrf_validation_failed", "CSRF validation failed.")
				return
			}
		}
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

func InternalAuthMiddleware(
	db *pgxpool.Pool, cfg *config.Config, maxBodyBytes int64,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes+1))
		if err != nil || int64(len(body)) > maxBodyBytes {
			writeError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "The request body is too large.")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		serviceID := c.GetHeader(HeaderServiceID)
		timestampValue := c.GetHeader(HeaderServiceTimestamp)
		nonce := c.GetHeader(HeaderServiceNonce)
		providedSignature := c.GetHeader(HeaderServiceSignature)
		if !isAllowedCaller(cfg.InternalAllowedCallers, serviceID) ||
			len(nonce) < 16 || len(nonce) > 64 {
			writeError(c, http.StatusUnauthorized, "unauthorized", "The internal request is invalid.")
			return
		}
		timestamp, err := time.Parse(time.RFC3339, timestampValue)
		if err != nil || time.Since(timestamp) > cfg.InternalAuthMaxSkew ||
			time.Until(timestamp) > cfg.InternalAuthMaxSkew {
			writeError(c, http.StatusUnauthorized, "unauthorized", "The internal request has expired.")
			return
		}
		canonical := canonicalRequest(c.Request.Method, requestURI(c.Request),
			serviceID, timestampValue, nonce, body)
		if !validSignature(providedSignature, canonical, cfg.InternalServiceSecret) &&
			!validSignature(providedSignature, canonical, cfg.InternalServiceSecretPrevious) {
			writeError(c, http.StatusUnauthorized, "unauthorized", "The internal request signature is invalid.")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if _, err = db.Exec(ctx, `DELETE FROM internal_request_nonces WHERE expires_at < now()`); err != nil {
			writeError(c, http.StatusServiceUnavailable, "internal_auth_unavailable", "Internal request authentication is unavailable.")
			return
		}
		_, err = db.Exec(ctx,
			`INSERT INTO internal_request_nonces (nonce, expires_at) VALUES ($1, $2)`,
			serviceID+":"+nonce, timestamp.Add(2*cfg.InternalAuthMaxSkew))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(c, http.StatusUnauthorized, "unauthorized", "The internal request nonce has already been used.")
				return
			}
			writeError(c, http.StatusServiceUnavailable, "internal_auth_unavailable", "Internal request authentication is unavailable.")
			return
		}
		c.Set(internalCallerContextKey, serviceID)
		c.Next()
	}
}

func RequireInternalCaller(serviceID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := c.Get(internalCallerContextKey)
		if !ok || caller != serviceID {
			writeError(c, http.StatusForbidden, "forbidden", "The internal caller is not allowed.")
			return
		}
		c.Next()
	}
}

func ClaimsFromContext(c *gin.Context) (*AccessClaims, bool) {
	claims, ok := c.Get(claimsContextKey)
	if !ok {
		return nil, false
	}
	result, ok := claims.(*AccessClaims)
	return result, ok
}

func HasRole(claims *AccessClaims, role string) bool {
	if claims == nil {
		return false
	}
	for _, current := range claims.Roles {
		if strings.EqualFold(current, role) {
			return true
		}
	}
	return false
}

func HasPermission(claims *AccessClaims, permission string) bool {
	if claims == nil {
		return false
	}
	for _, current := range claims.Permissions {
		if current == permission {
			return true
		}
	}
	return false
}

func SignRequest(request *http.Request, serviceID, secret string, body []byte) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	request.Header.Set(HeaderServiceID, serviceID)
	request.Header.Set(HeaderServiceTimestamp, timestamp)
	request.Header.Set(HeaderServiceNonce, nonce)
	request.Header.Set(HeaderServiceSignature, signature(secret,
		canonicalRequest(request.Method, requestURI(request), serviceID, timestamp, nonce, body)))
}

func canonicalRequest(method, uri, serviceID, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	return strings.Join([]string{
		strings.ToUpper(method), uri, serviceID, timestamp, nonce, hex.EncodeToString(digest[:]),
	}, "\n")
}

func signature(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func validSignature(provided, canonical, secret string) bool {
	if secret == "" {
		return false
	}
	expected, err := hex.DecodeString(signature(secret, canonical))
	actual, decodeErr := hex.DecodeString(provided)
	return err == nil && decodeErr == nil &&
		subtle.ConstantTimeCompare(expected, actual) == 1
}

func requestURI(request *http.Request) string {
	path := request.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if request.URL.RawQuery != "" {
		path += "?" + request.URL.RawQuery
	}
	return path
}

func isAllowedCaller(allowed []string, caller string) bool {
	for _, candidate := range allowed {
		if candidate == caller {
			return true
		}
	}
	return false
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{
		"code": code, "message": message, "request_id": c.Writer.Header().Get("X-Request-ID"),
	}})
}
