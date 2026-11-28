# Frontend/backend integration audit — Phase 6 source state

Audited at source commit `784552aa48332f6777706e99b5fd758ebd7bb1b7` before the Phase 6 changes, then updated for this branch. The previous version of this audit predated the email login, shared API client, and live course list. The architecture document also predates the same-origin frontend Nginx `/api/` proxy. The Go handlers, current routes, and OpenAPI contracts were checked directly.

The browser calls only same-origin `/api/v1/*`. Auth issues HttpOnly access and refresh cookies plus a readable CSRF cookie. Access tokens are verified by every service. The frontend uses `GET /auth/me` on load and rotating `/auth/refresh` when needed. It takes the CSRF value from the Auth response, including on page restoration, because cookie names differ by environment. It does not persist bearer or refresh tokens in web storage. React route rules improve navigation; Go authorization remains authoritative.

| Feature | Frontend page | Existing API | Current status | Required work | Test |
|---|---|---|---|---|---|
| Email login and session | `Login`, `session.tsx` | `/auth/login`, `/auth/me`, `/auth/refresh`, `/auth/logout` | Integrated in source; live verification pending | Run cookie, expiry, reload and revocation tests against a running stack | Live browser auth suite |
| Registration and recovery | `AccountFlows` | `/users`, `/auth/password/forgot`, `/auth/password/reset`, `/auth/activate` | Added on newer protected main and preserved in this branch | Live account lifecycle test | Browser and API tests |
| Route access | `App.tsx`, `Protected` | Auth roles and permissions from `/auth/me` | Role and `courses:manage` guards implemented | Verify with student, lecturer and admin accounts | Direct navigation, 401/403 |
| Course and offering catalog | `CoursesPage`, `CoursePage`, `OfferingsPage` | `/courses`, `/courses/:id`, `/courses/:id/offerings`, `/offerings`, `/offerings/:id` | Real records, IDs, pagination | Live data validation | Browser network and reload |
| Course creation | `CreateCoursePage` | `POST /admin/courses` | Restricted to `courses:manage`; real request | Live admin test | Admin success, student/lecturer denial |
| Lecturer offerings | `LecturerOfferingsPage`, `OfferingPage` | `/offerings/assigned`, `/offerings/:id`, `/:id/enrollments` | Assignment list and backend authorization wired | Verify unassigned denial | Negative browser/API test |
| Lecturer overview and assessment list | `LecDashboard`, `LecModuleAssignment` | `/offerings/assigned`, offering assessments | Added on newer protected main and registered behind session guards | Live assignment/navigation test | Browser network test |
| Student enrolment | `StudentEnrollmentsPage`, `OfferingPage` | `/students/:id/enrollments`, `POST /enrollments` | Self ID and real requests wired | Verify enrollment rules and persistence | Browser create/reload |
| Assessments | `CreateAssessmentPage`, `AssessmentPage` | Offering assessment list/create, assessment detail/publish | Real resources and versions wired | Verify workflow with assigned lecturer | Browser create/publish/reload |
| Submission files | `AssessmentPage`, `StudentSubmissionPage`, `SubmissionPage` | Assessment submit, submission detail, protected `/file` | Multipart, idempotency header, protected blob download wired | Verify file content and persistence | Browser upload/download/reload |
| Grades | `SubmissionPage` | `PUT /grade`, `POST /release` | Uses persisted grade version | Verify student sees released grade only | Browser save/release/reload |
| Analytics | Student report, lecturer offering report, admin dashboard | `/analytics/students/me`, `/analytics/lecturer/offerings/:id`, `/analytics/admin/summary` | Renders response data without sample metrics | Verify projection availability and rebuilding behavior | Browser and API tests |
| Student assignments and results | `StudentAssignments`, `StudentResults` | Self enrollments, offering assessments, submission, student report | Added on newer protected main and preserved in this branch | Live student workflow test | Browser upload/report test |
| Administrator dashboard | `Admin_Dashboard` | Admin summary, courses, users | Added on newer protected main and registered with role/permission guard | Live admin test | Browser/API authorization test |

Protected main advanced during this branch with account, student, lecturer and administrator screens that make real API calls. Those pages were merged and their functional routes preserved. The older `LecAssignmentSubmission` and `AssignmentSubmissionSetting` placeholder paths are not registered; the latter was removed by the newer main branch. The backend has no matching contract for an academic calendar, notice board, or Google sign-in, so these controls are not presented as live operations.

## Verification boundary

The frontend production build and TypeScript check pass in an isolated temporary copy. Live browser/API tests have **not** run: `docker ps` found no running stack, and this task prohibits deploying QA or PROD-like. Dedicated synthetic accounts and a running development gateway are required. No QA verdict, RC status, or production readiness is inferred from source compilation.
