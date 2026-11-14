# System Architecture

## Overview

The backend is split into two independently deployable Go services behind an
NGINX gateway:

```text
Client
  |
  v
NGINX :8080
  |-- /api/v1/auth/*, /api/v1/admin/auth/* ------> Authentication Service :8081
  |-- /api/v1/users/*, /api/v1/admin/users/* -----> User Service :8082
                                                        |
                                                        | signed internal HTTP
                                                        v
                                                  Authentication Service
```

Each service owns its PostgreSQL database. There are no shared tables.

- User Service database: `users` (`user_app`)
- Authentication Service database: `authentication_and_security` (`auth_app`)

## Components

### NGINX gateway

- Terminates public HTTP traffic on port `8080`.
- Routes requests by path without rewriting the upstream path.
- Applies request-size limits, timeouts, rate limits, and security headers.
- Does not expose `/internal/*` endpoints; only the service paths above are
  proxied, and all other paths return `404`.
- Exposes `GET /health` for gateway liveness.

### User Service

- Owns registration, profile data, lifecycle status, and status history.
- Owns the transactional outbox used to publish user events.
- Exposes eligibility information to the Authentication Service.
- Verifies access tokens issued by the Authentication Service.

### Authentication Service

- Owns credentials, sessions, refresh tokens, roles, permissions, audit data,
  activation tokens, and password-reset tokens.
- Issues Ed25519-signed JWT access tokens.
- Verifies user eligibility through the User Service on login and refresh.
- Consumes user events from the User Service's outbox.

## Primary flows

### Registration and activation

1. A user calls `POST /api/v1/users`.
2. The User Service stores the user as `pending` and records status history.
3. An administrator approves the user through the admin API.
4. The User Service writes a `user.approved` outbox event in the same
   transaction.
5. The outbox worker signs and delivers the event to
   `POST /internal/events/users` on the Authentication Service.
6. The Authentication Service creates the credential record, assigns the
   `STUDENT` role by default, and sends an activation token.
7. The user activates the account and sets a password with
   `POST /api/v1/auth/activate`.

### Login

1. The client calls `POST /api/v1/auth/login`.
2. The Authentication Service verifies the password first, then checks lockout
   and account availability.
3. It calls `GET /internal/users/{id}/eligibility` on the User Service using a
   signed internal request.
4. If the user is not eligible, login is rejected. If eligibility cannot be
   checked, login fails closed with `503 eligibility_unavailable`.
5. On success the service creates a session, stores a refresh-token hash,
   records audit data, and returns access and refresh tokens.

### Refresh

1. The client calls `POST /api/v1/auth/refresh`.
2. The Authentication Service inspects the refresh token, rejects reuse, and
   revokes the whole session if reuse is detected.
3. It checks eligibility with the User Service. An ineligible user has the
   session revoked.
4. The old refresh token is consumed and replaced by a new token in the same
   session.

### Profile update

The User Service updates the profile and emits `user.profile_updated` through
the outbox so the Authentication Service keeps its user projection in sync.

## Reliability and consistency

- Outbox writes and domain writes are in one transaction. Delivery is
  at-least-once with retries and a dead-letter status.
- The Authentication Service inbox deduplicates events by `event_id` and
  ignores stale events using the user version.
- Sessions have an absolute lifetime; refresh tokens rotate on every use.
- Role permission changes revoke affected users' sessions so they cannot
  refresh into stale claims; existing access tokens expire within the short
  access-token TTL.
- Eligibility checks are fail-closed: a dependency failure never grants access.

## Trust boundaries

- Public clients authenticate with JWT access tokens over the gateway.
- Internal service calls require HMAC-SHA256 signatures, a service identity,
  a timestamp, and a single-use nonce.
- Each service accepts internal calls only from explicitly allowed service
  identities configured with `INTERNAL_ALLOWED_CALLERS`.
- Databases are not exposed through the gateway.
