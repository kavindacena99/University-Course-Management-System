# Phase 4 implementation — GHCR publishing

Date: 2026-11-24. Source baseline: `main` at `69947a6` (merged PR #13).
Verdict: **PARTIALLY COMPLETED — BLOCKED BY PREREQUISITES**.
The pipeline is implemented and locally validated. No image was published.

## 1. Executive summary

Extended the existing CI with same-run, build-once OCI artifacts and a protected
default-branch publication job. Nine application images built and passed strict
HIGH/CRITICAL scans locally. BuildKit SBOM/provenance subjects, source metadata
and runtime contents passed local validation. A real network-disabled Skopeo
round-trip preserved the gateway runtime and both attestation statements.

Publication remains blocked: package access is unverified after HTTP 403, branch
protection inspection returns a private-plan restriction, Phase 3 remote success
is not established, and the storage exception covers local rehearsal only.
No successful-publication manifest, remote digest or GHCR package is invented.

## 2. Initial repository state

Working tree was clean on main. All existing Phase 0 audit/plan, Phase 1-3
implementation, integration, remediation, approval and ledger documents were
inspected and preserved. Authentication's twelve missing constants/catalogue
entries and exact catalogue/audit tests are present. Prior real database-backed
test evidence passed; current SQL/domain drift tests pass. No permission or
business-logic change was necessary in Phase 4.

The owner approved the existing Silo mitigation for isolated local rehearsal
on 2026-11-24. Its raw HIGH finding remains; that approval explicitly excludes
publishing/release/cloud scope. Legacy PostgreSQL remains stopped with 26 HIGH
and one CRITICAL in the earlier fresh scan; the 18.7 candidate had zero
HIGH/CRITICAL. Neither database image is published or upgraded here. Phase 3
remains remote-pending: read-only run inspection found historical failed runs,
not successful verification of the current remediation.

Docker baseline contained 91 existing containers (66 legacy plus retained
rehearsals) and 29 named/anonymous/cache volumes. Their identities, running/start
states and volume metadata were captured before validation. No application
environment was started; all application data remained untouched.

## 3. Final image inventory

Nine packages: auth, user, course, enrollment, assessment, analytics, gateway,
frontend and assessment-minio-init. Full Dockerfile/context/local/GHCR mapping,
runtime configuration, platform and classifications are in
[ghcr-image-registry.md](ghcr-image-registry.md).

Six migrations reuse service images. The Select guard reuses gateway. Supporting
PostgreSQL/Silo and development/control/scanner images are excluded. Existing
PostgreSQL candidate CI validation remains intact without publication.

## 4. Files created

| File | Purpose |
| --- | --- |
| `infrastructure/registry/publication-policy.json` | Explicit fail-closed prerequisite/owner/review record |
| `infrastructure/registry/image-manifest.schema.json` | Machine-readable successful publication schema |
| `infrastructure/scripts/registry_pipeline.py` | OCI identity/scan/attestation checks, protected-ref policy, no-rebuild copy/pull/scan and complete manifest generation |
| `tests/infrastructure/test_registry_pipeline.py` | Trust boundaries, failed prerequisite/no-network behavior, workflow permissions and strict manifest negatives |
| `docs/infrastructure/ghcr-image-registry.md` | Inventory, names, runtime inputs, private access and Compose consumption |
| `docs/infrastructure/ghcr-publishing-workflow.md` | Workflow, artifacts, permissions, provenance limits and owner prerequisites |
| `docs/infrastructure/phase-4-validation-evidence.md` | Actual commands/results and acceptance evidence |
| `docs/infrastructure/phase-4-implementation-report.md` | This implementation/acceptance report |

Ignored local orchestration, OCI archives, raw scans, metadata and validation logs
are under `.infra-state/phase4/20261124T031029Z`; they are not Git deliverables.

## 5. Files modified

`.github/workflows/ci.yml`: OCI export/attestations, extracted-layout scan,
same-run artifact handoff, protected-branch prerequisites, minimal-permission
publisher, actual digest/pull verification, evidence retention and credential
cleanup. Default-branch jobs are not canceled by later pushes during publication.

`.github/workflows/publish-images.yml`: retain a non-publishing compatibility
entry point directing manual operators to integrated CI. No duplicate pipeline.

`infrastructure/scripts/requirements.txt`: pin jsonschema 4.25.1 for schema
validation; inherited by existing infrastructure requirements and installed in
the publisher. No Dockerfile, Compose model, migration, business logic, runtime
input or historical report was modified. No source commit or push was created.

## 6. GitHub Actions changes

Existing backend/frontend/infrastructure checks remain mandatory. Image builds
use pinned existing Buildx/build actions, linux/amd64, final runtime stages,
OCI labels, SPDX SBOM and SLSA provenance. The nine scanned archives are copied
unchanged after all checks; no publisher build step exists. PRs cannot publish.
Only this repository's protected default branch push or manual dispatch is
eligible, with a second Python gate before credential use.

Action SHAs are retained from existing reviewed workflows. Newly used
download-artifact v4 SHA was independently resolved through the official
repository API. No unpinned GitHub Action or workflow_run trust was introduced.

## 7. GHCR authentication and permissions

The repository API confirms a private repository and current owner admin access.
Package enumeration still returns HTTP 403 for missing `read:packages` in the
CLI token. This does not prove GITHUB_TOKEN access; package linkage/access must
be verified by the owner. Main protection API returns HTTP 403 with an upgrade
plan/public-repository message. No plan, visibility or security setting changed.

Only publish has `contents: read` and `packages: write`. GITHUB_TOKEN is passed
through stdin to ephemeral Skopeo/Docker logins and environment-only scanner
authentication, never stored in repository files/build arguments/layers. No PAT
is introduced to work around publication permissions. Local private-pull reader
requirements are documented separately.

## 8. Build and publishing results

All nine images built once locally into separate OCI files using unique audit
names `ucm-<service>:phase4-audit-20261124`. These are local build labels/tags,
not published release identities. All nine corrected scans returned zero and
reported no HIGH/CRITICAL findings. Scan config IDs match the built image configs.

Frontend publication build uses an empty public API-base argument, preserving
gateway-only future integration and leaving local-development configuration
unchanged. OCI creation timestamps are build timestamps. Existing package-manager
repositories and SBOM-generator inputs still affect future rebuilds; source SHA
alone is not a byte-reproducibility claim. Artifact reuse preserves the tested
bytes for publication and future digest-based consumption.

Publication was not attempted, because prerequisites failed and the changed
workflow is uncommitted. Implementing a workflow locally does not install it on
GitHub. Source commit/push requires separate authorization under this task.

## 9. Actual published references and digests

**None.** No package existence, upload, registry digest or successful manifest
is claimed. Actual local runtime/config/attestation digests appear only as local
build evidence. The publication code reads the digest returned by Skopeo's
successful upload; verification hashes a real registry response against that
returned value. It does not fabricate a registry digest from a source SHA.

## 10. SBOM and provenance

All nine builds produced SPDX SBOM and SLSA v1 provenance in the OCI index.
Local checks verified content hashes, runtime subject linkage and source labels.
The gateway local-copy test preserved both statements. These are unsigned
BuildKit attestations; no GitHub-signed identity attestation is claimed. Remote
subject/content verification is implemented but has not run. Optional signing
would require a supported, explicitly authorized permission/verification design.

## 11. Pull verification

GHCR pulls and post-upload registry scans: **not executed**. The implemented
publisher resolves the actual digest, copies its complete OCI index, verifies
attestations, performs Docker pull/platform inspection and scans that digest
before allowing an image into the complete manifest. Local archive transport
testing is explicitly not proof of GHCR pull access.

## 12. Security findings and unresolved risks

Nine application scans passed at the tested date/severities, not a blanket
security guarantee. Silo remains one raw HIGH with a local-only mitigation;
Assessment and its helper depend on that supporting storage. Other independently
clean application images are not declared unsafe by that finding, but the
complete nine-image publication is intentionally blocked rather than emitting
a partial successful set. Supporting legacy PostgreSQL exposure remains recorded,
unfixed and outside this phase's upgrade scope.

SHA tags are immutable by pipeline policy; privileged external tag mutation
is not prevented by GHCR. Deploy by digest. Partial registry writes are possible
if a later copy or post-publication scan fails; partial evidence survives and
no complete manifest is issued. Artifact retention is finite (90 days for
publication evidence); long-term custody remains an owner responsibility.

## 13. Tests executed

See [phase-4-validation-evidence.md](phase-4-validation-evidence.md) for genuine
commands, outputs, initial failures and corrected results. Nine builds and strict
scans passed, six backend CI-style quality paths passed, 33 infrastructure tests
passed after dependency installation, ten Compose models passed and actionlint
passed after correcting its initial context errors. Backend database-dependent
tests were not newly run: those paths skip without explicit fixture URLs. Earlier
real-database Authentication and privilege/restore evidence remains historical.

## 14. GitHub-hosted execution evidence

No new run was dispatched and no new source was committed/pushed. Read-only
inspection found historical failures, including run
`45186338309` (pull_request) and `45186313447` (push). They are not successful
Phase 4 evidence. There is no Phase 4 run URL, manifest artifact or registry pull
to report.

## 15. Owner-side actions

Enable supported protection for main, establish GHCR repository/package access,
obtain actual successful Phase 3 remote verification, resolve or appropriately
approve the storage release dependency with named owner/review date, and review
the fail-closed policy updates. Then separately authorize a reviewed source
commit/push so GitHub can execute this implementation. No token/visibility/plan
change or broader storage exception was accepted on the owner's behalf.

## 16. Data and volume safety

Only disposable build/scanner/quality/control containers ran. Existing 91
container identities/start/running states and 29 volume metadata records were
preserved, including all 66 stopped legacy containers and 23 original volumes.
Build/Go/scanner caches and ignored evidence were used; no application data
volume was attached to validation tools or modified. No environment cutover,
database upgrade, semantic tag, deployment or later-phase implementation occurred.
Historical report hashes remain unchanged. Final resource checks are in
`final-local-summary.json`; source HEAD remains `69947a6`.

## 17. Phase 4 acceptance checklist

| ID | Result |
| --- | --- |
| P4-01 | VERIFIED: nine application images inventoried; supporting/control images classified |
| P4-02 | IMPLEMENTED: lowercase existing GHCR namespace; package existence unverified |
| P4-03 | IMPLEMENTED: integrated gated publisher; not remotely executed |
| P4-04 | VERIFIED LOCALLY: job-level least privilege; no publishing PAT |
| P4-05 | VERIFIED LOCALLY: protected-ref/event/repository checks and negative tests; server protection prerequisite blocked |
| P4-06 | VERIFIED LOCALLY: nine application builds passed |
| P4-07 | VERIFIED LOCALLY: nine strict scans passed; failure gates tested |
| P4-08 | IMPLEMENTED: full-SHA tags/refusal to overwrite; no published tag evidence |
| P4-09 | PENDING: actual registry digests require real publication |
| P4-10 | IMPLEMENTED/LOCAL CONTRACT VERIFIED: JSON Schema and semantic negatives pass; no real publication manifest |
| P4-11 | LOCAL GENERATION/SUBJECT CHECKS VERIFIED: unsigned SBOM/provenance; registry verification pending |
| P4-12 | PENDING: GHCR pull access/publication prerequisites |
| P4-13 | VERIFIED LOCALLY: ten models preserved; nine-variable digest output matches existing release loader; no deployment |
| P4-14 | COMPLETED: four Phase 4 documents with explicit incomplete publication evidence |
| P4-15 | VERIFIED: original container/volume identities preserved; no application data changes |
| P4-16 | VERIFIED: local storage exception, legacy findings and remote Phase 3 limitation retained |

## 18. Final verdict

**PARTIALLY COMPLETED — BLOCKED BY PREREQUISITES.** The build-once publishing
implementation is locally reviewable and validated. Actual publication and
Phase 4 completion require the owner-side prerequisites and genuine GitHub/GHCR
execution, digest, manifest and pull evidence. Phases 5-9 were not started.

## 2026-11-27 continuation: hosted CI and current publishing status

Starting state was clean `main` at
`e575487337ed5dce1eee3eb172c9e3f09355d13b`, matching GitHub's default
branch. GitHub CLI is authenticated as repository admin with `repo` and
`workflow`, but no `read:packages` scope. The private repository has Actions
enabled; it has zero environments. Branch protection and rulesets APIs both
return 403 with a GitHub Pro/public-repository requirement. The private package
list also returns 403 for missing `read:packages`; package existence,
visibility, linkage and `GITHUB_TOKEN` access are therefore unverified.

The deferred `publish-images.yml` was replaced locally with a manual reusable
call to `ci.yml`. It accepts no arbitrary commit or artifact input, requires
the protected default branch, and calls the CI file from the same commit.
Backend/frontend/infrastructure/images must all pass before the prerequisite
and publish jobs. Only the publish job in called CI has package-write
permission. The same OCI archive that passed the pre-publication scan is
copied by Skopeo; a successful run would record the registry-returned digest,
verify the index/attestation subjects, pull by digest, rescan, and validate
`image-manifest.json` plus the nine-variable `images.env`. BuildKit generates
unsigned SPDX SBOM and SLSA provenance. No signed GitHub attestation is claimed.
The manifest artifact is configured for 90-day retention, which is finite.

GitHub-hosted CI run
[46038430324](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46038430324)
used that source SHA. All six backend jobs, frontend, infrastructure and the
PostgreSQL support-image job succeeded. **All nine application image jobs failed
the HIGH vulnerability gate; publication-prerequisites and publish skipped.**
The raw scans report CVE-2026-124709 in seven Alpine OpenSSL runtimes and six
libxml2 HIGH findings in frontend/gateway. Dockerfiles now request the scanner's
fixed floors, OpenSSL 3.5.9-r2 and libxml2 2.13.9-r5. Those edits are local;
at this validation checkpoint they had not been committed, pushed or run in
GitHub. The previous local
Authentication 23-permission remediation remains intact, with no authorization
or business-logic edit here. Silo CVE-2026-39414 retains one raw HIGH and only
local-rehearsal mitigation approval; legacy PostgreSQL retains 26 HIGH and one
CRITICAL in stopped legacy images. Neither supporting image is an application
GHCR package.

**No application package, SHA tag, registry digest, complete publication
manifest, GHCR pull, or registry-verified attestation was produced by this run.**
The owner must resolve protected-main availability, inspect private GHCR
package access, provide a release-scope storage decision, and authorize a
reviewed commit/push of these local edits. A new hosted run must then pass all
checks before actual publication can be attempted. No environment deployment,
database mutation, volume deletion, package visibility change, commit or push
had been performed at this validation checkpoint.

The local continuation then built and scanned all nine application OCI archives
with the patched Dockerfiles. All nine fresh reports contain zero HIGH/CRITICAL
findings; OCI source/config identities, SPDX/SLSA subjects and runtime contents
passed local checks. Actionlint, ten Compose models, secret scanning and 34
infrastructure tests passed. The source tree is dirty, so these are test
artifacts, not commit-bound release images. No current GHCR identity exists.

### Phase 4 acceptance at this checkpoint

| Criterion | Status | Evidence or remaining gate |
| --- | --- | --- |
| P4-01 inventory | PASS | Nine application images; migrations reuse service images; PostgreSQL/Silo excluded |
| P4-02 naming | PASS locally | Lowercase GHCR namespace in publisher; no package verified |
| P4-03 publishing workflow | PASS locally | Manual entry calls same CI; remote execution pending |
| P4-04 least privilege | PASS locally | Only publisher gets package write in called CI |
| P4-05 untrusted PR exclusion | PASS locally | Event/ref/protection checks and unit tests |
| P4-06 remote required checks | FAIL | Run 46038430324 failed nine image scans; patched commit unrun |
| P4-07 eligible builds | PASS locally | Nine Buildx archives built; hosted old build also succeeded |
| P4-08 prepublication scan | PASS locally, FAIL remotely | Nine patched scans passed; hosted old scans failed |
| P4-09 publication | BLOCKED | No GHCR upload; policy/protection/remote gates closed |
| P4-10 full SHA tags | BLOCKED | Implemented, no published tags |
| P4-11 actual registry digests | BLOCKED | None returned by GHCR |
| P4-12 complete manifest | BLOCKED | Schema tests pass; no real manifest |
| P4-13 evidence retention | PASS for failed run, BLOCKED for publication | Scan artifacts uploaded; no publication artifact |
| P4-14 SBOM | PASS locally | BuildKit SPDX statements checked in OCI archives |
| P4-15 provenance/attestation | PASS locally, BLOCKED remotely | Unsigned BuildKit SLSA statements checked; no registry verification/signature |
| P4-16 pull by digest | BLOCKED | No published digest to pull |
| P4-17 infrastructure references | PASS locally | Existing nine-variable digest manifest and Compose interpolation validate |
| P4-18 documentation | PASS | Four Phase 4 documents updated with actual evidence and limits |
| P4-19 data safety | PASS | 69 container states and 29 volume metadata records unchanged |
| P4-20 earlier findings | PASS | Auth resolution, Silo HIGH, legacy PostgreSQL 26 HIGH/1 CRITICAL recorded |

**Current verdict: PARTIALLY COMPLETED — BLOCKED.** Remote publication awaits
a reviewed source commit/merge, successful hosted validation, supported branch
protection, package-access review and the unresolved storage release decision.
