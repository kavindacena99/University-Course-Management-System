# Frontend/backend integration audit — Phase 6 source state

Audited at source commit `784552aa48332f6777706e99b5fd758ebd7bb1b7` before the Phase 6 changes, then verified against merged main `b6a8935a341cc29808a93dc61f822ca106762c98` and a live DEV gateway on 2026-11-28. The previous version of this audit predated the email login, shared API client, and live course list. The architecture document also predates the same-origin frontend Nginx `/api/` proxy. The Go handlers, current routes, and OpenAPI contracts were checked directly.

The browser calls only same-origin `/api/v1/*`. Auth issues HttpOnly access and refresh cookies plus a readable CSRF cookie. Access tokens are verified by every service. The frontend uses `GET /auth/me` on load and rotating `/auth/refresh` when needed. It takes the CSRF value from the Auth response, including on page restoration, because cookie names differ by environment. It does not persist bearer or refresh tokens in web storage. React route rules improve navigation; Go authorization remains authoritative.

| Feature | Frontend page | Existing API | Current status | Required work | Test |
|---|---|---|---|---|---|
| Email login and session | `Login`, `session.tsx` | `/auth/login`, `/auth/me`, `/auth/refresh`, `/auth/logout` | Live DEV login, refresh, reload and logout passed | Repeat with QA digest set | Live browser auth suite |
| Registration and recovery | `AccountFlows` | `/users`, `/auth/password/forgot`, `/auth/password/reset`, `/auth/activate` | Added on newer protected main and preserved in this branch | Live account lifecycle test | Browser and API tests |
| Route access | `App.tsx`, `Protected` | Auth roles and permissions from `/auth/me` | Student, lecturer, admin and anonymous DEV checks passed | Repeat in QA | Direct navigation, 401/403 |
| Course and offering catalog | `CoursesPage`, `CoursePage`, `OfferingsPage` | `/courses`, `/courses/:id`, `/courses/:id/offerings`, `/offerings`, `/offerings/:id` | Real records, IDs, pagination | Live data validation | Browser network and reload |
| Course creation | `CreateCoursePage` | `POST /admin/courses` | Restricted to `courses:manage`; real request | Live admin test | Admin success, student/lecturer denial |
| Lecturer offerings | `LecturerOfferingsPage`, `OfferingPage` | `/offerings/assigned`, `/offerings/:id`, `/:id/enrollments` | Assigned access and unassigned denial passed in DEV | Repeat in QA | Negative browser/API test |
| Lecturer overview and assessment list | `LecDashboard`, `LecModuleAssignment` | `/offerings/assigned`, offering assessments | Added on newer protected main and registered behind session guards | Live assignment/navigation test | Browser network test |
| Student enrolment | `StudentEnrollmentsPage`, `OfferingPage` | `/students/:id/enrollments`, `POST /enrollments` | Self ID and real requests wired | Verify enrollment rules and persistence | Browser create/reload |
| Assessments | `LecAssignmentSubmissionSettings`, `AssessmentPage` | Offering assessment list/create, assessment detail/publish | DEV create/publish/reload passed | Repeat in QA | Browser create/publish/reload |
| Submission files | `AssessmentPage`, `StudentSubmissionPage`, `SubmissionPage` | Assessment submit, submission detail, protected `/file` | DEV multipart upload, idempotency, protected byte-for-byte download and reload passed | Repeat in QA | Browser upload/download/reload |
| Grades | `SubmissionPage` | `PUT /grade`, `POST /release` | DEV persisted grade, release and student reload passed | Repeat in QA | Browser save/release/reload |
| Analytics | `StudentResults`, lecturer offering report, `Admin_Dashboard` | `/analytics/students/me`, `/analytics/lecturer/offerings/:id`, `/analytics/admin/summary` | Renders response data without sample metrics | Verify projection availability and rebuilding behavior | Browser and API tests |
| Student assignments and results | `StudentAssignments`, `StudentResults` | Self enrollments, offering assessments, submission, student report | Added on newer protected main and preserved in this branch | Live student workflow test | Browser upload/report test |
| Administrator dashboard | `Admin_Dashboard` | Admin summary, courses, users | Added on newer protected main and registered with role/permission guard | Live admin test | Browser/API authorization test |

Protected main advanced during this branch with account, student, lecturer and administrator screens that make real API calls. Those pages were merged and their functional routes preserved. The older `LecAssignmentSubmission` and `AssignmentSubmissionSetting` placeholder paths are not registered; the latter was removed by the newer main branch. The backend has no matching contract for an academic calendar, notice board, or Google sign-in, so these controls are not presented as live operations.

## Verification boundary

The frontend production image build and TypeScript check passed in Docker. Five of five Playwright tests passed against the real DEV gateway with dedicated synthetic accounts and offering records; none were skipped. The suite observed real API responses and persisted state after reload, including the exact bytes of a protected submission download. Its report and traces are under ignored `tests/browser-phase6/artifacts/`. Registration/password recovery, pagination extremes, projection rebuilding, and a new QA digest deployment remain outside this DEV acceptance run. DEV success is not a QA verdict or production readiness claim.
