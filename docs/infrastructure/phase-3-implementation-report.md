# Phase 3 implementation — non-publishing GitHub Actions CI

Date: 2026-11-23. Status: **IMPLEMENTED BUT REMOTE VERIFICATION PENDING**.
No commit, push, remote workflow trigger, package publication or deployment was
performed. Local checks are distinguished below from historical GitHub results.

## Workflow inventory and scope

One CI workflow is simpler than splitting five overlapping workflows.
`.github/workflows/ci.yml` retains backend, frontend, infrastructure and image
matrix jobs on GitHub-hosted `ubuntu-latest`. Triggers: pull requests, branch
pushes (including main), reusable workflow calls and manual dispatch. Tag pushes
are excluded: this work does not introduce a release lifecycle. Concurrency
cancels superseded work per workflow/ref. Global permissions are `contents: read`;
checkouts do not persist credentials. Third-party Actions use existing full
commit-SHA pins. No untrusted PR publishing, registry login or local-PC dependency.

The existing `publish-images.yml` and `prepare-deployment.yml` were changed to
explicit manual-only **fail-safe holds** with read-only permissions and an exit-1
explanation. Repairing CI must not accidentally unlock their previous automatic
publication path. They contain no registry credentials, release mutation or
deployment steps. This is a deliberate scope restriction, not a new publishing
or promotion mechanism. Historical workflow implementations remain in Git.
Restoring a reviewed immutable-digest release mechanism requires a future phase.

## Backend and frontend coverage

Six independent Go modules are matrix entries:
`services/{security-and-authentication,user,course,enrollment,assessment,analytics}`.
Each uses its own `go.mod` version through setup-go. Checks are gofmt, module
download/verify, `go mod tidy -diff`, `go test -json ./...`, vet and binary build.
Test pipelines enable pipefail so tee cannot hide a failing suite. Raw JSON is
retained for 14 days; always-run summaries report pass/fail/skip test events.
There is no repository-configured extra Go linter to invent. Image dependency
scanning and history secret scanning remain mandatory gates.

Locally all six formatting/module/vet/build and baseline unit paths passed using
Go 1.27.3 containers. Five real DB fixture suites passed; Auth's additional real
DB suite fails due to the permission catalog mismatch described in Phase 1.
The baseline GitHub CI path does **not** configure these six DB fixtures and
therefore skips DB-dependent tests as the existing test code specifies. The
workflow explicitly reports skipped tests instead of calling this full DB
integration coverage. The optional local `rehearsal_tests.py` preserves its real
failure and never substitutes a skipped run for a successful integration result.

Frontend uses Node 22, Corepack/pnpm 10.34.3, `pnpm install --frozen-lockfile`,
available lint/test scripts, `pnpm build` and production dependency audit. Missing
lint/unit-test scripts are reported, not invented. Production build passed inside
the local Docker build; the new GitHub-hosted frontend job/audit has not executed.
No frontend API integration feature was added.

## Docker build and security gate

Ten matrix entries: six Go services, frontend, gateway, assessment storage helper
and the reviewed derived PostgreSQL image. Every entry has the actual service
build context/Dockerfile, Buildx cache scoped by image, `load: true`, **push:
false**, and a local-only `ucm-<name>:ci` tag. Trivy scans HIGH and CRITICAL,
returns nonzero on findings and retains JSON artifacts for 14 days even on failure.
There is no `continue-on-error`, blanket exclusion or fake passing security gate.
The ten corresponding local images built and scanned successfully. Supporting
object storage is separately recorded as an unresolved raw HIGH, not described
as passing merely because the application matrix is green.

## Historical Compose failures: identified and reproduced

Read-only GitHub API/job-log inspection found the same failure in jobs
`137733297828`, `137732809363`, `137732723203`, corresponding to runs
`45186478945`, `45186338309`, `45186313447` (main/PR/push). The original wrapper
suppressed the CLI diagnostic, leaving an unhelpful RuntimeError at QA setup.

The job runner was Ubuntu 26.04 image `20261116.202.1`; the
[published runner inventory](https://raw.githubusercontent.com/actions/runner-images/ubuntu26/20261116.202/images/ubuntu/Ubuntu2604-Readme.md)
records Docker Compose **5.1.3**. The same original repository/configuration
passed Desktop Compose 5.7.0 and Linux 5.7.1, so testing only the current Desktop
CLI did not reproduce the CI bug.

The exact Linux 5.1.3 CLI was checksum-verified (SHA-256
`a0298760c9772d2c06888fc8703a487c94c3c3b0134adeef830742a2fc7647b4`).
An untouched HEAD archive and fresh synthetic inputs reproduced exit 1:

```text
validating /workspace/infrastructure/docker/compose.qa.yaml:
services.frontend.security_opt items at 0 and 1 are equal
```

Base and override both specified `no-new-privileges:true`; the older Compose
merge retained duplicates and its schema rejected the result. The same problem
affected the gateway and PROD-like overlay. Four lines now use
`security_opt: !override [no-new-privileges:true]`. The protection is preserved;
the failing checks were not disabled. Final **eight models passed** under Linux
5.1.3, Linux 5.7.1 and Desktop 5.7.0.

## Fresh infrastructure validation

`ci_models.py` now creates temporary public fixtures, signing keys and synthetic
non-production secrets; it never reads developer `.env` files or overwrites
existing ignored inputs. It validates both `config --quiet` and resolved JSON for:

1. Legacy base.
2. Legacy DEV.
3. Legacy QA.
4. Legacy PROD-like.
5. Consolidated DEV target.
6. Consolidated QA target.
7. Consolidated PROD-like target.
8. Standalone PostgreSQL rehearsal.

Resolved checks cover project names, public/private ports, scoped networks and
volumes, no external/aliased volumes, one target PostgreSQL/no legacy DB services,
database/role mappings, migration/runtime credential separation, permission-job
readiness, required secret files, bind paths and Dockerfile paths. Only syntax
and model inspection runs; no `up`, migration, data operation or Docker socket
is needed for these checks. Three target inputs also validated separately with
their independently generated real local secrets (without logging values).

Infrastructure CI retains redacted full-history Gitleaks, 18 deployment/backup/
consolidation safety tests, three Ansible inventory graphs and 12 playbook syntax
checks. Existing Ansible deployment remains a legacy topology interface, not an
implicit target cutover mechanism. Syntax checks passed in the Linux controller;
no deployment playbook was run against existing environments. WSL socket access
is still unavailable; CI is independent of WSL and the Windows PC.

## Local verification versus pending execution

| Check | Outcome |
| --- | --- |
| Historical job logs / runner version | Read and exact merge failure reproduced |
| Eight models on 5.1.3 / 5.7.0 / 5.7.1 | PASSED |
| 18 safety tests | PASSED |
| All six Go formatting/unit/module/vet/build paths | PASSED |
| Additional Auth real-DB fixture test | FAILED; retained application blocker |
| Ten image builds/scans, isolated runtime | PASSED; vendor object-store finding separate |
| actionlint 1.7.12, all three workflows | PASSED locally |
| Ansible inventories / 12 syntax checks | PASSED locally |
| New GitHub-hosted workflow execution | NOT EXECUTED; requires later authorized push |
| GHCR package permissions / publication | Deferred; previous 403 is not resolved or needed by this CI |
| Deployment / release promotion | Not implemented in this phase |

Repository owner must review and later authorize a push/PR, ensure GitHub Actions
is enabled and select required CI checks in branch protection. GHCR package
permissions, protected release environments, QA approval and immutable release
manifests belong to the future publishing/release phase. No claim is made that
these settings were changed. The three old failure causes are fixed locally;
their historical failed GitHub runs remain failures.


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

Fresh actionlint, ten Compose models and 24 infrastructure tests passed. Local validation does not establish remote execution.
