# Phase 4 validation evidence

Date: 2026-11-24. Baseline main `69947a6`. Evidence root:
`.infra-state/phase4/20261124T031029Z`. `PY` below means
`.infra-venv/Scripts/python.exe -B`, run from repository root.

## Executed checks and results

| Area | Actual command / orchestration | Actual result | Evidence |
| --- | --- | --- | --- |
| Repository | `git status --short`; `git log -1 --oneline` | Initially clean main at merged PR #13 | Initial tool output; baseline.json |
| Repository permissions | `gh api repos/kavindacena99/University-Course-Management-System --jq '{default_branch,visibility,permissions}'` | Private; main default; current account admin | Read-only API output |
| GHCR packages | `gh api users/kavindacena99/packages?package_type=container` | HTTP 403; CLI lacks read:packages | Read-only API output; policy evidence |
| Protection | `gh api repos/kavindacena99/University-Course-Management-System/branches/main/protection` | HTTP 403; private-plan restriction | Read-only API output; policy evidence |
| Existing CI | `gh run list --repo kavindacena99/University-Course-Management-System --workflow ci.yml --limit 5 --json databaseId,headSha,status,conclusion,event,url` | Historical failed runs; no current successful verification | Actual API output; no new run |
| Official action pins | `gh api repos/actions/download-artifact/git/ref/tags/v4 --jq .object.sha`; login-action v3 same operation | Official download SHA d3f86a106a0bac45b974a628896c90dbdf5c8093 and login SHA c94ce9fb468520275223c153574b00df6fe4bcc9 | Actual API output; existing other pins retained |
| Nine builds | `PY .infra-state/phase4_validate.py` → one `docker buildx build --platform linux/amd64 --sbom=true --provenance=mode=max --output type=oci,dest=<image-directory>/image.tar --metadata-file <directory>/buildkit-metadata.json --tag ucm-<service>:phase4-audit-20261124 --file <inventory-Dockerfile> --label <six OCI identity fields> <inventory-context>` per service; frontend adds `--build-arg VITE_API_BASE_URL=` | All nine exit 0 | Exact argument arrays in results.json; build-*.log; image-*/buildkit-metadata.json |
| First scanner input | Same initial orchestration: Trivy `--input /reports/image.tar` | Failed tooling: tar unsupported as OCI input; no clean result inferred | Original scan-*.log retained |
| Corrected scanner input | `PY .infra-state/phase4_finish.py` → Trivy 0.76.2 pinned image, `image --no-progress --timeout 15m --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --format json --output /reports/image-scan.json --input /reports/scan-layout` | All nine exit 0, zero HIGH/CRITICAL; config IDs matched | corrected-results.json; corrected-scan-*.log; image-*/image-scan.json. Gateway correction ran separately with same flags |
| OCI unpack/metadata | `registry_pipeline.unpack()` with Python data-filtered tar extraction; `prepare()` and `archive_info()` | Nine linux/amd64 runtimes; matching source labels/blob digests, SPDX/SLSA subjects, scanner identity | image-*/build.json; contents-verified.json |
| Runtime contents | Final-check orchestration streams actual runtime layer archives and reads config | Seven app/helper users are app; no /src or /app Go source/go.mod/.env; existing edge convention retained | image-*/contents-verified.json |
| Local Skopeo transport | `docker run --rm --network none --mount type=bind,source=<gateway-artifact-directory>,target=/evidence quay.io/skopeo/stable@sha256:7e7d5b9c9f30896e8fd256d33ba351408299399442f04b8ef3a9452f3c2fa50e copy --all --preserve-digests --digestfile /evidence/local-copy-digest.txt oci-archive:/evidence/image.tar oci-archive:/evidence/local-copy.tar` | Exit 0; actual copied runtime and both statement digests matched | Local copy artifacts; final-local-summary.json; tool output. Not a registry upload |
| Six backend quality paths | Finish orchestration → pinned Go 1.27.3 image with network none, read-only temporary module source, existing Go caches: `test -z "$(gofmt -l .)"; go mod verify; go mod tidy -diff; go test -count=1 ./...; go vet ./...; go build -trimpath -o /tmp/service .` | All six exit 0; no TEST_DATABASE_URL supplied, so database-dependent tests skip | backend-quality-*.log; corrected-results.json |
| Initial actionlint | `docker run --rm --network none --mount type=bind,source=<repo>,target=/repo,readonly --workdir /repo rhysd/actionlint:1.7.12 -color .github/workflows/ci.yml .github/workflows/publish-images.yml .github/workflows/prepare-deployment.yml` | Failed: runner context unavailable in job env; corrected to GITHUB_ENV setup step | actionlint.log |
| Corrected/final actionlint | Same lint command after correction | Exit 0 | actionlint-corrected.log; actionlint-final.log |
| Infrastructure/schema tests | `PY -m unittest discover -s tests/infrastructure -v` | 33 passed after jsonschema dependency installation and the digest/Compose compatibility check | tests-contract-final.log. Earlier tests-final.log preserves missing-dependency failure; tests-with-schema.log records the preceding 32-test pass |
| Manifest schema | Runtime Draft202012Validator check_schema and format validation plus semantic checks; positive/negative unit fixtures | Passed: reject missing/duplicate images, placeholders, wrong tags, unverified pulls/scans, false signing claims | tests-with-schema.log; no successful real manifest generated |
| Compose | `PY infrastructure/scripts/ci_models.py` | Ten configurations passed; synthetic temporary inputs only | compose-final.log; compose-with-schema.log |
| Secret scan | `PY infrastructure/scripts/scan_secrets.py` | Initial post-code scan exit 0; final completed-document scan recorded separately | secret-scan.log; secret-scan-completed.log |
| Whitespace | `git diff --check` | Passed | diff-check.log; diff-final.log |
| Preservation | `PY .infra-state/phase4_final_checks.py` | Existing 91 containers and 29 volume metadata records unchanged; all historical report hashes/HEAD preserved | baseline.json; safety.json; final-local-summary.json |

The corrected local runner reuses all original OCI archives. It does not rebuild
images to replace failed scanner-input attempts. A dependency installation race
caused one manifest test to run before jsonschema was installed; its failure is
retained and the final schema-enabled suite passed. No assertion was removed to
make these checks pass.

## Scope of security evidence

Scanner is `aquasec/trivy:0.76.2@sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb`.
Database update checks were enabled; no ignore file, ignore-unfixed flag or
severity waiver was added. Build/source/OCI/platform/SBOM checks are local
evidence only. BuildKit statements are unsigned; no GitHub identity signature
was generated or verified. Same-run artifact and protected-branch restrictions
are tested locally and require real GitHub execution before remote acceptance.

Earlier supporting scans remain under
`.infra-state/remediation/security/recovery-ghcr-20261124`: PostgreSQL candidate
0 HIGH/CRITICAL, unchanged legacy PostgreSQL 26 HIGH/1 CRITICAL, Silo 1 HIGH.
These supporting scans and the earlier named database-backed Authentication
test (passed 0.21s) were inspected, not newly repeated as Phase 4 results.
The local-only storage exception is not publication-scope acceptance.

## Not executed and acceptance impact

No source commit/push, workflow dispatch, GHCR login/publication, package visibility
change, GHCR manifest resolution/pull, post-publication scan or signed attestation
occurred. There are no published image references/digests or successful publication
manifest artifacts. The registry 403 and branch-protection prerequisite were not
bypassed. No database/application environment was started or deployed.

The implementation report's P4-01 through P4-16 checklist distinguishes local
proof from remote-pending criteria. Final verdict remains **PARTIALLY COMPLETED —
BLOCKED BY PREREQUISITES**. No Phase 5 release candidate, QA deployment, PROD-like
promotion, new backup system or monitoring phase was implemented.

## 2026-11-27 continuation evidence

The historical section above describes the 2026-11-24 session. This session
started with clean `main` at `e575487337ed5dce1eee3eb172c9e3f09355d13b`;
the GitHub API reported the same current default-branch commit and a private
repository. Actions are enabled. The CLI account is repository admin but its
token lacks `read:packages`. `gh api users/kavindacena99/packages?package_type=container`
returned HTTP 403 for that scope. Both main branch protection and repository
ruleset APIs returned HTTP 403 with the private-plan restriction. Zero GitHub
environments are configured. These reads do not establish any existing GHCR
package or repository package permission.

`gh workflow run ci.yml --ref main` started actual GitHub-hosted run
[46038430324](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46038430324).
Its source SHA is `e575487337ed5dce1eee3eb172c9e3f09355d13b` and final
conclusion is **failure**. Six backend jobs, frontend, infrastructure and the
PostgreSQL supporting-image job passed. All nine application-image jobs failed
the strict Trivy HIGH/CRITICAL gate. `publication-prerequisites` and `publish`
skipped, as designed. Their `GITHUB_TOKEN` did not authenticate to GHCR and
there was no upload, registry digest, manifest artifact or registry pull.

The ten raw `image-scan-*` artifacts were downloaded to ignored local
`.infra-state/phase4/current-remote-scans`. Seven service/helper reports contain
HIGH CVE-2026-124709 for `libcrypto3` and `libssl3` 3.5.9-r0, with fixed version
3.5.9-r2. Frontend and gateway reports contain six HIGH libxml2 CVEs on
2.13.9-r3, with the highest fixed floor 2.13.9-r5. PostgreSQL's candidate scan
contained no HIGH/CRITICAL finding. The reports were preserved; no scanner rule,
ignore file, severity threshold or failed test was weakened. Dockerfiles now
request the fixed floors in edits that were uncommitted at this checkpoint.

Local checks after those edits: official actionlint 1.7.12 Windows archive was
downloaded from its release and its SHA-256 matched the published checksums file
(`6e7241b51e6817ea6a047693d8e6fed13b31819c9a0dd6c5a726e1592d22f6e9`).
It accepted all three workflows. Infrastructure unit tests passed 33/33 before
the new manual-entry contract test; the follow-up suite and image build/scan are
recorded below once complete. `ci_models.py` passed the ten isolated Compose
configurations. `git diff --check` passed. Docker Desktop was initially stopped;
it was started for local image validation, with no application deployment.

The new `publish-images.yml` edit was only in the local working tree at this
checkpoint. GitHub's
run above used the previously committed workflow; it cannot validate these
changes or the Dockerfile fixes. No commit or push had yet been made. The existing
90-day publication artifact retention is finite and has not yet held a
successful manifest. BuildKit SBOM/provenance statements remain unsigned;
remote attestation verification did not run.

### Local completion of the uncommitted fix

Ignored evidence directory: `.infra-state/phase4/20261127T170852Z`. The local
orchestrator recorded exact build/scan argument arrays and exit codes in
`results.json`, raw logs and nine `image-*/image-scan.json` files. Docker Buildx
built each of the nine application images once as a `linux/amd64` OCI archive,
with SBOM and provenance enabled. The extracted OCI layouts were scanned by
the same pinned Trivy 0.76.2 HIGH/CRITICAL exit-1 policy as CI. **Nine builds and
nine scans exited 0; all nine fresh reports have zero HIGH/CRITICAL findings.**
`registry_pipeline.prepare` verified each scanner ImageID against the OCI config
digest, full blob hashes, source labels and SPDX/SLSA statement subjects.
Runtime-layer inspection found no Go source or `.env` file in any image; the
six Go services and setup helper run as `app`.

The final local infrastructure suite passed 34/34 tests, including the new
manual-entry workflow boundary check. Actionlint passed all three workflow
files; ten Compose models, secret history and working-tree scans, and
`git diff --check` passed. `safety.json` records 69 pre-existing container
identities/start states and 29 volume metadata records unchanged. The images
were not deployed. The local archives were built from a **dirty working tree**;
their source-revision labels reflect the current HEAD for local testing and
must not be treated as published commit artifacts. GitHub must rebuild the
reviewed committed source, rescan it and return actual GHCR digests.

Manifest schema and semantic-negative tests passed locally; no successful
publication manifest exists. No GHCR package was created or pulled. This is
local validation of the fix, not remote acceptance of Phase 4.

## 2026-11-27 protected-main continuation evidence

| Check | Actual result |
| --- | --- |
| Repository | Public, clean `main` at `92fce6b02c15def11c9194137eca5387bfefa6dd` before this branch |
| Protection | Branch API `protected=true`; active ruleset 28252583 targets `~DEFAULT_BRANCH`, denies deletion/non-fast-forward; no bypass actors |
| Actions permission | Repository default workflow permission `read`; CI publisher alone requests `packages: write` |
| Public history | `scan_secrets.py`: 25 commits and 418 tracked/non-ignored files, no leaks; no runtime `.env`, key or dump tracked in history |
| GitHub secret scanning | Alerts API HTTP 404: feature disabled; no alert-list inference made |
| GHCR package list | CLI API HTTP 403: token lacks `read:packages`; no package existence/visibility inference made |
| Fresh main CI | [Run 46047552300](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46047552300), source SHA above: six backend, frontend, infrastructure, nine application image and PostgreSQL candidate jobs passed |
| Hosted protection context | Prerequisite log: `GITHUB_REF_PROTECTED=true` |
| Hosted publication gate | `publication-prerequisites` failed on `registry_access_verified`; `publish` skipped; no GHCR login/upload/digest/pull/manifest |
| Local policy/schema tests | 39 infrastructure tests passed after bootstrap/rerun changes; JSON policy/schema parsed and `git diff --check` passed |

The policy records `phase3_remote_verified=true` only after that fresh hosted
run. `registry_access_verified` remains false until a real scanned image is
uploaded and verified. The new bootstrap preflight tests cover absent tags,
identical reruns, partial reruns, conflicts, authentication denial, digest
mismatch, missing attestation and scanner failures. Historical run failures
remain recorded above. The publication-scope storage decision remains pending;
no source or registry publication has been claimed by this continuation.
