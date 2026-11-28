package assessment

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
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

	"assessment/internal/config"
)

const (
	claimsContextKey       = "auth_claims"
	requestIDContextKey    = "request_id"
	outboundRequestIDKey   = "assessment_request_id"
	HeaderServiceID        = "X-Service-Id"
	HeaderServiceTimestamp = "X-Service-Timestamp"
	HeaderServiceNonce     = "X-Service-Nonce"
	HeaderServiceSignature = "X-Service-Signature"
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
		publicKey, ok := parsed.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key %q is not Ed25519", keyID)
		}
		keys[keyID] = publicKey
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
		return nil, ErrInvalidToken
	}
	claims := &AccessClaims{}
	token, err := v.parser.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodEdDSA {
			return nil, ErrInvalidToken
		}
		keyID, ok := token.Header["kid"].(string)
		if !ok || keyID == "" {
			return nil, ErrInvalidToken
		}
		key, ok := v.keys[keyID]
		if !ok {
			return nil, ErrInvalidToken
		}
		return key, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid || claims.TokenType != "access" {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.Subject); err != nil || claims.SessionID == "" || len(claims.Roles) == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func AuthMiddleware(
	verifier *TokenVerifier,
	cookieEnabled bool,
	accessCookie, csrfCookie, csrfHeader string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := strings.TrimSpace(c.GetHeader("Authorization"))
		fromCookie := false
		if value != "" {
			parts := strings.SplitN(value, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				AbortWithError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
				return
			}
			value = strings.TrimSpace(parts[1])
		} else if cookieEnabled {
			cookieValue, err := c.Cookie(accessCookie)
			if err != nil || cookieValue == "" {
				AbortWithError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
				return
			}
			value, fromCookie = cookieValue, true
		} else {
			AbortWithError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		claims, err := verifier.Verify(value)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, "unauthorized", "The access token is invalid or expired.")
			return
		}
		if fromCookie && isUnsafeMethod(c.Request.Method) {
			cookieValue, err := c.Cookie(csrfCookie)
			headerValue := c.GetHeader(csrfHeader)
			if err != nil || cookieValue == "" || headerValue == "" ||
				subtle.ConstantTimeCompare([]byte(cookieValue), []byte(headerValue)) != 1 {
				AbortWithError(c, http.StatusForbidden, "csrf_validation_failed", "CSRF validation failed.")
				return
			}
		}
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

type ServiceSigner struct {
	serviceID string
	secret    []byte
}

func NewServiceSigner(serviceID, secret string) (*ServiceSigner, error) {
	if strings.TrimSpace(serviceID) == "" || len(secret) < 32 {
		return nil, fmt.Errorf("service identity and a 32-character internal secret are required")
	}
	return &ServiceSigner{serviceID: serviceID, secret: []byte(secret)}, nil
}

func (s *ServiceSigner) Sign(request *http.Request, body []byte) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	request.Header.Set(HeaderServiceID, s.serviceID)
	request.Header.Set(HeaderServiceTimestamp, timestamp)
	request.Header.Set(HeaderServiceNonce, nonce)
	request.Header.Set(HeaderServiceSignature, signature(s.secret, canonicalRequest(
		request.Method, requestURI(request), s.serviceID, timestamp, nonce, body,
	)))
}

type EnrollmentClient struct {
	baseURL string
	signer  *ServiceSigner
	client  *http.Client
}

var _ EnrollmentReader = (*EnrollmentClient)(nil)

func NewEnrollmentClient(baseURL string, signer *ServiceSigner, timeout time.Duration) (*EnrollmentClient, error) {
	if strings.TrimSpace(baseURL) == "" || signer == nil || timeout <= 0 {
		return nil, fmt.Errorf("enrollment URL, signer, and positive timeout are required")
	}
	return &EnrollmentClient{
		baseURL: strings.TrimRight(baseURL, "/"), signer: signer,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *EnrollmentClient) ActiveStudent(
	ctx context.Context,
	studentID, offeringID uuid.UUID,
) (bool, error) {
	path := "/internal/offerings/" + offeringID.String() +
		"/enrollments/" + studentID.String() + "/active"
	return c.getEligibility(ctx, path, "student_user_id", studentID, offeringID)
}

func (c *EnrollmentClient) ActiveLecturer(
	ctx context.Context,
	lecturerID, offeringID uuid.UUID,
) (bool, error) {
	path := "/internal/offerings/" + offeringID.String() +
		"/lecturers/" + lecturerID.String() + "/active"
	return c.getEligibility(ctx, path, "lecturer_user_id", lecturerID, offeringID)
}

func (c *EnrollmentClient) getEligibility(
	ctx context.Context,
	path, identityKey string,
	userID, offeringID uuid.UUID,
) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, fmt.Errorf("%w: create enrollment request: %v", ErrDependencyUnavailable, err)
	}
	request.Header.Set("Accept", "application/json")
	if requestID, _ := ctx.Value(outboundRequestIDKey).(string); requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	c.signer.Sign(request, nil)
	response, err := c.client.Do(request)
	if err != nil {
		return false, fmt.Errorf("%w: enrollment request failed", ErrDependencyUnavailable)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10+1))
	if err != nil || len(body) > 64<<10 {
		return false, fmt.Errorf("%w: invalid enrollment response", ErrDependencyUnavailable)
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%w: enrollment service returned %d", ErrDependencyUnavailable, response.StatusCode)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("%w: malformed enrollment response", ErrDependencyUnavailable)
	}
	active, ok := result["active"].(bool)
	actualUserID, userOK := result[identityKey].(string)
	actualOfferingID, offeringOK := result["offering_id"].(string)
	if !ok || !userOK || !offeringOK || actualUserID != userID.String() ||
		actualOfferingID != offeringID.String() {
		return false, fmt.Errorf("%w: malformed enrollment identity response", ErrDependencyUnavailable)
	}
	return active, nil
}

func canonicalRequest(method, uri, serviceID, timestamp, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	return strings.Join([]string{
		strings.ToUpper(method), uri, serviceID, timestamp, nonce, hex.EncodeToString(bodyHash[:]),
	}, "\n")
}

func signature(secret []byte, canonical string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func requestURI(request *http.Request) string {
	value := request.URL.EscapedPath()
	if value == "" {
		value = "/"
	}
	if request.URL.RawQuery != "" {
		value += "?" + request.URL.RawQuery
	}
	return value
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}
		c.Set(requestIDContextKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Request = c.Request.WithContext(context.WithValue(
			c.Request.Context(), outboundRequestIDKey, requestID,
		))
		c.Next()
	}
}

func RequestID(c *gin.Context) string {
	value, _ := c.Get(requestIDContextKey)
	requestID, _ := value.(string)
	return requestID
}

func ClaimsFromContext(c *gin.Context) (*AccessClaims, bool) {
	value, exists := c.Get(claimsContextKey)
	if !exists {
		return nil, false
	}
	claims, ok := value.(*AccessClaims)
	return claims, ok
}

func actorFromClaims(claims *AccessClaims) (Actor, bool) {
	if claims == nil {
		return Actor{}, false
	}
	userID, err := claims.UserID()
	if err != nil || userID == uuid.Nil {
		return Actor{}, false
	}
	return Actor{UserID: userID, Roles: claims.Roles, Permissions: claims.Permissions}, true
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

func requestBodyHash(body []byte) [32]byte {
	return sha256.Sum256(body)
}

func signRequestBody(request *http.Request, secret []byte, serviceID string, body []byte) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	digest := requestBodyHash(body)
	canonical := strings.Join([]string{
		strings.ToUpper(request.Method), requestURI(request), serviceID, timestamp, nonce,
		hex.EncodeToString(digest[:]),
	}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	request.Header.Set(HeaderServiceID, serviceID)
	request.Header.Set(HeaderServiceTimestamp, timestamp)
	request.Header.Set(HeaderServiceNonce, nonce)
	request.Header.Set(HeaderServiceSignature, hex.EncodeToString(mac.Sum(nil)))
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func isUnsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func readStrictBody(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("body exceeds limit")
	}
	return body, nil
}

func bodyReader(body []byte) io.Reader {
	return bytes.NewReader(body)
}
