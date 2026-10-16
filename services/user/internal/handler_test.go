package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeUserRepository struct {
	createFunc  func(context.Context, *User) error
	getByIDFunc func(context.Context, uuid.UUID) (*User, error)
}

func (f *fakeUserRepository) Create(ctx context.Context, u *User) error {
	return f.createFunc(ctx, u)
}

func (f *fakeUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return f.getByIDFunc(ctx, id)
}

func newTestHandler(repo userRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(repo).RegisterRoutes(router)

	return router
}

func TestCreateUserSuccess(t *testing.T) {
	repo := &fakeUserRepository{
		createFunc: func(_ context.Context, u *User) error {
			u.ID = uuid.New()
			return nil
		},
		getByIDFunc: func(context.Context, uuid.UUID) (*User, error) {
			return nil, ErrNotFound
		},
	}

	router := newTestHandler(repo)

	body := `{"email":"student@example.com","first_name":"Test","last_name":"Student"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusCreated, rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"registration_status":"pending"`) {
		t.Fatalf("expected pending registration status, got: %s", rec.Body.String())
	}
}

func TestCreateUserInvalidEmail(t *testing.T) {
	repo := &fakeUserRepository{
		createFunc: func(context.Context, *User) error {
			t.Fatal("repository should not be called for invalid input")
			return nil
		},
		getByIDFunc: func(context.Context, uuid.UUID) (*User, error) {
			return nil, ErrNotFound
		},
	}

	router := newTestHandler(repo)

	body := `{"email":"not-an-email","first_name":"Test","last_name":"Student"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	repo := &fakeUserRepository{
		createFunc: func(context.Context, *User) error {
			return ErrEmailTaken
		},
		getByIDFunc: func(context.Context, uuid.UUID) (*User, error) {
			return nil, ErrNotFound
		},
	}

	router := newTestHandler(repo)

	body := `{"email":"student@example.com","first_name":"Test","last_name":"Student"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusConflict, rec.Code, rec.Body.String())
	}
}

func TestGetUserInvalidID(t *testing.T) {
	repo := &fakeUserRepository{
		createFunc: func(context.Context, *User) error {
			return errors.New("unexpected Create call")
		},
		getByIDFunc: func(context.Context, uuid.UUID) (*User, error) {
			t.Fatal("repository should not be called for an invalid UUID")
			return nil, nil
		},
	}

	router := newTestHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
