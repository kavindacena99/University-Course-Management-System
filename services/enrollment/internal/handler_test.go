package enrollment

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testHTTPRouter(t *testing.T, service *Service) (*gin.Engine, ed25519.PrivateKey, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	tokenVerifier, err := NewTokenVerifier(map[string]string{"test": pemKey},
		"test-issuer", "test-audience", 0)
	if err != nil {
		t.Fatal(err)
	}
	secret := strings.Repeat("s", 32)
	internalVerifier, err := NewServiceVerifier(secret, "",
		[]string{"assessment-service"}, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(RequestIDMiddleware(), BodyLimitMiddleware(1<<20))
	NewHandler(service, nil).RegisterRoutes(router,
		AuthMiddleware(tokenVerifier, false, "", "", ""),
		InternalAuthMiddleware(internalVerifier, 1<<20),
	)
	return router, private, secret
}

func signedTestJWT(t *testing.T, private ed25519.PrivateKey, subject uuid.UUID, roles ...string) string {
	t.Helper()
	claims := AccessClaims{
		SessionID: uuid.NewString(), Roles: roles, TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: subject.String(), Issuer: "test-issuer",
			Audience:  jwt.ClaimStrings{"test-audience"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "test"
	raw, err := token.SignedString(private)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestEnrollmentHTTPAuthenticationValidationAndIDOR(t *testing.T) {
	studentID, otherID, offeringID, enrollmentID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	service := testService(stubStore{
		get: func(uuid.UUID) (*Enrollment, error) {
			return &Enrollment{ID: enrollmentID, StudentUserID: studentID,
				OfferingID: offeringID, Status: EnrollmentActive}, nil
		},
	}, nil, stubRoles{}, nil)
	router, key, _ := testHTTPRouter(t, service)
	path := "/api/v1/enrollments/" + enrollmentID.String()
	unsigned := httptest.NewRecorder()
	router.ServeHTTP(unsigned, httptest.NewRequest(http.MethodGet, path, nil))
	if unsigned.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned GET = %d", unsigned.Code)
	}
	if !strings.Contains(unsigned.Body.String(), `"code":"unauthorized"`) ||
		!strings.Contains(unsigned.Body.String(), `"request_id":"`) {
		t.Fatalf("unsigned GET lacks the error envelope: %s", unsigned.Body.String())
	}
	for _, tc := range []struct {
		user uuid.UUID
		want int
	}{
		{studentID, http.StatusOK},
		{otherID, http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+signedTestJWT(t, key, tc.user, RoleStudent))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != tc.want {
			t.Fatalf("student %s GET = %d, want %d", tc.user, response.Code, tc.want)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments",
		bytes.NewBufferString(`{"offering_id":"`+offeringID.String()+`","status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signedTestJWT(t, key, studentID, RoleStudent))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("mass-assignment payload = %d", response.Code)
	}
}

func TestEnrollmentHTTPCreateAndConflict(t *testing.T) {
	studentID, offeringID := uuid.New(), uuid.New()
	duplicate := false
	service := testService(
		stubStore{create: func(e *Enrollment, capacity *int) error {
			if duplicate {
				return ErrDuplicateActive
			}
			e.Version = 1
			return nil
		}},
		func(uuid.UUID) error { return nil },
		stubRoles{student: func(uuid.UUID) error { return nil }},
		func(uuid.UUID) (*OfferingFacts, error) {
			return &OfferingFacts{ID: offeringID, Status: "published", Version: 1}, nil
		},
	)
	router, key, _ := testHTTPRouter(t, service)
	for _, want := range []int{http.StatusCreated, http.StatusConflict} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enrollments",
			bytes.NewBufferString(`{"offering_id":"`+offeringID.String()+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+signedTestJWT(t, key, studentID, RoleStudent))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != want || response.Header().Get("X-Request-Id") == "" {
			t.Fatalf("create = %d, want %d: %s", response.Code, want, response.Body.String())
		}
		duplicate = true
	}
}

func TestAssessmentInternalEndpointRequiresValidOneUseSignature(t *testing.T) {
	studentID, offeringID := uuid.New(), uuid.New()
	service := testService(stubStore{active: func(student, offering uuid.UUID) (*Enrollment, error) {
		if student != studentID || offering != offeringID {
			t.Fatal("wrong active lookup")
		}
		return &Enrollment{Status: EnrollmentActive}, nil
	}}, func(uuid.UUID) error { return nil },
		stubRoles{student: func(uuid.UUID) error { return nil }}, nil)
	router, _, secret := testHTTPRouter(t, service)
	path := "/internal/offerings/" + offeringID.String() +
		"/enrollments/" + studentID.String() + "/active"
	unsigned := httptest.NewRecorder()
	router.ServeHTTP(unsigned, httptest.NewRequest(http.MethodGet, path, nil))
	if unsigned.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned internal request = %d", unsigned.Code)
	}
	rogue, err := NewServiceSigner("rogue-service", secret)
	if err != nil {
		t.Fatal(err)
	}
	rogueRequest := httptest.NewRequest(http.MethodGet, path, nil)
	if err := rogue.Sign(rogueRequest, nil); err != nil {
		t.Fatal(err)
	}
	rogueResponse := httptest.NewRecorder()
	router.ServeHTTP(rogueResponse, rogueRequest)
	if rogueResponse.Code != http.StatusUnauthorized {
		t.Fatalf("rogue service = %d", rogueResponse.Code)
	}
	assessment, err := NewServiceSigner("assessment-service", secret)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if err := assessment.Sign(req, nil); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"active":true`) {
		t.Fatalf("signed Assessment request = %d: %s", response.Code, response.Body.String())
	}
	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, req)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed request = %d", replay.Code)
	}
}
