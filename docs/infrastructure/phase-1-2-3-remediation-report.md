# Phase 1–3 remediation and acceptance closure

Date: 2026-11-23. Branch/HEAD remain `main` /
`a2df85f2bd655ccbe155ce781a2c95b9d8b41f2d`. No commit or push.

## Executive summary and current acceptance

| Phase | Current verdict | Evidence / outstanding condition |
| --- | --- | --- |
| Phase 1 — PostgreSQL and isolation | **COMPLETED AND VERIFIED** | Six databases, separate identities, SQL privilege tests, all six backend suites including Auth, fresh restores and persistence passed. This accepts the isolated target design/rehearsal, not a legacy cutover. |
| Phase 2 — runtime images | **IMPLEMENTED — OWNER APPROVAL PENDING** | Nine affected/local application images rebuilt/scanned, existing PostgreSQL candidate freshly scanned clean, full runtime and S3 compatibility passed. Silo retains one raw HIGH; tested Select mitigation requires explicit local-only exception approval. |
| Phase 3 — CI | **IMPLEMENTED — REMOTE VERIFICATION PENDING** | Local workflow lint, ten Compose models and 24 infrastructure tests passed. No new GitHub-hosted execution, publication or deployment. |

Detailed reports: [Authentication](remediation-authentication-permissions.md),
[object storage](remediation-object-storage-security.md),
[PostgreSQL](remediation-postgresql-security.md).
Previous reports/failures remain historical evidence, with dated addenda rather
than replaced claims. The Phase 0 audit remains unchanged.

## Resolution and limits

Authentication now recognizes all 23 legitimate seeded permissions. The missing
twelve came from later Assessment/Analytics migrations. ADMIN's actual default
19-permission subset was preserved instead of granting everything to satisfy a
count assertion. Exact catalogue, role sets, duplicate/invalid rejection, audit
details, middleware 401/403/204 and cross-service catalogue consistency are tested.
No schema migration, route authorization weakening or API field change.

Storage remains the exact PGSTY Silo vendor digest documented in the storage
report. Vendor binary/OCI revisions and source fix ancestry agree, but the raw
upstream module finding is not silently reclassified as clean. A credential-
independent Select guard is now mandatory in the **inactive consolidated target**
and two new synthetic storage rehearsals. The backend/console listen only on
loopback; the guard alone exposes the private S3 listener. Normal SDK, multipart,
metadata, presigned GET, IAM denial and logical object backup/restore tests passed.
All 54 safe Select variants were blocked. No storage product, format or license
migration was performed. The owner has not approved the remaining exception.

PostgreSQL candidate remains 18.7 and freshly scans 0 HIGH/CRITICAL. All six
logical backups were restored into fresh disposable DBs, with matching content
fingerprints and ownership. Recreating the retained new cluster preserved its
data and privileges; two migration repetitions preserved all data. Legacy
PostgreSQL remains stopped and retains 26 HIGH/one CRITICAL in fresh raw evidence.

The full remediated application rehearsal passed with **11 healthy runtime
containers and eight successful setup jobs** (19 components). The extra runtime
is the private supporting Select guard, reusing the Nginx image, not a seventh
domain service. Six APIs remained reachable through the unchanged API Gateway;
frontend HTML/JS and three Nginx syntax checks passed. All new runtimes then
stopped cleanly. A 401 routing probe is not authenticated end-to-end API coverage.

CI retains GitHub-hosted runners/read-only permissions and held publishing/
preparation workflows. Fresh-checkout model validation now also covers both
storage rehearsal models and target loopback/guard contracts. The historical
Compose5.1.3 duplicate-security-options fix still passes ten models on that
version and Linux5.7.1; Desktop5.7.0 model validation also passed. No local PC
runner, WSL reconfiguration, cloud server or Phase 4 mechanism was introduced.

## Genuine execution results

All Python commands below use `.infra-venv/Scripts/python.exe -B` at repo root.
Go checks use the pinned Go1.27.3 image; scanners retain their nonzero raw results.

| Area | Command / exact executed orchestration | Expected | Actual | Status | Evidence |
| --- | --- | --- | --- | --- | --- |
| Named Auth regression | `infrastructure/scripts/rehearsal_tests.py --only auth --test-run '^TestServicePermissionsAndAudit$' --evidence-dir .infra-state/remediation/auth-targeted` | Exact catalogue/defaults/audit pass | Named test passed0.21s | PASSED | `auth-targeted/auth.log` |
| All six backends | `infrastructure/scripts/rehearsal_tests.py --evidence-dir .infra-state/remediation/backend-tests` | Fresh migrations, available DB tests, modules/vet/build | All six passed; retained data unchanged | PASSED | Six separate backend logs |
| Supporting security | `infrastructure/scripts/scan_supporting_images.py --output-dir .infra-state/remediation/security/supporting` | Honest raw findings, exact image correlation | Candidate0/0; legacy26H/1C; Silo1H/0C; aggregate exit1 | RAW VULNERABILITY GATE FAILED as expected | New raw JSON/summary |
| Application image build | Python `pg.run(pg.compose(True)+['build'])`, nine image inputs set to `:remediation-20261123`, parallel limit2 | Build all affected artifacts, no publication | Nine passed, frontend build included | PASSED | Local image IDs/inventory |
| Application scans | `infrastructure/scripts/scan_images.py --tag remediation-20261123 --output-dir .infra-state/remediation/security/application` | Strict HIGH/CRITICAL gate | All nine0/0 | PASSED | Nine new reports/summary |
| S3 behavior/security | `infrastructure/scripts/storage_rehearsal.py init`, `start`, `test`, `stop` | Isolated normal operations and denied Select | Final SDK suite passed3.17s,54 probes denied, synthetic restore verified | PASSED | Protected timestamped storage results/manifests |
| S3 initial assertion | First `storage_rehearsal.py test` | Assumed ListBuckets403 | Failed: vendor returns filtered authorized set | FAILED, investigated/corrected | First timestamped failure log retained |
| SQL restore | `infrastructure/scripts/revalidate_postgres.py --snapshot "$env:LOCALAPPDATA/UCM/backups/qa-rehearsal/20261123T165903Z-8c294f57" --evidence-dir .infra-state/remediation/postgres-restore` | Six new restores/hash/owner checks | Six passed; fixtures removed only | PASSED | `postgres-restore/restore.json` |
| SQL persistence/security | `infrastructure/scripts/postgres_rehearsal.py recreate --evidence-dir .infra-state/remediation/postgres-persistence` | Same new volume survives recreation, SQL boundaries pass | Six databases passed | PASSED | New persistence JSON and SQL output |
| Idempotent migration/server | Python `pg.migrate()` twice; SQL SHOW server_version / pg_extension / catalogue/default-role queries; data fingerprint comparison | Same18.7, no custom extension dependency/data change | 18.7/plpgsql1.0;23 permissions;19/8/4/2 grants; both repeats passed | PASSED | `postgres-final.json` |
| Initial runtime probe | `validate_runtime.py --tag remediation-20261123 --evidence-dir .infra-state/remediation/runtime` | Verify403+marker | Wget403 observed but marker omitted by tool; assertion failed | FAILED, diagnostic corrected | Initial inventory and recorded response diagnostics |
| Final full runtime | `infrastructure/scripts/validate_runtime.py --tag remediation-20261123 --evidence-dir .infra-state/remediation/runtime-final` | Health, routes/assets, nginx syntax, guard, shutdown | 11 healthy/eight jobs; all checks/stop passed | PASSED | New runtime image inventory and actual tool output |
| Infrastructure safety | `-m unittest discover -s tests/infrastructure -v` | Isolation, backups, catalogue and guard boundaries | 24 passed | PASSED | Local output |
| Compose | `infrastructure/scripts/ci_models.py`; Linux controller with Compose5.1.3 plugin then built-in5.7.1, same command | All active/target/rehearsal models resolve safely | Ten models passed both Linux CLIs; Desktop checks passed | PASSED | Local output, no Docker socket in CLI controller |
| Workflow lint | `docker run --rm --network none --mount "type=bind,source=$PWD,target=/repo,readonly" --workdir /repo rhysd/actionlint:1.7.12 -color .github/workflows/ci.yml .github/workflows/publish-images.yml .github/workflows/prepare-deployment.yml` | No workflow syntax/expression errors | Exit0 | PASSED | Local output |

Final secret scanning, whitespace, baseline/hash/resource preservation results
are appended to the updated ledger after execution. Remote GitHub execution is
**not executed**; it cannot be inferred from this table. Complete authenticated
cross-service/frontend E2E, cloud security and full environment DR remain outside
this acceptance scope. WSL Docker integration remains unavailable/unmodified.

## Remediation-only changed-file inventory

The initial working tree already contained the previous Phase 1–3 implementation.
Those existing changes were preserved. The following lists changes **in this
remediation**, not every older uncommitted Dockerfile/workflow change in Git status.
No file was removed.

| Category | Path | Reason |
| --- | --- | --- |
| Modified application | `services/security-and-authentication/internal/model.go` | Add twelve legitimate catalogue entries |
| Modified test | `services/security-and-authentication/internal/service_test.go` | Exact catalogue/ADMIN/audit checks; restore original role fixture |
| New test | `services/security-and-authentication/internal/permissions_test.go` | Catalogue/duplicate/default-role/middleware regressions |
| New test | `services/assessment/internal/storage_rehearsal_test.go` | Real SDK, IAM, Select and synthetic object recovery |
| New test | `tests/infrastructure/test_permission_contract.py` | SQL/domain/six-service drift prevention |
| New test | `tests/infrastructure/test_storage_guard.py` | Private listener/namespace/port guards |
| Modified Compose | `infrastructure/docker/compose.target.yaml` | Inactive target guarded loopback S3 listener, bounded store and setup dependency |
| New Compose | `infrastructure/docker/compose.storage-guard.yaml` | Reusable least-privilege paired Nginx guard |
| New Compose | `infrastructure/docker/compose.storage-rehearsal.yaml` | Independently isolated synthetic stores and setup |
| New security policy | `infrastructure/object-storage/select-guard.conf` | Select POST/encoded-key denial, safe streaming, no signed-query logs |
| Modified tooling | `infrastructure/scripts/ci_models.py` | Validate ten models and mandatory target storage boundary |
| Modified tooling | `infrastructure/scripts/postgres_rehearsal.py` | Exclusive new restore/persistence evidence, no historical record overwrite |
| Modified tooling | `infrastructure/scripts/prepare_postgres_target.py` | Select local application audit tag independently of PG pin |
| Modified tooling | `infrastructure/scripts/rehearsal_tests.py` | Include new untracked test source safely, isolated evidence dirs, named-test filter |
| Modified tooling | `infrastructure/scripts/validate_runtime.py` | Selected local tag/evidence,19-component guard/HTTP checks |
| New tooling | `infrastructure/scripts/storage_rehearsal.py` | Guarded new-project setup, SDK test and stop; explicit synthetic env allowlist |
| New tooling | `infrastructure/scripts/scan_supporting_images.py` | Fresh exact-image scans with retained failing raw gates |
| New tooling | `infrastructure/scripts/revalidate_postgres.py` | Non-overwriting six-database restore/ownership/hash drill |
| Modified CI | `.github/workflows/ci.yml` | Accurate ten-model summary; successful existing CI capabilities retained |
| New documentation | `docs/infrastructure/remediation-authentication-permissions.md` | Complete23-permission audit and fix evidence |
| New documentation | `docs/infrastructure/remediation-object-storage-security.md` | Provenance/options/mitigation/raw finding and approval boundary |
| New documentation | `docs/infrastructure/remediation-postgresql-security.md` | Candidate/legacy security, restore/privilege evidence |
| New documentation | `docs/infrastructure/phase-1-2-3-remediation-report.md` | This acceptance, command and file inventory |
| Appended documentation | `docs/infrastructure/phase-1-implementation-report.md` | Dated Phase1 acceptance addendum |
| Appended documentation | `docs/infrastructure/phase-2-implementation-report.md` | Dated owner-pending storage/runtime addendum |
| Appended documentation | `docs/infrastructure/phase-3-implementation-report.md` | Dated local validation/remote-pending addendum |
| Appended documentation | `docs/infrastructure/phase-1-2-3-integration-report.md` | Current verdicts without deleting prior failures |
| Appended documentation | `docs/infrastructure/updated-infrastructure-validation-ledger.md` | New dated evidence after preserved historical records |

No SQL migration, module dependency, frontend feature, gateway routing, existing
IAM policy or Dockerfile was changed in this remediation. The old uncommitted
Dockerfile pinning remains intact. Three protected **inactive** target input files
were updated only to tested local image digests; credentials were compared and
preserved. No legacy env file was redirected and no target environment was started.

## Safety and evidence-retention disclosure

No commits/tags/pushes, GHCR publication, remote workflow trigger or Phase4 work.
No legacy volume deletion/recreation, data/role changes or environment cutover.
All66 original container IDs/start timestamps and all23 original volumes are
checked against the baseline; prior new rehearsal volumes are retained too.
Only authorized isolated resources/tooling ran; all are stopped after validation.
Two new object-data volumes are retained for the two synthetic storage projects.

**One evidence-retention restriction was unintentionally violated:** the first
Auth rerun replaced the prior raw `.infra-state/phase123/tests/auth.log` because
the existing helper used a fixed path. The historical failure remains in the
reports/ledger/transcript, but that raw log is not claimed byte-preserved. The
helper was corrected before further backend runs; new logs are separate/exclusive.
Historical Markdown content, Phase0, raw scan reports, restore evidence and
backups otherwise remain retained. No secret value was intentionally output;
final redacted scans/checks verify the completed deliverables.

## Owner decisions and next action

Review the concrete storage report and approve/reject the exact-digest,
guard-mandatory **local-only** security exception. Until explicit approval,
Phase2 acceptance stays pending and Phases1–3 are not all fully accepted.
Alternatively authorize an appropriate licensed vendor/replacement evaluation;
do not remove the guard or suppress the finding as a shortcut.

Separately authorize a future reviewed commit/push and GitHub-hosted CI run when
desired. No authorization was inferred here. Frontend API integration/browser
QA and future release/promotion design remain separate. **Phase4 was not started
and should wait for the remaining acceptance/remote-verification decisions.**


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

| Test area | Executed command | Expected | Actual | Status | Evidence |
| --- | --- | --- | --- | --- | --- |
| postgres-start | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py start` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-start.log` |
| auth-targeted | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/rehearsal_tests.py --only auth --test-run ^TestServicePermissionsAndAudit$ --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\auth-targeted` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/auth-targeted.log` |
| backend-tests | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/rehearsal_tests.py --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\backend-tests` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/backend-tests.log` |
| postgres-privileges | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py verify` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-privileges.log` |
| postgres-restore | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/revalidate_postgres.py --snapshot %LOCALAPPDATA%/UCM\backups\qa-rehearsal\20261123T165903Z-8c294f57 --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\postgres-restore` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-restore.log` |
| postgres-persistence | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py recreate --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\postgres-persistence` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-persistence.log` |
| storage-start | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py start` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-start.log` |
| storage-test | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py test` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-test.log` |
| storage-stop | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py stop` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-stop.log` |
| runtime | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/validate_runtime.py --tag remediation-20261123 --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\runtime` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/runtime.log` |
| infrastructure-tests | `.infra-venv\Scripts\python.exe -B -m unittest discover -s tests/infrastructure -v` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/infrastructure-tests.log` |
| compose-models | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/ci_models.py` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/compose-models.log` |
| actionlint | `docker run --rm --network none --mount type=bind,source=C:\Users\user\OneDrive\Desktop\ESD\project\University-Course-Management-System,target=/repo,readonly --workdir /repo rhysd/actionlint:1.7.12 -color .github/workflows/ci.yml .github/workflows/publish-images.yml .github/workflows/prepare-deployment.yml` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/actionlint.log` |
| secret-scan | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/scan_secrets.py` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/secret-scan.log` |
| diff-check | `git diff --check` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/diff-check.log` |
| storage-final-stop | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py stop` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-final-stop.log` |
| postgres-final-stop | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py stop` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-final-stop.log` |

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

The next decision is owner approval/rejection of the concrete storage exception. Phase 4 remains deferred until acceptance and remote verification decisions are resolved.


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
