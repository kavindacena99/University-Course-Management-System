# University Course Management System

Group 5 · COSC 52083

## Overview

This repository contains the current Go backend for identity, users, academic courses, student enrollment, assessment, and analytics, plus a React frontend and a local Docker Compose stack.

The codebase currently includes:

- `services/security-and-authentication` for login, session management, role handling, activation, and password flows
- `services/user` for registration, profile management, and admin approval workflows
- `services/course` for course definitions, offerings, lecturer assignment, and catalogue APIs
- `services/enrollment` for student enrollment, capacity enforcement, and Assessment-facing eligibility checks
- `services/assessment` for component assessments, private submissions, grading, and approved final-result imports
- `services/analytics` for rebuildable academic reporting projections and GPA analytics
- `frontend` for the React/Vite user interface served by Nginx
- `infrastructure/nginx` for the public API gateway
- `infrastructure/docker` for the local Compose stack, with its environment template in `infrastructure/environments`

The frontend is a static React application served by Nginx. It currently has no API integration; when wired, it calls the gateway directly at `VITE_API_BASE_URL` (default `http://localhost:8080`). Course remains the authority for lecturer assignment; Assessment remains the authority for grades and final results.

## Documentation

Detailed project documentation is available in the `docs/` directory:

- [System Architecture](docs/architecture.md)
- [Infrastructure Architecture](docs/infrastructure/architecture.md)
- [User Service](docs/user-service.md)
- [Course Service](docs/course-service.md)
- [Enrollment Service](services/enrollment/README.md)
- [Assessment Service](services/assessment/README.md)
- [Analytics Service](services/analytics/README.md)
- [Enrollment discovery](docs/enrollment-discovery.md)
- [Authentication Service](docs/authentication-service.md)
- [Service Integration](docs/service-integration.md)
- [Security](docs/security.md)
- [Database](docs/database.md)
- [API Documentation](docs/api-documentation.md)
- [Development Setup](docs/development-setup.md)
- [Implementation Report](docs/implementation-report.md)
- [OpenAPI: Authentication Service](docs/openapi/auth-service.yaml)
- [OpenAPI: User Service](docs/openapi/user-service.yaml)
- [OpenAPI: Course Service](docs/openapi/course-service.yaml)
- [OpenAPI: Enrollment Service](docs/openapi/enrollment-service.yaml)
- [OpenAPI: Assessment Service](docs/openapi/assessment-service.yaml)
- [OpenAPI: Analytics Service](docs/openapi/analytics-service.yaml)

Original design documents:

- [System Architecture (v1.1.0)](docs/System_Architecture_v1.1.0.pdf)
- [Solution Security (v1.0.0)](docs/Solution_Security_v1.0.0.pdf)

## Repository structure

```text
.
├── .gitattributes
├── LICENSE
├── README.md
├── consider.txt
├── docs/
│   ├── infrastructure/
│   ├── openapi/
│   ├── *.md
│   ├── Solution_Security_v1.0.0.pdf
│   └── System_Architecture_v1.1.0.pdf
├── frontend/
│   ├── Dockerfile
│   ├── nginx.conf
│   ├── src/
│   └── ...
├── infrastructure/
│   ├── docker/
│   │   ├── compose.yaml
│   │   └── compose.dev.yaml
│   ├── environments/
│   │   └── dev/.env.example
│   └── nginx/
├── services/
│   ├── security-and-authentication/
│   │   ├── cmd/
│   │   ├── database/
│   │   ├── internal/
│   │   ├── migrations/
│   │   ├── go.mod
│   │   └── main.go
│   ├── user/
│   │   ├── cmd/
│   │   ├── database/
│   │   ├── internal/
│   │   ├── migrations/
│   │   ├── go.mod
│   │   └── main.go
│   ├── course/
│   │   ├── cmd/
│   │   ├── database/
│   │   ├── internal/
│   │   ├── migrations/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── main.go
│   ├── enrollment/
│   │   ├── cmd/
│   │   ├── database/
│   │   ├── internal/
│   │   ├── migrations/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── main.go
│   ├── assessment/
│   │   ├── cmd/
│   │   ├── database/
│   │   ├── internal/
│   │   ├── migrations/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── main.go
│   └── analytics/
│       ├── cmd/
│       ├── database/
│       ├── internal/
│       ├── migrations/
│       ├── Dockerfile
│       ├── go.mod
│       └── main.go
```

## Stack

- Go 1.27.2
- Gin HTTP framework
- PostgreSQL 18 with `pgx/v5`
- Ed25519 JWT authentication
- Argon2 password hashing
- `golang-migrate` for schema migration
- NGINX reverse proxy
- Node 22, pnpm 10.34.3, React 19, Vite 8, and Tailwind 4
- Docker Compose for the local integration stack

## Architecture

```text
Browser
  ├── :3000 -> frontend Nginx (static React SPA)
  └── :8080 -> API gateway NGINX
        ├── /api/v1/auth/         -> auth service (auth:8081)
        ├── /api/v1/admin/auth/   -> auth service (auth:8081)
        ├── /api/v1/users/        -> user service (user:8082)
        ├── /api/v1/admin/users/  -> user service (user:8082)
        ├── /api/v1/courses/, /api/v1/admin/courses/ -> course service (course:8083)
        ├── /api/v1/offerings/, /api/v1/admin/offerings/ -> course service (course:8083)
        ├── /api/v1/enrollments/, student/Offering enrollment routes -> enrollment service (enrollment:8084)
        ├── assessment, submission, and result routes -> assessment service (assessment:8085)
        └── /api/v1/analytics/ -> analytics service (analytics:8086)
```

Docker Compose uses service-name upstreams and publishes only frontend port
`3000` and gateway port `8080`. Host-run development uses loopback upstreams
on ports `8081` through `8086`. The six domain services are separate Go
applications. Internal integrations use signed HTTP; released grades and
imported final results are transactionally written to Assessment's outbox and
projected by Analytics.

## Service responsibilities

### Authentication service

Location: `services/security-and-authentication`

Implements:
- login, refresh, logout, and sessions
- role catalog, permission catalog, and role/permission assignment
- permission claims in JWT access tokens
- security audit events
- account activation and password reset
- failed-login handling and lockout
- fail-closed eligibility checks against the user service
- internal user provisioning and event processing

### User service

Location: `services/user`

Implements:
- user registration as pending
- profile retrieval and update
- admin approval, rejection, suspension, and reinstatement workflows
- status-history tracking
- outbox-based user-event delivery to the auth service
- bootstrap of an initial admin user

### Course service

- course and offering lifecycle, catalogue, lecturer assignment, and local audit records
- JWT authorization with resource-level checks for assigned lecturers
- signed eligibility checks against User and Authentication services

### Enrollment service

- student enrollment in CourseOfferings with historical cancellation records
- transactional capacity enforcement and active-enrollment uniqueness
- signed User, Auth, and Course eligibility checks
- HMAC-protected student and lecturer verification for Assessment Service
- no lecturer assignment records; Course owns those assignments

### Assessment service

- offering assessments, per-student deadline extensions, and attempt-limited submissions in private MinIO storage
- component grades with versioned changes, release, and revision history
- permission-gated imports of approved raw final marks, with batch idempotency and audit history
- transactional outbox events for released component grades and imported final results
- no local copies of Enrollment membership or Course lecturer assignments

### Analytics service

- rebuildable projections of Assessment's released component grades and imported final results
- credit-weighted term and cumulative GPA using configurable grade bands and programme policies
- self-only student reports, currently-assigned lecturer reports, and aggregate admin reporting
- no ownership of grades, Course facts, enrollment, or student identities

## Data model

The current implementation persists data in PostgreSQL.

### User service
- `users`
- `user_status_history`
- `outbox_events`

### Auth service
- `auth_users`
- `auth_roles`
- `auth_permissions`
- `auth_user_roles`
- `auth_role_permissions`
- `auth_sessions`
- `auth_refresh_tokens`
- `auth_action_tokens`
- `auth_inbox_events`
- `auth_audit_events`

### Course service
- `courses`
- `course_offerings`
- `course_audit_events`

### Enrollment service
- `enrollments`
- `enrollment_offering_guards`

### Assessment service
- `assessments`
- `submissions`
- `grades`
- `grade_revisions`
- `final_course_results`
- `final_result_import_batches`
- `final_result_revisions`
- `assessment_outbox_events`

Enrollment records remain only in Enrollment Service. Assessment persists
eligibility-independent facts locally and verifies current membership or lecturer
assignment through Enrollment's signed internal endpoints.

## Security model

The current implementation includes:
- JWT access tokens signed with Ed25519
- roles `ADMIN`, `STAFF`, `LECTURER`, and `STUDENT`
- permission claims (`users:read`, `users:manage`, `roles:manage`,
  `sessions:read`, `sessions:revoke`, `account:unlock`, `audit:read`,
  `courses:manage`, `offerings:manage`, `enrollments:read`,
  `enrollments:manage`)
- CSRF validation support for cookie-based tokens
- HMAC-signed internal service requests with timestamp and nonce protection
- fail-closed login and refresh eligibility verification via the user service
- refresh-token rotation with reuse detection and session revocation

## Local setup

### Prerequisites

- Go toolchain
- PostgreSQL instance
- NGINX
- Docker Desktop (for the Compose stack)
- Node 22 and pnpm 10.34.3 (for the frontend)
- JWT key material for the auth service
- environment variables configured per service

### 1. Configure environment variables

```bash
cp services/security-and-authentication/.env.example services/security-and-authentication/.env
cp services/user/.env.example services/user/.env
cp services/course/.env.example services/course/.env
cp services/enrollment/.env.example services/enrollment/.env
cp services/assessment/.env.example services/assessment/.env
cp services/analytics/.env.example services/analytics/.env
```

Set the database credentials, a shared `INTERNAL_SERVICE_SECRET` (32+
characters), and matching `AUTH_JWT_PUBLIC_KEYS_JSON` values in User, Course,
Enrollment, Assessment, and Analytics generated from Auth's public key. Never
commit `.env` files.

### 2. Generate JWT keys

```bash
cd services/security-and-authentication
go run ./cmd/keygen --out-dir ./var/keys
```

### 3. Run migrations

```bash
cd services/security-and-authentication
go run ./cmd/migrate -direction up
```

```bash
cd services/user
go run ./cmd/migrate -direction up
```

```bash
cd services/course
go run ./cmd/migrate -direction up
```

```bash
cd services/enrollment
go run ./cmd/migrate -direction up
```

```bash
cd services/assessment
go run ./cmd/migrate -direction up
```

```bash
cd services/analytics
go run ./cmd/migrate -direction up
```

### 4. Start the services

```bash
cd services/security-and-authentication
go run .
```

```bash
cd services/user
go run .
```

```bash
cd services/course
go run .
```

```bash
cd services/enrollment
go run .
```

```bash
cd services/assessment
go run .
```

```bash
cd services/analytics
go run .
```

### 5. Optional admin bootstrap

```bash
cd services/user
go run ./cmd/bootstrap-admin --email admin@example.com --first-name Admin --last-name User --password "StrongPassword123!"
```

This requires `ADMIN_BOOTSTRAP_TOKEN` to be configured.

### 6. Frontend

The frontend uses Node 22 and pnpm 10.34.3 (see `.mise.toml`):

```bash
cd frontend
corepack enable
pnpm install --frozen-lockfile
pnpm dev
```

Vite serves on port `8443` by default; set `PORT` to override it. The
application currently has no API integration. The production image is built
from `frontend/Dockerfile`.

### 7. Docker Compose

The local integration stack is defined in `infrastructure/docker/compose.yaml`
with a development override in `compose.dev.yaml`:

```bash
cp infrastructure/environments/dev/.env.example infrastructure/docker/.env
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev config --quiet
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev up -d --build
```

Set values in `infrastructure/docker/.env` and the six service `.env` files.
Auth's generated key directory is mounted read-only. The stack publishes only
frontend port `3000` and gateway port `8080`; open `http://localhost:3000` and
`http://localhost:8080/health`. The object store uses the maintained
`pgsty/silo` fork because MinIO community-edition images are no longer
published. Stop with `docker compose ... down`; add `-v` only when
intentionally deleting local data. Scale a stateless service with
`docker compose ... up -d --scale auth=2`; the gateway re-resolves Docker DNS
every 10 seconds. See [Infrastructure Architecture](docs/infrastructure/architecture.md)
and [Development Setup](docs/development-setup.md).

## Testing

Unit tests run without a database. Integration tests run when
`TEST_DATABASE_URL` points at the dedicated `auth_test`, `users_test`,
`courses_test`, or `enrollments_test` database; tests refuse to run against
any other database.

```bash
cd services/security-and-authentication
go test ./...
TEST_DATABASE_URL='postgres://auth_app:password@127.0.0.1:5432/auth_test?sslmode=disable' go test ./internal/...
```

```bash
cd services/user
go test ./...
TEST_DATABASE_URL='postgres://user_app:password@127.0.0.1:5432/users_test?sslmode=disable' go test ./internal/...
```

```bash
cd services/course
go test ./...
TEST_DATABASE_URL='postgres://course_app:password@127.0.0.1:5432/courses_test?sslmode=disable' go test ./internal/...
```

```bash
cd services/enrollment
go test ./...
TEST_DATABASE_URL='postgres://enrollment_app:password@127.0.0.1:5432/enrollments_test?sslmode=disable' go test ./internal/...
```

```bash
cd services/assessment
go test ./...
```

```bash
cd services/analytics
go test ./...
```

## Known gaps

The repository does not yet include:
- frontend API integration (the UI is currently static)
- a complete end-to-end university workflow beyond identity and authorization
- CI/CD workflow configuration
- production deployment manifests
- production TLS and secret-management configuration
- external activation and password-reset notification delivery

## License

This project is licensed under the MIT License. See `LICENSE` for details.
