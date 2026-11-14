package enrollment

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

type signedHTTPClient struct {
	baseURL string
	signer  *ServiceSigner
	client  *http.Client
}

func newSignedHTTPClient(baseURL string, signer *ServiceSigner, timeout time.Duration) (*signedHTTPClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("internal service URL must be an absolute HTTP URL without a path")
	}
	if signer == nil || timeout <= 0 {
		return nil, fmt.Errorf("service signer and positive timeout are required")
	}
	return &signedHTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"), signer: signer,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *signedHTTPClient) get(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: create internal request: %v", ErrDependencyUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	if requestID := requestIDFromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	if err := c.signer.Sign(req, nil); err != nil {
		return nil, 0, fmt.Errorf("%w: sign internal request: %v", ErrDependencyUnavailable, err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: internal HTTP request: %v", ErrDependencyUnavailable, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10+1))
	if err != nil || len(body) > 64<<10 {
		return nil, 0, fmt.Errorf("%w: invalid internal response", ErrDependencyUnavailable)
	}
	return body, resp.StatusCode, nil
}

func decodeStrict(body []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing response data")
	}
	return nil
}

type UserEligibilityClient struct{ *signedHTTPClient }
type StudentRoleClient struct{ *signedHTTPClient }
type CourseOfferingClient struct{ *signedHTTPClient }

func NewUserEligibilityClient(baseURL string, signer *ServiceSigner, timeout time.Duration) (*UserEligibilityClient, error) {
	client, err := newSignedHTTPClient(baseURL, signer, timeout)
	if err != nil {
		return nil, err
	}
	return &UserEligibilityClient{client}, nil
}

func NewStudentRoleClient(baseURL string, signer *ServiceSigner, timeout time.Duration) (*StudentRoleClient, error) {
	client, err := newSignedHTTPClient(baseURL, signer, timeout)
	if err != nil {
		return nil, err
	}
	return &StudentRoleClient{client}, nil
}

func NewCourseOfferingClient(baseURL string, signer *ServiceSigner, timeout time.Duration) (*CourseOfferingClient, error) {
	client, err := newSignedHTTPClient(baseURL, signer, timeout)
	if err != nil {
		return nil, err
	}
	return &CourseOfferingClient{client}, nil
}

func (c *UserEligibilityClient) EnsureApproved(ctx context.Context, userID uuid.UUID) error {
	body, status, err := c.get(ctx, "/internal/users/"+userID.String()+"/eligibility")
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return ErrStudentIneligible
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: user service returned %d", ErrDependencyUnavailable, status)
	}
	var result struct {
		UserID             uuid.UUID `json:"user_id"`
		Eligible           *bool     `json:"eligible"`
		RegistrationStatus string    `json:"registration_status"`
		Version            int64     `json:"version"`
	}
	if err := decodeStrict(body, &result); err != nil ||
		result.UserID != userID || result.Eligible == nil || result.Version < 1 {
		return fmt.Errorf("%w: malformed user eligibility response", ErrDependencyUnavailable)
	}
	if !*result.Eligible || result.RegistrationStatus != "approved" {
		return ErrStudentIneligible
	}
	return nil
}

func (c *StudentRoleClient) EnsureActiveStudent(ctx context.Context, userID uuid.UUID) error {
	body, status, err := c.get(ctx, "/internal/users/"+userID.String()+"/student-eligibility")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: auth service returned %d", ErrDependencyUnavailable, status)
	}
	var result struct {
		UserID   uuid.UUID `json:"user_id"`
		Eligible *bool     `json:"eligible"`
	}
	if err := decodeStrict(body, &result); err != nil ||
		result.UserID != userID || result.Eligible == nil {
		return fmt.Errorf("%w: malformed student role response", ErrDependencyUnavailable)
	}
	if !*result.Eligible {
		return ErrStudentIneligible
	}
	return nil
}

func (c *StudentRoleClient) EnsureActiveLecturer(ctx context.Context, userID uuid.UUID) error {
	body, status, err := c.get(ctx, "/internal/users/"+userID.String()+"/lecturer-eligibility")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: auth service returned %d", ErrDependencyUnavailable, status)
	}
	var result struct {
		UserID   uuid.UUID `json:"user_id"`
		Eligible *bool     `json:"eligible"`
	}
	if err := decodeStrict(body, &result); err != nil ||
		result.UserID != userID || result.Eligible == nil {
		return fmt.Errorf("%w: malformed lecturer role response", ErrDependencyUnavailable)
	}
	if !*result.Eligible {
		return ErrStudentIneligible
	}
	return nil
}

func (c *CourseOfferingClient) GetOffering(ctx context.Context, offeringID uuid.UUID) (*OfferingFacts, error) {
	body, status, err := c.get(ctx, "/internal/offerings/"+offeringID.String())
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: course service returned %d", ErrDependencyUnavailable, status)
	}
	var result struct {
		Offering *OfferingFacts `json:"offering"`
	}
	if err := decodeStrict(body, &result); err != nil ||
		result.Offering == nil || result.Offering.ID != offeringID ||
		result.Offering.Version < 1 {
		return nil, fmt.Errorf("%w: malformed offering response", ErrDependencyUnavailable)
	}
	switch result.Offering.Status {
	case "draft", "published", "closed", "archived":
	default:
		return nil, fmt.Errorf("%w: unknown offering status", ErrDependencyUnavailable)
	}
	if result.Offering.EnrollmentCapacity != nil && *result.Offering.EnrollmentCapacity < 1 {
		return nil, fmt.Errorf("%w: invalid offering capacity", ErrDependencyUnavailable)
	}
	return result.Offering, nil
}
