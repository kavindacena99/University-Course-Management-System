CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ NULL,

    CONSTRAINT chk_outbox_events_status
        CHECK (status IN ('pending', 'delivered', 'dead')),

    CONSTRAINT chk_outbox_events_attempts
        CHECK (attempts >= 0)
);

CREATE INDEX idx_outbox_events_pending
    ON outbox_events (status, next_attempt_at)
    WHERE status = 'pending';

CREATE INDEX idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id);

CREATE INDEX idx_outbox_events_delivered
    ON outbox_events (delivered_at)
    WHERE status = 'delivered';
