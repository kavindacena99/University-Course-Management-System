# Enrollment Service

This Go 1.27.2, Gin, PostgreSQL, pgx/v5 service owns **student enrollments in
CourseOfferings**. It does not own user profiles, Auth roles, Course definitions,
offering configuration, assessment data, or lecturer assignments. The actual
Course Service entity is named `CourseOffering`; its UUID `id` is stored as
`offering_id` here.

## Ownership and lifecycle

Enrollment is the only writer of `enrollments`. A row starts `active` and may
transition once to `cancelled`. Cancellation retains the row and records who
ended it. Re-enrollment creates a new row. PostgreSQL enforces one active row
per student/Offering with a partial unique index. Version checks prevent stale
cancellations.

Course remains the authority for the one assigned lecturer on an Offering.
The user explicitly chose this ownership after the discovery review. Use
Course's assignment and assigned-offerings APIs to manage or list lecturers.
Enrollment's Assessment-facing lecturer check reads Course's current
assignment and verifies live User approval and Auth role/activation; there is
no competing lecturer assignment table or status.

Course configures optional capacity and enrollment opening/closing times.
Enrollment checks the signed Course facts before creation. A local
`enrollment_offering_guards` row serializes all seat-affecting writes; the
active count and insert occur in the same transaction. `NULL` capacity means
unlimited. Course currently rejects published capacity reductions. A future
cross-service reduction protocol is separately proposed in
`../../docs/proposals/enrollment-capacity-change.md`.

## API

See [OpenAPI](../../docs/openapi/enrollment-service.yaml) for complete schemas
and errors. Public routes require an Auth-issued Ed25519 access JWT. A student
may operate only on their own enrollment; Admin may manage any student;
lecturers may read enrollments only for their assigned Offering while live
eligibility checks pass. Admin management requires `enrollments:manage`;
Admin and lecturer reads require `enrollments:read`. Auth migration
`000004_enrollment_permissions` grants these defaults.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/v1/enrollments` | Create self or Admin-selected enrollment |
| GET | `/api/v1/enrollments/{id}` | Read authorized enrollment |
| POST | `/api/v1/enrollments/{id}/cancel` | Cancel with expected version |
| GET | `/api/v1/students/{id}/enrollments` | Paginated student history |
| GET | `/api/v1/offerings/{id}/enrollments` | Paginated Offering history |
| GET | `/internal/offerings/{id}/enrollments/{student_id}/active` | Assessment membership check |
| GET | `/internal/offerings/{id}/lecturers/{lecturer_id}/active` | Assessment lecturer check |

Internal routes require the existing HMAC-SHA256 request signature and
`assessment-service` caller identity. They are not exposed by NGINX. Active
membership checks fail closed if User or Auth cannot confirm current
eligibility. Errors use the existing
`{"error":{"code":"...","message":"...","request_id":"..."}}` envelope.

## Dependencies

- User: signed `GET /internal/users/{id}/eligibility` must report
  `registration_status=approved`.
- Auth: signed `GET /internal/users/{id}/student-eligibility` and
  `GET /internal/users/{id}/lecturer-eligibility` check active role state.
- Course: signed `GET /internal/offerings/{id}` returns status, enrollment
  window, capacity, version, and assigned lecturer.

Each request has a short configured timeout, rejects redirects and malformed
responses, and propagates `X-Request-ID`. A failed authority check does not
create an enrollment. No other service database is accessed.

## Database and migrations

Migration `000001_enrollment_schema` creates `enrollments` and
`enrollment_offering_guards`. User and Offering IDs are external UUIDs with
**no cross-service foreign keys**. The guard table is local transaction
coordination, not an Offering replica. Indexes support active uniqueness,
capacity counts, and stable history lists. The down migration deletes all
Enrollment data; use it only for disposable environments.

Create an independently owned `enrollments` database and an
`enrollment_app` role. Copy `.env.example` to an ignored `.env`, replace all
placeholders, then from `services/enrollment` run:

```sh
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction version
go run .
```

Run migrations before the server. `GET /health` is liveness;
`GET /health/ready` requires DB connectivity and a clean migration version
of at least 1. The server logs structured JSON and shuts down gracefully.

Important environment values:

| Variable | Purpose |
| --- | --- |
| `ENROLLMENT_SERVICE_PORT`, `BIND_HOST` | Listener; local default is loopback port 8084 |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSLMODE` | Owned PostgreSQL database |
| `AUTH_JWT_PUBLIC_KEYS_JSON`, issuer, audience, skew | Auth access-token verification |
| `INTERNAL_SERVICE_ID`, `INTERNAL_SERVICE_SECRET`, optional previous secret | Signed HTTP |
| `INTERNAL_ALLOWED_CALLERS`, `INTERNAL_AUTH_MAX_SKEW` | Assessment-only internal API |
| `USER_SERVICE_INTERNAL_URL`, `AUTH_SERVICE_INTERNAL_URL`, `COURSE_SERVICE_INTERNAL_URL` | Dependency addresses |
| Corresponding `*_TIMEOUT` values | Dependency deadlines |
| Cookie/CSRF, CORS, trusted proxy, HTTP timeout values | Edge security and operations |

In Docker, set `BIND_HOST=0.0.0.0` and use service DNS names, not localhost,
for dependencies. `../../compose.yaml` is a local integration stack with
private service and database ports; it requires configured ignored `.env`
files, Auth's mounted Ed25519 key, MinIO root credentials, scoped Assessment
object-store credentials, and five exported DB passwords. An isolated
Compose stack built and reached readiness on 2026-11-14; production deployment
readiness has not been established.

## Tests

```sh
go test -count=1 ./...
go vet ./...
go build ./...
```

Repository integration tests run only when `TEST_DATABASE_URL` points to a
dedicated database named exactly `enrollments_test`; apply the migration first.
Tests cover active uniqueness, capacity concurrency, cancellation history,
constraints, dependency failures, authorization, and signed internal routes.
The repository's `Enrollment Service` GitHub Actions workflow runs these tests
against an ephemeral PostgreSQL 18 database, then checks `go vet`, build,
formatting, migration rollback, and migration reapplication. This workflow has
not yet been observed running in GitHub Actions.

## Known consistency boundary

Enrollment validates live Course facts before its local transaction. A Course
close/archive or User suspension can race with the Enrollment commit across
service databases. Strong atomic closure requires an explicit Course-to-
Enrollment fencing protocol; no such contract is implemented. Do not present
the current check as a distributed transaction. The proposed capacity
reduction protocol is also not implemented.
