package course

import (
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

type UserServiceClient struct {
	baseURL string
	signer  *ServiceSigner
	client  *http.Client
}

type internalUserResponse struct {
	User struct {
		ID                 uuid.UUID `json:"id"`
		RegistrationStatus string    `json:"registration_status"`
		Version            int64     `json:"version"`
	} `json:"user"`
}

func NewUserServiceClient(
	baseURL string,
	signer *ServiceSigner,
	timeout time.Duration,
) (*UserServiceClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("user service URL must be absolute")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("user service URL must use http or https")
	}
	if signer == nil {
		return nil, fmt.Errorf("service signer is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("user service timeout must be positive")
	}

	return &UserServiceClient{
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

// EnsureEligibleLecturer verifies live registration approval. Role membership
// is verified separately against the Authentication Service.
func (c *UserServiceClient) EnsureEligibleLecturer(
	ctx context.Context,
	userID uuid.UUID,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/internal/users/"+url.PathEscape(userID.String()),
		nil,
	)
	if err != nil {
		return fmt.Errorf("%w: create user request: %v", ErrUserServiceUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")

	if err := c.signer.Sign(req, nil); err != nil {
		return fmt.Errorf("%w: sign user request: %v", ErrUserServiceUnavailable, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUserServiceUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return ErrLecturerNotFound
	case resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: user service returned %d",
			ErrUserServiceUnavailable, resp.StatusCode)
	}

	var payload internalUserResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("%w: malformed response: %v", ErrUserServiceUnavailable, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: trailing user response data", ErrUserServiceUnavailable)
	}
	if payload.User.ID != userID {
		return fmt.Errorf("%w: response user id does not match", ErrUserServiceUnavailable)
	}
	if !strings.EqualFold(payload.User.RegistrationStatus, "approved") {
		return ErrLecturerIneligible
	}

	return nil
}
