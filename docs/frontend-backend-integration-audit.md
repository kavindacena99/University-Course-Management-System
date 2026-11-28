# Frontend / Backend Integration Audit

## Service map

The public gateway is `http://localhost:8080` by default. The frontend should use
`VITE_API_BASE_URL` and send cookie credentials. Unsafe cookie-authenticated
requests must include `X-CSRF-Token`.

| Service | Port | Public prefix |
|---|---:|---|
| Authentication | 8081 | `/api/v1/auth`, `/api/v1/admin/auth` |
| User | 8082 | `/api/v1/users`, `/api/v1/admin/users` |
| Course | 8083 | `/api/v1/courses`, `/api/v1/offerings`, `/api/v1/admin/courses`, `/api/v1/admin/offerings` |
| Enrollment | 8084 | `/api/v1/enrollments`, `/api/v1/students`, `/api/v1/offerings/:id/enrollments` |
| Assessment | 8085 | `/api/v1/offerings`, `/api/v1/assessments`, `/api/v1/submissions` |
| Analytics | 8086 | `/api/v1/analytics` |

## Public endpoint inventory

Detailed payload and response schemas are in:

- [Authentication OpenAPI](openapi/auth-service.yaml)
- [Course OpenAPI](openapi/course-service.yaml)
- [Enrollment OpenAPI](openapi/enrollment-service.yaml)
- [Assessment OpenAPI](openapi/assessment-service.yaml)
- [Analytics OpenAPI](openapi/analytics-service.yaml)
- [User API documentation](api-documentation.md)

### Authentication

| Method | Path | Authentication / authorization |
|---|---|---|
| POST | `/api/v1/auth/login` | Public; login rate limit |
| POST | `/api/v1/auth/refresh` | Refresh token |
| POST | `/api/v1/auth/logout` | Refresh token |
| POST | `/api/v1/auth/logout-all` | Authenticated |
| GET | `/api/v1/auth/sessions` | Authenticated |
| DELETE | `/api/v1/auth/sessions/:id` | Authenticated |
| GET | `/api/v1/auth/me` | Authenticated |
| POST | `/api/v1/auth/activate` | Action token |
| POST | `/api/v1/auth/password/change` | Authenticated |
| POST | `/api/v1/auth/password/forgot` | Public; forgot rate limit |
| POST | `/api/v1/auth/password/reset` | Action token |

Login JSON: `{ "email": string, "password": string }`.
Login response: `{ "user": UserSummary, "access_token?", "refresh_token?",
"token_type", "expires_in", "refresh_expires_in", "csrf_token?" }`.

### User

| Method | Path | Authorization |
|---|---|---|
| POST | `/api/v1/users` | Public registration; registration rate limit |
| GET | `/api/v1/users/me` | Authenticated |
| PATCH | `/api/v1/users/me` | Authenticated |
| GET | `/api/v1/users/:id` | Self or `users:read` |
| GET | `/api/v1/admin/users` | `users:read` |
| GET | `/api/v1/admin/users/:id` | `users:read` |
| GET | `/api/v1/admin/users/:id/history` | `users:read` |
| POST | `/api/v1/admin/users/:id/approve` | `users:manage` |
| POST | `/api/v1/admin/users/:id/reject` | `users:manage` |
| POST | `/api/v1/admin/users/:id/suspend` | `users:manage` |
| POST | `/api/v1/admin/users/:id/reinstate` | `users:manage` |

Query parameters for user admin list: `status`, `search`, `limit`, `offset`.

### Course

| Method | Path | Authorization |
|---|---|---|
| GET | `/api/v1/courses` | Authenticated |
| GET | `/api/v1/courses/:id` | Authenticated |
| GET | `/api/v1/courses/:id/offerings` | Authenticated |
| GET | `/api/v1/offerings` | Authenticated |
| GET | `/api/v1/offerings/assigned` | Authenticated |
| GET | `/api/v1/offerings/:id` | Authenticated |
| PUT | `/api/v1/offerings/:id` | `offerings:manage` |
| POST | `/api/v1/offerings/:id/publish` | `offerings:manage` |
| POST | `/api/v1/offerings/:id/close` | `offerings:manage` |
| POST | `/api/v1/offerings/:id/archive` | `offerings:manage` |
| POST | `/api/v1/admin/courses` | `courses:manage` |
| PUT | `/api/v1/admin/courses/:id` | `courses:manage` |
| POST | `/api/v1/admin/courses/:id/publish` | `courses:manage` |
| POST | `/api/v1/admin/courses/:id/archive` | `courses:manage` |
| POST | `/api/v1/admin/courses/:id/restore` | `courses:manage` |
| POST | `/api/v1/admin/offerings` | `courses:manage` |
| PUT | `/api/v1/admin/offerings/:id/lecturer` | `courses:manage` |
| POST | `/api/v1/admin/offerings/:id/restore` | `courses:manage` |

Create-course payload:
`{ "code": string, "title": string, "description": string, "credits": number }`.

Course list filters: `status`, `search`, `limit`, `offset`, `sort`, `order`.

### Enrollment

| Method | Path | Authorization |
|---|---|---|
| POST | `/api/v1/enrollments` | Student self or `enrollments:manage` |
| GET | `/api/v1/enrollments/:id` | Student owner, admin/lecturer |
| POST | `/api/v1/enrollments/:id/cancel` | Student owner or admin |
| GET | `/api/v1/students/:id/enrollments` | Student owner or `enrollments:read` |
| GET | `/api/v1/offerings/:id/enrollments` | Admin or eligible assigned lecturer |

Create-enrollment payload:
`{ "offering_id": UUID, "student_user_id": UUID }`.

### Assessment

| Method | Path | Authorization |
|---|---|---|
| POST | `/api/v1/offerings/:offering_id/assessments` | Authenticated |
| GET | `/api/v1/offerings/:offering_id/assessments` | Authenticated |
| GET | `/api/v1/offerings/:offering_id/results` | Authenticated |
| POST | `/api/v1/offerings/:offering_id/results/import` | Authenticated |
| GET | `/api/v1/assessments/:id` | Authenticated |
| POST | `/api/v1/assessments/:id/publish` | Authenticated |
| POST | `/api/v1/assessments/:id/close` | Authenticated |
| POST | `/api/v1/assessments/:id/extensions` | Authenticated |
| POST | `/api/v1/assessments/:id/submissions` | Authenticated |
| GET | `/api/v1/assessments/:id/submissions` | Authenticated |
| GET | `/api/v1/submissions/:id` | Authenticated |
| GET | `/api/v1/submissions/:id/file` | Authenticated |
| PUT | `/api/v1/submissions/:id/grade` | Authenticated |
| POST | `/api/v1/submissions/:id/release` | Authenticated |

### Analytics

| Method | Path | Authorization |
|---|---|---|
| GET | `/api/v1/analytics/students/me` | Student |
| GET | `/api/v1/analytics/lecturer/offerings/:id` | Lecturer |
| GET | `/api/v1/analytics/admin/summary` | Admin |
| GET | `/api/v1/analytics/admin/grade-bands` | Admin |
| PUT | `/api/v1/analytics/admin/grade-bands` | Admin |
| GET | `/api/v1/analytics/admin/programmes` | Admin |
| POST | `/api/v1/analytics/admin/programmes` | Admin |
| PUT | `/api/v1/analytics/admin/programmes/:id` | Admin |
| PUT | `/api/v1/analytics/admin/students/:id/programme` | Admin |
| GET | `/api/v1/analytics/admin/students/:id` | Admin |
| POST | `/api/v1/analytics/admin/rebuild` | Admin |

## Frontend findings

- 7 routes currently exist, but only static/mock state is implemented.
- Login uses `username` and never calls the authentication API; the backend requires `email`.
- The frontend has no API client, auth context, domain types, loading/error hooks, or shared layout.
- `Courses.tsx` and `LecMyCourses.tsx` implement nearly identical course pages.
- `LecCreateCourse.tsx` imports `Courses.css` and submits `{ title, code, department, capacity, coordinator, description }`; the backend expects `{ code, title, description, credits }`.
- `LecModuleAssignment.tsx` references `/assets/course-management/...` while public assets are `/assets/...`, so icons can resolve to missing paths.
- Several screens use repeated sidebar/header implementations and placeholder `window.alert`/local-only state.
- Route names use uppercase and inconsistent paths (`/LecMyCourses`, `/LecCreateCourse`). Canonical lowercase routes should be added while preserving compatibility aliases.
- Public assets include many Figma-exported files; not all are referenced and no evidence proves which are duplicate/dead.

## Duplicate / cleanup candidates

Remove `LecMyCourses.tsx` after routes are consolidated because `Courses.tsx`
and it duplicate the course overview. The existing course, module, view, and
submission CSS files are distinct and should remain until each view is audited
visually. Remove only confirmed dead commented code, not asset exports without
runtime verification.

## Proposed design palette

- Navy: `#1E293B`
- Primary blue: `#2563EB`
- Secondary teal: `#0F766E`
- Surface: `#F8FAFC`
- Text: `#0F172A`
- Muted text: `#64748B`
- Success: `#15803D`
- Warning: `#B45309`
- Error: `#B91C1C`
- Border: `#CBD5E1`
