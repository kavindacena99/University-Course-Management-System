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
