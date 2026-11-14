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

func TestAuthRoleClientFailClosed(t *testing.T) {
	userID := uuid.New()
	signer, err := NewServiceSigner("course-service", strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"eligible", 200, `{"user_id":"` + userID.String() + `","eligible":true}`, nil},
		{"no role", 200, `{"user_id":"` + userID.String() + `","eligible":false}`, ErrLecturerIneligible},
		{"wrong identity", 200, `{"user_id":"` + uuid.NewString() + `","eligible":true}`, ErrAuthServiceUnavailable},
		{"missing eligibility", 200, `{"user_id":"` + userID.String() + `"}`, ErrAuthServiceUnavailable},
		{"trailing data", 200, `{"user_id":"` + userID.String() + `","eligible":true}{}`, ErrAuthServiceUnavailable},
		{"server failure", 503, `{}`, ErrAuthServiceUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/internal/users/"+userID.String()+"/lecturer-eligibility" ||
					r.Header.Get(HeaderServiceSignature) == "" {
					t.Error("role lookup must use the signed internal endpoint")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, err := NewAuthRoleClient(server.URL, signer, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.EnsureEligibleLecturer(context.Background(), userID); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLecturerVerifierRequiresBothSources(t *testing.T) {
	userID := uuid.New()
	users := &fakeEligibility{}
	auth := &fakeEligibility{err: ErrLecturerIneligible}
	verifier := &LecturerVerifier{users: users, auth: auth}
	if err := verifier.EnsureEligibleLecturer(context.Background(), userID); !errors.Is(err, ErrLecturerIneligible) {
		t.Fatalf("got %v", err)
	}
	if len(users.calls) != 1 || len(auth.calls) != 1 {
		t.Fatal("both services must be consulted")
	}
	users.err = ErrUserServiceUnavailable
	if err := verifier.EnsureEligibleLecturer(context.Background(), userID); !errors.Is(err, ErrUserServiceUnavailable) {
		t.Fatalf("got %v", err)
	}
	if len(auth.calls) != 1 {
		t.Fatal("auth lookup should stop after user failure")
	}
}
