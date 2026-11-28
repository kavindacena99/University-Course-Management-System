# Object-storage security remediation — CVE-2026-39414

Date: 2026-11-23. Status at implementation: **IMPLEMENTED — OWNER APPROVAL PENDING**.
The local mitigation and compatibility tests passed. No security exception has
been approved by Codex, and no claim is made that the raw image scan is clean.

On 2026-11-28 the owner approved Phase 4 application image publication only,
for the exact pinned Silo digest with the target guard mandatory. This does not
approve runtime deployment or erase the raw scanner finding. See
`infrastructure/registry/publication-policy.json` and
`docs/infrastructure/object-storage-guide.md` for the current scope.

## Exact implementation and provenance

Original and retained vendor image:
`pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329`.
ImageID and RepoDigest match that sha256. OCI labels identify PGSTY Silo,
`https://github.com/pgsty/silo`, revision
`44e848fedf8cec3d8c0da4b1aa8422390a2b7a8b`, AGPL-3.0-or-later and UBI9 Micro.
This is a maintained vendor fork, not an AIStor image or an unmodified Community
release. Executing `silo --version` in the new synthetic container independently
reported the same release/commit, Go 1.27.4, MinIO attribution and PGSTY changes.

The [upstream advisory](https://github.com/minio/minio/security/advisories/GHSA-h749-fxx7-pwpg)
identifies unbounded CSV Select record allocation and a patched AIStor release.
The [advisory database](https://github.com/advisories/GHSA-h749-fxx7-pwpg)
still marks the upstream Go module range without a fixed module version. The
[Silo advisory](https://silo.pgsty.com/blog/security/cve-2026-39414/) describes an
April bounded-reader fix and a June SIMD-independent follow-up. Read-only source
comparison confirms the installed revision is ahead of complete fix `fd69c89d0`
by 805 commits, behind by zero. This and matching binary/OCI revisions corroborate
vendor backport claims; they do not constitute a reproducible-build attestation
or permission to erase the scanner finding.

## Options reviewed

| Option | Evidence and decision |
| --- | --- |
| Vendor backported Silo | Current vendor release already claims the fix; retained for storage/API compatibility. Raw module-range finding remains visible; conservative endpoint restriction added. |
| Upstream AIStor patched release | Public manifest of `quay.io/minio/aistor/minio:RELEASE.2025-12-20T04-58-37Z` verified: index `sha256:1149a21d9669990e6340a76426bec8193b19f58391d06a854724c1d49cd617e6`, amd64 `sha256:24367c7d783ee119f26f3e1f67b192f575caa2116ceb1444fce2a38db318be35`. Metadata only: not pulled, run, selected or compatibility-certified. |
| Licensing/support | [Official AIStor container instructions](https://docs.min.io/aistor/installation/container/install/) require license enrollment; a Free tier exists. No project license/support decision or license file was provided. No terms were accepted on the owner's behalf. |
| IAM-only Select denial | The vulnerable API is authorized using object-read access. Removing required GetObject would break downloads; inventing a separate Select IAM action would not establish enforcement. Existing scoped normal-object IAM policy remains intact. |
| Endpoint mitigation | Selected: deny Select POST query requests independently of credentials, with no reachable unguarded network listener. Compatible normal operations tested below. |
| Different storage product | Not introduced: unnecessary format/IAM/backup migration when existing vendor compatibility plus a bounded mitigation is available. |

AIStor migration would require owner license/support authorization and a separate
new-volume object/metadata/IAM/presigned/multipart/backup rehearsal. PostgreSQL is
independent. No storage-format compatibility or legacy upgrade was assumed.

## Usage and enforced boundary

`services/assessment/internal/storage.go` uses minio-go v7 for bucket readiness,
PutObject, GetObject/Stat and RemoveObject. Large PutObject requests can use
multipart. No production SelectObjectContent, CSV Select or browser direct-store
integration was found. The new test's Select requests are explicitly negative
policy probes against tiny normal CSV input, not an exhaustion payload.

The existing assessment IAM policy permits GetBucketLocation/ListBucket only on
`submissions` and GetObject/PutObject/DeleteObject only below that bucket. No
policy was broadened. Select is not considered safe merely because application
code does not call it: any holder of these credentials could otherwise attempt it.

New `infrastructure/object-storage/select-guard.conf` uses the existing pinned
Nginx image, not a new application microservice. It blocks POST requests containing
the `select` query key, including single percent-encoded letters, independent of
query order/value or credentials. Other requests retain method, host and URI,
streaming bodies and responses; multipart uploads and presigned GET were tested.
Access logging is disabled to avoid logging signed query URLs.

Silo binds its backend to **127.0.0.1:19000** and console to **127.0.0.1:19001**.
The non-root, read-only, capability-free guard shares only that store's network
namespace and listens on port 9000. Applications and setup jobs retain their
existing `http://minio:9000` endpoint. No database or storage host port is published.
Only the guard owns the network-facing S3 listener; ordinary containers cannot
reach the loopback backend. Console access is not an alternate network path.
The paired guard's health is a setup dependency, so root provisioning also goes
through the guard. Silo and guard must be recreated together when replacing the
store container; a future deployment adapter must preserve that namespace pairing.

The mitigation is implemented only in the separately selected consolidated
target and new synthetic rehearsal configuration. Legacy base/DEV/QA/PROD-like
storage containers/configuration/volumes were not changed. The three inactive
targets have been validated, not deployed.

## Isolated functional and security rehearsal

Two new projects, each with its own `data` network and `object-data` volume:

- `platform-storage-remediation-20261123`
- `platform-storage-restore-remediation-20261123`

Credentials were independently generated in protected ignored input files. Each
project has Silo, its namespace-sharing guard and a successful setup job. Tests
use only the two exact rehearsal DNS names and explicitly opt in; ordinary
assessment test runs skip this integration fixture instead of using app `.env`.

`storage_rehearsal.py test` invokes
`go test -count=1 -v -run '^TestObjectStorageRehearsal$' ./internal` in a pinned
Go tool container attached only to the two new storage networks. Verified:

- Root setup creates `submissions`; the real Assessment `NewMinIOStore`, readiness,
  EnsureBucket, Put/Get/Stat/Delete path works with the scoped application user.
- Multipart upload and download, content type/custom metadata, listing and
  presigned GET work. Payloads are small synthetic data, not exploit material.
- Anonymous reads, wrong credentials, forbidden bucket creation and cross-bucket
  access are denied. Silo's ListBuckets intentionally returns an authorized
  filtered bucket set: the application sees only `submissions`, not the private
  fixture bucket. The final test also checks denied cross-bucket writes/deletes.
- **54** Select probes (nine query variants × root/app/anonymous × two stores)
  return 403 with `X-S3-Select-Guard: denied`. This distinguishes enforcement
  from an unrelated signature/authentication failure. Encoded query keys and
  reordered parameters were included. Unguarded backend/console ports are tested
  from the client network; neither is a published host port.
- Two disposable objects are logically backed up with SHA-256 and metadata,
  restored into the second new store, and read by its independently credentialed
  scoped user with matching content/type/metadata. This is object-level recovery,
  not a physical volume-format upgrade or full IAM/configuration disaster recovery.

The initial test incorrectly expected ListBuckets to return 403. Inspection of
pinned `cmd/bucket-handlers.go` showed intentional authorization filtering.
The corrected assertion requires the exact authorized bucket set and excludes
the known private bucket; the failed attempt remains in its timestamped log.
The main application runtime additionally verified the application's Select-path
403 and the guard response marker. Its first check used BusyBox wget, which omits
headers on errors; the corrected check retains application-path rejection and
independently confirms the header with curl. No authorization assertion was removed.

## Fresh scan and disposition

Trivy **0.76.2**, pinned scanner digest
`8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb`,
fresh scan on 2026-11-23 UTC against the exact vendor image above:

| Field | Result |
| --- | --- |
| CVE / severity | CVE-2026-39414 / HIGH |
| Component | `github.com/minio/minio` |
| Installed module version | `v0.0.0-20261112032852-44e848fedf8c` |
| Fixed version from scanner | None reported |
| Raw scanner exit | **1**, one HIGH / zero CRITICAL |
| Operational disposition | **Mitigated at network-facing API; raw image finding retained** |
| Backport evidence | Vendor advisory, fix ancestry, matching OCI/binary revision; not independently reproduced binary build |

Evidence: `.infra-state/remediation/security/supporting/{object-storage.json,summary.json}`
and protected `storage/results/test-*.log` / `backup-*/manifest.json`. No ignore
file, severity downgrade, allowlist, VEX auto-approval or scanner suppression was
installed. The image was not replaced and is not called scanner-clean.

## Residual risk and owner decision

Docker administrators can execute inside the shared namespace or change startup
configuration, and compromised guard code is inside that trust boundary. These
controls do not remediate unrelated future storage vulnerabilities or certify a
cloud deployment. Legacy stores remain outside this mitigation until authorized
cutover. Keep S3 private and never expose the backend/console listeners.

The owner must explicitly approve or reject a **local-only exception for this
exact image digest with the guard mandatory**. Until recorded, Phase 2 remains
**IMPLEMENTED — OWNER APPROVAL PENDING**. Codex has not accepted that exception.
Rejecting it requires a separately authorized vendor/license or replacement
decision and new-volume compatibility validation, not scanner suppression.


## 2026-11-24 shutdown recovery and fresh revalidation

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
