# Security

## Credentials

- Passwords are hashed with Argon2id using per-password salts.
- Parameters are configurable with `ARGON2_*`; defaults are 64 MiB, 3
  iterations, parallelism 2, 16-byte salt, and 32-byte key.
- Password reuse is checked against the current hash.
- Password hashes and tokens are never logged or returned in API responses.

## Access tokens

- Ed25519-signed JWT access tokens.
- Short lifetime (default 10 minutes).
- Include user ID, session ID, roles, and permissions.
- Verified by the User Service using configured public keys, issuer, audience,
  and clock skew.
- The algorithm is pinned to EdDSA and `kid` selects the key.

Generate a development key pair:

```bash
cd services/security-and-authentication
go run ./cmd/keygen --out-dir ./var/keys
```

Store the private key outside version control. Configure the matching public
key in the User Service with `AUTH_JWT_PUBLIC_KEYS_JSON`.

## Refresh tokens and sessions

- Refresh tokens are random opaque values; only SHA-256 hashes are stored.
- Tokens rotate on every refresh and are single-use.
- Presenting a consumed token revokes the whole session and is audited.
- Sessions have an absolute lifetime (default 7 days).
- Logout revokes the session and its refresh tokens.
- Password reset and role-permission changes revoke affected sessions.

## Cookie mode and CSRF

- Access, refresh, and CSRF cookies can be enabled with `AUTH_COOKIE_ENABLED`.
- Access and refresh cookies are `HttpOnly`.
- Unsafe requests authenticated by cookie require `X-CSRF-Token` to match the
  CSRF cookie using a constant-time comparison.
- Set `AUTH_COOKIE_SECURE=true` and `SameSite=None` with HTTPS in production.

## Authorization

Two layers are enforced:

- Roles in JWT claims for coarse compatibility.
- Permissions in JWT `perms` for admin operations.

The permission catalog is:

- `users:read`
- `users:manage`
- `roles:manage`
- `sessions:read`
- `sessions:revoke`
- `account:unlock`
- `audit:read`

Admin routes declare the exact permission they require. If a role's
permissions change, sessions of users holding that role are revoked. Users
must log in again to receive new claims; previously issued access tokens remain
valid only until their short TTL expires (default 10 minutes).

## Internal service authentication

- HMAC-SHA256 over method, request URI, service ID, timestamp, nonce, and body
  hash.
- Allowed caller list and clock-skew limit are explicit.
- Nonces are tracked to prevent replay.
- Previous secrets are supported for rotation.
- Internal endpoints are not exposed by the gateway.

## Fail-closed eligibility

Login and refresh call the User Service eligibility endpoint. The
Authentication Service:

- Rejects users who are not approved.
- Revokes sessions on refresh when a user becomes ineligible.
- Returns `503 eligibility_unavailable` when eligibility cannot be verified.

## Rate limiting and lockout

- Login and forgot-password endpoints are rate limited in the service.
- Failed logins increment a counter and lock the account after the configured
  threshold.
- The gateway also applies rate limits to login, registration, and general API
  traffic.
- The gateway overwrites `X-Forwarded-For` at the edge. Services trust
  forwarding headers only from `TRUSTED_PROXIES`; in Compose this is the Docker
  address pool. Behind a cloud load balancer, configure `set_real_ip_from` and
  `real_ip_header` in the gateway before trusting the forwarded client address.

## Audit trail

Security events are stored in `auth_audit_events` with actor, subject,
event type, success, details, and timestamp. Audit records are exposed only to
callers with `audit:read`.

## Transport and headers

- The gateway sets `X-Content-Type-Options`, `X-Frame-Options`,
  `Referrer-Policy`, `Permissions-Policy`, CSP, and `Cache-Control`.
- Use TLS in front of NGINX in production. HSTS is intentionally not set by the
  local HTTP configuration.
- Service responses set their own no-store and security headers.

## Operational guidance

- Never commit `.env` files, private keys, or shared secrets.
- Use separate database users per service with least privilege.
- Back up databases before applying migrations.
- Rotate `INTERNAL_SERVICE_SECRET` using the previous-secret window.
- Monitor `auth_audit_events` for repeated failures, lockouts, and refresh
  reuse.
