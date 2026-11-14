package course

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

	"course/database"
	"course/internal/config"
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

	verifier, err := NewTokenVerifierFromConfig(cfg)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure access token verification: %w", err)
	}

	signer, err := NewServiceSigner(cfg.InternalServiceID, cfg.InternalServiceSecret)
	if err != nil {
		db.Close()
		return nil, err
	}
	internalVerifier, err := NewServiceVerifier(
		cfg.InternalServiceSecret,
		cfg.InternalServiceSecretPrevious,
		cfg.AllowedInternalCallers,
		cfg.InternalAuthMaxSkew,
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure internal authentication: %w", err)
	}

	userClient, err := NewUserServiceClient(
		cfg.UserServiceInternalURL,
		signer,
		cfg.UserServiceTimeout,
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure user service client: %w", err)
	}
	authClient, err := NewAuthRoleClient(cfg.AuthServiceInternalURL, signer, cfg.AuthServiceTimeout)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure auth service client: %w", err)
	}

	repository := NewRepository(db)
	service := NewService(repository, &LecturerVerifier{users: userClient, auth: authClient})
	handler := NewHandler(service, logger)

	router, err := newRouter(cfg, logger, db, verifier, internalVerifier, handler)
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
	verifier *TokenVerifier,
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
		var schemaVersion int64
		var schemaDirty bool
		if err := db.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(
			&schemaVersion, &schemaDirty,
		); err != nil || schemaDirty || schemaVersion < 1 {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"service": cfg.ServiceName, "status": "not_ready",
				"database": "connected", "schema": "unavailable",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"service":  cfg.ServiceName,
			"status":   "ready",
			"database": "connected",
			"schema":   "ready",
		})
	})

	auth := AuthMiddleware(
		verifier,
		cfg.AuthCookieEnabled,
		cfg.AccessTokenCookieName,
		cfg.CSRFCookieName,
		cfg.CSRFHeaderName,
	)

	handler.RegisterRoutes(router, auth,
		InternalAuthMiddleware(internalVerifier, cfg.MaxRequestBytes))

	router.NoRoute(func(c *gin.Context) {
		RespondError(c, http.StatusNotFound, CodeNotFound,
			"The requested resource was not found.")
	})

	return router, nil
}
