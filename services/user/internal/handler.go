package user

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// userRepository defines the database operations needed by the handler.
type userRepository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}

// Handler handles HTTP requests for users.
type Handler struct {
	repository userRepository
}

func NewHandler(repository userRepository) *Handler {
	return &Handler{repository: repository}
}

type createUserRequest struct {
	Email     string `json:"email" binding:"required,email,max=254"`
	FirstName string `json:"first_name" binding:"required,max=100"`
	LastName  string `json:"last_name" binding:"required,max=100"`
}

// RegisterRoutes registers the user endpoints.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST("/", h.Create)
	router.GET("/:id", h.GetByID)
}

func (h *Handler) Create(c *gin.Context) {
	var req createUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request. Provide a valid email, first name, and last name.",
		})
		return
	}

	// Reject whitespace-only names.
	if strings.TrimSpace(req.FirstName) == "" ||
		strings.TrimSpace(req.LastName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "First name and last name must not be empty.",
		})
		return
	}

	u := &User{
		Email:              req.Email,
		FirstName:          req.FirstName,
		LastName:           req.LastName,
		RegistrationStatus: StatusPending,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.repository.Create(ctx, u); err != nil {
		if errors.Is(err, ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "An account with this email already exists.",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to create user.",
		})
		return
	}

	c.JSON(http.StatusCreated, u)
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID.",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	u, err := h.repository.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "User not found.",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to retrieve user.",
		})
		return
	}

	c.JSON(http.StatusOK, u)
}
