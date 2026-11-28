# Phase 5 — semantic versioning and release candidates

## 2026-11-28 implementation state

No earlier semantic Git tags or GitHub releases existed at the start of this
phase. `v0.1.0-rc.1` is the first pre-1.0 candidate; no stable release history
is implied. The [policy](release-management.md) defines SemVer increments,
candidate advancement, immutable digest use, QA acceptance, stable approval,
notes, retention, and rollback references. The
`infrastructure/registry/release-manifest.schema.json` schema and
`infrastructure/scripts/release_pipeline.py` validate release/channel/RC
consistency, publication checksum, exact nine-image equality to the real
Phase 4 manifest, source commit, workflow identity, and companion `images.env`.

The candidate record at `release-manifests/v0.1.0-rc.1/` imports the downloaded
successful Phase 4 manifest at source commit
`e83d2d131136624024175d16d2f41e1501d0b385`. QA status is `pending`.
No GHCR semantic aliases are needed: the existing SHA tags resolve to the
recorded registry digests, and deployments use digest references. The
candidate is not a stable release and does not claim runtime approval.

Five focused release contract tests passed locally, including RC number drift,
stable-before-QA rejection, source/image mismatch rejection, and overwrite
refusal. A real RC Git tag, prerelease, downloadable assets, and QA/stable
transition are recorded in a later execution section only after their GitHub
operations complete. Until then, **Phase 5 is in progress**.
