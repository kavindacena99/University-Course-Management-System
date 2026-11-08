package user

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeOutboxStore struct {
	events     []OutboxEvent
	delivered  []uuid.UUID
	failed     []failedOutboxEvent
	claimed    bool
	claimError error
}

type failedOutboxEvent struct {
	id      uuid.UUID
	dead    bool
	message string
	retryAt time.Time
}

func (f *fakeOutboxStore) ClaimOutboxBatch(_ context.Context, _ int) ([]OutboxEvent, error) {
	if f.claimError != nil {
		return nil, f.claimError
	}
	if f.claimed {
		return nil, nil
	}
	f.claimed = true
	return f.events, nil
}

func (f *fakeOutboxStore) MarkOutboxDelivered(_ context.Context, eventID uuid.UUID) error {
	f.delivered = append(f.delivered, eventID)
	return nil
}

func (f *fakeOutboxStore) MarkOutboxFailed(
	_ context.Context,
	eventID uuid.UUID,
	dead bool,
	lastError string,
	retryAt time.Time,
) error {
	f.failed = append(f.failed, failedOutboxEvent{
		id:      eventID,
		dead:    dead,
		message: lastError,
		retryAt: retryAt,
	})
	return nil
}

func (f *fakeOutboxStore) PurgeDeliveredOutbox(context.Context, time.Time) error {
	return nil
}

type fakeEventSender struct {
	err   error
	calls int
}

func (f *fakeEventSender) SendUserEvent(context.Context, OutboxEvent) error {
	f.calls++
	return f.err
}

func testDispatcher(
	t *testing.T,
	store OutboxStore,
	sender UserEventSender,
	maxAttempts int,
) *OutboxDispatcher {
	t.Helper()

	dispatcher, err := NewOutboxDispatcher(
		store,
		sender,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		time.Second,
		time.Second,
		10,
		maxAttempts,
	)
	if err != nil {
		t.Fatalf("create dispatcher: %v", err)
	}
	return dispatcher
}

func TestOutboxDispatcherMarksDelivered(t *testing.T) {
	eventID := uuid.New()
	store := &fakeOutboxStore{
		events: []OutboxEvent{{ID: eventID, EventType: EventUserApproved}},
	}
	sender := &fakeEventSender{}
	dispatcher := testDispatcher(t, store, sender, 3)

	dispatcher.dispatchOnce(context.Background())

	if sender.calls != 1 {
		t.Fatalf("sender calls = %d", sender.calls)
	}
	if len(store.delivered) != 1 || store.delivered[0] != eventID {
		t.Fatalf("unexpected delivered events: %+v", store.delivered)
	}
	if len(store.failed) != 0 {
		t.Fatalf("unexpected failed events: %+v", store.failed)
	}
}

func TestOutboxDispatcherRetriesOnFailure(t *testing.T) {
	eventID := uuid.New()
	store := &fakeOutboxStore{
		events: []OutboxEvent{{
			ID:        eventID,
			EventType: EventUserApproved,
			Attempts:  1,
		}},
	}
	sender := &fakeEventSender{err: errors.New("temporary failure")}
	dispatcher := testDispatcher(t, store, sender, 5)

	dispatcher.dispatchOnce(context.Background())

	if len(store.delivered) != 0 {
		t.Fatalf("unexpected delivered events: %+v", store.delivered)
	}
	if len(store.failed) != 1 {
		t.Fatalf("expected one failed event, got %+v", store.failed)
	}
	if store.failed[0].dead {
		t.Fatal("event was unexpectedly marked dead")
	}
	if store.failed[0].message != "temporary failure" {
		t.Fatalf("message = %q", store.failed[0].message)
	}
	if !store.failed[0].retryAt.After(time.Now()) {
		t.Fatal("retry time was not in the future")
	}
}

func TestOutboxDispatcherMarksDeadAfterMaxAttempts(t *testing.T) {
	eventID := uuid.New()
	store := &fakeOutboxStore{
		events: []OutboxEvent{{
			ID:        eventID,
			EventType: EventUserApproved,
			Attempts:  4,
		}},
	}
	sender := &fakeEventSender{err: errors.New("permanent failure")}
	dispatcher := testDispatcher(t, store, sender, 5)

	dispatcher.dispatchOnce(context.Background())

	if len(store.failed) != 1 {
		t.Fatalf("expected one failed event, got %+v", store.failed)
	}
	if !store.failed[0].dead {
		t.Fatal("expected event to be marked dead")
	}
}

func TestOutboxDispatcherHandlesClaimFailure(t *testing.T) {
	store := &fakeOutboxStore{claimError: errors.New("database unavailable")}
	sender := &fakeEventSender{}
	dispatcher := testDispatcher(t, store, sender, 3)

	dispatcher.dispatchOnce(context.Background())

	if sender.calls != 0 {
		t.Fatalf("sender calls = %d", sender.calls)
	}
}

func TestOutboxBackoffIsBounded(t *testing.T) {
	if got := outboxBackoff(1); got != 5*time.Second {
		t.Fatalf("backoff(1) = %v", got)
	}
	if got := outboxBackoff(100); got != 10*time.Minute {
		t.Fatalf("backoff(100) = %v", got)
	}
}
