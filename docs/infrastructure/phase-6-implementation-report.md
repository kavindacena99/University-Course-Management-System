# Phase 6 — consolidated QA target

## 2026-11-28 live DEV acceptance update

After PR #29 merged, main advanced again to
`b1db98eb39a2167c28128c9836db6029d7fc1418` with additional frontend
account, enrolment and results screens and a small Assessment CSV parser fix.
The frontend and Assessment images were rebuilt locally. The DEV controller
again reported 15 healthy containers, seven successful jobs and 12 passing
HTTP probes. The same five live Playwright scenarios passed against this newer
source with no skips. The new student enrolment screen loads asynchronously;
the test now waits for its HTTP 200 self-enrolment response before navigation.
The Assessment Go suite, frontend production build and Docker-based TypeScript
check passed. Infrastructure unittest discovery passed 51 tests.
This later run supersedes the earlier source-specific acceptance result for
release selection. The successful GHCR publication for PR #29's merge commit
`50963ad753ff9c7f4b27c6bae74eb4348da02130` predates these application
changes and must not be used as the new RC image set.

PR #28 merged at `b6a8935a341cc29808a93dc61f822ca106762c98`.
The preexisting DEV stack was started without deleting or replacing volumes;
its controller reported 15 healthy containers, seven completed setup jobs and
12 successful HTTP probes. Only the frontend image was rebuilt from merged
source for the live test. Dedicated synthetic admin, lecturer and student
accounts, two courses, two offerings and one enrollment were created through
the existing User, Auth, Course and Enrollment APIs. The second offering was
left unpublished because the Course service correctly rejects publishing an
offering without a lecturer. A failed first setup attempt left additional
synthetic DEV records; those records were retained, not silently removed.

Playwright against `http://localhost:3000` passed **5/5 tests, zero skips**.
Observed requests covered login, session restoration and refresh, logout,
anonymous and role denial, lecturer assigned and unassigned access, student
enrollment and report, lecturer/admin analytics, assessment creation and
publication, multipart file submission, protected byte-for-byte download,
grade save and release, and result persistence after reload. Student,
lecturer and administrator analytics each returned HTTP 200. The test runner
used installed Chrome; its local HTML report and traces remain ignored under
`tests/browser-phase6/artifacts/`. The run also checked anonymous backend
denial and student 403 access to administrator analytics. A protected draft
offering is hidden with a not-found response, while its enrollment endpoint
denies the lecturer with 403.

The frontend production image build and Docker-based TypeScript check passed.
Infrastructure unittest discovery passed 51 tests; ten Compose models passed;
both edited Playwright files passed `node --check`; and `git diff --check`
passed. The secret scan found no leaks in 50 reachable commits or 552
tracked/nonignored working-tree files. No Go service code changed in this
follow-up. These are DEV acceptance results; a new reviewed source commit,
protected-main image publication, new immutable RC, fresh verified external
QA backup, and full QA validation are still required. RC1/RC2 evidence remains
immutable. The recorded Silo HIGH CVE-2026-39414 and exact-digest guarded
runtime exception remain visible.

## Preparation and safety state

The prepared target is `platform-qa-target` with frontend port 3001 and
gateway port 8081. Its existing generated inputs under
`.infra-state/phase123/targets/qa/` were inspected by filename and selected
non-secret identity fields; they were not regenerated or overwritten.
`targetctl.py config` validated the consolidated `compose.target.yaml` model
against the exact nine-image RC, reviewed PostgreSQL candidate, Silo digest,
mandatory storage guard, private service ports, and independent volumes.
Read-only preflight found zero existing target containers and zero target
volumes. Docker Desktop is a Linux engine with about 8 GiB available. The
reviewed local PostgreSQL candidate image ID is
`sha256:cb90d02d2e03c00889b7d8c08e1bf522ef62891b121c02d7b0a4239170661a4c`;
its retained Trivy scan reports zero HIGH/CRITICAL findings. All nine RC
application images and the pinned Silo image were pulled by digest without
starting containers. The preflight record is in ignored
`.infra-state/releases/v0.1.0-rc.1/qa-preflight.json`.

The Phase 4 storage approval covers image publication only. On 2026-11-28 the
owner separately approved the exact Silo digest for isolated local QA and
PROD-like runtime rehearsal, with the Select guard mandatory. The protected,
ignored `.infra-state/releases/storage-runtime-approval.json` records that
scope. The controller refuses deployment without this exact
image/environment/guard record. At this checkpoint no target container,
network, or volume had been created.
The frontend has some real API integration, but many route components remain
static or incomplete; full browser and business acceptance must be observed
after an authorized runtime starts. **Phase 6 is not complete.**

## RC1 QA execution and failed verdict

The owner separately approved the exact Silo digest for isolated local QA and
PROD-like runtime use with the guard mandatory. RC1 was deployed into the
prepared `platform-qa-target` using the nine digest-pinned GHCR images, the
reviewed local PostgreSQL candidate and Silo image. Initial target volumes did
not exist, so no existing target data required a predeployment backup. The six
migration jobs, storage bootstrap, and final DB permissions job exited 0.
Ten runtimes were healthy, and the controller matched all nine running image
IDs against the release references. SQL ownership, non-superuser roles,
cross-database denial, runtime DDL denial and migration versions passed for
all six logical databases. The guard returned 403 for an S3 Select POST, and
the unguarded port 19000 was unreachable from the gateway. Gateway/frontend
Nginx syntax and limited HTTP smoke probes passed.

QA acceptance **failed** when the published frontend asset was found to contain
`http://localhost:8080`, while QA's gateway is at `http://localhost:8081`.
Browser API requests from the QA frontend would target the wrong environment.
This is a release blocker; full authenticated business and browser tests were
not run and cannot be counted as passes. The machine-readable failed verdict
is retained at `.infra-state/releases/v0.1.0-rc.1/qa-validation.json`.
QA was gracefully stopped. No QA target container remains running; the
`platform-qa-target_postgres-data` and `platform-qa-target_minio-data` volumes
remain, as do the legacy and DEV volumes. RC1 remains an immutable prerelease.
A reviewed source fix and new Phase 4 publication are required for RC2.

## RC2 preparation

PR #20 merged the frontend same-origin gateway proxy at
`1d76667cfb27077568f5dca283352386b67d6f02`. Run 46120731377
published and verified nine fresh images for that commit. The stopped QA
target's non-secret `frontend.conf` was updated to the reviewed proxy; its
previous bytes were retained in ignored `.infra-state/releases/config-backups/qa/`.
Credentials, signing keys, target data volumes and DEV remained unchanged.
RC2 deployment requires a verified external backup of the now-existing QA
PostgreSQL and object volumes before migrations.

## RC2 QA execution and failed verdict — 2026-11-28

The annotated [RC2 tag](https://github.com/kavindacena99/University-Course-Management-System/releases/tag/v0.1.0-rc.2) points to source commit
`1d76667cfb27077568f5dca283352386b67d6f02`. The GitHub prerelease is
downloadable; its release manifest and `images.env` hashes matched the
repository copies. The deployed manifest SHA-256 was
`eb58d421135506f8c9e16d019e2bf65b805b09aaac06f68812fc88037a5b94dc`.

Preflight found 19 stopped RC1 target containers and two existing target
volumes. A protected external backup of all six logical PostgreSQL databases
passed `pg_restore --list` checks. The submissions bucket was empty, and the
guarded object export correctly recorded an empty copy. The backup manifest
SHA-256 was `48819db48224221284b52fbbff3a2121237dd4baa74b2d16b69c4301a60e8a9e`.
The backup is outside the repository and synced directories; credentials and
signing keys were not copied into it. The first backup attempt exposed an
empty-bucket handling defect in `targetctl.py`; no backup marker was issued
until the corrected retry verified all files. The failed partial attempt was
retained rather than silently treated as a valid backup.

RC2 then pulled nine digest-pinned images and deployed without rebuilding.
All 19 target components passed their required runtime/job states and the
nine running image IDs matched the release manifest. SQL role, ownership,
cross-database denial, DDL restriction and migration checks passed for six
databases. S3 Select was denied and the unguarded Silo listener was not
reachable from the gateway. A scoped synthetic object passed upload, checksum
readback and deletion through the guard. Gateway health returned 200, frontend
returned 200, frontend `/api/v1/auth/me` returned 401, and six unauthenticated
backend routes all returned 401. The latter probes took about 2.0–3.5 seconds
each; no load test was performed. Selected container memory samples were about
108 MiB for PostgreSQL and 103 MiB for Silo.

Playwright with installed Chrome rendered the login and course pages without
page errors. The course API request used `http://localhost:3001/api/...`, the
frontend's same-origin proxy, and displayed an unauthenticated error. However,
`/lecturer/assignments`, `/lecturer/submissions`, and
`/lecturer/courses/view` rendered without login and made **no API requests**.
Their source uses local placeholder data/state. Required authenticated
enrollment, assessment/file, analytics and full browser workflows therefore
were not demonstrated. This is a release blocker, not an accepted limitation.

The [machine-readable RC2 QA verdict](../../release-manifests/v0.1.0-rc.2/qa-validation.json)
is **failed**. RC2 is not eligible for stable promotion. QA was stopped
gracefully; no `platform-qa-target` container remains running, and both target
volumes and all DEV volumes remain present. No PROD-like target was started.
**Phase 6 remains IN PROGRESS.** A reviewed application fix, fresh publication,
new RC and repeat full QA are required before acceptance.

## Authenticated application remediation branch — pending live acceptance

The RC2 browser failure was caused by registered lecturer routes rendering static
pages without authentication or API traffic. This branch replaces registered
operational routes with protected, lowercase, ID-based pages. A later protected
main commit also added live account, student, lecturer and administrator pages;
these were merged and their functional routes retained. The remaining static
placeholder paths are not registered. Existing email login, course API use,
and the same-origin proxy were recognized as existing work rather than rebuilt.

Authentication now uses a shared session provider. It calls `/api/v1/auth/me`
at startup, uses rotating cookie refresh after a 401, obtains the CSRF token
from the Auth response for unsafe methods, and calls backend logout. The unused
access token and user copies in session storage were removed. Role and
permission guards protect direct URL navigation. The backend continues to
enforce identity, permissions, assigned lecturer checks, enrollment ownership,
CSRF, submission file access, and version transitions.

The [updated integration matrix](../frontend-backend-integration-audit.md)
lists all current pages, endpoints, source status, and required tests. The
frontend now requests course and offering records, self enrollments, assigned
offering enrollments, assessments, submissions, protected files, grades, and
student/lecturer/admin analytics. Assessment publication and grade release send
persisted versions; upload uses multipart form data and an `Idempotency-Key`.
The submission UI checks the default 20 MiB bound and allowed extension while
the service remains responsible for content-type and authorization validation.
No GPA, grades, enrollment counts, or analytics are generated locally.

TypeScript validation and production build passed on an isolated copy of the
frontend source and committed npm lockfile. The repository's OneDrive
`node_modules` directory contained a locked native binary, preventing an
in-place dependency install. The committed pnpm lockfile was also rejected by
the local minimum-release-age policy for `lucide-react@1.83.0`; this did not
change the lockfile. The temporary build is evidence of source compilation,
not a live workflow pass.

A Playwright suite under `tests/browser-phase6/` now covers anonymous route and
backend denial, lecturer assignment and analytics, student self reports and
role denial, then real draft creation, publication, file upload, protected
download, grade save, release, and reload persistence, plus an optional
administrator dashboard check. The suite uses only environment-supplied
dedicated account credentials. Its test runner installed with zero npm audit
findings and discovered five tests; all five skipped
because no running gateway or synthetic account configuration was available.
The skips are recorded as **no browser acceptance evidence**.

Local non-runtime gates passed: 51 infrastructure tests, all ten Compose
models, `git diff --check`, formatting of changed frontend files, and a scan
of 49 reachable commits plus 552 tracked/non-ignored working-tree files with
no secret findings. The built frontend assets contained no direct service or
environment-specific API URL. No Go service source changed, so the Go test
matrix was checked by the PR's required CI jobs.

Draft [PR #28](https://github.com/kavindacena99/University-Course-Management-System/pull/28)
is mergeable. Its 19 application/infrastructure/image-input and image-build
checks passed, including all six Go service jobs, frontend, infrastructure,
and ten image-build jobs. Publication prerequisite/publish jobs were skipped
for this PR as intended. This CI result does not replace the five skipped live
Playwright tests, a reviewed merge, new immutable publication, or full QA
acceptance.

`docker ps` showed no running service stack. No QA or PROD-like target was
started, so authenticated browser tests, API persistence checks, negative
authorization tests with dedicated accounts, and projection rebuilding checks
remain unexecuted. The application cannot be accepted as Phase 6 complete or
as a new RC on this evidence. Before a new candidate, complete the live
Playwright matrix, resolve any defects it finds, pass CI, merge after review,
publish nine immutable images, and obtain a fresh verified external backup of
the existing QA PostgreSQL and object-store data before any QA migration or
potentially destructive change. RC1/RC2 evidence and the storage guard remain
unchanged. Phase 5 remains in progress; Phase 7 remains blocked by QA acceptance.

## RC3 live development and isolated QA acceptance — 2026-11-28

The section above records the earlier PR #28 state. PRs #28, #29 and #30 were
subsequently merged. The accepted application source is
`91f53ba8b3b68c74b4de99c70baa20639102c181`. In DEV, dedicated synthetic
administrator, lecturer and student accounts exercised the real gateway and
backend. All five Phase 6 Playwright tests passed with zero skips. The suite
checked anonymous denial, session restoration and logout, role restrictions,
assigned and unassigned offerings, student self enrolments and analytics,
administrator summary, assessment creation and publication, multipart file
submission, protected byte-exact download, grading, release, and persisted
results after navigation and reload. It required actual API responses.

The same nine-image source was published by protected-main
[run 46153596004](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46153596004)
and tagged as [RC3](https://github.com/kavindacena99/University-Course-Management-System/releases/tag/v0.1.0-rc.3).
The [RC3 manifest](../../release-manifests/v0.1.0-rc.3/release-manifest.json)
pins nine digests. Before QA migrations, a fresh external backup verified six
PostgreSQL archives and the empty submissions bucket. The consolidated QA
deployment passed 19 component health and exact image checks, six database
permission/migration checks, gateway/frontend smoke, S3 Select denial and
alternate-listener isolation. A separate synthetic S3 object was uploaded,
read back by checksum and deleted through the guard.

In QA, **five Playwright tests passed, none failed or skipped** using new
synthetic accounts and the actual frontend/gateway. A persisted released grade
and protected file survived an Assessment restart; an anonymous file request
returned 401. During a bounded Analytics outage, the route returned 5xx and
then recovered to 200; health returned to 19 components. A second external
backup verified six database archives plus the submission object. QA was
stopped gracefully, with both target volumes preserved. The detailed
[machine-readable QA verdict](../../release-manifests/v0.1.0-rc.3/qa-validation.json)
is **passed**. Authenticated screenshots show [student results](evidence/rc3-qa/student-results.png),
[lecturer analytics](evidence/rc3-qa/lecturer-analytics.png), and the
[administrator dashboard](evidence/rc3-qa/admin-dashboard.png). The report and
browser artifacts remain under ignored local state; the screenshots contain
synthetic records only.

Account settings, administrator final-results UI, registration/password
recovery, pagination extremes and Analytics rebuilding were not separately
browser tested. A later catalog change merged as `889c5b7` after RC3 image
publication; it is outside RC3 QA acceptance. The pinned Silo image retains
raw HIGH CVE-2026-39414. Its exact-digest local QA/PROD-like approval does not
erase that finding, and the guard remains mandatory. Phase 6 application and
isolated QA acceptance passed for **RC3's exact source and digests**; this does
not establish later-main acceptance, recovery, or public production readiness.
