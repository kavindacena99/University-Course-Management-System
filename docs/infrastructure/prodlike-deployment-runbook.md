# Local PROD-like promotion runbook

The prepared project is `platform-prodlike-target`; frontend port 3002 and
gateway port 8082 bind to localhost. Promotion needs a passed QA report for
the exact RC manifest, a stable release manifest with the same nine digests,
and owner runtime approval for the exact Silo digest in PROD-like. QA must be
stopped before PROD-like starts on this constrained Docker Desktop engine.

Use `infrastructure/docker/targetctl.ps1` with `-Environment prodlike` and the
stable release manifest for `config`, `preflight`, and `pull`. Compare its
source SHA and all nine digests with the accepted QA record before any `deploy`.
The controller uses `compose.target.yaml`, the prepared environment input,
and the manifest's `images.env`; it must never rebuild an application image.
Record existing target state and a verified external backup before migrations
if data exists. Do not copy QA or DEV credentials or data into PROD-like.

After guarded deployment, run non-destructive `health`, `validate`, and `smoke`
checks, including the S3 Select denial and image-ID comparison. Preserve the
deployment and rollback records. An image-only rollback requires an actual
previous immutable release, migration compatibility review, and an existing
verified backup; it does not reverse database migrations. Do not remove
containers, networks, volumes, or unknown data to recover.
