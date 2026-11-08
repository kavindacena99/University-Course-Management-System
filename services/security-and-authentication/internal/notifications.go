package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"security-and-authentication/internal/config"
)

type NotificationSender interface {
	SendActivation(ctx context.Context, user *AuthUser, token string) error
	SendPasswordReset(ctx context.Context, user *AuthUser, token string) error
}

type notification struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	Link      string    `json:"link"`
}

type fileNotificationSender struct {
	path    string
	baseURL string
	mu      sync.Mutex
}

func newFileNotificationSender(path, baseURL string) (*fileNotificationSender, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create notification directory: %w", err)
	}
	return &fileNotificationSender{
		path:    path,
		baseURL: strings.TrimRight(baseURL, "/"),
	}, nil
}

func (s *fileNotificationSender) SendActivation(_ context.Context, user *AuthUser, token string) error {
	return s.write(notification{
		Type:      "activation",
		Timestamp: time.Now().UTC(),
		UserID:    user.UserID.String(),
		Email:     user.Email,
		Token:     token,
		Link:      s.baseURL + "/activate?token=" + url.QueryEscape(token),
	})
}

func (s *fileNotificationSender) SendPasswordReset(_ context.Context, user *AuthUser, token string) error {
	return s.write(notification{
		Type:      "password_reset",
		Timestamp: time.Now().UTC(),
		UserID:    user.UserID.String(),
		Email:     user.Email,
		Token:     token,
		Link:      s.baseURL + "/reset-password?token=" + url.QueryEscape(token),
	})
}

func (s *fileNotificationSender) write(item notification) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(s.path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open notification file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write notification file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync notification file: %w", err)
	}

	return nil
}

type logNotificationSender struct {
	logger *slog.Logger
}

func (s *logNotificationSender) SendActivation(_ context.Context, user *AuthUser, token string) error {
	s.logger.Warn("development_notification_activation",
		"user_id", user.UserID,
		"email", user.Email,
		"token", token,
	)
	return nil
}

func (s *logNotificationSender) SendPasswordReset(_ context.Context, user *AuthUser, token string) error {
	s.logger.Warn("development_notification_password_reset",
		"user_id", user.UserID,
		"email", user.Email,
		"token", token,
	)
	return nil
}

type noopNotificationSender struct {
	logger *slog.Logger
}

func (s *noopNotificationSender) SendActivation(_ context.Context, user *AuthUser, _ string) error {
	s.logger.Error("activation_notification_not_delivered",
		"user_id", user.UserID,
		"reason", "no notification provider is configured",
	)
	return nil
}

func (s *noopNotificationSender) SendPasswordReset(_ context.Context, user *AuthUser, _ string) error {
	s.logger.Error("password_reset_notification_not_delivered",
		"user_id", user.UserID,
		"reason", "no notification provider is configured",
	)
	return nil
}

func NewNotificationSender(
	cfg *config.Config,
	logger *slog.Logger,
) (NotificationSender, error) {
	if logger == nil {
		logger = slog.Default()
	}

	switch cfg.NotificationMode {
	case "file":
		return newFileNotificationSender(cfg.NotificationFilePath, cfg.PublicBaseURL)
	case "log":
		return &logNotificationSender{logger: logger}, nil
	case "none":
		return &noopNotificationSender{logger: logger}, nil
	default:
		return nil, fmt.Errorf("unsupported notification mode %q", cfg.NotificationMode)
	}
}
