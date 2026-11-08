package user

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AuthClient struct {
	baseURL string
	signer  *ServiceSigner
	client  *http.Client
}

func NewAuthClient(
	baseURL string,
	signer *ServiceSigner,
	timeout time.Duration,
) (*AuthClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("auth service URL must be absolute")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("auth service URL must use http or https")
	}
	if signer == nil {
		return nil, fmt.Errorf("service signer is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("auth service timeout must be positive")
	}

	return &AuthClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		signer:  signer,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

type provisionUserRequest struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Version   int64     `json:"version"`
	Password  string    `json:"password"`
	Roles     []string  `json:"roles"`
}

func (c *AuthClient) ProvisionAdmin(
	ctx context.Context,
	u *User,
	password string,
) error {
	payload, err := json.Marshal(provisionUserRequest{
		UserID:    u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Version:   u.Version,
		Password:  password,
		Roles:     []string{RoleAdmin},
	})
	if err != nil {
		return fmt.Errorf("marshal provisioning request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/users",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create provisioning request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if err := c.signer.Sign(req, payload); err != nil {
		return fmt.Errorf("sign provisioning request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("call auth service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseError("provision user", resp)
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return nil
}

func (c *AuthClient) SendUserEvent(ctx context.Context, event OutboxEvent) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/internal/events/users",
		bytes.NewReader(event.Payload),
	)
	if err != nil {
		return fmt.Errorf("create user event request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if err := c.signer.Sign(req, event.Payload); err != nil {
		return fmt.Errorf("sign user event request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("deliver user event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return responseError("deliver user event", resp)
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return nil
}

func responseError(operation string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s: auth service returned %d", operation, resp.StatusCode)
	}
	return fmt.Errorf("%s: auth service returned %d: %s",
		operation, resp.StatusCode, message)
}
