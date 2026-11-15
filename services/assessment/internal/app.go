package assessment

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
	"github.com/jackc/pgx/v5/pgxpool"

	"assessment/database"
	"assessment/internal/config"
)

type App struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	server *http.Server
	outbox *OutboxWorker
}

func NewApp(cfg *config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, err
	}
	objects, err := NewMinIOStore(cfg)
	if err != nil {
		db.Close()
		return nil, err
	}
	verifier, err := NewTokenVerifier(cfg)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure access token verification: %w", err)
	}
	signer, err := NewServiceSigner(cfg.InternalServiceID, cfg.InternalServiceSecret)
	if err != nil {
		db.Close()
		return nil, err
	}
	enrollments, err := NewEnrollmentClient(
		cfg.EnrollmentServiceInternalURL, signer, cfg.EnrollmentServiceTimeout,
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure Enrollment client: %w", err)
	}
	allowedFiles := DefaultAllowedFileTypes()
	var outbox *OutboxWorker
	if cfg.AnalyticsServiceInternalURL != "" {
		analytics, err := NewAnalyticsClient(
			cfg.AnalyticsServiceInternalURL, signer, 5*time.Second,
		)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("configure Analytics client: %w", err)
		}
		outbox = NewOutboxWorker(NewRepository(db), analytics, logger)
	} else {
		logger.Info("assessment_outbox_dispatch_disabled",
			"reason", "ANALYTICS_SERVICE_INTERNAL_URL is not configured")
	}
	service := NewAssessmentService(
		NewRepository(db), enrollments, objects, cfg.MaxUploadBytes, allowedFiles,
	)
	handler := NewHandler(service, logger, cfg.MaxUploadBytes, allowedFiles)
	router, err := newRouter(cfg, logger, db, objects, verifier, handler)
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
	return &App{cfg: cfg, logger: logger, db: db, server: server, outbox: outbox}, nil
}

func (a *App) Run(ctx context.Context) error {
	workerCtx, cancelWorker := context.WithCancel(ctx)
	var workerDone chan struct{}
	defer func() {
		cancelWorker()
		if workerDone != nil {
			<-workerDone
		}
	}()
	if a.outbox != nil {
		workerDone = make(chan struct{})
		go func() {
			defer close(workerDone)
			a.outbox.Run(workerCtx)
		}()
	}
	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info("server_started", "service", a.cfg.ServiceName,
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
		a.logger.Info("server_stopped", "service", a.cfg.ServiceName)
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
	objects *MinIOStore,
	verifier *TokenVerifier,
	handler *Handler,
) (*gin.Engine, error) {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	router := gin.New()
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	router.Use(
		RequestIDMiddleware(),
		AccessLogMiddleware(logger),
		RecoveryMiddleware(logger),
		SecurityHeadersMiddleware(),
		BodyLimitMiddleware(cfg.MaxRequestBytes),
		CORSMiddleware(cfg.CORSAllowedOrigins, cfg.CORSAllowCredentials),
	)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": cfg.ServiceName, "status": "healthy"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": cfg.ServiceName, "status": "not_ready", "database": "unavailable",
			})
			return
		}
		var version int64
		var dirty bool
		err := db.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
		if err != nil || dirty || version < 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": cfg.ServiceName, "status": "not_ready", "schema": "unavailable",
			})
			return
		}
		if err := objects.Ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": cfg.ServiceName, "status": "not_ready", "object_storage": "unavailable",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"service": cfg.ServiceName, "status": "ready", "database": "connected",
			"schema": "ready", "object_storage": "ready",
		})
	})
	auth := AuthMiddleware(verifier, cfg.AuthCookieEnabled, cfg.AccessTokenCookieName,
		cfg.CSRFCookieName, cfg.CSRFHeaderName)
	handler.RegisterRoutes(router, auth,
		InternalAuthMiddleware(db, cfg, cfg.MaxRequestBytes))
	router.NoRoute(func(c *gin.Context) {
		AbortWithError(c, http.StatusNotFound, "not_found", "The requested resource was not found.")
	})
	return router, nil
}

func AccessLogMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http_request", "method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(), "latency_ms", time.Since(start).Milliseconds(),
			"request_id", RequestID(c))
	}
}

func RecoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			logger.Error("panic_recovered", "error", fmt.Sprint(recovered),
				"path", c.Request.URL.Path, "request_id", RequestID(c))
			if !c.Writer.Written() {
				AbortWithError(c, http.StatusInternalServerError, "internal_error",
					"An internal error occurred.")
			}
		}()
		c.Next()
	}
}

func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		header.Set("Cross-Origin-Resource-Policy", "same-site")
		header.Set("Cache-Control", "no-store")
		c.Next()
	}
}

func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			AbortWithError(c, http.StatusRequestEntityTooLarge, "payload_too_large",
				"The request body is too large.")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func CORSMiddleware(origins []string, allowCredentials bool) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			allowAll = true
		} else if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		_, isAllowed := allowed[origin]
		if !isAllowed && !allowAll {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}
		if allowAll && !allowCredentials {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		if allowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, X-Request-ID, Idempotency-Key")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
