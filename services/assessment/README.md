# Assessment Service

Assessment owns component assessments, submissions, marks, grade revisions,
and approved imported final results. It does not own enrollments, lecturer
assignments, grade-band rules, or GPA calculations. It reads eligibility from
Enrollment's signed internal API and stores submission files in a private
MinIO bucket.

## API

All public routes require an Auth-issued access JWT. Mutating cookie-authenticated
requests also require the configured CSRF header. Errors use the common
`{"error":{"code":"...","message":"...","request_id":"..."}}` envelope.
See [OpenAPI](../../docs/openapi/assessment-service.yaml) for request and
response schemas.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/v1/offerings/{offering_id}/assessments` | Create a draft component assessment |
| GET | `/api/v1/offerings/{offering_id}/assessments` | List assessments visible to the caller |
| GET | `/api/v1/assessments/{id}` | Read an assessment |
| POST | `/api/v1/assessments/{id}/publish` | Publish a draft (requires `version`) |
| POST | `/api/v1/assessments/{id}/close` | Close a published assessment (requires `version`) |
| POST | `/api/v1/assessments/{id}/extensions` | Grant a per-student deadline extension |
| POST | `/api/v1/assessments/{id}/submissions` | Upload a submission (multipart `file`, `Idempotency-Key`) |
| GET | `/api/v1/assessments/{id}/submissions` | List submissions for authorized teaching staff |
| GET | `/api/v1/submissions/{id}` | Read an owned/authorized submission and released grade |
| GET | `/api/v1/submissions/{id}/file` | Download a private submission |
| PUT | `/api/v1/submissions/{id}/grade` | Create or revise a draft grade |
| POST | `/api/v1/submissions/{id}/release` | Release a grade and create its outbox event |
| POST | `/api/v1/offerings/{offering_id}/results/import` | Idempotently import approved final results from CSV |
| GET | `/api/v1/offerings/{offering_id}/results` | List authorized final results |

Students can access only their own submissions and results. Lecturers must be
assigned to the offering. Admin access to student-level submission and result
details requires the explicit `assessments:student:read` permission. Final
result imports require `assessments:results:import` (granted to Admin by the
Auth migration).

### Approved final-result CSV

The upload must be a `.csv` file of at most 5 MiB with this exact header and
between 1 and 1000 unique student rows:

```csv
student_user_id,final_mark,max_mark
7e7e6a76-544e-4ad4-a6c1-813241a5e1b5,78.50,100
```

Marks are non-negative, have at most two decimal places, and cannot exceed
`max_mark`. Grade codes and grade points are deliberately not imported;
Analytics derives them from its configured grade scheme. Each batch records
the importing user and is idempotent per offering and `Idempotency-Key`.
Re-imported results retain revision history. The import is an explicit
permission-gated administrative approval boundary; it does not independently
re-verify historical Enrollment membership.

### Submission uploads

The service limit defaults to 20 MiB. Allowed extensions are CSV, DOCX, JPEG,
JPG, PDF, PNG, PPTX, TXT, and XLSX. Content type is detected from the file
bytes and must match both the extension and the assessment's allowed list.
Object storage is private and the Assessment container receives a bucket-
scoped MinIO account with only list/location and object read/write/delete
rights. Downloads require the same resource-level access checks as submission
reads.

## Data and event delivery

Assessment has an independently owned PostgreSQL database. The migrations
create assessment, submission, grade, revision, final-result, and outbox
tables. No cross-service foreign keys or database reads are used.

Released component grades and imported final results are committed with an
outbox event in the same transaction. If `ANALYTICS_SERVICE_INTERNAL_URL` is
configured, a worker retries signed `POST /internal/events` delivery with
exponential backoff and eventually marks exhausted events dead. Delivery is
at-least-once; Analytics must deduplicate by `event_id`. If Analytics is not
configured, events remain safely pending.

## Local development

Copy `.env.example` to `.env`, configure a valid Auth Ed25519 public key and
the shared internal service secret, then start PostgreSQL and MinIO. The
configured private bucket and scoped MinIO account must exist before the API
starts. Compose provisions them with a bootstrap job; the Assessment API never
receives MinIO root credentials. Standalone runs must provision the equivalent
bucket and policy separately. The Assessment database role must own an
independent `assessments` database.
Configure `ENROLLMENT_SERVICE_INTERNAL_URL` to the Enrollment service and add
`assessment-service` to Enrollment's signed-caller allowlist.

From this directory:

```sh
go run ./cmd/migrate -direction up
go run ./cmd/migrate -direction version
go run .
```

The repository Compose stack includes Assessment PostgreSQL, the private
object store, migration and API containers; see
[`../../infrastructure/docker/compose.yaml`](../../infrastructure/docker/compose.yaml).
The stack uses the maintained `pgsty/silo` object-storage fork because MinIO
community-edition images are no longer published. Its `.env` files and
generated Auth keys are intentionally not committed.

## Validation

```sh
go test -count=1 ./...
go vet ./...
go build ./...
```
