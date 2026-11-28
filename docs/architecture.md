# System Architecture

## Overview

The backend is split into six independently deployable Go services behind an
NGINX gateway:

```text
Browser
  |
  |-- :3000 -> frontend Nginx (static React SPA)
  |
  v
API gateway NGINX :8080
  |-- /api/v1/auth/*, /api/v1/admin/auth/* ------> Authentication Service :8081
  |-- /api/v1/users/*, /api/v1/admin/users/* -----> User Service :8082
  |-- /api/v1/courses/*, /api/v1/offerings/* ------> Course Service :8083
  |-- /api/v1/enrollments/*, student enrollment ---> Enrollment Service :8084
                                                        |
                                                        | signed internal HTTP
                                                        v
                                                  Authentication Service
  |-- /api/v1/offerings/*/assessments, /api/v1/assessments/* -> Assessment :8085
  |-- /api/v1/analytics/* -------------------------------> Analytics :8086

Assessment -- transactional outbox / signed HTTP --> Analytics
Analytics -- signed Course offering facts -----------> Course
```

The browser calls the gateway directly. The frontend Nginx server does not
proxy `/api/`; this keeps the gateway as the public API edge and preserves the
client address seen by the gateway. The frontend currently has no API calls.

Each service owns its PostgreSQL database. There are no shared tables.

- User Service database: `users` (`user_app`)
- Authentication Service database: `authentication_and_security` (`auth_app`)
- Course Service database: `courses` (`course_app`)
- Enrollment Service database: `enrollments` (`enrollment_app`)
- Assessment Service database: `assessments` (`assessment_app`)
- Analytics Service database: `analytics` (`analytics_app`)

## Components

### Frontend

- React 19, Vite 8, and Tailwind 4 static application served by Nginx.
- The production image is built from `frontend/Dockerfile` and served on
  container port `80` (host port `3000` in Compose).
- The browser calls the API gateway directly at `VITE_API_BASE_URL`; the
  frontend Nginx server does not proxy `/api/`.
- Exposes `GET /health`, applies a CSP and security headers, caches hashed
  assets, returns `404` for missing assets, and falls back to `index.html` for
  SPA routes.
- Currently has no API integration. The API base URL build argument and CSP
  `connect-src` entry are pre-wired for the gateway at
  `http://localhost:8080`.

### NGINX gateway

- Terminates public HTTP traffic on port `8080`.
- Routes requests by path without rewriting the upstream path.
- Applies request-size limits, timeouts, rate limits, and security headers.
- Does not expose `/internal/*` endpoints; only the service paths above are
  proxied, and all other paths return `404`.
- Exposes `GET /health` for gateway liveness.
- Host development uses loopback upstreams. The Docker Compose gateway uses
  service-name upstreams with dynamic Docker DNS re-resolution. Each service
  has its own `least_conn` upstream group, so replicas are load balanced and
  failing peers are marked down by passive health checks.
- In Compose, only frontend port `3000` and gateway port `8080` are published;
  service, database, and object-store ports stay on private Compose networks.

### Local container topology

The Compose stack in `infrastructure/docker/compose.yaml` uses three networks:

- `edge`: `frontend` and `gateway`.
- `backend`: `gateway` and the six services.
- `data`: the six services, six PostgreSQL databases, and `minio`.

Each service has its own PostgreSQL container and named volume. The
`assessment-minio-init` one-shot job creates the private submission bucket,
policy, and scoped user before Assessment starts. The stack uses the
maintained `pgsty/silo` fork because MinIO community-edition images are no
longer published.

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

### Course Service

- Owns Course definitions, CourseOfferings, publication, enrollment
  configuration, and the one assigned lecturer on each Offering.
- Exposes signed minimal Offering facts to Enrollment Service.

### Enrollment Service

- Owns student enrollment records and transactionally enforces configured
  Offering capacity with a per-Offering guard row.
- Checks live User approval, Auth role/activation, and CourseOffering facts.
- Exposes HMAC-protected active student and lecturer checks for Assessment.
  Lecturer assignment remains owned by Course; Enrollment does not duplicate
  its assignment records.

### Assessment Service

- Owns component assessments, submissions, marks, and imported final results.
- Stores submission files in the private MinIO-compatible object store; the
  local Compose stack uses `pgsty/silo`.
- Writes grade/result events to a transactional outbox and exposes the retained
  event history to Analytics through signed internal HTTP.

### Analytics Service

- Owns rebuildable read projections, configurable grade bands/programme
  policies, and report APIs.
- Reads Assessment events from its outbox and current Course offering facts
  through signed internal HTTP; it does not read other service databases.
- Enforces self-only student reports, assigned-offering lecturer reports,
  aggregate admin reports, and separate permission for admin student details.

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
- Analytics deduplicates Assessment events by `event_id` and can rebuild its
  projection from Assessment's retained outbox history.
- The Authentication Service inbox deduplicates events by `event_id` and
  ignores stale events using the user version.
- Sessions have an absolute lifetime; refresh tokens rotate on every use.
- Role permission changes revoke affected users' sessions so they cannot
  refresh into stale claims; existing access tokens expire within the short
  access-token TTL.
- Eligibility checks are fail-closed: a dependency failure never grants access.

## Local deployment

`infrastructure/docker/compose.yaml` builds and runs the full local stack,
including the frontend, gateway, six services, six PostgreSQL databases, and
the object store. `infrastructure/docker/compose.dev.yaml` adds development
logging. Only frontend port `3000` and gateway port `8080` are published. This
is a development integration stack, not a production TLS deployment. See
[Infrastructure Architecture](infrastructure/architecture.md) for topology,
startup ordering, and validation evidence.

## Trust boundaries

- Public clients authenticate with JWT access tokens over the gateway.
- Internal service calls require HMAC-SHA256 signatures, a service identity,
  a timestamp, and a single-use nonce.
- Each service accepts internal calls only from explicitly allowed service
  identities configured with `INTERNAL_ALLOWED_CALLERS`.
- Databases are not exposed through the gateway.
