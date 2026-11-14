package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignedClientsValidateAllThreeAuthorities(t *testing.T) {
	studentID, offeringID, lecturerID := uuid.New(), uuid.New(), uuid.New()
	secret := strings.Repeat("s", 32)
	verifier, err := NewServiceVerifier(secret, "", []string{"enrollment-service"}, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := verifier.Verify(r, nil); err != nil {
			t.Errorf("invalid internal signature: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/internal/users/" + studentID.String() + "/eligibility":
			json.NewEncoder(w).Encode(map[string]any{
				"user_id": studentID, "eligible": true,
				"registration_status": "approved", "version": 2,
			})
		case "/internal/users/" + studentID.String() + "/student-eligibility",
			"/internal/users/" + lecturerID.String() + "/lecturer-eligibility":
			id := studentID
			if strings.Contains(r.URL.Path, lecturerID.String()) {
				id = lecturerID
			}
			json.NewEncoder(w).Encode(map[string]any{"user_id": id, "eligible": true})
		case "/internal/offerings/" + offeringID.String():
			json.NewEncoder(w).Encode(map[string]any{
				"offering": map[string]any{
					"id": offeringID, "status": "published", "version": 4,
					"lecturer_user_id": lecturerID, "enrollment_capacity": 5,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	signer, err := NewServiceSigner("enrollment-service", secret)
	if err != nil {
		t.Fatal(err)
	}
	users, err := NewUserEligibilityClient(server.URL, signer, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := NewStudentRoleClient(server.URL, signer, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	courses, err := NewCourseOfferingClient(server.URL, signer, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := users.EnsureApproved(ctx, studentID); err != nil {
		t.Fatal(err)
	}
	if err := roles.EnsureActiveStudent(ctx, studentID); err != nil {
		t.Fatal(err)
	}
	if err := roles.EnsureActiveLecturer(ctx, lecturerID); err != nil {
		t.Fatal(err)
	}
	offering, err := courses.GetOffering(ctx, offeringID)
	if err != nil {
		t.Fatal(err)
	}
	if offering.EnrollmentCapacity == nil || *offering.EnrollmentCapacity != 5 ||
		offering.LecturerUserID == nil || *offering.LecturerUserID != lecturerID {
		t.Fatalf("offering fields were not decoded: %+v", offering)
	}
	if _, err := courses.GetOffering(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing offering = %v", err)
	}
}

func TestUserEligibilityFailsClosedOnMalformedResponseAndTimeout(t *testing.T) {
	studentID := uuid.New()
	secret := strings.Repeat("s", 32)
	signer, err := NewServiceSigner("enrollment-service", secret)
	if err != nil {
		t.Fatal(err)
	}
	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"user_id":"` + uuid.NewString() + `","eligible":true,"registration_status":"approved","version":1}`))
	}))
	client, err := NewUserEligibilityClient(malformed.URL, signer, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureApproved(context.Background(), studentID); !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("mismatched identity = %v", err)
	}
	malformed.Close()

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()
	client, err = NewUserEligibilityClient(slow.URL, signer, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureApproved(context.Background(), studentID); !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("timeout = %v", err)
	}
}
