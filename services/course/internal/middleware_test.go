package course

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddlewareBearerAndCookieCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	keys := newJWTTestKeys(t)
	verifier := newJWTTestVerifier(t, keys.public)
	token := signJWTTestToken(t, keys.private, "test-key", jwtTestClaims())

	router := gin.New()
	router.Use(RequestIDMiddleware())
	auth := AuthMiddleware(verifier, true, "access_token", "csrf_token", "X-CSRF-Token")
	router.POST("/protected", auth, func(c *gin.Context) {
		claims, ok := ClaimsFromContext(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.String(http.StatusOK, claims.Subject)
	})

	newRequest := func(withCookie bool) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/protected", nil)
		if withCookie {
			req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		}
		return req
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newRequest(true))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cookie without CSRF status = %d, want 403", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), CodeCSRFFailed) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}

	req := newRequest(true)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "csrf-value"})
	req.Header.Set("X-CSRF-Token", "csrf-value")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("cookie with CSRF status = %d, want 200", recorder.Code)
	}

	req = newRequest(false)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("bearer status = %d, want 200", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d, want 401", recorder.Code)
	}
}

func TestRequestIDMiddlewareAndBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestIDMiddleware(), BodyLimitMiddleware(4))
	router.POST("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	req.Header.Set("X-Request-ID", "request-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") != "request-123" {
		t.Errorf("request id = %q", recorder.Header().Get("X-Request-ID"))
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("1234"))
	req.Header.Set("X-Request-ID", "bad header!")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") == "bad header!" ||
		recorder.Header().Get("X-Request-ID") == "" {
		t.Errorf("request id was not replaced: %q", recorder.Header().Get("X-Request-ID"))
	}
}
