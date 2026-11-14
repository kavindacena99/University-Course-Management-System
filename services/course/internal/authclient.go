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

// AuthRoleClient checks the role and activation state owned by Auth Service.
type AuthRoleClient struct {
	baseURL string
	signer  *ServiceSigner
	client  *http.Client
}

func NewAuthRoleClient(baseURL string, signer *ServiceSigner, timeout time.Duration) (*AuthRoleClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Path != "" || parsed.RawQuery != "" {
		return nil, fmt.Errorf("auth service URL must be an absolute HTTP URL without a path")
	}
	if signer == nil || timeout <= 0 {
		return nil, fmt.Errorf("auth service signer and positive timeout are required")
	}
	return &AuthRoleClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		signer:  signer,
		client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (c *AuthRoleClient) EnsureEligibleLecturer(ctx context.Context, userID uuid.UUID) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/internal/users/"+userID.String()+"/lecturer-eligibility", nil)
	if err != nil {
		return fmt.Errorf("%w: create role request: %v", ErrAuthServiceUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	if err := c.signer.Sign(req, nil); err != nil {
		return fmt.Errorf("%w: sign role request: %v", ErrAuthServiceUnavailable, err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthServiceUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: auth service returned %d", ErrAuthServiceUnavailable, resp.StatusCode)
	}
	var result struct {
		UserID   uuid.UUID `json:"user_id"`
		Eligible *bool     `json:"eligible"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4096))
	if err := decoder.Decode(&result); err != nil || result.UserID != userID || result.Eligible == nil {
		return fmt.Errorf("%w: malformed eligibility response", ErrAuthServiceUnavailable)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%w: trailing eligibility response data", ErrAuthServiceUnavailable)
	}
	if !*result.Eligible {
		return ErrLecturerIneligible
	}
	return nil
}

// LecturerVerifier requires both service owners to affirm eligibility.
type LecturerVerifier struct {
	users LecturerEligibility
	auth  LecturerEligibility
}

func (v *LecturerVerifier) EnsureEligibleLecturer(ctx context.Context, userID uuid.UUID) error {
	if v.users == nil || v.auth == nil {
		return ErrAuthServiceUnavailable
	}
	if err := v.users.EnsureEligibleLecturer(ctx, userID); err != nil {
		return err
	}
	return v.auth.EnsureEligibleLecturer(ctx, userID)
}
