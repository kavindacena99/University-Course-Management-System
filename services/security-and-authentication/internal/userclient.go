package auth

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

// EligibilityChecker verifies the canonical registration state of a user
// against the User Service before authentication or token issuance.
type EligibilityChecker interface {
	CheckEligibility(ctx context.Context, userID uuid.UUID) error
}

type eligibilityResponse struct {
	UserID             uuid.UUID          `json:"user_id"`
	Eligible           bool               `json:"eligible"`
	RegistrationStatus RegistrationStatus `json:"registration_status"`
	StatusReason       string             `json:"status_reason"`
	Version            int64              `json:"version"`
}

type UserServiceClient struct {
	baseURL string
	signer  *ServiceSigner
	client  *http.Client
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

func (c *UserServiceClient) CheckEligibility(
	ctx context.Context,
	userID uuid.UUID,
) error {
	requestURL := c.baseURL + "/internal/users/" + userID.String() + "/eligibility"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("%w: create request: %v", ErrEligibilityUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")

	if err := c.signer.Sign(req, nil); err != nil {
		return fmt.Errorf("%w: sign request: %v", ErrEligibilityUnavailable, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEligibilityUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return fmt.Errorf("%w: read response: %v", ErrEligibilityUnavailable, err)
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return ErrUserIneligible
	case http.StatusOK:
	default:
		return fmt.Errorf("%w: user service returned %d",
			ErrEligibilityUnavailable, resp.StatusCode)
	}

	var result eligibilityResponse
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return fmt.Errorf("%w: decode response: %v", ErrEligibilityUnavailable, err)
	}

	if result.UserID != userID {
		return fmt.Errorf("%w: user identity mismatch", ErrEligibilityUnavailable)
	}
	if !result.Eligible || result.RegistrationStatus != StatusApproved {
		return ErrUserIneligible
	}

	return nil
}
