# GHCR publishing workflow

Date: 2026-11-24. Implementation lives in `.github/workflows/ci.yml` and
`infrastructure/scripts/registry_pipeline.py`. No source commit/push or remote
execution was performed during this phase implementation.

## One build, same-run artifacts, gated publication

Existing backend, frontend and infrastructure checks are retained. The ten-entry
image matrix still validates the supporting PostgreSQL candidate, but only nine
application artifacts can reach publication. Every image is built once with
Buildx into an OCI archive, with OCI source/revision/version/creation/title/
description labels, `linux/amd64`, BuildKit SPDX SBOM and maximal provenance.

Trivy 0.76.2 scans the extracted OCI layout using the unchanged HIGH/CRITICAL
exit-1 policy. Its image ID must match the OCI configuration digest. The artifact
preparer checks runtime architecture, source labels, blob hashes and attestation
subjects; both SPDX and SLSA provenance predicates must exist. Only then is the
image archive, scan JSON and build identity uploaded as `image-<service>`.
Build artifacts retain seven days; scan reports retain fourteen days.

Both `publication-prerequisites` and `publish` depend on successful backend,
frontend, infrastructure and all image jobs. PRs, forks, feature branches,
reusable-workflow events and unprotected refs cannot publish. Only push/manual
dispatch on this repository's protected default branch is eligible. The publisher
also repeats these checks in Python before using credentials. Manual dispatch
has no arbitrary source/artifact input. No workflow_run or cross-run artifact
trust is introduced. Only the publisher receives `packages: write`; all other
jobs retain read-only repository permissions on GitHub-hosted runners.

`publication-policy.json` is deliberately fail-closed: registry access, Phase 3
remote verification and storage release-scope approval remain false. A named
owner and unexpired review date are additionally required. These are reviewed
prerequisites, not switchable workflow-dispatch inputs or scanner suppressions.
Current local-only storage approval is preserved and is not extended by Codex.
Branch protection is independently checked through GitHub's event context, even
if a policy file is changed. The deprecated `publish-images.yml` manual entry
point directs operators to CI and exits unsuccessfully; it never duplicates
builds/publication. Deployment preparation remains on its existing hold.

## Publish and verify

The publish job downloads only artifacts from its own successful run. It installs
Skopeo from the runner's distribution repositories, records no persistent login,
and authenticates both Skopeo and Docker with GITHUB_TOKEN through stdin.
Credentials exist only in runner-temp files, removed by an always-run cleanup.
The publisher verifies all nine bundles and registry tag nonexistence before
uploading the first image. Authentication errors are not treated as missing tags.

Skopeo `copy --all --preserve-digests --digestfile ...` copies the already built
OCI archive, including SBOM/provenance manifests. It performs no build. The
returned digest file is the source of the published reference. The publisher
then resolves that digest, checks the returned manifest bytes, downloads the
complete OCI index, rechecks attestation contents/subjects, pulls the runtime
with Docker, verifies platform and re-scans by registry digest with Trivy.

Publication concurrency is serialized for a commit; default-branch runs are
not automatically canceled by newer pushes. Existing SHA tags are refused.
GHCR has no cross-package transaction here: a failed later upload/scan can leave
partial packages. Per-image publication evidence is retained even on failure,
and no complete manifest is generated. Reruns refuse existing tags and require
operator investigation; they do not delete, overwrite or rebuild them. No
automatic resume or release-promotion mechanism is implemented.

## Manifest and evidence

`infrastructure/registry/image-manifest.schema.json` is JSON Schema 2020-12.
Runtime validation uses pinned jsonschema plus semantic checks: exact nine-image
inventory, source-bound lowercase names/full-SHA tags, non-placeholder digests,
platform, successful build/scan/pull, matching SBOM/provenance references and
workflow identity. Unit-test fixtures use synthetic values only inside tests;
no fabricated successful publication manifest is delivered.

Successful output includes source repository/commit/ref, actual workflow run ID,
attempt/URL, publication timestamp, image/tag/registry digest/runtime digest,
platform, build and both scan results, pull result and attestation references.
The nine-variable `images.env` companion is generated without deploying it.
The successful manifest artifact name includes commit/run/attempt and retains
90 days. This is durable within the configured Actions retention window, not
permanent archival; owners must retain it before expiry for later phases.
Partial digest/scan evidence also retains 90 days.

BuildKit attestations are **unsigned** in-toto statements embedded in the OCI
index. Blob hashes and runtime subjects are checked locally and after pulling;
no GitHub-signed attestation or identity-signature verification is claimed.
No `id-token: write` or `attestations: write` permission is requested. Introducing
GitHub artifact signing would require an explicit supported permission/feature
decision and verification procedure; its absence is visible, not silently
represented as signing success. Local OCI generation and digest-preserving
copy were actually tested; registry preservation remains unverified.

## Owner prerequisites before execution

1. Enable supported protected-default-branch controls. Current main protection
   API returns HTTP 403 with a private-repository plan restriction. No protection
   or visibility was changed.
2. Verify GHCR namespace/repository package access without bypassing the existing
   403. Grant the repository appropriate Actions access to existing packages.
3. Obtain actual successful Phase 3 GitHub-hosted verification. Existing run
   inspection found historical failures, not a successful current remediation run.
4. Resolve the storage release dependency or explicitly approve an appropriate
   release/publication scope, owner and review date. The existing approval covers
   isolated local rehearsal only. Legacy PostgreSQL findings also remain unresolved
   in stopped legacy environments; excluding its publication does not fix them.
5. Review and authorize committing/pushing this implementation, then review policy
   evidence/owner/date updates. This task did not authorize a new source commit.

After those gates are satisfied, dispatch **CI** on protected main or use its
default-branch push trigger. A dispatch of today's held publisher cannot execute
uncommitted local code. No remote run was attempted in this implementation.

References: [Docker attestations](https://docs.docker.com/build/metadata/attestations/),
[Skopeo copy semantics](https://github.com/podman-container-tools/skopeo/blob/main/docs/skopeo-copy.1.md),
[GitHub Actions package authentication](https://docs.github.com/en/packages/managing-github-packages/using-github-packages-with-github-actions).

## 2026-11-27 update: manual entry and current gate

`publish-images.yml` now calls the existing `ci.yml` reusable workflow on the
same commit. It accepts no SHA, artifact-run or branch input. Its job requires
this repository's protected default branch and grants `packages: write` only
as the maximum permission passed to the called workflow. The called workflow
reruns backend, frontend, infrastructure, image-build and scanner jobs; its
`publish` job alone uses package-write permission after the prerequisite job.
Default-branch pushes still use that same pipeline. GitHub's reusable-workflow
context is associated with the caller, and a same-repository relative reference
selects the caller's commit. At this validation checkpoint, the edit had not
yet been committed or pushed.

The current private repository cannot report branch protection or rulesets:
both APIs return HTTP 403 with the GitHub Pro/public-repository requirement.
The policy remains fail-closed for package access, complete remote validation,
and the storage release-scope decision. No setting or package visibility was
changed. The owner must establish an available protected-main policy, confirm
private package access for this repository, and approve the outstanding storage
scope before the policy can be reviewed for activation.

Run 46038430324 executed the committed CI on GitHub-hosted runners. Backend,
frontend and infrastructure passed; nine application image scans failed on new
HIGH findings, so prerequisite and publish jobs skipped. The fixed-package
Dockerfile edits require a reviewed commit/merge before remote
revalidation. No GHCR digest, signed attestation or successful registry pull
exists yet. BuildKit's SPDX/SLSA statements remain unsigned until a separate
verified signing design is enabled. The publication manifest retains 90 days
in Actions, subject to repository retention settings; long-term custody needs
an owner-selected durable store in a later phase.

## 2026-11-27 protected-main and registry bootstrap update

The public repository's active ruleset now protects `main`. Fresh hosted run
[46047552300](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46047552300)
observed `GITHUB_REF_PROTECTED=true`; all backend, frontend, infrastructure and
image jobs passed. The prerequisite stopped at the old registry-access policy,
before a registry login. The policy now records this remote validation while
retaining the unresolved storage decision.

The publisher's explicit `first_verified_application_upload` bootstrap avoids
claiming registry write access from a CLI package-list result. Only its job has
`packages: write`. It validates all same-run image archives and scanner results,
authenticates with the ephemeral `GITHUB_TOKEN`, and checks the full-SHA tags
before mutation. A missing tag may be uploaded only from its scanned OCI
archive. The digest returned by Skopeo must equal the OCI index digest, and
registry resolution, complete index/attestation download, digest pull and
post-publication scan must pass before the first write counts as access proof.
The successful manifest records `registry_access_verified=true`.

For a partial rerun, preflight reads each existing tag's raw registry index.
An identical digest is safely reused and verified again; a different digest,
an unrecognized error or an authentication denial stops the run. The publisher
never overwrites a tag. Retrying failed jobs against the same run can reuse the
earlier same-run OCI artifacts; a newly built archive with different bytes is
rejected even if its source SHA is the same. If all tags already exist, the
publisher verifies all nine against the same-run archives, pulls and rescans
them, then completes the manifest without overwriting them. This proves
registry access and complete publication, though no new write occurs on the
rerun.

The manual `publish-images.yml` workflow remains the preferred final entry
point after its policy changes merge to protected `main`. It reruns all required
checks and accepts no arbitrary SHA or artifact-run input. BuildKit SPDX/SLSA
statements are verified by subject and digest but remain **unsigned**. No
GitHub-signed attestation claim or extra signing key is introduced. The complete
manifest and scan evidence use finite 90-day Actions retention; retain their
artifact URL and arrange durable custody in a later release phase.
