package course

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newUserClientTestSigner(t *testing.T) *ServiceSigner {
	t.Helper()
	signer, err := NewServiceSigner("course-service", strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	return signer
}

func TestUserServiceClientAcceptsApprovedUser(t *testing.T) {
	userID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/users/"+userID.String() {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get(HeaderServiceID) != "course-service" {
			t.Errorf("service id = %q", r.Header.Get(HeaderServiceID))
		}
		if r.Header.Get(HeaderServiceSignature) == "" {
			t.Error("expected signature header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"` + userID.String() +
			`","registration_status":"approved","version":2}}`))
	}))
	defer server.Close()

	client, err := NewUserServiceClient(server.URL, newUserClientTestSigner(t), time.Second)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := client.EnsureEligibleLecturer(context.Background(), userID); err != nil {
		t.Fatalf("eligibility: %v", err)
	}
}

func TestUserServiceClientErrorMapping(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{
			name:    "not found",
			status:  http.StatusNotFound,
			body:    `{"error":{"code":"not_found"}}`,
			wantErr: ErrLecturerNotFound,
		},
		{
			name:    "suspended",
			status:  http.StatusOK,
			body:    `{"user":{"id":"` + userID.String() + `","registration_status":"suspended"}}`,
			wantErr: ErrLecturerIneligible,
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			body:    `{}`,
			wantErr: ErrUserServiceUnavailable,
		},
		{
			name:    "malformed",
			status:  http.StatusOK,
			body:    `not-json`,
			wantErr: ErrUserServiceUnavailable,
		},
		{
			name:    "trailing data",
			status:  http.StatusOK,
			body:    `{"user":{"id":"` + userID.String() + `","registration_status":"approved"}}{}`,
			wantErr: ErrUserServiceUnavailable,
		},
		{
			name:    "identity mismatch",
			status:  http.StatusOK,
			body:    `{"user":{"id":"` + uuid.NewString() + `","registration_status":"approved"}}`,
			wantErr: ErrUserServiceUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			client, err := NewUserServiceClient(
				server.URL, newUserClientTestSigner(t), time.Second,
			)
			if err != nil {
				t.Fatalf("create client: %v", err)
			}
			err = client.EnsureEligibleLecturer(context.Background(), userID)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestUserServiceClientValidatesConfiguration(t *testing.T) {
	signer := newUserClientTestSigner(t)

	if _, err := NewUserServiceClient("relative", signer, time.Second); err == nil {
		t.Fatal("expected invalid URL error")
	}
	if _, err := NewUserServiceClient("ftp://localhost", signer, time.Second); err == nil {
		t.Fatal("expected unsupported scheme error")
	}
	if _, err := NewUserServiceClient("http://localhost", nil, time.Second); err == nil {
		t.Fatal("expected missing signer error")
	}
	if _, err := NewUserServiceClient("http://localhost", signer, 0); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}
