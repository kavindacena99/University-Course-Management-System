package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CourseClient struct {
	baseURL   string
	serviceID string
	secret    string
	client    *http.Client
}

func NewCourseClient(baseURL, serviceID, secret string) *CourseClient {
	return &CourseClient{
		baseURL: strings.TrimRight(baseURL, "/"), serviceID: serviceID, secret: secret,
		client: &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

func (c *CourseClient) GetOfferingFacts(
	ctx context.Context, offeringID uuid.UUID,
) (*CourseFacts, error) {
	path := "/internal/analytics/offerings/" + offeringID.String()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create Course facts request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	SignRequest(request, c.serviceID, c.secret, nil)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: facts request failed: %v", ErrDependencyUnavailable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10+1))
	if err != nil || len(body) > 64<<10 {
		return nil, fmt.Errorf("%w: invalid facts response", ErrDependencyUnavailable)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: facts request returned HTTP %d", ErrDependencyUnavailable, response.StatusCode)
	}
	var result struct {
		Offering CourseFacts `json:"offering"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%w: decode facts response: %v", ErrDependencyUnavailable, err)
	}
	facts := &result.Offering
	if facts.ID != offeringID || facts.CourseID == uuid.Nil || facts.Credits < 1 ||
		facts.AcademicYear == "" || facts.Semester == "" {
		return nil, fmt.Errorf("%w: Course returned incomplete offering facts", ErrDependencyUnavailable)
	}
	return facts, nil
}

type AssessmentClient struct {
	baseURL   string
	serviceID string
	secret    string
	client    *http.Client
}

func NewAssessmentClient(baseURL, serviceID, secret string) *AssessmentClient {
	return &AssessmentClient{
		baseURL: strings.TrimRight(baseURL, "/"), serviceID: serviceID, secret: secret,
		client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}
}

func (c *AssessmentClient) ListEvents(
	ctx context.Context, after time.Time, afterID uuid.UUID,
) (*AssessmentEventPage, error) {
	query := url.Values{}
	query.Set("after_occurred_at", after.UTC().Format(time.RFC3339Nano))
	query.Set("after_event_id", afterID.String())
	query.Set("limit", "250")
	path := "/internal/analytics/events?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create Assessment event request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	SignRequest(request, c.serviceID, c.secret, nil)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Assessment event request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil || len(body) > 4<<20 {
		return nil, fmt.Errorf("Assessment returned an invalid event response")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Assessment event request returned HTTP %d", response.StatusCode)
	}
	var page AssessmentEventPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decode Assessment event response: %w", err)
	}
	if page.HasMore && (page.NextOccurredAt == nil || page.NextEventID == nil) {
		return nil, fmt.Errorf("Assessment returned an incomplete event cursor")
	}
	if page.HasMore && len(page.Events) == 0 {
		return nil, fmt.Errorf("Assessment returned an empty page with more events")
	}
	for _, event := range page.Events {
		if event.EventID == uuid.Nil || event.OccurredAt.IsZero() || len(event.Payload) == 0 {
			return nil, fmt.Errorf("Assessment returned an invalid source event")
		}
		var payload struct {
			EventID uuid.UUID `json:"event_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil ||
			payload.EventID != event.EventID {
			return nil, errors.New("Assessment event identity does not match its payload")
		}
	}
	return &page, nil
}

type AssessmentEvent struct {
	EventID    uuid.UUID       `json:"event_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

type AssessmentEventPage struct {
	Events         []AssessmentEvent `json:"events"`
	HasMore        bool              `json:"has_more"`
	NextOccurredAt *time.Time        `json:"next_occurred_at,omitempty"`
	NextEventID    *uuid.UUID        `json:"next_event_id,omitempty"`
}
