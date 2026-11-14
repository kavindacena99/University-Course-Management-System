# Proposed Enrollment contract for published capacity reductions

**Status: proposal for review. No endpoint in this document exists yet.**

Course Service currently rejects lowering a published Offering's finite
capacity, switching finite capacity to unlimited, and switching unlimited
capacity to a finite limit. The last transition is also a reduction because
existing enrollments may already exceed the new limit. Course Service stores
configuration but has no enrollment count. A simple `GET count` followed by a
Course update is unsafe: an enrollment could commit between those operations.

## Ownership and concurrency rule

Enrollment Service remains the only writer of enrollment records and the only
service that enforces capacity. Every enrollment creation, seat reservation,
cancellation, and status change that affects occupied seats must lock the same
per-Offering guard row in the Enrollment database in its transaction. Capacity
checks and the enrollment write commit in that transaction. The occupied-seat
definition must include every status or reservation that consumes a seat; the
Enrollment team must settle those statuses before this contract is adopted.

Enrollment Service stores an effective capacity fence for a pending reduction.
Enrollment transactions enforce the stricter of the current effective
capacity and any pending fence. This prevents a new enrollment from racing
past a reduction after the count has been checked. Course Service stores no
count or enrollment projection.

## Proposed API and protocol

All internal calls use the existing HMAC-SHA256 request signature, timestamp,
nonce, caller allowlist, request ID, and short timeouts. Enrollment accepts
these endpoints only from `course-service`. The Course-facing mutation is a
new dedicated administrative operation with an idempotency key and an
`expected_version`; the existing generic Offering update must continue to
reject these reductions until the protocol is implemented.

Before calling Enrollment, Course durably creates one pending change-operation
record for the Offering with the expected version, proposed capacity, and
`change_id`. A database uniqueness rule permits only one unresolved capacity
change per Offering. The operation has explicit `preparing`, `committed`, and
`aborting`/`aborted` states. A worker lease or equivalent compare-and-swap
prevents the original request and a reconciler from finalizing it concurrently.

1. **Prepare:** Course calls `POST /internal/offerings/{id}/capacity-changes`
   with `{change_id, expected_offering_version, previous_capacity,
   proposed_capacity}`. `change_id` is a stable UUID. In one Enrollment
   transaction, Enrollment locks the Offering guard row, rejects another
   unresolved change, computes occupied seats, rejects a finite proposed
   capacity below occupied seats (`409 capacity_below_occupancy`), and
   persists a `prepared` fence. It returns the change ID and accepted count
   snapshot. Retry with the same ID and identical payload returns the same
   result; a changed payload is `409`.
2. **Commit Course state:** Course performs its Offering version compare and
   swap in its own PostgreSQL transaction, changing capacity, incrementing
   version, writing an audit record, and marking the existing change operation
   `committed`. The transaction must verify that the operation is still owned
   by this worker and has not entered `aborting`. If the compare and swap
   fails, Course marks the operation `aborting` and calls Enrollment's proposed
   `POST /internal/offerings/{id}/capacity-changes/{change_id}/abort` and
   reports the version conflict. Abort is idempotent. If abort delivery fails,
   the fence remains conservative until reconciliation.
3. **Confirm:** After the Course transaction commits, Course calls the
   proposed `POST /internal/offerings/{id}/capacity-changes/{change_id}/confirm`
   with the committed Offering version. Enrollment atomically promotes the
   fence to effective capacity under the same guard lock. Confirmation is
   idempotent. If confirmation fails, Course retains the durable pending
   operation and returns a response that explicitly says the Course write
   committed but integration finalization is pending (for example `202` with
   an operation ID). It must not return an ambiguous error implying rollback.
4. **Reconcile:** Enrollment periodically resolves prepared operations and
   Course retries pending confirmations. Enrollment uses a **proposed** signed
   Course endpoint, `GET /internal/offerings/{id}/capacity-state`, to inspect
   durable operation state, committed version, capacity, and change ID. If
   Course committed the matching change, Enrollment confirms it. An old Course
   version alone is not proof that no Course transaction can commit: one may
   still be in flight. To abort, a Course reconciler first atomically claims
   the operation as `aborting` so the original commit cannot succeed, then
   sends the idempotent abort. If Course is unavailable or state is ambiguous,
   Enrollment keeps the fence and alerts operators. Prepared
   fences must **not** expire merely by elapsed time: expiry after a Course
   commit could admit enrollments above the committed limit.

The client can retry the Course mutation with the same idempotency key and
query operation status. Course must serialize capacity changes for an Offering
or reject a second change while one is unresolved. No distributed database
transaction or shared database is required. A temporary conservative
restriction on enrollment is acceptable; over-enrollment is not.

## Failure and consistency cases

| Failure | Required outcome |
| --- | --- |
| Enrollment unavailable before prepare | Course makes no capacity change; return `503` |
| Occupied seats exceed proposed finite capacity | Reject with `409`; no Course change |
| Enrollment commits during prepare | Guard-row serialization decides order; count and fence are atomic |
| Course version conflict after prepare | Abort fence; return `409` |
| Course crashes before its commit | Reconciler fences the pending Course operation against a late commit, then aborts the Enrollment fence |
| Course crashes after commit, before confirm | Fence remains enforced; reconciler confirms matching Course state |
| Confirm or abort request times out | Retry idempotently; never assume timeout means no effect |
| Course unavailable during reconciliation | Keep fence and surface an operational alert |
| Duplicate or replayed messages | HMAC nonce rejects transport replay; `change_id` makes business retries idempotent |

## Decisions needed before implementation

- Which enrollment and reservation statuses consume capacity, and how long
  reservations can remain pending.
- Whether Course may later relax a published capacity to unlimited, and what
  notification or cache invalidation Enrollment needs for increases.
- Exact external endpoint shape, error codes, operation-status retention, and
  reconciliation interval.
- Whether an Offering may be archived or closed while reservations or active
  enrollments exist, and how Enrollment learns those lifecycle changes.
