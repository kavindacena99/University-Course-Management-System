package user

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestSigner(t *testing.T, serviceID string) *ServiceSigner {
	t.Helper()

	signer, err := NewServiceSigner(serviceID, strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	return signer
}

func newTestServiceVerifier(t *testing.T, previous string) *ServiceVerifier {
	t.Helper()

	verifier, err := NewServiceVerifier(
		strings.Repeat("s", 32),
		previous,
		[]string{"auth-service", "user-service"},
		2*time.Minute,
	)
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	return verifier
}

func TestServiceSignerAndVerifierRoundTrip(t *testing.T) {
	signer := newTestSigner(t, "auth-service")
	verifier := newTestServiceVerifier(t, "")

	body := []byte(`{"event":"test"}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users?source=outbox", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	caller, err := verifier.Verify(req, body)
	if err != nil {
		t.Fatalf("verify request: %v", err)
	}
	if caller != "auth-service" {
		t.Fatalf("caller = %q", caller)
	}
}

func TestServiceVerifierRejectsTamperedBody(t *testing.T) {
	signer := newTestSigner(t, "auth-service")
	verifier := newTestServiceVerifier(t, "")

	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)
	body := []byte(`{"event":"test"}`)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	_, err := verifier.Verify(req, []byte(`{"event":"tampered"}`))
	if !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected ErrInvalidServiceAuth, got %v", err)
	}
}

func TestServiceVerifierRejectsReplay(t *testing.T) {
	signer := newTestSigner(t, "auth-service")
	verifier := newTestServiceVerifier(t, "")
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)

	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}
	if _, err := verifier.Verify(req, body); err != nil {
		t.Fatalf("first verification failed: %v", err)
	}
	if _, err := verifier.Verify(req, body); !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestServiceVerifierRejectsStaleTimestamp(t *testing.T) {
	signer := newTestSigner(t, "auth-service")
	signer.now = func() time.Time {
		return time.Now().Add(-10 * time.Minute)
	}
	verifier := newTestServiceVerifier(t, "")

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	if _, err := verifier.Verify(req, body); !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected stale request rejection, got %v", err)
	}
}

func TestServiceVerifierAcceptsPreviousSecretDuringRotation(t *testing.T) {
	signer, err := NewServiceSigner("auth-service", strings.Repeat("o", 32))
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	verifier := newTestServiceVerifier(t, strings.Repeat("o", 32))

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	if _, err := verifier.Verify(req, body); err != nil {
		t.Fatalf("expected previous secret to be accepted: %v", err)
	}
}

func TestServiceVerifierRejectsUntrustedService(t *testing.T) {
	signer := newTestSigner(t, "unknown-service")
	verifier := newTestServiceVerifier(t, "")

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	if _, err := verifier.Verify(req, body); !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected untrusted service rejection, got %v", err)
	}
}

func TestServiceVerifierRejectsMissingHeaders(t *testing.T) {
	verifier := newTestServiceVerifier(t, "")
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)

	if _, err := verifier.Verify(req, []byte(`{}`)); !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected missing header rejection, got %v", err)
	}
}

func TestServiceVerifierRejectsInvalidNonce(t *testing.T) {
	signer := newTestSigner(t, "auth-service")
	verifier := newTestServiceVerifier(t, "")
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}
	req.Header.Set(HeaderServiceNonce, "bad nonce!")

	if _, err := verifier.Verify(req, body); !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected invalid nonce rejection, got %v", err)
	}
}

func TestServiceVerifierRejectsWrongSecret(t *testing.T) {
	signer := newTestSigner(t, "auth-service")
	verifier, err := NewServiceVerifier(
		strings.Repeat("d", 32),
		"",
		[]string{"auth-service"},
		2*time.Minute,
	)
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost,
		"http://localhost/internal/events/users", nil)
	if err := signer.Sign(req, body); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	if _, err := verifier.Verify(req, body); !errors.Is(err, ErrInvalidServiceAuth) {
		t.Fatalf("expected wrong secret rejection, got %v", err)
	}
}
