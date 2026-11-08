package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"security-and-authentication/database"
	"security-and-authentication/internal/config"
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

	hasher, err := NewPasswordHasher(
		cfg.Argon2MemoryKiB,
		cfg.Argon2Iterations,
		cfg.Argon2Parallelism,
		cfg.Argon2SaltLength,
		cfg.Argon2KeyLength,
	)
	if err != nil {
		db.Close()
		return nil, err
	}

	tokenManager, err := NewTokenManager(
		cfg.JWTPrivateKey,
		cfg.JWTKeyID,
		cfg.JWTIssuer,
		cfg.JWTAudience,
		cfg.AccessTokenTTL,
		cfg.JWTClockSkew,
	)
	if err != nil {
		db.Close()
		return nil, err
	}

	notifications, err := NewNotificationSender(cfg, logger)
	if err != nil {
		db.Close()
		return nil, err
	}

	verifier, err := NewServiceVerifier(
		cfg.InternalServiceSecret,
		cfg.InternalServiceSecretPrevious,
		cfg.AllowedInternalCallers,
		cfg.InternalAuthMaxSkew,
	)
	if err != nil {
		db.Close()
		return nil, err
	}

	repository := NewRepository(
		db,
		cfg.ActivationTokenTTL,
		cfg.PasswordResetTokenTTL,
		cfg.RefreshTokenTTL,
	)

	service, err := NewService(
		repository,
		hasher,
		tokenManager,
		notifications,
		cfg.AccessTokenTTL,
		cfg.RefreshTokenTTL,
		cfg.SessionAbsoluteTTL,
		cfg.PasswordResetTokenTTL,
		cfg.MaxFailedLoginAttempts,
		cfg.AccountLockoutDuration,
	)
	if err != nil {
		db.Close()
		return nil, err
	}

	handler := NewHandler(service, cfg, logger)

	router, err := newRouter(cfg, logger, db, tokenManager, verifier, handler)
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

	return &App{
		cfg:    cfg,
		logger: logger,
		db:     db,
		server: server,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)

	go func() {
		a.logger.Info("server_started",
			"service", a.cfg.ServiceName,
			"address", a.server.Addr,
			"environment", a.cfg.Environment,
		)

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
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), a.cfg.ShutdownTimeout,
		)
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
	tokens *TokenManager,
	internalVerifier *ServiceVerifier,
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
		c.JSON(http.StatusOK, gin.H{
			"service": cfg.ServiceName,
			"status":  "healthy",
		})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service":  cfg.ServiceName,
				"status":   "not_ready",
				"database": "unavailable",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"service":  cfg.ServiceName,
			"status":   "ready",
			"database": "connected",
		})
	})

	authMiddleware := AuthMiddleware(
		tokens,
		cfg.CookieEnabled,
		cfg.AccessTokenCookieName,
		cfg.CSRFCookieName,
		cfg.CSRFHeaderName,
	)

	loginLimiter := newIPRateLimiter(cfg.LoginRateLimitPerMinute, time.Minute)
	forgotLimiter := newIPRateLimiter(cfg.ForgotRateLimitPerMinute, time.Minute)

	handler.RegisterRoutes(router, RouteDependencies{
		Auth:            authMiddleware,
		Admin:           RequireRoles(RoleAdmin),
		Internal:        InternalAuthMiddleware(internalVerifier, cfg.MaxRequestBytes),
		LoginRateLimit:  RateLimitMiddleware(loginLimiter, clientIPKey),
		ForgotRateLimit: RateLimitMiddleware(forgotLimiter, clientIPKey),
	})

	router.NoRoute(func(c *gin.Context) {
		RespondError(c, http.StatusNotFound, CodeNotFound,
			"The requested resource was not found.")
	})

	return router, nil
}

func clientIPKey(c *gin.Context) string {
	return c.ClientIP()
}
