CREATE TABLE assessment_internal_request_nonces (
    nonce VARCHAR(160) PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX assessment_internal_request_nonces_expiry_idx
    ON assessment_internal_request_nonces (expires_at);
