# Enrollment Service discovery and integration decisions

This report records the source-code investigation before Enrollment Service
implementation. It describes current contracts, not proposed endpoints.

## Repository and conventions

- The implemented services are `services/security-and-authentication`,
  `services/user`, and `services/course`. Each is a separate Go 1.27.2 module
  using Gin, pgx/v5, PostgreSQL, `golang-migrate` versioned up/down SQL files,
  environment-based configuration, structured `slog` JSON logs, request IDs,
  health/readiness routes, and graceful HTTP shutdown.
- `infrastructure/nginx` routes the three public `/api/v1` namespaces and
  does not proxy `/internal/*`. The current upstreams use host loopback. The
  `frontend` directory has no tracked application files. No Compose file or
  CI workflow is present.
- Each service owns its own database. Repository integration tests use a
  dedicated `TEST_DATABASE_URL` and refuse a non-test database name. The
  error envelope is `{"error":{"code":"...","message":"...","request_id":"..."}}`.
  List routes use `limit`, `offset`, stable sort order, and a total count.
  Request decoders reject unknown JSON fields.
- There is no shared Go security module. Existing services implement the same
  Ed25519 JWT verification and HMAC-SHA256 internal-request wire contract
  locally; Enrollment must follow those contracts without introducing another
  authentication protocol.

## Existing integration contracts

- Auth issues Ed25519 access JWTs with subject, roles, permissions, session ID,
  issuer, audience, expiry, token type, and key ID. Services verify these
  locally. Auth owns role membership and account activation.
- Internal requests sign method, escaped path and query, service ID, RFC3339
  timestamp, nonce, and SHA-256 body digest with HMAC-SHA256. Receivers enforce
  a caller allowlist, bounded timestamp skew, and one-use nonces. Internal
  endpoints are absent from NGINX.
- User exposes signed `GET /internal/users/{id}/eligibility` and
  `GET /internal/users/{id}`. `registration_status=approved` is required;
  missing or unavailable data must fail closed. User does not own roles.
- Auth exposes signed
  `GET /internal/users/{id}/lecturer-eligibility`, restricted to
  `course-service`. It verifies active Auth account and LECTURER role. There
  is no corresponding student-role endpoint for an Enrollment service caller.
- Course calls User and Auth to check lecturers. Its actual entity is
  `CourseOffering`, with UUID `id` and statuses `draft`, `published`,
  `closed`, and `archived`. It owns publication, enrollment open/close
  timestamps, optional capacity, and one `lecturer_user_id` per offering.
  Course provides JWT-protected public offering reads, but no HMAC-protected
  internal offering read for Enrollment. Course rejects published capacity
  reductions pending a separate proposed coordination protocol.

## Compatibility decisions and implementation response

1. **Lecturer assignment ownership (resolved):** Course currently owns assignment and
   reassignment, publication requires its assigned lecturer, and lecturer
   mutation authorization compares the JWT subject with that field. A second
   authoritative Enrollment assignment table would conflict with those rules.
   The user chose Course as the authority. Enrollment stores no lecturer
   assignment rows; it verifies Course's assignment for Assessment. Course's
   existing assignment and own-offerings APIs remain the management routes.
2. **Offering facts (implemented):** Enrollment needs a narrowly scoped, signed Course
   offering-read contract returning ID, status, enrollment window, capacity,
   version, and lecturer ID. The minimal `GET /internal/offerings/{id}`
   contract was added with an `enrollment-service` caller restriction.
3. **Role eligibility (implemented):** Auth needs a narrowly scoped, signed
   student-role eligibility contract for an Admin to enroll another user.
   `GET /internal/users/{id}/student-eligibility` was added for
   `enrollment-service`. The existing lecturer eligibility route also accepts
   that caller. User approval and Auth role/activation remain separate checks.
4. **Concurrent Course lifecycle changes:** A Course Offering may close or
   archive while Enrollment is processing a request. The desired guarantee
   across services needs an explicit coordination rule. The current public
   Course API alone cannot make a cross-service enrollment commit atomic with
   a Course status change.

## Recommended Enrollment structure

Follow the existing module pattern: `services/enrollment/internal` for domain
and use cases, narrow repository and dependency interfaces, Gin handlers and
middleware, `database` for pgx pool setup, `cmd/migrate` and numbered SQL
migrations, `Dockerfile`, `.env.example`, and dedicated PostgreSQL tests.
Student enrollments should have a partial unique index for one active row per
student and Offering; historical ended rows remain. Course remains the
lecturer authority, so Enrollment does not store a competing assignment table.
Enrollment capacity enforcement, already configured in Course, requires a
per-Offering transactional guard in the Enrollment database.

No RabbitMQ or new database technology is justified by the current repository.
