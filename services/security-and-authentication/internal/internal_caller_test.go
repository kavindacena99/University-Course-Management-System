package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInternalEndpointCallerIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		caller  string
		allowed string
		status  int
	}{
		{"course-service", "course-service", http.StatusNoContent},
		{"course-service", "user-service", http.StatusForbidden},
		{"user-service", "course-service", http.StatusForbidden},
	} {
		router := gin.New()
		router.GET("/internal/test", func(c *gin.Context) {
			c.Set(internalCallerContextKey, tc.caller)
			c.Next()
		}, RequireInternalCaller(tc.allowed), func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
		result := httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/internal/test", nil))
		if result.Code != tc.status {
			t.Fatalf("caller %s on %s: got %d, want %d", tc.caller, tc.allowed, result.Code, tc.status)
		}
	}
}
