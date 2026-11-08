package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type OutboxStore interface {
	ClaimOutboxBatch(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkOutboxDelivered(ctx context.Context, eventID uuid.UUID) error
	MarkOutboxFailed(ctx context.Context, eventID uuid.UUID, dead bool, lastError string, retryAt time.Time) error
	PurgeDeliveredOutbox(ctx context.Context, olderThan time.Time) error
}

type UserEventSender interface {
	SendUserEvent(ctx context.Context, event OutboxEvent) error
}

type OutboxDispatcher struct {
	store          OutboxStore
	sender         UserEventSender
	logger         *slog.Logger
	pollInterval   time.Duration
	sendTimeout    time.Duration
	batchSize      int
	maxAttempts    int
	purgeRetention time.Duration
}

func NewOutboxDispatcher(
	store OutboxStore,
	sender UserEventSender,
	logger *slog.Logger,
	pollInterval time.Duration,
	sendTimeout time.Duration,
	batchSize int,
	maxAttempts int,
) (*OutboxDispatcher, error) {
	if store == nil {
		return nil, fmt.Errorf("outbox store is required")
	}
	if sender == nil {
		return nil, fmt.Errorf("event sender is required")
	}
	if pollInterval <= 0 {
		return nil, fmt.Errorf("outbox poll interval must be positive")
	}
	if sendTimeout <= 0 {
		return nil, fmt.Errorf("outbox send timeout must be positive")
	}
	if batchSize <= 0 || batchSize > 100 {
		return nil, fmt.Errorf("outbox batch size must be between 1 and 100")
	}
	if maxAttempts <= 0 {
		return nil, fmt.Errorf("outbox max attempts must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &OutboxDispatcher{
		store:          store,
		sender:         sender,
		logger:         logger,
		pollInterval:   pollInterval,
		sendTimeout:    sendTimeout,
		batchSize:      batchSize,
		maxAttempts:    maxAttempts,
		purgeRetention: 7 * 24 * time.Hour,
	}, nil
}

func (d *OutboxDispatcher) Run(ctx context.Context) {
	d.dispatchOnce(ctx)

	pollTicker := time.NewTicker(d.pollInterval)
	defer pollTicker.Stop()

	purgeTicker := time.NewTicker(time.Hour)
	defer purgeTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			d.dispatchOnce(ctx)
		case <-purgeTicker.C:
			d.purge(ctx)
		}
	}
}

func (d *OutboxDispatcher) dispatchOnce(ctx context.Context) {
	events, err := d.store.ClaimOutboxBatch(ctx, d.batchSize)
	if err != nil {
		d.logger.Error("outbox_claim_failed", "error", err)
		return
	}

	for _, event := range events {
		if ctx.Err() != nil {
			return
		}

		sendCtx, cancel := context.WithTimeout(ctx, d.sendTimeout)
		sendErr := d.sender.SendUserEvent(sendCtx, event)
		cancel()

		if sendErr == nil {
			if err := d.store.MarkOutboxDelivered(ctx, event.ID); err != nil {
				d.logger.Error("outbox_mark_delivered_failed",
					"event_id", event.ID,
					"error", err,
				)
			}
			continue
		}

		attempts := event.Attempts + 1
		dead := attempts >= d.maxAttempts
		retryAt := time.Now().UTC().Add(outboxBackoff(attempts))

		if err := d.store.MarkOutboxFailed(
			ctx, event.ID, dead, sendErr.Error(), retryAt,
		); err != nil {
			d.logger.Error("outbox_mark_failed_failed",
				"event_id", event.ID,
				"error", err,
			)
		}

		level := slog.LevelWarn
		if dead {
			level = slog.LevelError
		}
		d.logger.Log(ctx, level, "outbox_delivery_failed",
			"event_id", event.ID,
			"event_type", event.EventType,
			"attempts", attempts,
			"dead", dead,
			"error", sendErr,
		)
	}
}

func (d *OutboxDispatcher) purge(ctx context.Context) {
	if err := d.store.PurgeDeliveredOutbox(
		ctx, time.Now().UTC().Add(-d.purgeRetention),
	); err != nil {
		d.logger.Error("outbox_purge_failed", "error", err)
	}
}

func outboxBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 8 {
		attempts = 8
	}

	backoff := 5 * time.Second * time.Duration(1<<uint(attempts-1))
	if backoff > 10*time.Minute {
		return 10 * time.Minute
	}
	return backoff
}
