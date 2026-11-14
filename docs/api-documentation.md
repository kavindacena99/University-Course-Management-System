# API Documentation

Machine-readable OpenAPI 3 specifications:

- [Authentication Service](openapi/auth-service.yaml)
- [User Service](openapi/user-service.yaml)
- [Course Service](openapi/course-service.yaml)

All public routes are available through the gateway at `http://localhost:8080`
with the same paths.

## Conventions

- JSON request and response bodies (`Content-Type: application/json`).
- Authenticated requests use `Authorization: Bearer <access_token>` or the
  `access_token` cookie.
- Cookie-authenticated unsafe requests must send `X-CSRF-Token`.
- IDs are UUIDs.
- Timestamps are RFC3339 UTC strings.
- Pagination uses `limit` and `offset`.
- Errors use this shape:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "The request is invalid.",
    "request_id": "uuid"
  }
}
```

## Common status codes

| Status | Meaning |
| --- | --- |
| 200 | Success |
| 201 | Created |
| 204 | Success with no body |
| 400 | Validation or malformed request |
| 401 | Missing/invalid authentication |
| 403 | Authenticated but forbidden |
| 404 | Not found |
| 409 | Version conflict or duplicate |
| 429 | Rate limited |
| 503 | Dependency or eligibility unavailable |

## Authentication summary

| Method | Path | Auth |
| --- | --- | --- |
| POST | `/api/v1/users` | none (rate limited) |
| POST | `/api/v1/auth/login` | none (rate limited) |
| POST | `/api/v1/auth/refresh` | refresh token |
| POST | `/api/v1/auth/logout` | refresh token |
| POST | `/api/v1/auth/activate` | action token |
| POST | `/api/v1/auth/password/forgot` | none (rate limited) |
| POST | `/api/v1/auth/password/reset` | action token |

## User endpoints

| Method | Path | Permission |
| --- | --- | --- |
| GET | `/api/v1/users/me` | authenticated |
| PATCH | `/api/v1/users/me` | authenticated |
| GET | `/api/v1/users/{id}` | self or `users:read` |
| GET | `/api/v1/admin/users` | `users:read` |
| GET | `/api/v1/admin/users/{id}` | `users:read` |
| GET | `/api/v1/admin/users/{id}/history` | `users:read` |
| POST | `/api/v1/admin/users/{id}/approve` | `users:manage` |
| POST | `/api/v1/admin/users/{id}/reject` | `users:manage` |
| POST | `/api/v1/admin/users/{id}/suspend` | `users:manage` |
| POST | `/api/v1/admin/users/{id}/reinstate` | `users:manage` |

## Authentication administration

| Method | Path | Permission |
| --- | --- | --- |
| GET | `/api/v1/auth/me` | authenticated |
| GET | `/api/v1/auth/sessions` | authenticated |
| DELETE | `/api/v1/auth/sessions/{id}` | authenticated |
| POST | `/api/v1/auth/logout-all` | authenticated |
| POST | `/api/v1/auth/password/change` | authenticated |
| GET | `/api/v1/admin/auth/roles` | `roles:manage` |
| GET | `/api/v1/admin/auth/permissions` | `roles:manage` |
| GET | `/api/v1/admin/auth/roles/{code}/permissions` | `roles:manage` |
| PUT | `/api/v1/admin/auth/roles/{code}/permissions` | `roles:manage` |
| PUT | `/api/v1/admin/auth/users/{id}/roles` | `roles:manage` |
| POST | `/api/v1/admin/auth/users/{id}/unlock` | `account:unlock` |
| DELETE | `/api/v1/admin/auth/users/{id}/sessions` | `sessions:revoke` |
| GET | `/api/v1/admin/auth/audit` | `audit:read` |

## Internal endpoints

Not exposed by the gateway. They require HMAC service authentication; see
[service-integration.md](service-integration.md).

- `POST /internal/users`
- `POST /internal/events/users`
- `GET /internal/users/{id}`
- `GET /internal/users/{id}/eligibility`
- `POST /internal/bootstrap`

## Health

- Gateway: `GET http://localhost:8080/health`
- Auth Service: `GET http://localhost:8081/health`, `/health/ready`
- User Service: `GET http://localhost:8082/health`, `/health/ready`
