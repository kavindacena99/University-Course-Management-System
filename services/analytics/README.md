# Analytics Service

Analytics owns rebuildable read projections and reports. It does not own marks,
course credits, academic periods, lecturer assignments, or enrollment records.
Assessment is the source of published component grades and imported final
results; Course is consulted for current offering facts.

## API

All public routes require an Auth-issued access JWT. Cookie-authenticated
mutations also require the configured CSRF header.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/v1/analytics/students/me` | Own final results, component grades, term GPA, and cumulative GPA |
| GET | `/api/v1/analytics/lecturer/offerings/{id}` | Results and released component grades for a currently assigned offering |
| GET | `/api/v1/analytics/admin/summary` | Aggregate score/grade distribution and programme/category summaries |
| GET/PUT | `/api/v1/analytics/admin/grade-bands` | Read or replace the configured grade-to-point scale |
| GET/POST | `/api/v1/analytics/admin/programmes` | List or create dynamic programme/category policies |
| PUT | `/api/v1/analytics/admin/programmes/{id}` | Update a programme and its GPA policies |
| PUT | `/api/v1/analytics/admin/students/{id}/programme` | Assign a student's programme |
| GET | `/api/v1/analytics/admin/students/{id}` | Read an individual student's report (explicit permission required) |
| POST | `/api/v1/analytics/admin/rebuild` | Rebuild projections from Assessment's retained outbox event history |

`analytics:admin:student:read` is deliberately not granted to the Admin role
by default. Aggregate reporting and Analytics configuration use separate
permissions.

### Grade scale and GPA policies

Grade bands are configured through the admin API; no thresholds are guessed or
seeded. Each band has a code, inclusive minimum percentage, exclusive maximum
percentage (100 is inclusive), grade points from 0.00 to 4.00, and display
order. Bands may not overlap. The approved scale should be configured before
relying on GPA reports.

Programmes, categories, repeat treatment (`best`, `latest`, or `all`), failed
result treatment (`include` or `exclude`), and the programme pass-point
threshold are admin-managed data. The service does not hard-code institution
or programme names.

GPA uses official imported final results only. Published component grades are
available for progress reporting but do not become a course's final result.
Term and cumulative GPA are credit-weighted; cumulative GPA is calculated from
the selected course attempts rather than averaging term GPAs. Calculations use
unrounded values, with GPA displayed to two decimal places. A report explicitly
indicates when its grade scale, programme, or result coverage is incomplete.

### Event projection and rebuild

Assessment sends signed `GradePublished` and `FinalResultImported` events to
`POST /internal/events`. Analytics validates the event envelope, stores each
event in its inbox archive, and updates its projection in one transaction.
Duplicate `event_id` values are acknowledged without applying the event twice.

The admin rebuild reads all retained events from Assessment using the signed,
non-gateway `GET /internal/analytics/events` cursor API. Reports return
`503 projection_unavailable` while rebuilding or after a failed rebuild. Failed
rebuilds can be retried by an administrator; the source outbox is not deleted
by event delivery.

For student reports and lecturer authorization, Analytics calls Course's
signed internal Offering facts endpoint. It never reads another service's
database. Assessment's source outbox and the Analytics inbox both retain the
event payloads needed to reconstruct the projection.

## Local development

Copy `.env.example` to `.env`, configure a matching Auth public key and shared
internal service secret, then run:

```sh
go run ./cmd/migrate -direction up
go run .
```

Set `COURSE_SERVICE_INTERNAL_URL` and `ASSESSMENT_SERVICE_INTERNAL_URL` to
Course and Assessment. Add `analytics-service` to Course's internal caller
allowlist and `assessment-service` to Analytics's allowlist. Assessment must
allow `analytics-service` for historical event reads.
