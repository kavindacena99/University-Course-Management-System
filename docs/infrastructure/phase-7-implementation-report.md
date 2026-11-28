# Phase 7 — local PROD-like promotion

## Preparation and gate

The prepared target is `platform-prodlike-target` with frontend port 3002 and
gateway port 8082. Its generated inputs exist independently of QA and DEV.
`targetctl.py config` validated its consolidated target model with the same
nine RC digest references, reviewed PostgreSQL candidate, exact pinned Silo
image, and mandatory guard. Read-only preflight found zero existing
consolidated PROD-like containers and zero target volumes; its record is in
`.infra-state/releases/v0.1.0-rc.1/prodlike-preflight.json`.

PROD-like promotion requires a successfully QA-validated RC and a stable tag
and manifest with identical digests. On 2026-11-28 the owner separately
approved the exact Silo image for isolated local QA and PROD-like runtime
rehearsal with the Select guard mandatory; the ignored local approval record
retains the scope. No PROD-like target runtime has been started, and no stable
release or promotion evidence exists yet. **Phase 7 is not complete.**
