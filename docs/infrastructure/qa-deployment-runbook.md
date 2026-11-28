# QA consolidated-target runbook

Run commands from the repository root on the authorized Windows PC. Use the
prepared `platform-qa-target` inputs; do not call `platformctl.py` for this
target. The selected release manifest must already validate against its
retained Phase 4 publication manifest.

```powershell
$release = 'release-manifests/v0.1.0-rc.1/release-manifest.json'
powershell -NoProfile -File infrastructure/docker/targetctl.ps1 -Action config -Environment qa -Release $release
powershell -NoProfile -File infrastructure/docker/targetctl.ps1 -Action preflight -Environment qa -Release $release
powershell -NoProfile -File infrastructure/docker/targetctl.ps1 -Action pull -Environment qa -Release $release
```

These steps do not start containers. Inspect the saved preflight record,
Docker Desktop capacity, candidate PostgreSQL scan, image digests, and existing
target volumes. Before any runtime step, obtain the owner's separate approval
for the exact Silo digest and QA scope, with the Select guard mandatory. The
approval must be recorded in the protected, ignored runtime approval file;
publication policy is not a runtime approval. If existing target data is
present, create and verify an external backup before migrations. Review schema
compatibility and never import legacy or DEV data as a shortcut.

When those gates are satisfied, use the controller's guarded `deploy` action,
then `health`, `validate`, and `smoke`. Record SQL privileges, all six APIs,
normal S3 operations, Select denial, browser flows, image IDs, and response
times using [the QA checklist](qa-validation-checklist.md). A healthy
container alone is insufficient acceptance. Stop QA with the controller's
`stop` action and verify the preserved volumes and DEV identities afterward.

If any required test fails, retain its evidence, keep the RC in prerelease,
and do not promote it. A source fix needs a new published commit and RC number.
