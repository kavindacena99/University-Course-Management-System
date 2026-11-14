# User Service

Location: `services/user`
Default port: `8082`
Database: `users`

## Responsibilities

- Register users with status `pending`.
- Maintain profile fields and a monotonic version.
- Enforce lifecycle transitions.
- Record immutable status history.
- Deliver user events to the Authentication Service through a transactional
  outbox.
- Provide eligibility information for authentication decisions.
- Bootstrap the first administrator.

## Lifecycle

```text
pending --approve--> approved --suspend--> suspended
   |                     ^                    |
   |                     +-----reinstate------+
   +--reject--> rejected
```

Transitions are validated server-side. Admin mutations require the expected
version and return `409 version_conflict` when the row changed concurrently.
Each transition writes a `user_status_history` row and an outbox event in the
same transaction.

Event types:

- `user.approved`
- `user.rejected`
- `user.suspended`
- `user.reinstated`
- `user.profile_updated`

## Endpoints

Public:

- `POST /api/v1/users` - register (rate limited).
- `GET /api/v1/users/me` - profile of the authenticated user.
- `PATCH /api/v1/users/me` - update own profile; requires `version`.
- `GET /api/v1/users/{id}` - own profile or an admin read with `users:read`.

Admin (all require a JWT with the stated permission):

- `GET /api/v1/admin/users` - `users:read`; filters: `status`, `search`,
  `limit`, `offset`.
- `GET /api/v1/admin/users/{id}` - `users:read`.
- `GET /api/v1/admin/users/{id}/history` - `users:read`.
- `POST /api/v1/admin/users/{id}/approve` - `users:manage`.
- `POST /api/v1/admin/users/{id}/reject` - `users:manage`.
- `POST /api/v1/admin/users/{id}/suspend` - `users:manage`.
- `POST /api/v1/admin/users/{id}/reinstate` - `users:manage`.

Internal (HMAC-signed, never exposed by the gateway):

- `GET /internal/users/{id}` - user projection for the auth service.
- `GET /internal/users/{id}/eligibility` - eligibility decision.
- `POST /internal/bootstrap` - creates the initial admin; additionally requires
  `X-Bootstrap-Token` when `ADMIN_BOOTSTRAP_TOKEN` is configured.

## Eligibility

`GET /internal/users/{id}/eligibility` returns:

```json
{
  "user_id": "c6c7c4c0-8c86-4f0f-9f4b-d6f6f6e7a111",
  "eligible": true,
  "registration_status": "approved",
  "version": 3
}
```

A user is eligible only when the registration status is `approved`. The
Authentication Service treats any non-2xx response or malformed body as a
failure to verify and denies login/refresh.

## Outbox delivery

- New events are inserted with the domain change in one transaction.
- A background worker polls pending events, signs the HTTP request, and posts
  to `POST /internal/events/users`.
- Successful delivery marks the event `delivered`.
- Retryable failures reschedule delivery with backoff.
- Exhausted or permanent failures mark the event `dead`.

Configuration:

- `OUTBOX_POLL_INTERVAL`
- `OUTBOX_BATCH_SIZE`
- `OUTBOX_MAX_ATTEMPTS`
- `OUTBOX_HTTP_TIMEOUT`
- `AUTH_SERVICE_INTERNAL_URL`

## Bootstrap

The bootstrap command creates an approved administrator in the User Service and
provisions credentials through the Authentication Service internal API.

```bash
cd services/user
go run ./cmd/bootstrap-admin \
  --email admin@example.com \
  --first-name Admin \
  --last-name User \
  --password 'StrongPassword123!'
```

`ADMIN_BOOTSTRAP_TOKEN` must be configured and at least 32 characters long.

## Configuration

See `services/user/.env.example`. Required values include database settings,
`AUTH_JWT_PUBLIC_KEYS_JSON`, `INTERNAL_SERVICE_SECRET`, and
`AUTH_SERVICE_INTERNAL_URL`.
