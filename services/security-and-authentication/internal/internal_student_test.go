package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestStudentEligibilityRouteIsEnrollmentOnly(t *testing.T) {
	service, _, pool, _ := newTestAuthService(t)
	user, err := service.ProvisionUser(context.Background(), ProvisionUserInput{
		UserID: uuid.New(), Email: uniqueTestEmail(),
		FirstName: "Enrollment", LastName: "Student",
		Version: 1, Password: testPassword, Roles: []string{RoleStudent},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupAuthUser(t, pool, user.UserID) })

	gin.SetMode(gin.TestMode)
	secret := strings.Repeat("s", 32)
	verifier, err := NewServiceVerifier(secret, "",
		[]string{"course-service", "enrollment-service"}, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	pass := func(c *gin.Context) { c.Next() }
	NewHandler(service, nil, nil).RegisterRoutes(router, RouteDependencies{
		Auth: pass, Internal: InternalAuthMiddleware(verifier, 1024),
		LoginRateLimit: pass, ForgotRateLimit: pass,
	})
	path := "/internal/users/" + user.UserID.String() + "/student-eligibility"
	unsigned := httptest.NewRecorder()
	router.ServeHTTP(unsigned, httptest.NewRequest(http.MethodGet, path, nil))
	if unsigned.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status = %d", unsigned.Code)
	}
	for _, tc := range []struct {
		caller string
		want   int
	}{
		{"course-service", http.StatusForbidden},
		{"enrollment-service", http.StatusOK},
	} {
		signer, err := NewServiceSigner(tc.caller, secret)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if err := signer.Sign(req, nil); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != tc.want {
			t.Fatalf("caller %s: got %d, want %d", tc.caller, response.Code, tc.want)
		}
		if tc.want == http.StatusOK {
			if !strings.Contains(response.Body.String(), `"eligible":true`) {
				t.Fatalf("unexpected eligibility: %s", response.Body.String())
			}
			replay := httptest.NewRecorder()
			router.ServeHTTP(replay, req)
			if replay.Code != http.StatusUnauthorized {
				t.Fatalf("replay status = %d", replay.Code)
			}
		}
	}
}
