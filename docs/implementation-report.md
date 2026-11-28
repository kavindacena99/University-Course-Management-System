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

## Course Service extension (2026-11-14)

The Course Service now owns courses, offerings, assignment, catalogue reads,
versioned updates, lifecycle transitions, and transactional local audit. Its
schema is migration `000001_course_schema` in a separate `courses` database.
Auth migration `000003_course_permissions` grants Course permissions. Auth's
new signed lecturer eligibility endpoint is restricted to `course-service`;
provisioning and user events remain restricted to `user-service`. Course
assignment and publication require live User and Auth eligibility checks.
The API contract is [Course OpenAPI](openapi/course-service.yaml), with
operational details in [Course Service](course-service.md).

Verification on dedicated local databases:

```text
courses_test migration down -> none; up -> 1, dirty=false: PASS
auth_test migration 3 -> 2 -> 3, dirty=false: PASS
Course Service go test -count=1 ./... (courses_test): PASS
Auth Service go test -count=1 ./... (auth_test): PASS
User Service go test -count=1 ./... (users_test): PASS
Course Service go vet ./...: PASS
Course Service go build ./...: PASS
NGINX configuration test: PASS
Course Service Linux amd64 service and migration builds: PASS
Course and Auth OpenAPI YAML parsing and local references: PASS
Course runtime smoke: /health 200, /health/ready 200, unauthenticated catalogue 401
Docker build: UNVERIFIED (Docker Desktop Linux engine stopped)
Race test: UNAVAILABLE (CGO disabled; no C compiler installed)
```

The existing User and Auth migration histories were retained; the Course
test database was recreated only for migration verification. At the time of
that extension, no Compose or CI configuration existed; see the infrastructure
update below. Capacity reductions on published offerings remain disallowed
pending an Enrollment Service validation contract.

## Infrastructure update (2026-11-22)

The repository now includes a full local Docker Compose stack in
`infrastructure/docker/compose.yaml`, a development override in
`compose.dev.yaml`, and a tracked environment template in
`infrastructure/environments/dev/.env.example`. It runs the React frontend,
gateway, six services, six PostgreSQL databases, the private object store, and
one-shot migration/bootstrap jobs.

Local validation on 2026-11-22:

- All nine images built.
- All long-running containers became healthy; all migrations and the MinIO
  bootstrap job exited 0.
- Gateway routing reached all six services; only frontend port 3000 and
  gateway port 8080 were published.
- Frontend health, SPA fallback, assets, security headers, and CORS preflight
  passed.
- Scaling Auth to two replicas split traffic 10/10; stopping one replica left
  the other serving requests.
- A Course outage produced gateway 504/502 responses; restart restored
  routing, and Course logged `server_stopped` during shutdown.

Production deployment remains unverified; see
[deployment-readiness.md](deployment-readiness.md) and
[infrastructure/architecture.md](infrastructure/architecture.md).
