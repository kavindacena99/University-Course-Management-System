package course

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceSignerSetsVerifiableHeaders(t *testing.T) {
	secret := strings.Repeat("s", 32)
	signer, err := NewServiceSigner("course-service", secret)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	fixedTime := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	signer.now = func() time.Time { return fixedTime }

	body := []byte(`{"name":"course"}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events?source=outbox", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	if req.Header.Get(HeaderServiceID) != "course-service" {
		t.Errorf("service id = %q", req.Header.Get(HeaderServiceID))
	}
	if req.Header.Get(HeaderServiceTimestamp) != fixedTime.Format(time.RFC3339) {
		t.Errorf("timestamp = %q", req.Header.Get(HeaderServiceTimestamp))
	}
	if req.Header.Get(HeaderServiceNonce) == "" {
		t.Fatal("expected nonce")
	}

	expected := signCanonical([]byte(secret), canonicalRequest(
		req.Method,
		requestURI(req),
		"course-service",
		req.Header.Get(HeaderServiceTimestamp),
		req.Header.Get(HeaderServiceNonce),
		body,
	))
	if req.Header.Get(HeaderServiceSignature) != expected {
		t.Fatal("signature does not match canonical request")
	}

	second := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events?source=outbox", nil)
	if err := signer.Sign(second, body); err != nil {
		t.Fatalf("sign second request: %v", err)
	}
	if second.Header.Get(HeaderServiceNonce) == req.Header.Get(HeaderServiceNonce) {
		t.Fatal("expected a unique nonce per request")
	}
	if second.Header.Get(HeaderServiceSignature) == req.Header.Get(HeaderServiceSignature) {
		t.Fatal("expected nonce-bound signatures to differ")
	}
}

func TestServiceSignerValidatesConfiguration(t *testing.T) {
	if _, err := NewServiceSigner("", strings.Repeat("s", 32)); err == nil {
		t.Fatal("expected missing service id error")
	}
	if _, err := NewServiceSigner("course-service", "short"); err == nil {
		t.Fatal("expected short secret error")
	}
	if _, err := NewServiceSigner("course-service", strings.Repeat("s", 32)); err != nil {
		t.Fatalf("create signer: %v", err)
	}
}

func TestCanonicalRequestIncludesPathAndQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost/internal/users/a?b=c", nil)
	uri := requestURI(req)
	if uri != "/internal/users/a?b=c" {
		t.Fatalf("uri = %q", uri)
	}

	withoutQuery := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	if requestURI(withoutQuery) != "/" {
		t.Fatalf("root uri = %q", requestURI(withoutQuery))
	}
}
