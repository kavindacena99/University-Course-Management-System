# Release management

## Version and source identity

This project follows Semantic Versioning 2.0.0. Use `vMAJOR.MINOR.PATCH-rc.N`
for candidates and `vMAJOR.MINOR.PATCH` for stable releases. There were no
existing semantic Git tags or GitHub releases on 2026-11-28, so the first
candidate is `v0.1.0-rc.1`. This pre-1.0 version does not imply stable API
compatibility.

Increment MAJOR for intentionally incompatible public behavior after 1.0,
MINOR for compatible features or pre-1.0 planned feature milestones, and PATCH
for compatible fixes. Before 1.0, document breaking changes explicitly even
when the minor number changes. RC numbering starts at 1 for each base version
and increases monotonically. Failed RCs retain their tags, releases and
evidence. A fix requires a new reviewed commit, full CI and a new publication
manifest, followed by the next RC number.

Use reviewed short-lived branches for release preparation; the release tag is
the permanent identity. An annotated RC tag points to the exact source commit
used by its nine published images. A stable tag points to that same commit if
QA accepted the candidate without application-source changes. Any source
change requires a new RC and a newly verified image set before stable release.
Never move or overwrite a Git tag or an existing GHCR SHA/version tag.

## Authoritative artifacts

The registry digest, not an image tag, identifies deployment bytes. Every
candidate stores a validated `release-manifest.json` and matching `images.env`
under `release-manifests/<version>/`. The manifest imports the actual successful
Phase 4 publication manifest, records its SHA-256 and artifact URLs, and
preserves all nine GHCR image/runtime digests, scan/pull results, SPDX/SLSA
references and unsigned-attestation status. No application image is rebuilt
for versioning or promotion. Semantic GHCR aliases are optional; create one
only through a digest-preserving registry copy, after checking that the alias
is absent, and verify the resulting digest. The initial RC uses SHA tags and
digest references without semantic image aliases.

GitHub Actions retains publication artifacts for 90 days. The repository
retains non-secret manifests and `images.env`; GitHub releases attach the
candidate's release files. Release notes record source, CI, security findings,
known limits, image digests and QA evidence. Never add credentials or backups
to release assets.

## QA acceptance and stable release

An RC remains a prerelease until the exact digest-pinned set passes isolated
consolidated-target QA: migration and privilege checks, object-store guard and
normal S3 operations, six services, gateway/frontend, authenticated workflows,
browser validation, persistence and recovery checks. Record the QA run or
immutable report reference in the release manifest. Runtime storage approval
for the exact Silo digest and QA scope is separate from Phase 4 image
publication approval.

Stable release requires a reviewed QA acceptance record, no unresolved release
blocking finding, unchanged source and image digests, and owner approval of
the stable tag and notes. PROD-like promotion consumes the same nine digests;
it never rebuilds. Its runtime storage approval is separate. Rollback uses a
previous compatible immutable release manifest, a verified backup, migration
compatibility review and the target controller's guarded rollback path. Never
select a mutable tag as a rollback reference or delete volumes to recover.
