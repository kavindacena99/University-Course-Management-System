package assessment

import (
	"bytes"
	"crypto/hmac"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"assessment/internal/config"
)

const internalCallerContextKey = "internal_caller"

func InternalAuthMiddleware(
	db *pgxpool.Pool, cfg *config.Config, maxBodyBytes int64,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes+1))
		if err != nil || int64(len(body)) > maxBodyBytes {
			AbortWithError(c, http.StatusRequestEntityTooLarge,
				"payload_too_large", "The request body is too large.")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		serviceID := c.GetHeader(HeaderServiceID)
		timestampValue := c.GetHeader(HeaderServiceTimestamp)
		nonce := c.GetHeader(HeaderServiceNonce)
		provided := c.GetHeader(HeaderServiceSignature)
		if !internalCallerAllowed(cfg.InternalAllowedCallers, serviceID) ||
			len(nonce) < 16 || len(nonce) > 64 {
			AbortWithError(c, http.StatusUnauthorized, "unauthorized",
				"The internal request is invalid.")
			return
		}
		timestamp, err := time.Parse(time.RFC3339, timestampValue)
		if err != nil || time.Since(timestamp) > cfg.InternalAuthMaxSkew ||
			time.Until(timestamp) > cfg.InternalAuthMaxSkew {
			AbortWithError(c, http.StatusUnauthorized, "unauthorized",
				"The internal request has expired.")
			return
		}
		canonical := canonicalRequest(c.Request.Method, requestURI(c.Request),
			serviceID, timestampValue, nonce, body)
		if !internalSignatureValid(provided, canonical, cfg.InternalServiceSecret) &&
			!internalSignatureValid(provided, canonical, cfg.InternalServiceSecretPrevious) {
			AbortWithError(c, http.StatusUnauthorized, "unauthorized",
				"The internal request signature is invalid.")
			return
		}
		if _, err = db.Exec(c.Request.Context(),
			`DELETE FROM assessment_internal_request_nonces WHERE expires_at < now()`); err != nil {
			AbortWithError(c, http.StatusServiceUnavailable, "internal_auth_unavailable",
				"Internal request authentication is unavailable.")
			return
		}
		_, err = db.Exec(c.Request.Context(), `
			INSERT INTO assessment_internal_request_nonces (nonce, expires_at)
			VALUES ($1, $2)
		`, serviceID+":"+nonce, timestamp.Add(2*cfg.InternalAuthMaxSkew))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				AbortWithError(c, http.StatusUnauthorized, "unauthorized",
					"The internal request nonce has already been used.")
				return
			}
			AbortWithError(c, http.StatusServiceUnavailable, "internal_auth_unavailable",
				"Internal request authentication is unavailable.")
			return
		}
		c.Set(internalCallerContextKey, serviceID)
		c.Next()
	}
}

func RequireInternalCaller(serviceID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := c.Get(internalCallerContextKey)
		actual, isString := caller.(string)
		if !ok || !isString || actual != serviceID {
			AbortWithError(c, http.StatusForbidden, "forbidden",
				"The internal caller is not allowed.")
			return
		}
		c.Next()
	}
}

func internalCallerAllowed(allowed []string, caller string) bool {
	for _, candidate := range allowed {
		if candidate == caller {
			return true
		}
	}
	return false
}

func internalSignatureValid(provided, canonical, secret string) bool {
	if secret == "" {
		return false
	}
	expected := signature([]byte(secret), canonical)
	expectedBytes, err := hex.DecodeString(expected)
	providedBytes, providedErr := hex.DecodeString(provided)
	if err != nil || providedErr != nil {
		return false
	}
	return hmac.Equal(expectedBytes, providedBytes)
}

