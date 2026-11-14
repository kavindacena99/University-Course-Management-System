# University Course Management System

Group 5 · COSC 52083

## Overview

This repository contains the current backend implementation of the project as it exists today: a Go-based microservice system focused on identity, authentication, and user lifecycle management.

The codebase currently includes:

- `services/security-and-authentication` for login, session management, role handling, activation, and password flows
- `services/user` for registration, profile management, and admin approval workflows
- `infrastructure/nginx` as a basic reverse proxy in front of the services

This is not a complete university course-management application. The repository does not include a frontend UI, course-domain models, or a full academic workflow beyond identity and access management.

## Documentation

Detailed project documentation is available in the `docs/` directory:

- [System Architecture (v1.1.0)](docs/System_Architecture_v1.1.0.pdf)
- [Solution Security (v1.0.0)](docs/Solution_Security_v1.0.0.pdf)

## Repository structure

```text
.
├── LICENSE
├── README.md
├── consider.txt
├── docs/
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
│   └── user/
│       ├── cmd/
│       ├── database/
│       ├── internal/
│       ├── migrations/
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
  ├── /api/v1/auth/  -> auth service (127.0.0.1:8081)
  └── /api/v1/users/ -> user service (127.0.0.1:8082)
```

The authentication and user services are implemented as separate Go applications and communicate using signed internal HTTP requests.

## Service responsibilities

### Authentication service

Location: `services/security-and-authentication`

Implements:
- login, refresh, logout, and sessions
- role catalog and role assignment
- account activation and password reset
- failed-login handling and lockout
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

## Data model

The current implementation persists data in PostgreSQL.

### User service
- `users`
- `user_status_history`
- `outbox_events`

### Auth service
- `auth_users`
- `auth_roles`
- `auth_user_roles`
- `auth_sessions`
- `auth_refresh_tokens`
- `auth_action_tokens`
- `auth_inbox_events`

The domain model is centered on identity, account state, permissions, and approval workflows rather than academic course operations.

## Security model

The current implementation includes:
- JWT access tokens signed with Ed25519
- role-based access for `ADMIN`, `STAFF`, `LECTURER`, and `STUDENT`
- CSRF validation support for cookie-based tokens
- HMAC-signed internal service requests with timestamp and nonce protection

## Local setup

### Prerequisites

- Go toolchain
- PostgreSQL instance
- NGINX
- JWT key material for the auth service
- environment variables configured per service

### 1. Generate JWT keys

```bash
cd services/security-and-authentication
go run ./cmd/keygen --out-dir ./var/keys
```

### 2. Run migrations

```bash
cd services/security-and-authentication
go run ./cmd/migrate -direction up
```

```bash
cd services/user
go run ./cmd/migrate -direction up
```

### 3. Start the services

```bash
cd services/security-and-authentication
go run .
```

```bash
cd services/user
go run .
```

### 4. Optional admin bootstrap

```bash
cd services/user
go run ./cmd/bootstrap-admin --email admin@example.com --first-name Admin --last-name User --password "StrongPassword123!"
```

This requires `ADMIN_BOOTSTRAP_TOKEN` to be configured.

## Testing

Go tests are included for the implemented services.

```bash
cd services/security-and-authentication
go test ./...
```

```bash
cd services/user
go test ./...
```

## Known gaps

The repository does not yet include:
- a frontend or browser application
- course, enrollment, timetable, result, or academic-domain models
- Docker/Compose deployment files
- CI/CD workflow configuration
- production deployment manifests
- a complete end-to-end university workflow beyond identity and authorization

## License

This project is licensed under the MIT License. See `LICENSE` for details.
