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

## RC1 GitHub operation and advancement

After PR #19 merged at `3cc681a773941607db7e960136b4b1c1c19c71ad`,
the annotated `v0.1.0-rc.1` tag was created at the published source commit
`e83d2d131136624024175d16d2f41e1501d0b385`. GitHub confirms the tag
object points to that commit. The
[GitHub prerelease](https://github.com/kavindacena99/University-Course-Management-System/releases/tag/v0.1.0-rc.1)
is not a draft and has downloadable `release-manifest.json` and `images.env`
assets. Both were downloaded and their hashes matched the repository copies.
No GHCR semantic alias or image rebuild was made.

QA found a release-blocking frontend API-base defect in the published RC1
bundle. The failed RC and evidence remain intact. The fix changes frontend
source and Nginx routing, so a reviewed new commit, complete CI publication,
new manifest and `v0.1.0-rc.2` are required. RC1 is not eligible for stable
promotion. Phase 5 remains in progress until the next candidate and QA/stable
transition are validated.

## RC2 publication evidence

The frontend fix merged under PR #20 at source commit
`1d76667cfb27077568f5dca283352386b67d6f02`. Protected-main
[CI run 46120731377](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46120731377)
completed its required jobs and published a new nine-image set. The actual
[manifest artifact](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46120731377/artifacts/15253576986)
and [publication evidence](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46120731377/artifacts/15253417313)
were downloaded. The manifest passed schema and semantic validation; all nine
publication records, digest files and `images.env` entries agreed. Each SHA
tag independently resolved on GHCR to the manifest's digest. The manifest's
SHA-256 is `bd95ab4398ae8a107e87b8d30f0caca059ead1bf5ba41e846baee1fc07e1228e`.
The RC2 record under `release-manifests/v0.1.0-rc.2/` refers to this exact
source and image set; QA status remains pending. No RC1 tag or digest changed.

## RC2 prerelease and QA outcome

The annotated `v0.1.0-rc.2` tag points to the exact publication source commit
`1d76667cfb27077568f5dca283352386b67d6f02`. Its
[GitHub prerelease](https://github.com/kavindacena99/University-Course-Management-System/releases/tag/v0.1.0-rc.2)
contains the real release manifest and nine-image `images.env`. Both assets
were downloaded and SHA-256 matched their repository copies. RC2 deployed
in QA, but the [QA verdict](../../release-manifests/v0.1.0-rc.2/qa-validation.json)
failed on unauthenticated, static lecturer pages. The RC2 tag, image digests and
failure evidence remain preserved. No stable tag or release was created.
Phase 5 remains **IN PROGRESS** because the stable transition cannot pass
without accepted QA.
