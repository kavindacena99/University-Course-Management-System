# Implementation Report

## Scope

Completed and verified the User Service and Authentication & Security Service,
including the gateway configuration, integration contract, migrations, tests,
and documentation.

## Implemented

### Authentication Service

- Fail-closed eligibility verification with the User Service on login and
  refresh (`internal/userclient.go`).
- Login reordered to verify the password before revealing lockout or account
  availability state.
- Permission catalog and permission claims in access tokens (`perms`).
- Role-permission assignment APIs.
- Security audit events for login, password, activation, provisioning, role
  changes, session revocation, and applied user events.
- Admin endpoints now require permissions instead of a hard-coded admin role:
  - `roles:manage`
  - `account:unlock`
  - `sessions:revoke`
  - `audit:read`
- New migration `000002_auth_permissions_and_audit` with permission and audit
  tables, indexes, and default role mappings.
- Session revocation when a role's permissions change.

### User Service

- Permission-based authorization for admin lifecycle endpoints and user reads.
- `users:read` and `users:manage` constants and permission middleware.
- Removed the previous role-only admin dependency from routing.

### Gateway

- Fixed upstream path handling so `/api/v1/...` prefixes are preserved.
- Added routes for admin auth and admin user APIs.
- Added request-size limits, connect/send/read timeouts, security headers,
  general/login/registration rate limits, and a health endpoint.
- Internal endpoints remain unexposed; unknown paths return `404`.
- Excluded accidental empty tracked files `services/user/go` and
  `services/user/gofmt`.

### Documentation and configuration

- Added architecture, service, integration, security, database, API, and
  development setup documentation.
- Added OpenAPI 3 specifications for both services.
- Added sanitized `.env.example` files.
- Updated the README with routing, permissions, setup, and testing guidance.

## Verification evidence

Commands and results:

```text
services/security-and-authentication:
  go build ./...                              PASS
  go vet ./...                                PASS
  TEST_DATABASE_URL=auth_test go test ./...   PASS

services/user:
  go build ./...                              PASS
  go vet ./...                                PASS
  TEST_DATABASE_URL=users_test go test ./...  PASS
```

Migration versions after upgrade:

```text
authentication_and_security: version 2, dirty=false
auth_test:                   version 2, dirty=false
users:                       version 4, dirty=false
users_test:                  version 4, dirty=false
```

Seeded permission matrix:

```text
auth_permissions: 7 rows
ADMIN:    7 permissions
STAFF:    4 permissions
LECTURER: 1 permission
STUDENT:  0 permissions
```

Live end-to-end run through the NGINX gateway:

```text
SERVICES healthy
GATEWAY healthy
BOOTSTRAP ok
ADMIN_LOGIN roles=ADMIN perms=account:unlock,audit:read,roles:manage,sessions:read,sessions:revoke,users:manage,users:read
PERMISSIONS count=7
REGISTER status=pending
APPROVE status=approved version=2
ACTIVATION_NOTIFIED
STUDENT_LOGIN roles=STUDENT
ADMIN_LIST total=1
SUSPEND status=suspended
INELIGIBLE_LOGIN denied=403
AUDIT total=5 latest=login_succeeded
FAIL_CLOSED unavailable=503
```

The end-to-end run exercised registration, admin approval through the gateway,
outbox delivery to the auth service, activation-token notification, student
login, admin search, suspension, denial of an ineligible login, audit listing,
and fail-closed behavior with the User Service stopped.

## Operational notes

- Access tokens issued before this change do not contain `perms`; affected
  administrators must log in again to receive permission claims.
- Apply migration `000002` to every environment before starting the updated
  Authentication Service.
- Existing role mappings are preserved; the new default permission mapping is
  seeded only on migration.
- No secrets, `.env` files, databases, or migration history were modified
  destructively.
