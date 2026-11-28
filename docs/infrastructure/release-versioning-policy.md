# Versioning rules

The authoritative versioning policy is
[release-management.md](release-management.md). This short operator sheet
states the enforceable release decisions:

1. Use SemVer 2.0.0 tags `vMAJOR.MINOR.PATCH-rc.N` and
   `vMAJOR.MINOR.PATCH`; keep every tag permanent.
2. Bind an RC tag to the exact source commit in a successful nine-image
   publication manifest. Deploy only `image@sha256:...` references.
3. Advance `rc.N` after any source correction. Run the full build, scan,
   publication, and QA sequence again. Preserve failed RC evidence.
4. Create a stable tag only after QA accepted the exact RC digest set.
   Point it to the same source commit, attach the same nine image digests,
   and do not rebuild for promotion.
5. Keep SHA tags and semantic aliases non-overwriting. An optional GHCR
   semantic alias must preserve OCI index bytes and resolve to the same digest.
6. Review migration compatibility, backup, and the previous immutable
   manifest before image rollback; database migrations are not reversed
   automatically.
