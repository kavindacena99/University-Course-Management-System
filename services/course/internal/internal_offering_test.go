package course

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestInternalOfferingFactsRequireSignedEnrollmentCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := strings.Repeat("s", 32)
	verifier, err := NewServiceVerifier(secret, "",
		[]string{"enrollment-service", "other-service"}, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	offeringID, lecturerID := uuid.New(), uuid.New()
	service := &fakeCourseService{getOfferingFactsFn: func(_ context.Context, id uuid.UUID) (*OfferingFacts, error) {
		if id != offeringID {
			t.Fatalf("unexpected offering ID %s", id)
		}
		return &OfferingFacts{
			ID: offeringID, Status: OfferingPublished,
			LecturerUserID: &lecturerID, Version: 3,
		}, nil
	}}
	router := gin.New()
	NewHandler(service, nil).RegisterRoutes(router,
		func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		InternalAuthMiddleware(verifier, 1024),
	)
	path := "/internal/offerings/" + offeringID.String()
	unsigned := httptest.NewRecorder()
	router.ServeHTTP(unsigned, httptest.NewRequest(http.MethodGet, path, nil))
	if unsigned.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned request = %d", unsigned.Code)
	}
	for _, tc := range []struct {
		caller string
		want   int
	}{
		{"other-service", http.StatusForbidden},
		{"enrollment-service", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		signer, err := NewServiceSigner(tc.caller, secret)
		if err != nil {
			t.Fatal(err)
		}
		if err := signer.Sign(req, nil); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != tc.want {
			t.Fatalf("caller %s returned %d, want %d", tc.caller, response.Code, tc.want)
		}
		if tc.want == http.StatusOK {
			var payload struct {
				Offering OfferingFacts `json:"offering"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Offering.ID != offeringID || payload.Offering.Version != 3 ||
				payload.Offering.LecturerUserID == nil ||
				*payload.Offering.LecturerUserID != lecturerID {
				t.Fatalf("unexpected offering facts: %+v", payload.Offering)
			}
			replay := httptest.NewRecorder()
			router.ServeHTTP(replay, req)
			if replay.Code != http.StatusUnauthorized {
				t.Fatalf("replayed request = %d", replay.Code)
			}
		}
	}
}
