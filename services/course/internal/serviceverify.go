package course

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ServiceVerifier follows the same signed-request wire contract as User and
// Auth, including caller allowlists and one-use nonces.
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
	currentSecret, previousSecret string,
	allowedCallers []string,
	maxSkew time.Duration,
) (*ServiceVerifier, error) {
	if len(currentSecret) < 32 {
		return nil, fmt.Errorf("internal service secret must be at least 32 characters")
	}
	if previousSecret != "" && len(previousSecret) < 32 {
		return nil, fmt.Errorf("previous internal service secret must be at least 32 characters")
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
	return &ServiceVerifier{
		secret: []byte(currentSecret), previous: []byte(previousSecret),
		allowed: allowed, maxSkew: maxSkew, now: time.Now,
		seenNonces: make(map[string]time.Time),
	}, nil
}

func (v *ServiceVerifier) Verify(req *http.Request, body []byte) (string, error) {
	serviceID := strings.TrimSpace(req.Header.Get(HeaderServiceID))
	if _, ok := v.allowed[serviceID]; !ok || serviceID == "" {
		return "", ErrInvalidServiceAuth
	}
	timestampValue := strings.TrimSpace(req.Header.Get(HeaderServiceTimestamp))
	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return "", ErrInvalidServiceAuth
	}
	now := v.now().UTC()
	skew := now.Sub(timestamp)
	if skew < 0 {
		skew = -skew
	}
	if skew > v.maxSkew {
		return "", ErrInvalidServiceAuth
	}
	nonce := strings.TrimSpace(req.Header.Get(HeaderServiceNonce))
	if !validNonce(nonce) {
		return "", ErrInvalidServiceAuth
	}
	signature, err := hex.DecodeString(strings.TrimSpace(req.Header.Get(HeaderServiceSignature)))
	if err != nil || len(signature) != sha256.Size {
		return "", ErrInvalidServiceAuth
	}
	canonical := canonicalRequest(
		req.Method, requestURI(req), serviceID, timestampValue, nonce, body,
	)
	expected := signCanonicalBytes(v.secret, canonical)
	if !hmac.Equal(signature, expected) &&
		(len(v.previous) == 0 ||
			!hmac.Equal(signature, signCanonicalBytes(v.previous, canonical))) {
		return "", ErrInvalidServiceAuth
	}
	if !v.acceptNonce(serviceID+"|"+nonce, timestamp.Add(2*v.maxSkew)) {
		return "", ErrInvalidServiceAuth
	}
	return serviceID, nil
}

func (v *ServiceVerifier) acceptNonce(key string, expiresAt time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.now().UTC()
	for existing, expiry := range v.seenNonces {
		if now.After(expiry) {
			delete(v.seenNonces, existing)
		}
	}
	if _, exists := v.seenNonces[key]; exists {
		return false
	}
	v.seenNonces[key] = expiresAt
	return true
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
