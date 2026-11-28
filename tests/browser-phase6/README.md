# Phase 6 live browser checks

Run only against a development stack with dedicated synthetic lecturer and
student accounts. The student must be actively enrolled in the assigned
offering, and the offering must have room for a new 1% assessment component.
Set `PHASE6_BASE_URL`, `PHASE6_LECTURER_EMAIL`,
`PHASE6_LECTURER_PASSWORD`, `PHASE6_STUDENT_EMAIL`,
`PHASE6_STUDENT_PASSWORD`, `PHASE6_ASSIGNED_OFFERING_ID`,
and `PHASE6_UNASSIGNED_OFFERING_ID` in the
local process environment. Never commit their values. Then install this
directory's dev dependency and run `npm test` here. Reports, traces and
screenshots are ignored under `artifacts/`.

The suite issues real same-origin API requests and checks persistence after
reload. It intentionally cannot run against stopped QA without an authorized
new deployment and verified backup. A skipped or unavailable live suite is
not QA acceptance.
