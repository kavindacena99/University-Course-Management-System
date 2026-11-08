package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	HeaderServiceID        = "X-Service-Id"
	HeaderServiceTimestamp = "X-Service-Timestamp"
	HeaderServiceNonce     = "X-Service-Nonce"
	HeaderServiceSignature = "X-Service-Signature"
)

type ServiceSigner struct {
	ServiceID string
	secret    []byte
	now       func() time.Time
}

func NewServiceSigner(serviceID, secret string) (*ServiceSigner, error) {
	if strings.TrimSpace(serviceID) == "" {
		return nil, fmt.Errorf("service id is required")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("internal service secret must be at least 32 characters")
	}

	return &ServiceSigner{
		ServiceID: serviceID,
		secret:    []byte(secret),
		now:       time.Now,
	}, nil
}

func (s *ServiceSigner) Sign(req *http.Request, body []byte) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	timestamp := s.now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	signature := signCanonical(s.secret, canonicalRequest(
		req.Method,
		requestURI(req),
		s.ServiceID,
		timestamp,
		nonce,
		body,
	))

	req.Header.Set(HeaderServiceID, s.ServiceID)
	req.Header.Set(HeaderServiceTimestamp, timestamp)
	req.Header.Set(HeaderServiceNonce, nonce)
	req.Header.Set(HeaderServiceSignature, signature)

	return nil
}

type ServiceVerifier struct {
	secret     []byte
	previous   []byte
	allowed    map[string]struct{}
	maxSkew    time.Duration
	now        func() time.Time
	mu         sync.Mutex
	seenNonces map[string]time.Time
}

func NewServiceVerifier(
	currentSecret string,
	previousSecret string,
	allowedCallers []string,
	maxSkew time.Duration,
) (*ServiceVerifier, error) {
	if len(currentSecret) < 32 {
		return nil, fmt.Errorf("internal service secret must be at least 32 characters")
	}
	if maxSkew <= 0 || maxSkew > 10*time.Minute {
		return nil, fmt.Errorf("internal authentication skew must be between 0 and 10 minutes")
	}
	if len(allowedCallers) == 0 {
		return nil, fmt.Errorf("at least one allowed internal caller is required")
	}

	allowed := make(map[string]struct{}, len(allowedCallers))
	for _, caller := range allowedCallers {
		caller = strings.TrimSpace(caller)
		if caller == "" {
			return nil, fmt.Errorf("allowed internal caller must not be empty")
		}
		allowed[caller] = struct{}{}
	}

	verifier := &ServiceVerifier{
		secret:     []byte(currentSecret),
		allowed:    allowed,
		maxSkew:    maxSkew,
		now:        time.Now,
		seenNonces: make(map[string]time.Time),
	}
	if previousSecret != "" {
		if len(previousSecret) < 32 {
			return nil, fmt.Errorf("previous internal service secret must be at least 32 characters")
		}
		verifier.previous = []byte(previousSecret)
	}

	return verifier, nil
}

func (v *ServiceVerifier) Verify(req *http.Request, body []byte) (string, error) {
	serviceID := strings.TrimSpace(req.Header.Get(HeaderServiceID))
	if serviceID == "" {
		return "", fmt.Errorf("%w: missing service id", ErrInvalidServiceAuth)
	}
	if _, ok := v.allowed[serviceID]; !ok {
		return "", fmt.Errorf("%w: untrusted service", ErrInvalidServiceAuth)
	}

	timestampValue := strings.TrimSpace(req.Header.Get(HeaderServiceTimestamp))
	if timestampValue == "" {
		return "", fmt.Errorf("%w: missing timestamp", ErrInvalidServiceAuth)
	}
	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return "", fmt.Errorf("%w: invalid timestamp", ErrInvalidServiceAuth)
	}

	now := v.now().UTC()
	skew := now.Sub(timestamp)
	if skew < 0 {
		skew = -skew
	}
	if skew > v.maxSkew {
		return "", fmt.Errorf("%w: stale timestamp", ErrInvalidServiceAuth)
	}

	nonce := strings.TrimSpace(req.Header.Get(HeaderServiceNonce))
	if !validNonce(nonce) {
		return "", fmt.Errorf("%w: invalid nonce", ErrInvalidServiceAuth)
	}

	signatureValue := strings.TrimSpace(req.Header.Get(HeaderServiceSignature))
	signature, err := hex.DecodeString(signatureValue)
	if err != nil || len(signature) != sha256.Size {
		return "", fmt.Errorf("%w: invalid signature encoding", ErrInvalidServiceAuth)
	}

	canonical := canonicalRequest(
		req.Method,
		requestURI(req),
		serviceID,
		timestampValue,
		nonce,
		body,
	)
	expected := signCanonicalBytes(v.secret, canonical)
	if !hmac.Equal(signature, expected) {
		if len(v.previous) == 0 || !hmac.Equal(signature, signCanonicalBytes(v.previous, canonical)) {
			return "", fmt.Errorf("%w: signature mismatch", ErrInvalidServiceAuth)
		}
	}

	expiresAt := timestamp.Add(2 * v.maxSkew)
	if !v.acceptNonce(serviceID+"|"+nonce, expiresAt) {
		return "", fmt.Errorf("%w: replayed nonce", ErrInvalidServiceAuth)
	}

	return serviceID, nil
}

func (v *ServiceVerifier) acceptNonce(key string, expiresAt time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now().UTC()
	for existingKey, expiry := range v.seenNonces {
		if now.After(expiry) {
			delete(v.seenNonces, existingKey)
		}
	}

	if _, exists := v.seenNonces[key]; exists {
		return false
	}
	v.seenNonces[key] = expiresAt

	return true
}

func canonicalRequest(method, requestURI, serviceID, timestamp, nonce string, body []byte) string {
	bodyHash := sha256.Sum256(body)

	return strings.Join([]string{
		strings.ToUpper(method),
		requestURI,
		serviceID,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
}

func signCanonical(secret []byte, canonical string) string {
	return hex.EncodeToString(signCanonicalBytes(secret, canonical))
}

func signCanonicalBytes(secret []byte, canonical string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(canonical))
	return mac.Sum(nil)
}

func requestURI(req *http.Request) string {
	if req.URL == nil {
		return "/"
	}
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	return path
}

func validNonce(nonce string) bool {
	if len(nonce) < 8 || len(nonce) > 128 {
		return false
	}
	for _, r := range nonce {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
