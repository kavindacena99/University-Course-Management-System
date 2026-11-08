package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Store interface {
	Ping(ctx context.Context) error
	GetUserByID(ctx context.Context, userID uuid.UUID) (*AuthUser, error)
	GetUserByEmail(ctx context.Context, email string) (*AuthUser, error)
	RecordLoginFailure(ctx context.Context, userID uuid.UUID, maxAttempts int, lockedUntil time.Time) error
	RecordLoginSuccess(ctx context.Context, userID uuid.UUID, rehashedPassword string) error
	CreateSession(ctx context.Context, session NewSession) error
	RotateRefreshToken(ctx context.Context, rawHash []byte, newTokenID uuid.UUID, newHash []byte, now time.Time) (*RotationResult, error)
	RevokeSessionByRefreshHash(ctx context.Context, rawHash []byte, reason string) (bool, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, reason string, skipOwnership bool) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID, exceptSessionID *uuid.UUID, reason string) error
	ListSessions(ctx context.Context, userID uuid.UUID, currentSessionID uuid.UUID) ([]Session, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string, revokeSessionsExcept *uuid.UUID) error
	CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	ConsumeActionTokenAndSetPassword(ctx context.Context, tokenHash []byte, purpose string, newPasswordHash string, now time.Time) (*AuthUser, error)
	ProvisionUser(ctx context.Context, input ProvisionUserInput, passwordHash string) (*AuthUser, error)
	ListRoles(ctx context.Context) ([]Role, error)
	SetUserRoles(ctx context.Context, userID uuid.UUID, roles []string) (*AuthUser, error)
	UnlockUser(ctx context.Context, userID uuid.UUID) (*AuthUser, error)
	ProcessUserEvent(ctx context.Context, event UserEvent, now time.Time, notify NotifyFunc) (InboxResult, error)
}

type Service struct {
	store         Store
	hasher        *PasswordHasher
	tokens        *TokenManager
	notifications NotificationSender
	now           func() time.Time
	accessTTL     time.Duration
	refreshTTL    time.Duration
	sessionTTL    time.Duration
	resetTTL      time.Duration
	maxFailed     int
	lockout       time.Duration
	dummyHash     string
}

func NewService(
	store Store,
	hasher *PasswordHasher,
	tokens *TokenManager,
	notifications NotificationSender,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	sessionTTL time.Duration,
	resetTTL time.Duration,
	maxFailed int,
	lockout time.Duration,
) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("auth store is required")
	}
	if hasher == nil {
		return nil, fmt.Errorf("password hasher is required")
	}
	if tokens == nil {
		return nil, fmt.Errorf("token manager is required")
	}
	if notifications == nil {
		return nil, fmt.Errorf("notification sender is required")
	}
	if accessTTL <= 0 || refreshTTL <= 0 || sessionTTL <= 0 || resetTTL <= 0 {
		return nil, fmt.Errorf("token lifetimes must be positive")
	}
	if maxFailed < 1 {
		return nil, fmt.Errorf("max failed login attempts must be positive")
	}
	if lockout <= 0 {
		return nil, fmt.Errorf("account lockout duration must be positive")
	}

	dummyHash, err := hasher.Hash("dummy-password-used-for-timing-only")
	if err != nil {
		return nil, fmt.Errorf("prepare password verification: %w", err)
	}

	return &Service{
		store:         store,
		hasher:        hasher,
		tokens:        tokens,
		notifications: notifications,
		now:           time.Now,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		sessionTTL:    sessionTTL,
		resetTTL:      resetTTL,
		maxFailed:     maxFailed,
		lockout:       lockout,
		dummyHash:     dummyHash,
	}, nil
}

type ClientInfo struct {
	UserAgent string
	IPAddress string
}

type LoginInput struct {
	Email    string
	Password string
	Client   ClientInfo
}

func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	email := normalizeEmail(input.Email)
	if !validEmail(email) || input.Password == "" || len(input.Password) > 128 {
		_, _ = s.hasher.Verify(input.Password, s.dummyHash)
		return nil, ErrInvalidCredentials
	}

	now := s.now().UTC()
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_, _ = s.hasher.Verify(input.Password, s.dummyHash)
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if user.IsLocked(now) {
		return nil, ErrAccountLocked
	}
	if err := loginAvailability(user); err != nil {
		return nil, err
	}

	matches, err := s.hasher.Verify(input.Password, user.PasswordHash)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !matches {
		if err := s.store.RecordLoginFailure(
			ctx,
			user.UserID,
			s.maxFailed,
			now.Add(s.lockout),
		); err != nil {
			return nil, err
		}
		return nil, ErrInvalidCredentials
	}

	rehashed := ""
	if s.hasher.NeedsRehash(user.PasswordHash) {
		rehashed, err = s.hasher.Hash(input.Password)
		if err != nil {
			return nil, fmt.Errorf("rehash password: %w", err)
		}
	}
	if err := s.store.RecordLoginSuccess(ctx, user.UserID, rehashed); err != nil {
		return nil, err
	}

	sessionID := uuid.New()
	refreshRaw, refreshHash, err := GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}
	absoluteExpiry := now.Add(s.sessionTTL)
	refreshExpiry := now.Add(s.refreshTTL)
	if refreshExpiry.After(absoluteExpiry) {
		refreshExpiry = absoluteExpiry
	}

	if err := s.store.CreateSession(ctx, NewSession{
		ID:                    sessionID,
		UserID:                user.UserID,
		UserAgent:             sanitizeText(input.Client.UserAgent, 512),
		IPAddress:             sanitizeText(input.Client.IPAddress, 64),
		AbsoluteExpiresAt:     absoluteExpiry,
		RefreshTokenID:        uuid.New(),
		RefreshTokenHash:      refreshHash,
		RefreshTokenExpiresAt: refreshExpiry,
	}); err != nil {
		return nil, err
	}

	accessToken, accessExpiry, csrfToken, err := s.issueAccess(user, sessionID)
	if err != nil {
		_ = s.store.RevokeSession(ctx, sessionID, uuid.Nil, "token issuance failed", true)
		return nil, err
	}

	return &LoginResult{
		User:          user,
		AccessToken:   accessToken,
		RefreshToken:  refreshRaw,
		SessionID:     sessionID,
		AccessExpiry:  accessExpiry,
		RefreshExpiry: refreshExpiry,
		CSRFToken:     csrfToken,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*RefreshResult, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" || len(rawRefreshToken) > 512 {
		return nil, ErrRefreshInvalid
	}

	newRaw, newHash, err := GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	rotation, err := s.store.RotateRefreshToken(
		ctx,
		HashOpaqueToken(rawRefreshToken),
		uuid.New(),
		newHash,
		now,
	)
	if err != nil {
		return nil, err
	}

	if err := loginAvailability(rotation.User); err != nil {
		_ = s.store.RevokeSession(ctx, rotation.SessionID, uuid.Nil,
			"account is not available", true)
		return nil, err
	}

	accessToken, accessExpiry, csrfToken, err := s.issueAccess(
		rotation.User,
		rotation.SessionID,
	)
	if err != nil {
		_ = s.store.RevokeSession(ctx, rotation.SessionID, uuid.Nil,
			"token issuance failed", true)
		return nil, err
	}

	return &RefreshResult{
		User:          rotation.User,
		AccessToken:   accessToken,
		RefreshToken:  newRaw,
		SessionID:     rotation.SessionID,
		AccessExpiry:  accessExpiry,
		RefreshExpiry: rotation.RefreshExpiry,
		CSRFToken:     csrfToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) (bool, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" || len(rawRefreshToken) > 512 {
		return false, nil
	}
	return s.store.RevokeSessionByRefreshHash(
		ctx,
		HashOpaqueToken(rawRefreshToken),
		"logout",
	)
}

func (s *Service) LogoutAll(ctx context.Context, claims *AccessClaims) error {
	userID, err := claimsUserID(claims)
	if err != nil {
		return err
	}
	return s.store.RevokeAllSessions(ctx, userID, nil, "logout all")
}

func (s *Service) Sessions(ctx context.Context, claims *AccessClaims) ([]Session, error) {
	userID, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	currentSessionID := uuid.Nil
	if claims.SessionID != "" {
		if parsed, parseErr := uuid.Parse(claims.SessionID); parseErr == nil {
			currentSessionID = parsed
		}
	}

	return s.store.ListSessions(ctx, userID, currentSessionID)
}

func (s *Service) RevokeSession(
	ctx context.Context,
	claims *AccessClaims,
	sessionID uuid.UUID,
) error {
	userID, err := claimsUserID(claims)
	if err != nil {
		return err
	}
	if sessionID == uuid.Nil {
		return newValidationError("session_id", "must be a valid UUID")
	}
	return s.store.RevokeSession(ctx, sessionID, userID, "revoked by user", false)
}

func (s *Service) Me(ctx context.Context, claims *AccessClaims) (*UserSummary, error) {
	userID, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	summary := user.Summary()
	return &summary, nil
}

func (s *Service) Activate(ctx context.Context, token, password string) (*AuthUser, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 512 {
		return nil, ErrActionTokenInvalid
	}
	if err := ValidatePasswordPolicy(password); err != nil {
		return nil, err
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	return s.store.ConsumeActionTokenAndSetPassword(
		ctx,
		HashOpaqueToken(token),
		ActionTokenActivation,
		passwordHash,
		s.now().UTC(),
	)
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

func (s *Service) ChangePassword(
	ctx context.Context,
	claims *AccessClaims,
	input ChangePasswordInput,
) error {
	userID, err := claimsUserID(claims)
	if err != nil {
		return err
	}
	if input.CurrentPassword == "" {
		return newValidationError("current_password", "is required")
	}
	if err := ValidatePasswordPolicy(input.NewPassword); err != nil {
		return err
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := loginAvailability(user); err != nil {
		return err
	}

	matches, err := s.hasher.Verify(input.CurrentPassword, user.PasswordHash)
	if err != nil || !matches {
		return ErrInvalidCredentials
	}

	reused, err := s.hasher.Verify(input.NewPassword, user.PasswordHash)
	if err != nil || reused {
		return ErrPasswordReuse
	}

	passwordHash, err := s.hasher.Hash(input.NewPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	var currentSessionID *uuid.UUID
	if parsed, parseErr := uuid.Parse(claims.SessionID); parseErr == nil {
		currentSessionID = &parsed
	}

	return s.store.ChangePassword(ctx, userID, passwordHash, currentSessionID)
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	if !user.IsActive() {
		return nil
	}

	rawToken, tokenHash, err := GenerateOpaqueToken()
	if err != nil {
		return err
	}

	if err := s.store.CreatePasswordResetToken(
		ctx,
		user.UserID,
		tokenHash,
		s.now().UTC().Add(s.resetTTL),
	); err != nil {
		return err
	}

	if err := s.notifications.SendPasswordReset(ctx, user, rawToken); err != nil {
		return fmt.Errorf("send password reset notification: %w", err)
	}
	return nil
}

func (s *Service) ResetPassword(
	ctx context.Context,
	token string,
	newPassword string,
) (*AuthUser, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 512 {
		return nil, ErrActionTokenInvalid
	}
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return nil, err
	}

	passwordHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	return s.store.ConsumeActionTokenAndSetPassword(
		ctx,
		HashOpaqueToken(token),
		ActionTokenPasswordReset,
		passwordHash,
		s.now().UTC(),
	)
}

func (s *Service) ProvisionUser(
	ctx context.Context,
	input ProvisionUserInput,
) (*AuthUser, error) {
	if input.UserID == uuid.Nil {
		return nil, newValidationError("user_id", "must be a valid UUID")
	}
	if !validEmail(normalizeEmail(input.Email)) {
		return nil, newValidationError("email", "must be a valid email address")
	}
	firstName, err := validateName("first_name", input.FirstName)
	if err != nil {
		return nil, err
	}
	lastName, err := validateName("last_name", input.LastName)
	if err != nil {
		return nil, err
	}
	if err := ValidatePasswordPolicy(input.Password); err != nil {
		return nil, err
	}

	roles, err := normalizeRoles(input.Roles)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, newValidationError("roles", "must contain at least one role")
	}

	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	input.Email = normalizeEmail(input.Email)
	input.FirstName = firstName
	input.LastName = lastName
	input.Roles = roles

	return s.store.ProvisionUser(ctx, input, passwordHash)
}

func (s *Service) ProcessUserEvent(
	ctx context.Context,
	event UserEvent,
) (InboxResult, error) {
	notify := func(user *AuthUser, token string) error {
		return s.notifications.SendActivation(ctx, user, token)
	}
	return s.store.ProcessUserEvent(ctx, event, s.now().UTC(), notify)
}

func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.store.ListRoles(ctx)
}

func (s *Service) SetUserRoles(
	ctx context.Context,
	userID uuid.UUID,
	roles []string,
) (*AuthUser, error) {
	if userID == uuid.Nil {
		return nil, newValidationError("user_id", "must be a valid UUID")
	}
	normalized, err := normalizeRoles(roles)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return nil, newValidationError("roles", "must contain at least one role")
	}
	return s.store.SetUserRoles(ctx, userID, normalized)
}

func (s *Service) UnlockUser(ctx context.Context, userID uuid.UUID) (*AuthUser, error) {
	if userID == uuid.Nil {
		return nil, newValidationError("user_id", "must be a valid UUID")
	}
	return s.store.UnlockUser(ctx, userID)
}

func (s *Service) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return newValidationError("user_id", "must be a valid UUID")
	}
	if _, err := s.store.GetUserByID(ctx, userID); err != nil {
		return err
	}
	return s.store.RevokeAllSessions(ctx, userID, nil, "revoked by administrator")
}

func (s *Service) issueAccess(
	user *AuthUser,
	sessionID uuid.UUID,
) (string, time.Time, string, error) {
	accessToken, accessExpiry, err := s.tokens.IssueAccessToken(
		user.UserID,
		sessionID,
		user.Roles,
	)
	if err != nil {
		return "", time.Time{}, "", err
	}

	csrfToken, err := GenerateCSRFToken()
	if err != nil {
		return "", time.Time{}, "", err
	}

	return accessToken, accessExpiry, csrfToken, nil
}

func loginAvailability(user *AuthUser) error {
	if user == nil {
		return ErrAccountUnavailable
	}
	if user.Disabled || len(user.Roles) == 0 {
		return ErrAccountUnavailable
	}
	switch user.RegistrationStatus {
	case StatusApproved:
	case StatusPending, StatusRejected, StatusSuspended:
		return ErrAccountUnavailable
	default:
		return ErrAccountUnavailable
	}
	if user.PasswordHash == "" || user.ActivatedAt == nil {
		return ErrAccountNotActivated
	}
	return nil
}

func claimsUserID(claims *AccessClaims) (uuid.UUID, error) {
	if claims == nil {
		return uuid.Nil, ErrUnauthorized
	}
	userID, err := claims.UserID()
	if err != nil || userID == uuid.Nil {
		return uuid.Nil, ErrUnauthorized
	}
	return userID, nil
}

func validEmail(value string) bool {
	if value == "" || len(value) > 254 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(address.Address, value) {
		return false
	}
	return strings.Contains(address.Address, "@")
}

func validateName(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", newValidationError(field, "is required")
	}
	if utf8.RuneCountInString(value) > 100 {
		return "", newValidationError(field, "must not exceed 100 characters")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", newValidationError(field, "must not contain control characters")
		}
	}
	return value, nil
}

func normalizeRoles(roles []string) ([]string, error) {
	if len(roles) > len(AllRoles) {
		return nil, newValidationError("roles", "contains too many roles")
	}

	seen := make(map[string]struct{}, len(roles))
	normalized := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if !IsValidRole(role) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRole, role)
		}
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		normalized = append(normalized, role)
	}
	return normalized, nil
}

func sanitizeText(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	if len(value) > maxLength {
		value = value[:maxLength]
	}
	return value
}
