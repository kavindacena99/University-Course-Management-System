package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type fakeUserService struct {
	registerFn       func(context.Context, RegisterInput) (*User, error)
	getMeFn          func(context.Context, *AccessClaims) (*User, error)
	updateMeFn       func(context.Context, *AccessClaims, UpdateProfileInput) (*User, error)
	getUserFn        func(context.Context, *AccessClaims, uuid.UUID) (*User, error)
	listUsersFn      func(context.Context, ListUsersFilter) (*UserList, error)
	changeStatusFn   func(context.Context, *AccessClaims, ChangeStatusRequest) (*User, error)
	getHistoryFn     func(context.Context, uuid.UUID, int, int) ([]StatusHistory, int64, error)
	internalGetFn    func(context.Context, uuid.UUID) (*User, error)
	eligibilityFn    func(context.Context, uuid.UUID) (*Eligibility, error)
	bootstrapAdminFn func(context.Context, BootstrapInput) (*User, error)
}

func (f *fakeUserService) Register(ctx context.Context, input RegisterInput) (*User, error) {
	if f.registerFn == nil {
		return nil, errors.New("unexpected Register call")
	}
	return f.registerFn(ctx, input)
}

func (f *fakeUserService) GetMe(ctx context.Context, claims *AccessClaims) (*User, error) {
	if f.getMeFn == nil {
		return nil, errors.New("unexpected GetMe call")
	}
	return f.getMeFn(ctx, claims)
}

func (f *fakeUserService) UpdateMe(ctx context.Context, claims *AccessClaims, input UpdateProfileInput) (*User, error) {
	if f.updateMeFn == nil {
		return nil, errors.New("unexpected UpdateMe call")
	}
	return f.updateMeFn(ctx, claims, input)
}

func (f *fakeUserService) GetUser(ctx context.Context, claims *AccessClaims, userID uuid.UUID) (*User, error) {
	if f.getUserFn == nil {
		return nil, errors.New("unexpected GetUser call")
	}
	return f.getUserFn(ctx, claims, userID)
}

func (f *fakeUserService) ListUsers(ctx context.Context, filter ListUsersFilter) (*UserList, error) {
	if f.listUsersFn == nil {
		return nil, errors.New("unexpected ListUsers call")
	}
	return f.listUsersFn(ctx, filter)
}

func (f *fakeUserService) ChangeStatus(ctx context.Context, claims *AccessClaims, request ChangeStatusRequest) (*User, error) {
	if f.changeStatusFn == nil {
		return nil, errors.New("unexpected ChangeStatus call")
	}
	return f.changeStatusFn(ctx, claims, request)
}

func (f *fakeUserService) GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]StatusHistory, int64, error) {
	if f.getHistoryFn == nil {
		return nil, 0, errors.New("unexpected GetHistory call")
	}
	return f.getHistoryFn(ctx, userID, limit, offset)
}

func (f *fakeUserService) InternalGetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	if f.internalGetFn == nil {
		return nil, errors.New("unexpected InternalGetUser call")
	}
	return f.internalGetFn(ctx, userID)
}

func (f *fakeUserService) Eligibility(ctx context.Context, userID uuid.UUID) (*Eligibility, error) {
	if f.eligibilityFn == nil {
		return nil, errors.New("unexpected Eligibility call")
	}
	return f.eligibilityFn(ctx, userID)
}

func (f *fakeUserService) BootstrapAdmin(ctx context.Context, input BootstrapInput) (*User, error) {
	if f.bootstrapAdminFn == nil {
		return nil, errors.New("unexpected BootstrapAdmin call")
	}
	return f.bootstrapAdminFn(ctx, input)
}

func testClaims(role string) *AccessClaims {
	permissions := []string{}
	if role == RoleAdmin {
		permissions = []string{PermUsersRead, PermUsersManage}
	}
	return &AccessClaims{
		SessionID:   uuid.NewString(),
		Roles:       []string{role},
		Permissions: permissions,
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "university-auth",
			Audience:  jwt.ClaimStrings{"university-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

func testRouter(
	service userService,
	claims *AccessClaims,
	bootstrapToken string,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	auth := func(c *gin.Context) {
		if claims == nil {
			AbortWithError(c, http.StatusUnauthorized, CodeUnauthorized,
				"Authentication is required.")
			return
		}
		c.Set(claimsContextKey, claims)
		c.Next()
	}
	internal := func(c *gin.Context) { c.Next() }

	NewHandler(service, nil).RegisterRoutes(router, RouteDependencies{
		Auth:      auth,
		Internal:  internal,
		Bootstrap: BootstrapTokenMiddleware(bootstrapToken),
	})

	return router
}

func performJSON(
	router http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestCreateUserSuccess(t *testing.T) {
	service := &fakeUserService{
		registerFn: func(_ context.Context, input RegisterInput) (*User, error) {
			if input.Email != "student@example.com" {
				t.Fatalf("unexpected email %q", input.Email)
			}
			return &User{
				ID:                 uuid.New(),
				Email:              input.Email,
				FirstName:          input.FirstName,
				LastName:           input.LastName,
				RegistrationStatus: StatusPending,
				Version:            1,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			}, nil
		},
	}

	router := testRouter(service, nil, "")
	recorder := performJSON(router, http.MethodPost, "/api/v1/users",
		`{"email":"student@example.com","first_name":"Test","last_name":"Student"}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"registration_status":"pending"`) {
		t.Fatalf("expected pending status in response: %s", recorder.Body.String())
	}
}

func TestCreateUserInvalidEmail(t *testing.T) {
	service := &fakeUserService{
		registerFn: func(context.Context, RegisterInput) (*User, error) {
			return nil, newValidationError("email", "must be a valid email address")
		},
	}

	router := testRouter(service, nil, "")
	recorder := performJSON(router, http.MethodPost, "/api/v1/users",
		`{"email":"not-an-email","first_name":"Test","last_name":"Student"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "must be a valid email address") {
		t.Fatalf("expected validation message, got %s", recorder.Body.String())
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	service := &fakeUserService{
		registerFn: func(context.Context, RegisterInput) (*User, error) {
			return nil, ErrEmailTaken
		},
	}

	router := testRouter(service, nil, "")
	recorder := performJSON(router, http.MethodPost, "/api/v1/users",
		`{"email":"student@example.com","first_name":"Test","last_name":"Student"}`)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusConflict, recorder.Code, recorder.Body.String())
	}
}

func TestCreateUserRejectsUnknownFields(t *testing.T) {
	service := &fakeUserService{
		registerFn: func(context.Context, RegisterInput) (*User, error) {
			t.Fatal("service must not be called for malformed JSON")
			return nil, nil
		},
	}

	router := testRouter(service, nil, "")
	recorder := performJSON(router, http.MethodPost, "/api/v1/users",
		`{"email":"student@example.com","first_name":"Test","last_name":"Student","role":"ADMIN"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestGetMeRequiresAuthentication(t *testing.T) {
	router := testRouter(&fakeUserService{}, nil, "")
	recorder := performJSON(router, http.MethodGet, "/api/v1/users/me", "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
}

func TestGetMeReturnsAuthenticatedUser(t *testing.T) {
	claims := testClaims(RoleStudent)
	expectedID, _ := uuid.Parse(claims.Subject)

	service := &fakeUserService{
		getMeFn: func(_ context.Context, gotClaims *AccessClaims) (*User, error) {
			if gotClaims.Subject != claims.Subject {
				t.Fatalf("unexpected claims subject %q", gotClaims.Subject)
			}
			return &User{ID: expectedID, Email: "student@example.com",
				RegistrationStatus: StatusApproved, Version: 1}, nil
		},
	}

	router := testRouter(service, claims, "")
	recorder := performJSON(router, http.MethodGet, "/api/v1/users/me", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestAdminListUsersRequiresAdminRole(t *testing.T) {
	router := testRouter(&fakeUserService{}, testClaims(RoleStudent), "")
	recorder := performJSON(router, http.MethodGet, "/api/v1/admin/users", "")

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}

func TestAdminListUsersPassesFilters(t *testing.T) {
	claims := testClaims(RoleAdmin)
	service := &fakeUserService{
		listUsersFn: func(_ context.Context, filter ListUsersFilter) (*UserList, error) {
			if filter.Status != StatusPending {
				t.Fatalf("status = %q", filter.Status)
			}
			if filter.Search != "student" {
				t.Fatalf("search = %q", filter.Search)
			}
			if filter.Limit != 10 || filter.Offset != 5 {
				t.Fatalf("limit/offset = %d/%d", filter.Limit, filter.Offset)
			}
			return &UserList{
				Users:  []User{{ID: uuid.New(), RegistrationStatus: StatusPending}},
				Total:  1,
				Limit:  10,
				Offset: 5,
			}, nil
		},
	}

	router := testRouter(service, claims, "")
	recorder := performJSON(router, http.MethodGet,
		"/api/v1/admin/users?status=pending&search=student&limit=10&offset=5", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestChangeStatusPassesActionAndVersion(t *testing.T) {
	claims := testClaims(RoleAdmin)
	userID := uuid.New()
	service := &fakeUserService{
		changeStatusFn: func(_ context.Context, gotClaims *AccessClaims, request ChangeStatusRequest) (*User, error) {
			if request.UserID != userID {
				t.Fatalf("user id = %s", request.UserID)
			}
			if request.Action != ActionApprove {
				t.Fatalf("action = %q", request.Action)
			}
			if request.ExpectedVersion != 3 {
				t.Fatalf("version = %d", request.ExpectedVersion)
			}
			return &User{ID: userID, RegistrationStatus: StatusApproved, Version: 4}, nil
		},
	}

	router := testRouter(service, claims, "")
	recorder := performJSON(router, http.MethodPost,
		"/api/v1/admin/users/"+userID.String()+"/approve", `{"version":3}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestGetUserForbidden(t *testing.T) {
	claims := testClaims(RoleStudent)
	service := &fakeUserService{
		getUserFn: func(context.Context, *AccessClaims, uuid.UUID) (*User, error) {
			return nil, ErrForbidden
		},
	}

	router := testRouter(service, claims, "")
	recorder := performJSON(router, http.MethodGet,
		"/api/v1/users/"+uuid.NewString(), "")

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}

func TestInvalidUserIDReturnsBadRequest(t *testing.T) {
	router := testRouter(&fakeUserService{}, testClaims(RoleAdmin), "")
	recorder := performJSON(router, http.MethodGet, "/api/v1/admin/users/not-a-uuid", "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}

func TestInternalEligibility(t *testing.T) {
	userID := uuid.New()
	service := &fakeUserService{
		eligibilityFn: func(_ context.Context, gotID uuid.UUID) (*Eligibility, error) {
			if gotID != userID {
				t.Fatalf("user id = %s", gotID)
			}
			return &Eligibility{
				UserID:             userID,
				Eligible:           true,
				RegistrationStatus: StatusApproved,
				Version:            2,
			}, nil
		},
	}

	router := testRouter(service, nil, "")
	recorder := performJSON(router, http.MethodGet,
		"/internal/users/"+userID.String()+"/eligibility", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s",
			http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response Eligibility
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Eligible {
		t.Fatal("expected eligible user")
	}
}

func TestBootstrapRequiresToken(t *testing.T) {
	token := strings.Repeat("b", 32)
	called := false
	service := &fakeUserService{
		bootstrapAdminFn: func(context.Context, BootstrapInput) (*User, error) {
			called = true
			return &User{ID: uuid.New(), RegistrationStatus: StatusApproved}, nil
		},
	}

	router := testRouter(service, nil, token)
	body := `{"email":"admin@example.com","first_name":"Admin","last_name":"User","password":"StrongPassword1!"}`

	recorder := performJSON(router, http.MethodPost, "/internal/bootstrap", body)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected %d without token, got %d: %s",
			http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("bootstrap service was called without a valid token")
	}

	request := httptest.NewRequest(http.MethodPost, "/internal/bootstrap",
		strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Bootstrap-Token", token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected %d with token, got %d: %s",
			http.StatusCreated, recorder.Code, recorder.Body.String())
	}
	if !called {
		t.Fatal("bootstrap service was not called with a valid token")
	}
}
