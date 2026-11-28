# Infrastructure roadmap status

Updated 2026-11-28. Historical reports retain their original checkpoint
results; this page records the current acceptance state.

| Phase | Status | Evidence and next gate |
| --- | --- | --- |
| 1–3 consolidated target preparation | Implemented and locally rehearsed | Prepared QA/PROD-like inputs are isolated; legacy and DEV preserved |
| 4 image publication | Completed | Protected-main run 46108269083, validated nine-image manifest and GHCR digests |
| 5 semantic release management | In progress | RC1 and RC2 annotated tags and GitHub prereleases exist; both QA verdicts failed, so no stable transition |
| 6 QA deployment | Failed RC2 validation; new RC required | RC2 digest-pinned deployment, external backup, SQL and guard checks passed; static unauthenticated lecturer pages blocked full QA; QA stopped with volumes preserved |
| 7 PROD-like promotion | Pending QA acceptance | RC2 cannot be promoted; no stable release or consolidated PROD-like deployment claimed |
| 8 backup, restore, rollback | Next phase | Only scoped predeployment backup and guarded image rollback planning belong to this task; no full DR claim |
| 9 monitoring and operations | Deferred | No production service or cloud target selected |

The local PROD-like target is a rehearsal, not an externally accessible
Production system. No cloud provider or live Production server has been chosen.

The latest protected-main source `9cc70c16d60854a4c987e226cb201aaa237369a9`
has its own successful nine-image publication in run 46141408825. It is not
an accepted RC. RC2 remains the last QA-deployed candidate and failed QA.
