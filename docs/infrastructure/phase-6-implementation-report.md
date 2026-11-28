# Phase 6 — consolidated QA target

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
