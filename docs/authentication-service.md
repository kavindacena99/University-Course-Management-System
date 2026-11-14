# Authentication Service

Location: `services/security-and-authentication`
Default port: `8081`
Database: `authentication_and_security`

## Responsibilities

- Password verification with Argon2id.
- JWT access-token issuance and verification keys.
- Session and refresh-token lifecycle with rotation and reuse detection.
- Role and permission catalogs, role-permission mapping, and user roles.
- Account activation, password change, and password reset.
- Lockout, login rate limiting, and security audit events.
- Internal provisioning and user-event ingestion from the User Service.

## Authentication flows

### Login

`POST /api/v1/auth/login`

- Validates the request and verifies the password before exposing account
  state, reducing account enumeration.
- Applies lockout and failed-attempt accounting.
- Requires the user to be approved, activated, enabled, and eligible according
  to the User Service.
- Creates a session and writes audit events (`login_succeeded` or
  `login_failed`).
- Returns access and refresh tokens. With cookie delivery the tokens are also
  set as `HttpOnly` cookies; the CSRF token is returned for the client to echo
  in `X-CSRF-Token`.

### Refresh

`POST /api/v1/auth/refresh`

- Accepts the refresh token from the body or the refresh cookie.
- Rejects unknown, expired, used, or revoked tokens.
- If a used token is presented again, the whole session is revoked and
  `refresh_reuse_detected` is audited.
- Rechecks eligibility; an ineligible user has the session revoked.
- Rotates the refresh token within the same session up to the session's
  absolute expiry.

### Logout

- `POST /api/v1/auth/logout` revokes the session for the supplied refresh token
  and clears cookies.
- `POST /api/v1/auth/logout-all` revokes all sessions for the authenticated
  user.

### Password reset

- `POST /api/v1/auth/password/forgot` always returns a generic success response
  to avoid account enumeration.
- `POST /api/v1/auth/password/reset` consumes a single-use token and sets the
  new password. Previous sessions are revoked.

## Tokens

Access tokens are JWTs signed with Ed25519 and include:

- `sub` - user ID
- `sid` - session ID
- `roles` - role codes
- `perms` - permission codes
- `typ` - `access`
- `iss`, `aud`, `exp`, `iat`

Defaults:

- Access token TTL: `10m`
- Refresh token TTL: `24h`
- Session absolute TTL: `168h`

Refresh tokens are random opaque values. Only their SHA-256 hashes are stored.
Rotated tokens are linked through `replaced_by_id`.

## Roles and permissions

Roles: `ADMIN`, `STAFF`, `LECTURER`, `STUDENT`.

Permissions:

| Permission | Purpose |
| --- | --- |
| `users:read` | View users in the User Service |
| `users:manage` | Approve, reject, suspend, reinstate users |
| `roles:manage` | Manage roles and role permissions |
| `sessions:read` | View sessions |
| `sessions:revoke` | Revoke user sessions |
| `account:unlock` | Unlock locked accounts |
| `audit:read` | View security audit events |

Default mappings (migration `000002`):

- `ADMIN`: all permissions
- `STAFF`: `users:read`, `sessions:read`, `sessions:revoke`, `account:unlock`
- `LECTURER`: `users:read`
- `STUDENT`: none

Changing a role's permissions revokes all active sessions belonging to users
with that role, preventing them from refreshing into stale claims. Already
issued access tokens expire within the short access-token TTL.

## Endpoints

Public auth endpoints:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/activate`
- `POST /api/v1/auth/password/forgot`
- `POST /api/v1/auth/password/reset`

Authenticated user endpoints:

- `GET /api/v1/auth/me`
- `GET /api/v1/auth/sessions`
- `DELETE /api/v1/auth/sessions/{id}`
- `POST /api/v1/auth/logout-all`
- `POST /api/v1/auth/password/change`

Admin endpoints:

- `GET /api/v1/admin/auth/roles` - `roles:manage`
- `GET /api/v1/admin/auth/permissions` - `roles:manage`
- `GET /api/v1/admin/auth/roles/{code}/permissions` - `roles:manage`
- `PUT /api/v1/admin/auth/roles/{code}/permissions` - `roles:manage`
- `PUT /api/v1/admin/auth/users/{id}/roles` - `roles:manage`
- `POST /api/v1/admin/auth/users/{id}/unlock` - `account:unlock`
- `DELETE /api/v1/admin/auth/users/{id}/sessions` - `sessions:revoke`
- `GET /api/v1/admin/auth/audit` - `audit:read`

Internal endpoints:

- `POST /internal/users` - provision a user.
- `POST /internal/events/users` - apply a user lifecycle event.

Internal endpoints require HMAC service authentication.

## Audit events

`auth_audit_events` records security-relevant actions, including:

- `login_succeeded`, `login_failed`
- `password_changed`, `password_reset_completed`
- `account_activated`, `account_provisioned`
- `roles_updated`, `role_permissions_updated`
- `sessions_revoked`
- `user_event_applied`
- `refresh_reuse_detected`

Records include actor ID, subject ID, success flag, JSON details, and creation
time. `GET /api/v1/admin/auth/audit` supports `limit` (1-200) and `offset`.

## Configuration

See `services/security-and-authentication/.env.example`. Important values:

- `AUTH_JWT_PRIVATE_KEY_FILE` or `AUTH_JWT_PRIVATE_KEY_PEM`
- `AUTH_JWT_KEY_ID`
- `INTERNAL_SERVICE_SECRET` and `INTERNAL_ALLOWED_CALLERS`
- `USER_SERVICE_INTERNAL_URL` and `USER_ELIGIBILITY_TIMEOUT`
- `NOTIFICATIONS_MODE` and `NOTIFICATIONS_FILE_PATH`
- `AUTH_MAX_FAILED_LOGIN_ATTEMPTS` and lockout durations
