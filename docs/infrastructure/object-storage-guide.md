# Object storage: operation, setup, and publication scope

## What runs

The Assessment service stores submission files in an S3-compatible object store.
The Compose service is named `minio`, but its server image is the pinned PGSTY
Silo release defined by `OBJECT_STORE_IMAGE`. The server writes objects to the
named `minio-data` Docker volume. Assessment uses the MinIO Go client with a
scoped access key. It checks bucket readiness and uploads, reads, and deletes
objects; its application code does not call S3 Select. Submission metadata and
other service data belong in PostgreSQL, separately from the object volume.

The one-shot `assessment-minio-init` container waits for storage, creates the
private `submissions` bucket if needed, installs the `assessment-submissions`
policy, creates a scoped user, and attaches that policy. Its image is built from
`services/assessment/MinIOInit.Dockerfile`. The build copies the `mc` client
from the pinned Silo image into a small Alpine runtime; the helper is not the
storage server. The helper runs `services/assessment/minio-init.sh` and exits
after successful setup. Assessment starts after that setup completes.

## How the target setup protects the server

The consolidated target in `infrastructure/docker/compose.target.yaml` starts
Silo on `127.0.0.1:19000` inside its network namespace. A separate Nginx
`storage-guard` container shares that namespace and listens on port 9000.
Application containers and the bootstrap helper use `http://minio:9000`, so
requests pass through the guard. The guard returns HTTP 403 for POST requests
with a `select` query key, including single URL-encoded letters, and forwards
ordinary S3 requests to Silo. It has a health check, and the bootstrap helper
depends on that health check. The target publishes no storage port to the host.

The guard is specific to the consolidated target and storage rehearsal Compose
configuration. The legacy base Compose configuration does not add this guard.
Do not treat an unguarded base Compose start as the approved target setup.

## How images are obtained and started

`OBJECT_STORE_IMAGE` in the target environment selects the server image. The
reviewed image is:

```text
pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329
```

Docker pulls that digest-pinned vendor image when an authorized target Compose
stack starts. Docker builds or pulls the separate `assessment-minio-init` image
according to the selected deployment inputs. The Compose configuration passes
the storage root credentials to Silo and the helper, and the scoped user
credentials to the helper and Assessment. Keep real credentials in the protected
environment inputs; the committed `.env.example` files contain placeholders.
The `minio-data` volume persists independently of container replacement.

The prepared target has its own environment inputs and project name. Consult
`docs/infrastructure/deployment-guide.md` before any local start or cutover; its
current target commands are for inspection and rehearsal, not an automatic
deployment. Publishing images in GitHub Actions does not run Compose or install
Silo on a machine.

## Why publication has a gate

The pinned Silo server still has a raw HIGH scanner match for CVE-2026-39414,
which concerns S3 Select memory exhaustion. The vendor reports a fix in this
release; repository review found source and binary revision evidence, but no
reproducible-build attestation. The target's guard blocks the unused Select API
as an additional boundary. The raw finding remains visible; normal object
operations passed the documented rehearsal. See
`docs/infrastructure/remediation-object-storage-security.md` for the evidence
and limits.

The owner approved **Phase 4 application image publication only** on
2026-11-28 for the exact server digest above, with the guard mandatory for the
target runtime. `infrastructure/registry/publication-policy.json` records the
owner and review date. The CI `publication-prerequisites` job checks that policy
on protected `main` before the `publish` job can authenticate to GHCR. CI builds
and scans nine application images, including `assessment-minio-init`, then
publishes those images and verifies their registry digests. It does not publish
the Silo server image or deploy any runtime. QA, PROD-like, and production
deployment need their own authorization and checks.

If the review date expires, the gate closes again until a new review is recorded.
