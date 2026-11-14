package course

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	HeaderServiceID        = "X-Service-Id"
	HeaderServiceTimestamp = "X-Service-Timestamp"
	HeaderServiceNonce     = "X-Service-Nonce"
	HeaderServiceSignature = "X-Service-Signature"
)

// ServiceSigner signs internal service-to-service HTTP requests with a shared
// HMAC-SHA256 secret.
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
