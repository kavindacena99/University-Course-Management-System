# University Course Management System

Group 5 · COSC 52083

## Overview

This repository contains the current Go backend for identity, users, and academic courses.

The codebase currently includes:

- `services/security-and-authentication` for login, session management, role handling, activation, and password flows
- `services/user` for registration, profile management, and admin approval workflows
- `services/course` for course definitions, offerings, lecturer assignment, and catalogue APIs
- `infrastructure/nginx` as a basic reverse proxy in front of the services

The repository does not yet include a frontend UI or enrollment and assessment workflows.

## Documentation

Detailed project documentation is available in the `docs/` directory:

- [System Architecture](docs/architecture.md)
- [User Service](docs/user-service.md)
- [Course Service](docs/course-service.md)
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

Original design documents:

- [System Architecture (v1.1.0)](docs/System_Architecture_v1.1.0.pdf)
- [Solution Security (v1.0.0)](docs/Solution_Security_v1.0.0.pdf)

## Repository structure

```text
.
├── LICENSE
├── README.md
├── consider.txt
├── docs/
│   ├── openapi/
│   ├── *.md
│   ├── Solution_Security_v1.0.0.pdf
│   └── System_Architecture_v1.1.0.pdf
├── infrastructure/
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
│   └── course/
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
- PostgreSQL with `pgx/v5`
- Ed25519 JWT authentication
- Argon2 password hashing
- `golang-migrate` for schema migration
- NGINX reverse proxy

## Architecture

```text
NGINX (port 8080)
  ├── /api/v1/auth/         -> auth service (127.0.0.1:8081)
  ├── /api/v1/admin/auth/   -> auth service (127.0.0.1:8081)
  ├── /api/v1/users/        -> user service (127.0.0.1:8082)
  ├── /api/v1/admin/users/  -> user service (127.0.0.1:8082)
  ├── /api/v1/courses/, /api/v1/admin/courses/ -> course service (127.0.0.1:8083)
  └── /api/v1/offerings/, /api/v1/admin/offerings/ -> course service (127.0.0.1:8083)
```

The three services are separate Go applications and use signed internal HTTP requests.

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

Enrollment and assessment records are not stored in the Course Service.

## Security model

The current implementation includes:
- JWT access tokens signed with Ed25519
- roles `ADMIN`, `STAFF`, `LECTURER`, and `STUDENT`
- permission claims (`users:read`, `users:manage`, `roles:manage`,
  `sessions:read`, `sessions:revoke`, `account:unlock`, `audit:read`,
  `courses:manage`, `offerings:manage`)
- CSRF validation support for cookie-based tokens
- HMAC-signed internal service requests with timestamp and nonce protection
- fail-closed login and refresh eligibility verification via the user service
- refresh-token rotation with reuse detection and session revocation

## Local setup

### Prerequisites

- Go toolchain
- PostgreSQL instance
- NGINX
- JWT key material for the auth service
- environment variables configured per service

### 1. Configure environment variables

```bash
cp services/security-and-authentication/.env.example services/security-and-authentication/.env
cp services/user/.env.example services/user/.env
cp services/course/.env.example services/course/.env
```

Set the database credentials, a shared `INTERNAL_SERVICE_SECRET` (32+
characters), and the user-service `AUTH_JWT_PUBLIC_KEYS_JSON` value generated
from the auth service public key. Never commit `.env` files.

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

### 5. Optional admin bootstrap

```bash
cd services/user
go run ./cmd/bootstrap-admin --email admin@example.com --first-name Admin --last-name User --password "StrongPassword123!"
```

This requires `ADMIN_BOOTSTRAP_TOKEN` to be configured.

## Testing

Unit tests run without a database. Integration tests run when
`TEST_DATABASE_URL` points at the dedicated `auth_test` or `users_test`
database; tests refuse to run against any other database.

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

## Known gaps

The repository does not yet include:
- a frontend or browser application
- enrollment, timetable, result, and assessment workflows
- Docker/Compose deployment files
- CI/CD workflow configuration
- production deployment manifests
- a complete end-to-end university workflow beyond identity and authorization

## License

This project is licensed under the MIT License. See `LICENSE` for details.
