# Phase 2 implementation — runtime images

Date: 2026-11-23. Status: **PARTIALLY COMPLETED** overall: all ten locally built
images, startup/shutdown and runtime gates passed, but supporting object storage
still has an unsuppressed HIGH finding requiring owner/vendor disposition.

## Complete image inventory and changes

| Image / build context | Dockerfile | Review / result |
| --- | --- | --- |
| `ucm-auth` / `services/security-and-authentication` | `Dockerfile` there | Go multi-stage; builder/runtime digest pins |
| `ucm-user` / `services/user` | `Dockerfile` there | Same standard |
| `ucm-course` / `services/course` | `Dockerfile` there | Same standard |
| `ucm-enrollment` / `services/enrollment` | `Dockerfile` there | Same standard |
| `ucm-assessment` / `services/assessment` | `Dockerfile` there | Same standard |
| `ucm-analytics` / `services/analytics` | `Dockerfile` there | Same standard |
| `ucm-frontend` / `frontend` | `frontend/Dockerfile` | Frozen pnpm lockfile, pinned Node builder, existing static Nginx runtime |
| `ucm-gateway` / `infrastructure/nginx` | `infrastructure/nginx/Dockerfile` | Already suitable; built/validated, unchanged |
| `ucm-assessment-minio-init` / `services/assessment` | `MinIOInit.Dockerfile` | Pinned Alpine, CA trust, non-root `app`, `/tmp/mc` configuration |
| `ucm-postgres` / `infrastructure/postgres` | New `Dockerfile` | Same PostgreSQL 18.7, patched libxml2 and rebuilt same upstream gosu |
| Legacy PostgreSQL vendor image | Referenced by original Compose | Remains pinned and unchanged; vulnerable, not cut over |
| Object store / mc source vendor image | `pgsty/silo` pinned in Compose/helper | Unchanged; vendor backport review below |
| Ansible CLI controller | `infrastructure/ansible/Controller.Dockerfile` | Existing Docker CLI/Python/Ansible tool image, syntax checks passed; no application runtime role |
| Trivy / Gitleaks / Go / actionlint | Pinned tool images in scripts/workflow | Disposable validation tools, not application services |

There are six existing Go migration jobs, each running its dedicated
`/app/<service>-migrate` binary with `-path /app/migrations -direction up` from
the corresponding service image; no six extra migration images. User also
retains its required bootstrap-admin binary. The permissions job uses
the same PostgreSQL client image, mounts two scripts, and receives only migration
credentials. Its PostgreSQL image-declared data directory is tmpfs to avoid an
unintended anonymous persistent volume. No additional microservice was introduced.

Six module-required Go 1.27.3 builders are pinned to index digest
`fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18`.
Alpine 3.22 runtime pin:
`49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f`.
Node 22 builder pin:
`1314fb30ec37592d5845e5a160d522c0e680972aa534549940b8d73bde88481a`.
Existing pinned Nginx 1.30.7 is preserved. Runtime package installation still
depends on the distribution repository; digest-recorded built artifacts are the
deployment identity, not a claim of byte-identical future apk resolution.

## Runtime standards and validation

Existing service `.dockerignore` files and multi-stage layouts were adequate and
retained. Builders consume each independent `go.mod`/`go.sum`, download
dependencies, build stripped static binaries and copy only runtime artifacts.
Explicit `go mod verify` also passed in the separate test/CI validation path.
All six run as `app`, contain CA certificates, and exclude source modules,
tests, Markdown development docs, Go compiler, gcc and npm. Actual image
inspection checked these properties. No timezone-specific runtime requirement
was found in the inspected code; no speculative tzdata dependency was added.

The target applies read-only application filesystems, dropped capabilities,
no-new-privileges inherited from the base, bounded tmpfs, PID/memory/CPU limits.
Edges retain only initialization/bind capabilities required by their existing
Nginx master; they are not falsely described as entirely non-root. PostgreSQL
retains official root-entrypoint-to-postgres privilege dropping and a writable
data volume. Object-store security settings remain compatible with its existing
data-write requirements; no blind read-only enforcement. Secret values are not
Docker build arguments and are absent from final source contexts/artifacts.

All nine application/helper images were built with unique local `:phase123`
tags, followed by the derived PostgreSQL image. The helper was rebuilt/scanned
after adding its non-root identity. Final full rehearsal with these images:
**10 healthy long-running containers, eight successful setup jobs**. All six
Go listeners started using runtime database identities. Frontend HTML and its
compiled JavaScript asset returned 200; both Nginx runtimes passed `nginx -t`;
gateway health 200, unknown/internal paths 404, six protected API routes 401 with
security headers. A 401 demonstrates gateway/backend/auth-boundary reachability,
not complete authenticated API or frontend integration coverage. All new
containers then stopped cleanly with exit code 0.

Frontend source/features were untouched. Its existing build succeeds with two
unresolved SVG warnings (`0c1d9.svg`, `5b43a.svg`); no lint/unit-test scripts exist.
The Vite API-origin argument does not by itself provide a browser runtime
configuration contract. Frontend engineers still own API wiring/runtime-origin
agreement and browser/end-to-end QA. No secrets were added to browser variables.

## PostgreSQL security remediation and compatibility

Fresh Trivy 0.76.2 scan of the original pinned PostgreSQL image
`postgres:18-alpine@sha256:0115267ab5a509022df9319c77be365cef335adb71556c75f60f5360ee247e71`
confirmed **26 HIGH and one CRITICAL**, rather than relying on retained evidence.
The affected groups were libxml2 and Go 1.24.6 standard library embedded in
`/usr/local/bin/gosu`. Findings were not excluded or assigned a passing exit.

| Package / finding | Installed | Scanner-reported fix |
| --- | --- | --- |
| libxml2 | `2.13.9-r2` | `2.13.9-r3` |
| gosu's Go stdlib, CRITICAL `CVE-2025-68121` | `v1.24.6` | `1.24.13`, `1.25.7`, or later supported fixed compiler |
| Other HIGH Go stdlib findings | `v1.24.6` | Latest listed fixes include `1.26.10` / `1.27.3`; rebuild used `1.27.3` |

The full per-CVE package/fixed-version records remain in the original scan JSON;
the compiler/runtime-support update was validated by a fresh candidate scan.

The new supporting Dockerfile retains PostgreSQL 18.7 and its data format,
updates libxml2 to at least `2.13.9-r3`, and rebuilds **the same upstream gosu
1.20 source** with Go 1.27.3. Source commit
`29b939aa5bd4dc588af765b8248f6e39150d2565`, downloaded tar SHA-256
`2b70da8a5480fafedc16bb1fc91b572f064d03df736cbd91a616af2f1b66ab51`,
is checked by Docker ADD. Dependency modules are verified. No server-major,
extension, SQL migration or business-schema upgrade was performed.

Fresh derived image scan: **0 HIGH / 0 CRITICAL**. Actual `gosu --version`
reported 1.20 built with Go 1.27.3. Its immutable local reference is
`ucm-postgres@sha256:cb90d02d2e03c00889b7d8c08e1bf522ef62891b121c02d7b0a4239170661a4c`.
The new rehearsal was recreated on this image, retaining all restored table
fingerprints and passing SQL privilege/migration checks. This is a compatibility
rehearsal, not proof of arbitrary extension compatibility or a legacy cutover.
The vulnerable legacy images remain in the stopped existing environments until
an authorized migration. The strict CI image gate now covers this derived image.

## Remaining object-storage finding

Fresh scan of the unchanged
`pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329`
returned **one HIGH, CVE-2026-39414**. The raw scanner result remains failing.
The [vendor advisory](https://silo.pgsty.com/blog/security/cve-2026-39414/)
describes a backport. OCI source revision
`44e848fedf8cec3d8c0da4b1aa8422390a2b7a8b` was checked through the upstream GitHub
compare API: ahead of fix `fd69c89` by 805 commits, behind by zero. This supports
the backport investigation but is not an owner-approved binary VEX disposition.
No broad allowlist, suppressed HIGH result or unverified supporting-image upgrade.
Keep the new store private and defer exposure/approval until the owner reviews
the advisory, source-to-binary provenance and applicable threat model.

All **nine current application/helper** scans passed 0 HIGH / 0 CRITICAL and
their report ImageIDs match final local images. Enrollment's first scanner
execution produced no report; its isolated retry passed. This tool failure
was not counted as a passing scan. A passing vulnerability scan is limited to
the scanner database/time and requested severities; it is not vulnerability-free
certification. Machine-readable raw reports and scan logs are retained under
ignored `.infra-state/phase123/security`.

## Measured image sizes

Docker image-inspect logical bytes, not compressed registry transfer sizes.
Final tags and IDs are retained in `image-inventory.json` / `final-evidence.json`.
The integration report includes measured before/after sizes where old local tags
exist. There is no GHCR publication or release tagging in this work.

| Image | Final bytes |
| --- | ---: |
| Auth | 55,040,826 |
| User | 64,632,148 |
| Course | 54,700,592 |
| Enrollment | 54,536,409 |
| Assessment | 57,046,159 |
| Analytics | 54,550,508 |
| Gateway | 94,614,550 |
| Frontend | 97,618,925 |
| Storage setup helper | 57,334,921 |

Authentication's real database permission-catalog test failure remains an
application issue recorded in the Phase 1 report. No business-code assertion
was weakened to produce an image/runtime pass.


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
