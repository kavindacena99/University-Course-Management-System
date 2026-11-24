# Authentication permission remediation

Date: 2026-11-23. Result: **COMPLETED AND VERIFIED** for the affected catalogue,
authorization and database compatibility scope. No legacy database was accessed.

## Root cause and authoritative contracts

Authentication migrations 000002–000004 established eleven permissions. Later
migrations 000005 and 000006 added seven Assessment and five Analytics permissions,
but `internal/model.go` was never extended. Both `Service.SetRolePermissions` and
`Repository.SetRolePermissions` call `IsValidPermission`, so legitimate seeded
permissions were rejected by administrative permission management. This was an
application defect, not just a stale test count.

The old `TestServicePermissionsAndAudit` also assumed ADMIN receives the entire
catalogue. Actual seed SQL grants ADMIN **19**, STAFF **4**, LECTURER **8** and
STUDENT **2**. Two individual-record permissions are opt-in, and student/lecturer
Analytics permissions belong to their corresponding default roles. Granting all
23 to ADMIN merely to satisfy the test would change the authorization design.

The authoritative valid-identifier catalogue is now Auth's `AllPermissions`,
matching all 23 migration-seeded identifiers and downstream permission constants.
Existing SQL migrations remain the contract for **default role grants**, while
persisted role assignments can still be explicitly managed through existing
`roles:manage`-protected endpoints. Catalogue membership is not an automatic grant.
No legitimate permission was removed or renamed; no case-normalizing/wildcard
authorization was introduced.

## Full inventory

Sources below are under `services/security-and-authentication/migrations`:
M2=`000002_auth_permissions_and_audit.up.sql`,
M3=`000003_course_permissions.up.sql`, M4=`000004_enrollment_permissions.up.sql`,
M5=`000005_assessment_permissions.up.sql`, M6=`000006_analytics_permissions.up.sql`.
Roles: A=ADMIN, S=STAFF, L=LECTURER, T=STUDENT; “opt-in” means no seeded default.
Routes are under `/api/v1`. C=exact domain/seed catalogue tests, M=23-permission
Gin middleware matrix, R=database default-role contract, A=audit/management test.

| Exact permission | Seed | Domain before → after | Route / operation and authorization reference | Default roles | Tests | Classification |
| --- | --- | --- | --- | --- | --- | --- |
| `users:read` | M2 | Present → present | User `/admin/users`, details/history; User `handler.go` RequirePermissions | A,S,L | C,M,R,A | Valid |
| `users:manage` | M2 | Present → present | User approval/rejection/suspension/reinstatement; RequirePermissions | A | C,M,R | Valid |
| `roles:manage` | M2 | Present → present | Auth `/admin/auth/roles`, `/permissions`, role/user assignments; RequirePermissions | A | C,M,R,A | Valid |
| `sessions:read` | M2 | Present → present | Documented session-read capability; no current route-specific consumer; own-session route uses authenticated ownership | A,S | C,M,R | Valid, reserved/documented |
| `sessions:revoke` | M2 | Present → present | Auth DELETE `/admin/auth/users/:id/sessions`; RequirePermissions | A,S | C,M,R | Valid |
| `account:unlock` | M2 | Present → present | Auth POST `/admin/auth/users/:id/unlock`; RequirePermissions | A,S | C,M,R | Valid |
| `audit:read` | M2 | Present → present | Auth GET `/admin/auth/audit`; RequirePermissions | A | C,M,R,A | Valid |
| `courses:manage` | M3 | Present → present | Course `/admin/courses` lifecycle and admin offering creation/lecturer assignment; RequirePermissions | A | C,M,R | Valid |
| `offerings:manage` | M3 | Present → present | Course `/offerings/:id` update/publish/close/archive; RequirePermissions plus service ownership rules | A,L | C,M,R | Valid |
| `enrollments:read` | M4 | Present → present | Enrollment reads/listing; `internal/service.go` actor role/permission and ownership checks | A,L | C,M,R | Valid |
| `enrollments:manage` | M4 | Present → present | Enrollment creation/cancellation for administrators; service actor checks, own-student rules separate | A | C,M,R | Valid |
| `assessments:read` | M5 | Missing → added | Assessment offering lists/details/results; `internal/service.go`, lecturer assignment enforced | A,L | C,M,R | Valid |
| `assessments:manage` | M5 | Missing → added | Assessment creation/status/extensions; service role and offering ownership checks | A,L | C,M,R | Valid |
| `assessments:grade` | M5 | Missing → added | PUT `/submissions/:id/grade`; service grading authorization | A,L | C,M,R | Valid |
| `assessments:release` | M5 | Missing → added | POST `/submissions/:id/release`; service release authorization | A,L | C,M,R | Valid |
| `assessments:submit` | M5 | Missing → added | POST `/assessments/:id/submissions`; STUDENT role and active enrollment also required | A,T | C,M,R | Valid; ADMIN grant alone does not satisfy STUDENT role |
| `assessments:student:read` | M5 | Missing → added | Individual submissions/files/results for an administrator; service role + explicit permission | Opt-in | C,M | Valid, sensitive opt-in |
| `assessments:results:import` | M5 | Missing → added | POST `/offerings/:id/results/import`; `final_results.go` ADMIN + import permission | A | C,M,R | Valid |
| `analytics:student:read` | M6 | Missing → added | GET `/analytics/students/me`; role/permission, subject is current student | T | C,M,R | Valid |
| `analytics:lecturer:read` | M6 | Missing → added | GET `/analytics/lecturer/offerings/:id`; role/permission plus offering assignment | L | C,M,R | Valid |
| `analytics:admin:read` | M6 | Missing → added | GET `/analytics/admin/summary`; `handler.go` requirePermission | A | C,M,R | Valid |
| `analytics:admin:student:read` | M6 | Missing → added | GET `/analytics/admin/students/:id`; ADMIN + explicit individual access | Opt-in | C,M | Valid, sensitive opt-in |
| `analytics:manage` | M6 | Missing → added | Analytics programme/grade-band configuration and rebuild; ADMIN + permission | A | C,M,R | Valid |

Downstream evidence: `services/{user,course,enrollment,assessment,analytics}/internal`
models, middleware, handlers and service authorization. `sessions:read` remains
documented in `docs/authentication-service.md`; absence of an admin-read route
does not demonstrate it is obsolete. It was preserved, not used to invent a new
endpoint. No deprecated/duplicate/obsolete identifier was found. Future session
administration remains a product decision, outside this correction.

## Changes and security impact

- `services/security-and-authentication/internal/model.go`: added twelve named
  constants and catalogue entries. Validation now recognizes the existing seeds.
- `internal/service_test.go`: compare exact sorted identifiers instead of count
  only; assert the actual ADMIN subset; restore original STUDENT grants during
  cleanup; verify audit actor, success, role, previous and resulting grants.
- New `internal/permissions_test.go`: independent 23-identifier contract, duplicate
  and malformed/case/wildcard rejection; all four seeded role sets; unauthenticated
  401, role-only/unrelated-permission 403, exact-permission 204 for every identifier.
- New `tests/infrastructure/test_permission_contract.py`: compare actual SQL seeds
  and domain catalogue, and ensure all six modules' permission constants are
  registered. This catches cross-module drift that a single-module test cannot.

No SQL migration was changed or added. The persisted seeds/grants already match
the implemented routes; no data correction is necessary. Applied migration
history is respected. Fresh migrations and repeated no-change migration execution
were checked only on the isolated rehearsal. Default privileges, token contents,
route guards, session revocation, HMAC caller isolation and API response fields
were not broadened or removed. Newly recognized opt-in permissions still require
an explicit authorized assignment; their downstream role/ownership rules remain.

## Executed validation

Python commands use `.infra-venv/Scripts/python.exe -B` from the repository root.
Go checks run with pinned Go 1.27.3, using disposable test databases on
`platform-qa-pg-rehearsal-20261123` only.

| Command | Result |
| --- | --- |
| `infrastructure/scripts/rehearsal_tests.py --only auth` | Full Auth suite, modules/vet/build PASSED after remediation |
| `infrastructure/scripts/rehearsal_tests.py --evidence-dir .infra-state/remediation/backend-tests` | All six services PASSED, including available real-DB tests and fresh migration fixtures |
| `infrastructure/scripts/rehearsal_tests.py --only auth --test-run '^TestServicePermissionsAndAudit$' --evidence-dir .infra-state/remediation/auth-targeted` | Named test PASSED (0.21s); exact catalogue/default grants/audit assertions retained |
| `-m unittest discover -s tests/infrastructure -v` | 24 PASSED, including cross-service permission and storage boundaries |

The Go wrapper executes `go mod download`, `go mod verify`, `go mod tidy -diff`,
`go test -count=1 ./...`, vet and build independently in each module. It removes
only fixture databases it created and verifies the six restored business DB
fingerprints are unchanged. Assessment/Analytics have smaller existing suites;
this is not a claim of complete authenticated end-to-end coverage. The separate
real storage SDK suite is recorded in the storage report.

Evidence: `.infra-state/remediation/backend-tests/{auth,user,course,enrollment,assessment,analytics}.log`
and `auth-targeted/auth.log`. The first Auth rerun accidentally used the old
wrapper's fixed `phase123/tests/auth.log` path and overwrote that one raw prior
log. Its original failed result remains in the prior report/ledger and tool
transcript; it was not retroactively called a pass. The wrapper now creates
exclusive timestamped/explicit new evidence directories. This evidence-retention
mistake is disclosed; subsequent tests and historical report text are preserved.


## 2026-11-24 shutdown recovery and fresh revalidation

Fresh named database-backed `TestServicePermissionsAndAudit` passed (0.21s).
All six full backend suites, module verification/tidy, vet and builds passed on
fresh disposable fixture databases. SQL confirmed 23 distinct permissions and
unchanged default role grants: ADMIN 19, LECTURER 8, STAFF 4, STUDENT 2.
Migrations 000005/000006 added twelve valid Assessment/Analytics permissions that
the original eleven-entry Auth allowlist omitted. The existing correction adds
those definitions without changing migrations or broadening default grants.
The full 23-permission inventory and classification remain in
`remediation-authentication-permissions.md`. Authorization/audit and caller-boundary
regressions passed within the Authentication suite; the separate middleware matrix
checks 401/403/204 for all catalogue entries. No obsolete identifier was identified.
