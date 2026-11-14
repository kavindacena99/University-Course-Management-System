# Database

Two PostgreSQL databases are used. Each service owns its schema and migrations.

## User Service database

Migrations: `services/user/migrations`

### `users`

- `id` UUID primary key
- `email` unique, case-insensitive index
- `first_name`, `last_name`
- `registration_status`: `pending`, `approved`, `rejected`, `suspended`
- `status_reason`
- `reviewed_by`, `reviewed_at`
- `version` optimistic-concurrency counter
- `created_at`, `updated_at`

### `user_status_history`

Immutable lifecycle history: previous and new status, reason, actor, version,
and timestamp.

### `outbox_events`

Transactional outbox:

- event ID and type
- JSON payload
- delivery attempts and status (`pending`, `delivered`, `dead`)
- next attempt time
- timestamps

## Authentication Service database

Migrations: `services/security-and-authentication/migrations`

### Credential and profile projection

- `auth_users` - credential record keyed by `user_id`, with status, version,
  password hash, activation time, disabled flag, failed attempts, and lockout.
- `auth_roles` - role catalog.
- `auth_permissions` - permission catalog.
- `auth_user_roles` - user-to-role mapping.
- `auth_role_permissions` - role-to-permission mapping.

### Sessions and tokens

- `auth_sessions` - session ID, user, timestamps, absolute expiry, revocation.
- `auth_refresh_tokens` - SHA-256 token hash, expiry, use/revocation times, and
  replacement chain.
- `auth_action_tokens` - activation and password-reset token hashes.

### Integration and audit

- `auth_inbox_events` - deduplication of user events by `event_id`.
- `auth_audit_events` - security audit trail with JSON details.

## Migrations

Run migrations with each service's migration command:

```bash
cd services/security-and-authentication
go run ./cmd/migrate -direction up

cd services/user
go run ./cmd/migrate -direction up
```

Check version:

```bash
go run ./cmd/migrate -direction version
```

Roll back one step:

```bash
go run ./cmd/migrate -direction down -steps 1
```

Never edit an applied migration. Add a new numbered migration instead.

## Test databases

Integration tests require:

- `auth_test` for the Authentication Service
- `users_test` for the User Service

The test helpers refuse to run against any other database name. Apply all
migrations to the test databases before running integration tests.

```bash
cd services/security-and-authentication
DB_NAME=auth_test go run ./cmd/migrate -direction up

cd services/user
DB_NAME=users_test go run ./cmd/migrate -direction up
```
