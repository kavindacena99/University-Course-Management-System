package assessment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	ID            uuid.UUID
	Payload       []byte
	CorrelationID string
	Attempts      int
}

type OutboxStore interface {
	ListPendingOutboxEvents(context.Context, int) ([]OutboxEvent, error)
	MarkOutboxDelivered(context.Context, uuid.UUID) error
	RecordOutboxFailure(context.Context, uuid.UUID, int, string) error
}

type AnalyticsClient struct {
	endpoint string
	signer   *ServiceSigner
	client   *http.Client
}

func NewAnalyticsClient(endpoint string, signer *ServiceSigner, timeout time.Duration) (*AnalyticsClient, error) {
	if strings.TrimSpace(endpoint) == "" || signer == nil || timeout <= 0 {
		return nil, fmt.Errorf("Analytics URL, signer, and positive timeout are required")
	}
	return &AnalyticsClient{
		endpoint: strings.TrimRight(endpoint, "/") + "/internal/events",
		signer:   signer,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *AnalyticsClient) Publish(ctx context.Context, event OutboxEvent) error {
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.endpoint, bytes.NewReader(event.Payload),
	)
	if err != nil {
		return fmt.Errorf("create Analytics event request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if event.CorrelationID != "" {
		request.Header.Set("X-Request-ID", event.CorrelationID)
	}
	c.signer.Sign(request, event.Payload)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("send Analytics event: %w", err)
	}
	defer response.Body.Close()
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("read Analytics response: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Analytics returned HTTP %d", response.StatusCode)
	}
	return nil
}

type OutboxWorker struct {
	store   OutboxStore
	client  *AnalyticsClient
	logger  *slog.Logger
}

func NewOutboxWorker(store OutboxStore, client *AnalyticsClient, logger *slog.Logger) *OutboxWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboxWorker{store: store, client: client, logger: logger}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.DispatchOnce(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("assessment_outbox_dispatch_failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *OutboxWorker) DispatchOnce(ctx context.Context) error {
	events, err := w.store.ListPendingOutboxEvents(ctx, 50)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := w.client.Publish(ctx, event); err != nil {
			if saveErr := w.store.RecordOutboxFailure(
				ctx, event.ID, event.Attempts, err.Error(),
			); saveErr != nil {
				return fmt.Errorf("record failed event delivery: %w", saveErr)
			}
			w.logger.Warn("assessment_outbox_event_delivery_failed",
				"event_id", event.ID, "attempt", event.Attempts+1, "error", err)
			continue
		}
		if err := w.store.MarkOutboxDelivered(ctx, event.ID); err != nil {
			return fmt.Errorf("mark event %s delivered: %w", event.ID, err)
		}
	}
	return nil
}
