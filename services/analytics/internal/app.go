package analytics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"analytics/database"
	"analytics/internal/config"
)

type App struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	server *http.Server
}

func NewApp(cfg *config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, err
	}
	verifier, err := NewTokenVerifier(cfg)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure access token verification: %w", err)
	}
	course := NewCourseClient(
		cfg.CourseServiceInternalURL, cfg.InternalServiceID, cfg.InternalServiceSecret,
	)
	assessment := NewAssessmentClient(
		cfg.AssessmentServiceInternalURL, cfg.InternalServiceID, cfg.InternalServiceSecret,
	)
	service := NewService(NewRepository(db), course, assessment)
	handler := NewHandler(service)
	router, err := newRouter(cfg, logger, db, verifier, handler)
	if err != nil {
		db.Close()
		return nil, err
	}
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.BindHost, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    32 << 10,
	}
	return &App{cfg: cfg, logger: logger, db: db, server: server}, nil
}

func (a *App) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info("server_started", "service", "analytics-service",
			"address", a.server.Addr, "environment", a.cfg.Environment)
		err := a.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()
	select {
	case err := <-serverErrors:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			_ = a.server.Close()
			return fmt.Errorf("shutdown server: %w", err)
		}
		<-serverErrors
		a.logger.Info("server_stopped", "service", "analytics-service")
		return nil
	}
}

func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}

func newRouter(
	cfg *config.Config,
	logger *slog.Logger,
	db *pgxpool.Pool,
	verifier *TokenVerifier,
	handler *Handler,
) (*gin.Engine, error) {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	router.Use(
		requestIDMiddleware(),
		accessLogMiddleware(logger),
		recoveryMiddleware(logger),
		securityHeadersMiddleware(),
		bodyLimitMiddleware(cfg.MaxRequestBytes),
	)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "analytics-service", "status": "healthy"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": "analytics-service", "status": "not_ready", "database": "unavailable",
			})
			return
		}
		var version int64
		var dirty bool
		err := db.QueryRow(ctx,
			`SELECT version, dirty FROM schema_migrations LIMIT 1`,
		).Scan(&version, &dirty)
		if err != nil || dirty || version < 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": "analytics-service", "status": "not_ready", "schema": "unavailable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"service": "analytics-service", "status": "ready",
			"database": "connected", "schema": "ready",
		})
	})
	auth := AuthMiddleware(verifier, cfg.AuthCookieEnabled, cfg.AccessTokenCookieName,
		cfg.CSRFCookieName, cfg.CSRFHeaderName)
	internal := InternalAuthMiddleware(db, cfg, cfg.MaxRequestBytes)
	handler.RegisterRoutes(router, auth, internal)
	router.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "not_found", "The requested resource was not found.")
	})
	return router, nil
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func accessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http_request", "method", c.Request.Method,
			"path", c.Request.URL.Path, "status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", c.Writer.Header().Get("X-Request-ID"))
	}
}

func recoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic_recovered", "error", fmt.Sprint(recovered),
					"path", c.Request.URL.Path)
				if !c.Writer.Written() {
					writeError(c, http.StatusInternalServerError,
						"internal_error", "An internal error occurred.")
				}
			}
		}()
		c.Next()
	}
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cache-Control", "no-store")
		c.Next()
	}
}

func bodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			writeError(c, http.StatusRequestEntityTooLarge,
				"payload_too_large", "The request body is too large.")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
