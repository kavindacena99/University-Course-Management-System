# Course Service deployment-readiness audit

Audit date: 2026-11-14. Scope: Course, Authentication and Security, User,
PostgreSQL, and the repository NGINX gateway. **Status: not deployment-ready.**
The local integration path was exercised, but the Docker Desktop Linux engine
was unavailable, and the production notification and gateway arrangements
still need decisions and verification.

## Verified checks

| Check | Evidence and result |
| --- | --- |
| Dockerfile and environment review | All three services have multi-stage Go 1.27.2 Alpine Dockerfiles with unprivileged runtime users, migration binaries, and `.dockerignore` rules excluding local `.env`, key, and log files. This is static review, not an image build. |
| Local configuration consistency | The three ignored local environment files have matching HMAC secrets, JWT issuer/audience, Auth key ID and User/Course Ed25519 public keys. Auth and User allow `course-service` for the required signed calls. No secret values were printed. |
| Migrations | On dedicated local PostgreSQL test databases, all migrations were rolled down to no version and reapplied: Auth reached clean version 3, User clean version 4, Course clean version 1. Migration execution is a separate operator step; service startup does not migrate. |
| Readiness | With a live local server, each service returned 503 when its schema was absent and 200 after the required clean migration. Readiness checks connectivity and migration version/dirty state. Course's in-flight request shutdown behavior has an automated test. |
| Ed25519 JWT | Live Auth login issued a token accepted by Auth, User, and Course through NGINX; a tampered token was rejected with 401. |
| Signed internal HTTP | Live Course-to-User and Course-to-Auth eligibility requests succeeded. Missing signatures, a rogue caller, a replayed nonce, and `course-service` attempting Auth's User-only provisioning operation were rejected (401 or 403). |
| Lecturer workflow | A user was registered, activated, approved, granted LECTURER, and logged in. An Admin created and activated a Course, created an Offering, and assigned the lecturer. The assigned lecturer published it, and the published Offering was retrievable through the gateway. |
| NGINX host routing | `nginx -t` passed. Live gateway requests reached Auth, User, and Course. Internal paths returned 404. The corrected `/api/v1/auth/password/forgot` route returned 202. |
| Local restart | Stopping Course caused a gateway 502 while Auth and User stayed ready; restarting Course restored readiness and access to the previously stored Course. This was a forced local process restart, not a container restart or live graceful-signal test. |
| Log sample | The local Auth, User, and Course JSON request logs sampled during the workflow had no ERROR/WARN entries and no token, password, secret, or authorization fields. The NGINX error log recorded the expected 502 during the forced Course outage. |
| Final code checks | PostgreSQL-backed `go test -count=1 ./...`, `go vet ./...`, and `go build ./...` passed independently in all three modules. All seven service, migration, and bootstrap binaries cross-compiled for Linux amd64 with CGO disabled. `gofmt -l` was empty for Course and the audit-modified Auth/User Go files. `git diff --check` and `nginx -t` passed. Cross-compilation does not verify a Docker image. |

The clean-schema exercise used existing dedicated test databases after a full
migration rollback, rather than creating new database instances. Test records
from the lecturer workflow were removed by the later migration rollback.

## Failed checks and deployment blockers

- Auth's production configuration permits only `NOTIFICATIONS_MODE=none`
  because there is no external notification provider. Account activation and
  password reset delivery therefore cannot operate in production as currently
  implemented. Choose and implement a delivery contract before production
  onboarding. The file notification mode used in local testing writes
  activation tokens to a sensitive file and must remain development-only.
- The checked-in NGINX configuration listens on HTTP port 8080 and routes to
  `127.0.0.1:8081/8082/8083`. It is a host-local gateway configuration.
  It does not provide TLS termination or container DNS upstreams. A reviewed
  production ingress/gateway configuration is required.
- A repository-wide `gofmt -l` lists pre-existing files in Auth and User.
  This audit did not reformat unrelated working code. The Go files modified by
  this audit are formatted.

## Blocked checks

Docker Desktop's Linux engine was not running. `docker info` failed because
`//./pipe/dockerDesktopLinuxEngine` did not exist. Therefore **no service image
was built or run**, and container startup, networking, image contents, mounted
key paths, container health/restart, and graceful container shutdown are
unverified. Base image availability is also unverified. No Compose or CI
deployment definition exists in the repository.

## Required runtime configuration

Use the service-specific `.env.example` files as the full variable inventory.
Provide secrets at runtime through a secret manager or protected environment;
do not include local `.env` or the private key in an image.

| Service | Required settings and cross-service alignment |
| --- | --- |
| All three | `APP_ENV`, `BIND_HOST`, service port, `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSLMODE`, timeouts, trusted proxies, and allowed CORS origins. Use distinct owned databases and users. Container `BIND_HOST` must be reachable on its network. |
| Auth | `AUTH_JWT_KEY_ID`, `AUTH_JWT_PRIVATE_KEY_FILE` (mounted read-only), issuer/audience, token/cookie settings, `INTERNAL_SERVICE_ID=auth-service`, `INTERNAL_SERVICE_SECRET`, `INTERNAL_ALLOWED_CALLERS=user-service,course-service`, `USER_SERVICE_INTERNAL_URL`, and notification settings. Production cookies require secure transport and `AUTH_COOKIE_SECURE=true`. |
| User | `AUTH_JWT_PUBLIC_KEYS_JSON` with Auth's key ID, matching issuer/audience, `INTERNAL_SERVICE_ID=user-service`, matching `INTERNAL_SERVICE_SECRET`, `INTERNAL_ALLOWED_CALLERS=auth-service,course-service`, `AUTH_SERVICE_INTERNAL_URL`, and `ADMIN_BOOTSTRAP_TOKEN` during controlled bootstrap. |
| Course | `AUTH_JWT_PUBLIC_KEYS_JSON` with Auth's key ID, matching issuer/audience, `INTERNAL_SERVICE_ID=course-service`, matching `INTERNAL_SERVICE_SECRET`, `USER_SERVICE_INTERNAL_URL`, and `AUTH_SERVICE_INTERNAL_URL`. |

Do not use `localhost` in a container for another service. Configure DNS
names reachable on the deployment network, synchronize clocks for JWT and HMAC
timestamp checks, and protect the shared HMAC secret and Auth private key.

## Deployment sequence once Docker is available

1. Provision PostgreSQL with separate databases and least-privilege roles for
   Auth, User, and Course; set DNS, TLS, secret injection, and a gateway
   configuration for the chosen deployment network.
2. From the repository root, build `docker build -t ucm-auth
   ./services/security-and-authentication`, `docker build -t ucm-user
   ./services/user`, and `docker build -t ucm-course ./services/course`.
3. Mount Auth's Ed25519 private key at its configured in-container path.
   Inject runtime environment values. Run each image's migration binary
   (`/app/auth-migrate`, `/app/user-migrate`, `/app/course-migrate`) on
   its own database with `-path /app/migrations -direction up`. Run each
   migration as a one-shot job before the corresponding service starts.
4. Start Auth, User, and Course on the same reachable network. Check each
   `/health` and `/health/ready`; the latter must return 200 after migration.
   Configure the ingress to route public Auth/User/Course paths and block
   `/internal/*`.
5. Repeat the signed-call, JWT, lecturer assignment/publication, unauthorized
   caller, restart, shutdown, and log checks against the actual containers and
   TLS gateway. Only then make a deployment-readiness decision.

The current repository has no Compose file; these are operator steps, not a
claim that a runnable container deployment already exists. Local host
development commands are in [development-setup.md](development-setup.md).

## Integration risks and remaining work

- Published Offering capacity reductions remain rejected, including
  unlimited-to-finite. A transactional Enrollment Service contract is required
  before enabling them. The **proposal, not an existing API**, is in
  [enrollment-capacity-change.md](proposals/enrollment-capacity-change.md).
- Decide and implement production activation/reset notification delivery.
- Provide reviewed TLS gateway/container routing and runtime secret mounts.
- Start Docker Desktop, build all images, run migrations and the three-service
  stack in containers, and repeat the end-to-end and failure checks.
- Exercise a live graceful signal and restart in the actual orchestrator.
- Resolve the baseline formatting gap and add automated deployment verification
  when the deployment topology is established.
