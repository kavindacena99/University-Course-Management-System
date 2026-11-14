# Service Integration

The User Service and Authentication Service communicate over signed internal
HTTP calls. These endpoints are not routed by the gateway.

## Authentication of internal calls

Every internal request carries:

| Header | Meaning |
| --- | --- |
| `X-Service-Id` | Identity of the calling service |
| `X-Service-Timestamp` | UTC timestamp in RFC3339 format |
| `X-Service-Nonce` | Unique value per request |
| `X-Service-Signature` | Hex-encoded HMAC-SHA256 signature |

The canonical string is:

```text
UPPERCASE_METHOD
/path?query
service_id
timestamp
nonce
hex(sha256(body))
```

Each line is separated by `\n`. The signature is
`hex(HMAC-SHA256(secret, canonical))`.

Verification requirements:

- The service ID must be in `INTERNAL_ALLOWED_CALLERS`.
- The timestamp must be within `INTERNAL_AUTH_MAX_SKEW`.
- The nonce must be unique and is remembered until the timestamp plus twice
  the allowed skew has passed.
- The signature must match the current secret. During secret rotation the
  previous secret is accepted when `INTERNAL_SERVICE_SECRET_PREVIOUS` is set.

Both services must use the same `INTERNAL_SERVICE_SECRET`. Use at least 32
characters and rotate by configuring the new secret on the receiver as
`INTERNAL_SERVICE_SECRET_PREVIOUS`, then switching both sides to the new
secret.

## User Service to Authentication Service

### Provision

`POST /internal/users`

Used by bootstrap and tests to create credential records directly.

```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "first_name": "Ada",
  "last_name": "Lovelace",
  "registration_status": "approved",
  "version": 1,
  "password": "initial-password",
  "roles": ["STUDENT"]
}
```

### User event

`POST /internal/events/users`

Sent by the outbox worker. Body:

```json
{
  "event_id": "uuid",
  "event_type": "user.approved",
  "occurred_at": "2026-11-14T10:00:00Z",
  "user": {
    "user_id": "uuid",
    "email": "user@example.com",
    "first_name": "Ada",
    "last_name": "Lovelace",
    "registration_status": "approved",
    "version": 2
  }
}
```

Responses:

- `200` with `{"result":"processed"}` when applied.
- `200` with `{"result":"duplicate"}` or `{"result":"stale"}` for inbox
  deduplication.
- `4xx` for malformed or unauthorized requests.
- `5xx` when the event should be retried.

## Authentication Service to User Service

### Eligibility

`GET /internal/users/{id}/eligibility`

The Authentication Service calls this during login and refresh. It maps:

- `404` or `eligible=false` to "user ineligible".
- Timeouts, connection errors, malformed responses, and other non-2xx
  statuses to "eligibility unavailable", which fails closed.

The call timeout is `USER_ELIGIBILITY_TIMEOUT` (default `2s`).

## Failure behavior

- Login and refresh require a successful eligibility check.
- If the User Service is unavailable, authentication returns
  `503 eligibility_unavailable`; it does not grant access.
- Outbox delivery retries until success or until `OUTBOX_MAX_ATTEMPTS`, then
  marks the event `dead` for operator inspection.
- User events are idempotent by `event_id` and ignore stale versions.

## Course Service lecturer eligibility

Course Service signs `GET /internal/users/{id}` to User Service to check live
registration approval. It then signs
`GET /internal/users/{id}/lecturer-eligibility` to Authentication Service to
check active credentials and the `LECTURER` role. The latter returns
`{"user_id":"uuid","eligible":true|false}`. Either dependency failure
returns `503` from Course Service and does not assign or publish an unverified
lecturer. Configure `course-service` in both receiver allowlists. Auth Service
further restricts provisioning and user-event endpoints to `user-service`, and
lecturer eligibility to `course-service`. Course Service does not access either
service's database.
