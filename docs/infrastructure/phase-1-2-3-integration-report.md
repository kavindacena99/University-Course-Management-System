# Phases 1–3 integration and completion report

Date: 2026-11-23. Source baseline: `main` at
`a2df85f2bd655ccbe155ce781a2c95b9d8b41f2d`; no commit was made.

## Executive decision

| Phase | Status | Evidence / remaining gate |
| --- | --- | --- |
| 1 — isolation/PostgreSQL | **PARTIALLY COMPLETED** | One-cluster/six-database target prepared for three environments; isolated QA restore, role/SQL security, fresh migrations and persistent recreation passed. Authentication real-DB permission-catalog test remains failing; no cutover. |
| 2 — runtime images | **PARTIALLY COMPLETED** | Ten local builds/scans and full isolated runtime passed. Original PostgreSQL 26 HIGH/1 CRITICAL remediated in derived same-major candidate. Object-store HIGH remains unsuppressed pending owner/vendor disposition. |
| 3 — CI | **IMPLEMENTED BUT REMOTE VERIFICATION PENDING** | Original Compose failure reproduced/fixed; eight models, actionlint, safety tests and Ansible syntax passed locally. No remote run was triggered. |

The five Phase 0 audit files and historical reports remain unchanged. Their
recommendations were adapted to the user's newer strict boundary: rehearse in
a unique QA-only project, do not cut over any existing environment, and do not
unlock registry publishing while repairing CI.

Detailed results: [Phase 1](phase-1-implementation-report.md),
[Phase 2](phase-2-implementation-report.md),
[Phase 3](phase-3-implementation-report.md),
[updated command ledger](updated-infrastructure-validation-ledger.md).

## Cross-phase compatibility

Shared service definitions and build contexts remain in the original Compose
file. The separately selected target extends them, swaps only database
host/migration credentials/dependencies, preserves all six service DNS names
and ports 8081–8086, and adds one explicit permissions job. Nginx global,
upstream, routing, host/Docker variants, snippets, limits, headers and timeouts
were not rewritten. Generated local copies only adjust writable PID location
and frontend CSP origin for the rehearsal's port. Gateway route checks reached
all six backends; they do not prove authenticated end-to-end functionality.

Runtime images expose the same binaries/configuration/ports and migration flags.
Six migration jobs use service migrators; runtime services use `*_app`. The
permission barrier prevents startup after a failed migration or grant step.
Targets have independent project-scoped networks/data volumes, secrets, JWT
issuers/keys and cookie names. Backend/PostgreSQL/object-store ports remain
unpublished. Target application read-only/tmpfs/capability settings were exercised
in the new rehearsal; old environment containers were not recreated to apply them.

All three prepared targets have 18 components, one PostgreSQL database service,
six logical database mappings, two persistent volumes and pinned local image
references. They were **configuration-validated only**. The one executed project
was `platform-qa-pg-rehearsal-20261123`, using loopback 3101/8181; 10 long-running
containers became healthy and eight setup jobs exited 0, then all stopped cleanly.
No additional domain service, cloud product or release mechanism was introduced.

Existing Ansible inventories/roles still deploy the legacy topology. Three
inventory graphs and twelve syntax checks passed without deployment. Existing
`platformctl.py`, backup and scaling interfaces were preserved; they do not
implicitly understand consolidated PostgreSQL. New guarded scripts provide the
separate rehearsal/backup interface. Do not apply legacy six-DB backup assumptions
or DEV-oriented scaling scripts to a target without a reviewed adapter. Ansible
runtime idempotency against the new target is not claimed.

CI validates both legacy and target selection using temporary synthetic files;
neither Windows paths nor ignored local inputs are required on GitHub runners.
The older runner's duplicate security-options merge issue is resolved with four
explicit `!override` lines. Existing automatic publication/preparation workflows
are now manual failure holds so successful CI cannot publish out of scope.

## Local artifact measurements

These are Docker logical image bytes, not compressed registry sizes. Old values
are the retained local `:phase345` tags; final values are `:phase123`. Tiny
metadata differences are not claimed as optimization wins. All final image IDs,
local digests and scan/image matches are in ignored
`.infra-state/phase123/final-evidence.json`.

| Image | Retained old bytes | Final bytes |
| --- | ---: | ---: |
| Auth | 55,040,804 | 55,040,826 |
| User | 64,632,126 | 64,632,148 |
| Course | 54,700,570 | 54,700,592 |
| Enrollment | 54,536,387 | 54,536,409 |
| Assessment | 57,046,137 | 57,046,159 |
| Analytics | 54,550,486 | 54,550,508 |
| Gateway | 94,614,528 | 94,614,550 |
| Frontend | 97,618,906 | 97,618,925 |
| Storage setup helper | 55,840,357 | 57,334,921 |
| New derived PostgreSQL | Not previously built | 437,697,383 |

The helper increase provides CA trust and a non-root runtime. The supporting
PostgreSQL image retains its vendor database tools; it is not a Go application
image and is not described as a minimal/distroless runtime.

## Complete repository change inventory

The five pre-existing untracked Phase 0 documents are **not changes made by this
implementation**. No file was removed. Ignored local evidence, generated secret
inputs, external backups, image layers and retained new Docker volumes are not
Git deliverables.

### New implementation files

| Path | Purpose |
| --- | --- |
| `infrastructure/docker/compose.postgres.yaml` | Guarded, private one-cluster database layer |
| `infrastructure/docker/compose.target.yaml` | Explicit inactive consolidated application target |
| `infrastructure/docker/postgres-rehearsal.ps1` | Windows-compatible guarded command wrapper |
| `infrastructure/environments/dev/postgres.env.example` | DEV target input template |
| `infrastructure/environments/qa/postgres.env.example` | QA target input template |
| `infrastructure/environments/prod/postgres.env.example` | PROD-like target input template |
| `infrastructure/postgres/.dockerignore` | Narrow supporting-image build context |
| `infrastructure/postgres/Dockerfile` | Same-major PostgreSQL security candidate |
| `infrastructure/postgres/init/10-platform.sh` | Fail-closed fresh initialization marker |
| `infrastructure/postgres/finalize-permissions.sh` | Per-service post-migration grant orchestration |
| `infrastructure/postgres/permissions.sql` | Business-table runtime grants, protected history |
| `infrastructure/scripts/postgres_rehearsal.py` | Provision/restore/SQL checks/recreate/backup of only new rehearsal |
| `infrastructure/scripts/prepare_postgres_target.py` | Exclusive non-deploying independent target input preparation |
| `infrastructure/scripts/rehearsal_tests.py` | Disposable DB-backed Go test fixtures with retained failures |
| `infrastructure/scripts/validate_runtime.py` | New-image artifact, edge, API, health and graceful-stop validation |
| `tests/infrastructure/test_postgres_layout.py` | Seven meaningful isolation/credential/readiness guard tests |

### Modified implementation files

| Path | Purpose |
| --- | --- |
| `.github/workflows/ci.yml` | Nonpublishing CI, test diagnostics, eight models, ten build/scan jobs |
| `.github/workflows/publish-images.yml` | Explicit manual fail-safe publishing hold |
| `.github/workflows/prepare-deployment.yml` | Explicit manual fail-safe preparation hold |
| `.gitignore` | Exclude real target env/runtime/secret material |
| `frontend/Dockerfile` | Pin Node build image; no application changes |
| `services/security-and-authentication/Dockerfile` | Pin Go builder/Alpine runtime |
| `services/user/Dockerfile` | Pin Go builder/Alpine runtime |
| `services/course/Dockerfile` | Pin Go builder/Alpine runtime |
| `services/enrollment/Dockerfile` | Pin Go builder/Alpine runtime |
| `services/assessment/Dockerfile` | Pin Go builder/Alpine runtime |
| `services/analytics/Dockerfile` | Pin Go builder/Alpine runtime |
| `services/assessment/MinIOInit.Dockerfile` | Pin runtime, CA trust and non-root helper |
| `infrastructure/docker/compose.qa.yaml` | Preserve edge security options with explicit merge replacement |
| `infrastructure/docker/compose.prod.yaml` | Same CI compatibility fix |
| `infrastructure/scripts/ci_models.py` | Fresh fixtures, resolved isolation/path validation and safe diagnostics |
| `infrastructure/scripts/scan_images.py` | Separate evidence directories, tool-error distinction and safe per-image retry |

### Documentation files

| Path | Change |
| --- | --- |
| `docs/infrastructure/architecture.md` | Current-state/target safety section preceding historical content |
| `docs/infrastructure/deployment-guide.md` | Guarded rehearsal commands and explicit legacy/target selection |
| `docs/infrastructure/phase-1-implementation-report.md` | New database implementation evidence and cutover gates |
| `docs/infrastructure/phase-2-implementation-report.md` | New image inventory, scans and runtime results |
| `docs/infrastructure/phase-3-implementation-report.md` | New CI root-cause, changes and execution limits |
| `docs/infrastructure/phase-1-2-3-integration-report.md` | This integration/status/file inventory |
| `docs/infrastructure/updated-infrastructure-validation-ledger.md` | New evidence plus verbatim historical ledger snapshot |

Original Compose base/DEV, all six Go source/module files and business migrations,
Nginx implementation, frontend features, legacy secret files and Ansible roles
were preserved. Only documentation modifies the two living architecture/guide
files; historical reports and Phase 0 documents are hash-verified unchanged.

## Remaining blockers and owner actions

1. **Application compatibility:** reconcile Auth `internal/model.go` permission
   allowlist with 23 migrated permissions and the failing `internal/service_test.go:504`
   assertion. Application-team change is outside this infrastructure task. Repeat
   the isolated real-DB suite after its reviewed fix; do not weaken the test.
2. **Object storage:** retain the raw HIGH CVE-2026-39414 result; owner review of
   vendor backport/source-to-binary evidence is required before acceptance or
   choosing a compatibility-tested replacement. No suppression was installed.
3. **Legacy PostgreSQL:** old stopped clusters retain bootstrap privileges and
   vulnerable image layers. The new candidate's clean scan does not fix them
   automatically. Cutover needs separate authorization, current verified backups,
   live role/ownership/extension inventory, write fencing and rollback planning.
4. **CI:** review changes, then authorize a future commit/push and verify real
   GitHub-hosted jobs. Configure Actions/required checks administratively where
   needed; no repository settings were changed here. GHCR previously returned
   403 and remains outside this nonpublishing phase.
5. **Frontend:** API-origin/runtime contract, missing SVG assets, lint/unit tests
   and browser/end-to-end API integration remain frontend-team responsibilities.
6. **Operations:** WSL Docker socket remains unavailable; no Desktop/WSL/firewall
   reconfiguration was made. Windows Desktop is the verified path. Shared local
   8 GiB resources require sequential rehearsals and do not provide HA/DR.
7. **Backup/recovery:** source QA SQL restore and new SQL backup integrity passed.
   Complete object-store/secret/config recovery and destructive disaster-recovery
   drills were not executed. Keep external backup retention/escrow under owner
   policy; never delete old volumes as rollback preparation.

No cloud servers, provider credentials, production TLS/SSH changes or remote
deployment exist. Future release manifests, GHCR publication, RC/final approvals
and exact-digest promotion remain unimplemented/held for their own phase.

## Final safety confirmation

No commits, tags, pushes, GHCR publication or remote workflow triggers. No legacy
volume deletion/recreation or legacy PostgreSQL data modification. No DEV,
existing QA or PROD-like cutover. None of the original 66 containers was started
or recreated; all retain original IDs/start timestamps and remain stopped.
All 23 original volumes remain. Only four new project-prefixed volumes were
added: PostgreSQL data, object data and two Go build/module caches. No prune,
`down -v`, legacy reset or database permission change was performed.

Only the authorized new QA rehearsal application/database resources were
started. Disposable scanner/lint/Go/controller validation containers also ran
as required tooling; they were removed normally without removing data volumes.
All containers are stopped at final verification. Ten preserved historical/audit
documents match their baseline hashes. No discovered safety violation occurred.

## Exact next action

Hand the reproducible Auth permission-catalog failure to the application owner,
then rerun the isolated database test suite after its reviewed fix. In parallel,
obtain the object-store advisory disposition and review this uncommitted diff.
The CI change can proceed to a separately authorized PR/remote validation, but
**Phase 4 release publishing/promotion and all environment cutovers should wait
for these acceptance gates**. Phase 4 was not started automatically.


## 2026-11-24 shutdown recovery and fresh revalidation

The shutdown recovery found the application remediation, new reports and raw
2026-11-23 evidence already present as uncommitted work. All were preserved.
The earlier summary claimed phase/ledger addenda had been appended, but inspection
found them absent; this dated addendum completes that unfinished documentation.
Earlier failure statements above remain historical, not the current verdict.

Fresh evidence root: `.infra-state/remediation/recovery-20261124T020340Z`. Exact process arguments and exit codes are in
`results.json`; sanitized initial resource identities/file hashes are in
`initial.json`; final baseline preservation is in `final-safety.json`.

| Phase | Current acceptance | Remaining condition |
| --- | --- | --- |
| 1 | **COMPLETED AND VERIFIED** | Accepts isolated target/rehearsal only; legacy cutovers remain unauthorized. |
| 2 | **IMPLEMENTED — OWNER APPROVAL PENDING** | Owner must explicitly approve the exact-digest, guard-mandatory local-only Silo exception, or select a separately evaluated vendor option. |
| 3 | **IMPLEMENTED — REMOTE VERIFICATION PENDING** | GitHub-hosted execution has not been authorized or triggered. |

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

The exact PostgreSQL candidate remains
`ucm-postgres@sha256:cb90d02d2e03c00889b7d8c08e1bf522ef62891b121c02d7b0a4239170661a4c`.
Fresh SQL returned server 18.7. All six role/ownership, runtime DML, denied DDL,
administration, migration-history mutation and cross-database checks passed.
All six verified backup archives were restored into new disposable databases;
content/ownership checks passed and only those fixtures were removed. Recreating
the isolated PostgreSQL container reused the retained rehearsal volume and
preserved all six data fingerprints; privilege checks passed again. Fresh fixture
migrations passed for all six services. The earlier extension inventory and two
no-change migration repetitions remain 2026-11-23 evidence, not newly repeated
claims. No legacy database was started, upgraded or modified.

The original and retained image is
`pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329`.
Fresh metadata confirms PGSTY Silo, revision
`44e848fedf8cec3d8c0da4b1aa8422390a2b7a8b`, AGPL-3.0-or-later, based on UBI Micro;
it is not AIStor. Official upstream/GitHub database, Silo backport advisory and
AIStor license-installation references were rechecked. No license was accepted.
The retained mitigation denies Select POST query requests before Silo. Only its
paired guard listens on the private network; Silo backend/console bind loopback.
No IAM grant was broadened. Fresh SDK tests passed normal object operations,
multipart, metadata, listing, presigned GET, anonymous/wrong-credential/cross-bucket
denials, 54 guarded Select probes and unreachable backend/console listener checks.
Two disposable objects were backed up/restored with matching bytes and metadata
between the two dedicated storage rehearsals. Timestamped raw SDK and backup
evidence remains under `.infra-state/remediation/storage/results`.
The application runtime separately verified the guarded application access path.
This remains a mitigation with a raw image finding, not a claim of a scanner-clean
or independently reproducible patched binary. Explicit owner exception approval
is still absent. Legacy stores were not changed or mitigated in place.

Fresh Trivy 0.76.2 scans on 2026-11-24 retained strict HIGH/CRITICAL gates:
PostgreSQL candidate 0 HIGH / 0 CRITICAL, legacy PostgreSQL 26 HIGH / 1 CRITICAL,
and Silo 1 HIGH / 0 CRITICAL (CVE-2026-39414). Image IDs matched exact inspected
digests. Evidence: `.infra-state/remediation/security/recovery-ghcr-20261124/summary.json`
and three raw JSON reports. The first attempt downloaded the updated vulnerability
database but exceeded its scan deadline before image inspection completed; its
failure log remains in `security/recovery-20261124/postgres-candidate.log`.
The successful retry used `.infra-state/recovery_scan.py --output-dir
.infra-state/remediation/security/recovery-ghcr-20261124`, which delegates to the
existing scanner with `TRIVY_DB_REPOSITORY=ghcr.io/aquasec/trivy-db:2`.
No vulnerability suppression, severity change or false clean result was used.

Final safety: all 66 legacy container IDs/start timestamps are unchanged and
stopped. All 23 original volumes and the additional baseline rehearsal/cache
volumes remain present (27 baseline names checked). Only authorized isolated
rehearsals/tool containers ran; all rehearsal runtimes were stopped afterward.
No commit, tag, push, GHCR publication, remote workflow trigger, legacy volume
deletion/recreation, legacy database write, DEV/QA/PROD-like cutover or Phase 4
work occurred. No secret was disclosed. Existing application/migration/Compose
changes were preserved. No application, SQL migration, Compose or policy file
was further changed during recovery; nine-image builds/scans from 2026-11-23
remain historical and were not newly repeated. Runtime execution was repeated.

Evidence-retention limitation from the earlier session remains disclosed: its
first Auth rerun overwrote `.infra-state/phase123/tests/auth.log`. The old failure
survives in reports/transcript; that raw log cannot be claimed byte-preserved.
This recovery did not overwrite prior validation evidence. Phase 0 is unchanged.


## 2026-11-24 owner approval — local storage exception

Approval recorded at 2026-11-24 02:22 UTC from the project owner's explicit
conversation instruction: "Approve the local-only exception with the guard required".
This is owner approval, not an exception self-approved by Codex.

Approved image:
`pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329`.

The exception accepts the residual HIGH CVE-2026-39414 scanner finding for
isolated local rehearsal use only, with the tested Select guard mandatory.
Silo's backend and console must remain bound to loopback, with no network-facing
unguarded alternate endpoint. Existing IAM restrictions remain required. The
image is not declared scanner-clean; raw findings and strict scan exits remain
unchanged. A different digest or removal/bypass of the guard is outside this
approval and requires a new security assessment and owner decision.

Acceptance relies on the already executed 2026-11-24 validation: all six backend
suites and named database-backed Authentication test passed; PostgreSQL candidate
scan, privileges, restores and persistence passed; normal S3/multipart/presigned/
metadata/IAM and object recovery tests passed; 54 Select probes were denied and
backend/console bypass ports were unreachable; full runtime, ten Compose models,
24 infrastructure tests, actionlint, secret scans and whitespace checks passed.
Evidence: `.infra-state/remediation/recovery-20261124T020340Z/` and
`.infra-state/remediation/security/recovery-ghcr-20261124/summary.json`.
No runtime test or scan was repeated merely to record this documentation approval.

| Phase | Current acceptance after owner approval |
| --- | --- |
| Phase 1 | **COMPLETED AND VERIFIED** — isolated target/rehearsal scope |
| Phase 2 | **COMPLETED AND VERIFIED** — isolated local scope with this owner-approved, guard-mandatory storage exception |
| Phase 3 | **IMPLEMENTED — REMOTE VERIFICATION PENDING** |

The earlier owner-pending statements are retained as historical evidence and
superseded by this dated approval for the scope above. This approval does not
authorize DEV, existing QA or PROD-like cutover, legacy component replacement,
cloud/production use, commits, tags, pushes, GHCR publishing, remote workflow
execution or Phase 4. Phase 3 remote verification still needs separate
authorization; Phase 4 remains deferred. Legacy storage remains unchanged.

This approval-recording operation changes documentation only. Previous report
bytes, application/configuration changes and validation evidence are preserved.
No Docker resource or database was started or modified.
