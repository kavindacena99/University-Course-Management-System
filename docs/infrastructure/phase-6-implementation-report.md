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
