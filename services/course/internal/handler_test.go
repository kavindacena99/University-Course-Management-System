package course

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeCourseService struct {
	createCourseFn         func(context.Context, *AccessClaims, CreateCourseInput) (*Course, error)
	getCourseFn            func(context.Context, *AccessClaims, uuid.UUID) (*Course, error)
	listCoursesFn          func(context.Context, *AccessClaims, CourseFilter) (*CourseList, error)
	updateCourseFn         func(context.Context, *AccessClaims, uuid.UUID, UpdateCourseInput) (*Course, error)
	changeCourseStatusFn   func(context.Context, *AccessClaims, uuid.UUID, string, int64) (*Course, error)
	createOfferingFn       func(context.Context, *AccessClaims, CreateOfferingInput) (*CourseOffering, error)
	getOfferingFn          func(context.Context, *AccessClaims, uuid.UUID) (*CourseOffering, error)
	getOfferingFactsFn     func(context.Context, uuid.UUID) (*OfferingFacts, error)
	listCourseOfferingsFn  func(context.Context, *AccessClaims, uuid.UUID, OfferingFilter) (*OfferingList, error)
	listOfferingsFn        func(context.Context, *AccessClaims, OfferingFilter) (*OfferingList, error)
	updateOfferingFn       func(context.Context, *AccessClaims, uuid.UUID, UpdateOfferingInput) (*CourseOffering, error)
	assignLecturerFn       func(context.Context, *AccessClaims, uuid.UUID, AssignLecturerInput) (*CourseOffering, error)
	changeOfferingStatusFn func(context.Context, *AccessClaims, uuid.UUID, string, int64) (*CourseOffering, error)
}

func (f *fakeCourseService) CreateCourse(ctx context.Context, claims *AccessClaims, input CreateCourseInput) (*Course, error) {
	if f.createCourseFn == nil {
		return nil, errors.New("unexpected CreateCourse call")
	}
	return f.createCourseFn(ctx, claims, input)
}

func (f *fakeCourseService) GetCourse(ctx context.Context, claims *AccessClaims, id uuid.UUID) (*Course, error) {
	if f.getCourseFn == nil {
		return nil, errors.New("unexpected GetCourse call")
	}
	return f.getCourseFn(ctx, claims, id)
}

func (f *fakeCourseService) ListCourses(ctx context.Context, claims *AccessClaims, filter CourseFilter) (*CourseList, error) {
	if f.listCoursesFn == nil {
		return nil, errors.New("unexpected ListCourses call")
	}
	return f.listCoursesFn(ctx, claims, filter)
}

func (f *fakeCourseService) UpdateCourse(ctx context.Context, claims *AccessClaims, id uuid.UUID, input UpdateCourseInput) (*Course, error) {
	if f.updateCourseFn == nil {
		return nil, errors.New("unexpected UpdateCourse call")
	}
	return f.updateCourseFn(ctx, claims, id, input)
}

func (f *fakeCourseService) ChangeCourseStatus(ctx context.Context, claims *AccessClaims, id uuid.UUID, action string, version int64) (*Course, error) {
	if f.changeCourseStatusFn == nil {
		return nil, errors.New("unexpected ChangeCourseStatus call")
	}
	return f.changeCourseStatusFn(ctx, claims, id, action, version)
}

func (f *fakeCourseService) CreateOffering(ctx context.Context, claims *AccessClaims, input CreateOfferingInput) (*CourseOffering, error) {
	if f.createOfferingFn == nil {
		return nil, errors.New("unexpected CreateOffering call")
	}
	return f.createOfferingFn(ctx, claims, input)
}

func (f *fakeCourseService) GetOffering(ctx context.Context, claims *AccessClaims, id uuid.UUID) (*CourseOffering, error) {
	if f.getOfferingFn == nil {
		return nil, errors.New("unexpected GetOffering call")
	}
	return f.getOfferingFn(ctx, claims, id)
}

func (f *fakeCourseService) GetOfferingFacts(ctx context.Context, id uuid.UUID) (*OfferingFacts, error) {
	if f.getOfferingFactsFn == nil {
		return nil, errors.New("unexpected GetOfferingFacts call")
	}
	return f.getOfferingFactsFn(ctx, id)
}

func (f *fakeCourseService) ListCourseOfferings(ctx context.Context, claims *AccessClaims, id uuid.UUID, filter OfferingFilter) (*OfferingList, error) {
	if f.listCourseOfferingsFn == nil {
		return nil, errors.New("unexpected ListCourseOfferings call")
	}
	return f.listCourseOfferingsFn(ctx, claims, id, filter)
}

func (f *fakeCourseService) ListOfferings(ctx context.Context, claims *AccessClaims, filter OfferingFilter) (*OfferingList, error) {
	if f.listOfferingsFn == nil {
		return nil, errors.New("unexpected ListOfferings call")
	}
	return f.listOfferingsFn(ctx, claims, filter)
}

func (f *fakeCourseService) UpdateOffering(ctx context.Context, claims *AccessClaims, id uuid.UUID, input UpdateOfferingInput) (*CourseOffering, error) {
	if f.updateOfferingFn == nil {
		return nil, errors.New("unexpected UpdateOffering call")
	}
	return f.updateOfferingFn(ctx, claims, id, input)
}

func (f *fakeCourseService) AssignLecturer(ctx context.Context, claims *AccessClaims, id uuid.UUID, input AssignLecturerInput) (*CourseOffering, error) {
	if f.assignLecturerFn == nil {
		return nil, errors.New("unexpected AssignLecturer call")
	}
	return f.assignLecturerFn(ctx, claims, id, input)
}

func (f *fakeCourseService) ChangeOfferingStatus(ctx context.Context, claims *AccessClaims, id uuid.UUID, action string, version int64) (*CourseOffering, error) {
	if f.changeOfferingStatusFn == nil {
		return nil, errors.New("unexpected ChangeOfferingStatus call")
	}
	return f.changeOfferingStatusFn(ctx, claims, id, action, version)
}

func newHandlerTestRouter(service courseService, claims *AccessClaims) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestIDMiddleware())
	auth := func(c *gin.Context) {
		if claims != nil && c.GetHeader("Authorization") == "Bearer test" {
			c.Set(claimsContextKey, claims)
		}
		c.Next()
	}
	NewHandler(service, nil).RegisterRoutes(router, auth, func(c *gin.Context) { c.Next() })
	return router
}

func performJSONRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body any,
	authenticated bool,
) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if authenticated {
		req.Header.Set("Authorization", "Bearer test")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeErrorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var response apiErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response %q: %v", recorder.Body.String(), err)
	}
	return response.Error.Code
}

func TestHandlerRequiresAuthentication(t *testing.T) {
	router := newHandlerTestRouter(&fakeCourseService{}, nil)

	for _, path := range []string{
		"/api/v1/courses",
		"/api/v1/offerings",
		"/api/v1/offerings/assigned",
	} {
		recorder := performJSONRequest(t, router, http.MethodGet, path, nil, false)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", path, recorder.Code)
		}
		if code := decodeErrorCode(t, recorder); code != CodeUnauthorized {
			t.Errorf("%s error code = %q", path, code)
		}
	}
}

func TestHandlerCreateCourseValidation(t *testing.T) {
	adminID := uuid.New()
	claims := testAccessClaims(adminID, RoleAdmin, PermCoursesManage)
	called := false
	service := &fakeCourseService{
		createCourseFn: func(_ context.Context, _ *AccessClaims, input CreateCourseInput) (*Course, error) {
			called = true
			return &Course{ID: uuid.New(), Code: input.Code, Version: 1}, nil
		},
	}
	router := newHandlerTestRouter(service, claims)

	recorder := performJSONRequest(t, router, http.MethodPost,
		"/api/v1/admin/courses", map[string]any{
			"code":       "CS-101",
			"title":      "Introduction",
			"credits":    3,
			"unexpected": true,
		}, true)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", recorder.Code)
	}
	if code := decodeErrorCode(t, recorder); code != CodeInvalidRequest {
		t.Errorf("error code = %q", code)
	}
	if called {
		t.Fatal("service should not be called for invalid payload")
	}

	recorder = performJSONRequest(t, router, http.MethodPost,
		"/api/v1/admin/courses", map[string]any{
			"code": "CS-101", "title": "Introduction", "credits": 3,
		}, true)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !called {
		t.Fatal("expected service call")
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Error("expected request id header")
	}
}

func TestHandlerRejectsUnsupportedAdminActionByPermission(t *testing.T) {
	lecturer := testAccessClaims(uuid.New(), RoleLecturer, PermOfferingsManage)
	service := &fakeCourseService{
		createCourseFn: func(context.Context, *AccessClaims, CreateCourseInput) (*Course, error) {
			t.Fatal("service must not be called")
			return nil, nil
		},
	}
	router := newHandlerTestRouter(service, lecturer)

	recorder := performJSONRequest(t, router, http.MethodPost,
		"/api/v1/admin/courses", map[string]any{
			"code": "CS-101", "title": "Introduction", "credits": 3,
		}, true)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	if code := decodeErrorCode(t, recorder); code != CodeForbidden {
		t.Errorf("error code = %q", code)
	}
}

func TestHandlerMapsServiceErrorsAndParsesUUIDs(t *testing.T) {
	adminID := uuid.New()
	claims := testAccessClaims(adminID, RoleAdmin, PermCoursesManage, PermOfferingsManage)
	courseID := uuid.New()
	service := &fakeCourseService{
		changeCourseStatusFn: func(_ context.Context, _ *AccessClaims, id uuid.UUID, action string, version int64) (*Course, error) {
			if id != courseID || action != "publish" || version != 2 {
				t.Fatalf("unexpected service input: %s %s %d", id, action, version)
			}
			return nil, ErrVersionConflict
		},
	}
	router := newHandlerTestRouter(service, claims)

	recorder := performJSONRequest(t, router, http.MethodPost,
		"/api/v1/admin/courses/"+courseID.String()+"/publish",
		map[string]any{"version": 2}, true)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if code := decodeErrorCode(t, recorder); code != CodeVersionConflict {
		t.Errorf("error code = %q", code)
	}

	recorder = performJSONRequest(t, router, http.MethodGet,
		"/api/v1/courses/not-a-uuid", nil, true)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid UUID status = %d, want 400", recorder.Code)
	}
	if code := decodeErrorCode(t, recorder); code != CodeValidationFailed {
		t.Errorf("error code = %q", code)
	}

	recorder = performJSONRequest(t, router, http.MethodGet,
		"/api/v1/courses?limit=1000", nil, true)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status = %d, want 400", recorder.Code)
	}
}

func TestHandlerOfferingFiltersAndAssignedRoute(t *testing.T) {
	lecturerID := uuid.New()
	claims := testAccessClaims(lecturerID, RoleLecturer, PermOfferingsManage)
	var captured OfferingFilter
	service := &fakeCourseService{
		listOfferingsFn: func(_ context.Context, _ *AccessClaims, filter OfferingFilter) (*OfferingList, error) {
			captured = filter
			return &OfferingList{Offerings: []CourseOffering{}, Limit: filter.Limit}, nil
		},
	}
	router := newHandlerTestRouter(service, claims)

	recorder := performJSONRequest(t, router, http.MethodGet,
		"/api/v1/offerings/assigned?status=draft&sort=created_at&order=desc",
		nil, true)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !captured.AssignedToMe {
		t.Fatal("expected assigned_to_me filter")
	}
	if captured.Status != OfferingDraft {
		t.Errorf("status = %q", captured.Status)
	}
	if captured.Sort != "created_at" || captured.Order != "desc" {
		t.Errorf("unexpected sort: %s %s", captured.Sort, captured.Order)
	}
}

func TestHandlerRejectsMultipleJSONValues(t *testing.T) {
	claims := testAccessClaims(uuid.New(), RoleAdmin, PermCoursesManage)
	router := newHandlerTestRouter(&fakeCourseService{}, claims)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/courses",
		bytes.NewBufferString(`{"code":"CS-101","title":"Title","credits":3} {}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if code := decodeErrorCode(t, recorder); code != CodeInvalidRequest {
		t.Errorf("error code = %q", code)
	}
}
