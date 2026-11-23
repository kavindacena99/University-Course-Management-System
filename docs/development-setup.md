# Development Setup

## Prerequisites

- Go 1.27.2 or later
- PostgreSQL 16 or later
- NGINX (optional for gateway testing)
- Docker Desktop (optional; for the local Compose stack)
- Node 22 and pnpm 10.34.3 (for the frontend)
- Git

## 1. Create databases and roles

Create separate databases and least-privilege users:

```sql
CREATE USER user_app WITH PASSWORD 'change-me';
CREATE USER auth_app WITH PASSWORD 'change-me';
CREATE USER course_app WITH PASSWORD 'change-me';
CREATE USER enrollment_app WITH PASSWORD 'change-me';
CREATE USER assessment_app WITH PASSWORD 'change-me';
CREATE USER analytics_app WITH PASSWORD 'change-me';
CREATE DATABASE users OWNER user_app;
CREATE DATABASE authentication_and_security OWNER auth_app;
CREATE DATABASE users_test OWNER user_app;
CREATE DATABASE auth_test OWNER auth_app;
CREATE DATABASE courses OWNER course_app;
CREATE DATABASE courses_test OWNER course_app;
CREATE DATABASE enrollments OWNER enrollment_app;
CREATE DATABASE enrollments_test OWNER enrollment_app;
CREATE DATABASE assessments OWNER assessment_app;
CREATE DATABASE analytics OWNER analytics_app;
```

Do not reuse production credentials in development.

## 2. Configure environment files

```bash
cp services/security-and-authentication/.env.example services/security-and-authentication/.env
cp services/user/.env.example services/user/.env
cp services/course/.env.example services/course/.env
cp services/enrollment/.env.example services/enrollment/.env
cp services/assessment/.env.example services/assessment/.env
cp services/analytics/.env.example services/analytics/.env
```

Set at minimum:

- database host, port, name, user, and password for each service
- a shared `INTERNAL_SERVICE_SECRET` of at least 32 characters
- `AUTH_JWT_KEY_ID` and the auth service private key path
- `AUTH_JWT_PUBLIC_KEYS_JSON` in User, Course, Enrollment, Assessment, and Analytics with the matching public key
- `enrollment-service` in Auth and User `INTERNAL_ALLOWED_CALLERS`; Course allows it on the signed Offering facts route
- Enrollment `INTERNAL_ALLOWED_CALLERS=assessment-service` for Assessment eligibility verification
- Course `INTERNAL_ALLOWED_CALLERS` includes `analytics-service`; Analytics allows `assessment-service`; Assessment allows `analytics-service`
- `ADMIN_BOOTSTRAP_TOKEN` if you plan to bootstrap an administrator

`.env` files are gitignored and must never be committed.

## 3. Generate JWT keys

```bash
cd services/security-and-authentication
go run ./cmd/keygen --out-dir ./var/keys --key-id dev-1
```

The command writes an Ed25519 private key and public key. Keep the private key
server-side and put the public key into User, Course, Enrollment, Assessment,
and Analytics configurations. Newlines
can be escaped as `\n` inside the JSON value.

## 4. Run migrations

```bash
cd services/security-and-authentication
go run ./cmd/migrate -direction up

cd ../user
go run ./cmd/migrate -direction up

cd ../course
go run ./cmd/migrate -direction up

cd ../enrollment
go run ./cmd/migrate -direction up

cd ../assessment
go run ./cmd/migrate -direction up

cd ../analytics
go run ./cmd/migrate -direction up
```

Apply the same migrations to `auth_test`, `users_test`, `courses_test`, and `enrollments_test` before running
integration tests.

## 5. Start the services

Use separate terminals:

```bash
cd services/security-and-authentication
go run .
```

```bash
cd services/user
go run .
```

```bash
cd services/course
go run .
```

```bash
cd services/enrollment
go run .
```

```bash
cd services/assessment
go run .
```

```bash
cd services/analytics
go run .
```

The services bind to loopback ports `8081` through `8086` by default.

## 6. Start the gateway

From the repository root, validate and then run NGINX with this
configuration. On Windows:

```powershell
nginx -t -p infrastructure/nginx/ -c nginx.conf
nginx -p infrastructure/nginx/ -c nginx.conf
```

Stop it with:

```powershell
nginx -p infrastructure/nginx/ -c nginx.conf -s stop
```

The gateway listens on `http://localhost:8080`.

## 7. Bootstrap an administrator

```bash
cd services/user
go run ./cmd/bootstrap-admin \
  --email admin@example.com \
  --first-name Admin \
  --last-name User \
  --password 'StrongPassword123!'
```

Then log in through the gateway:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"StrongPassword123!"}'
```

## Frontend development

The frontend uses Node 22 and pnpm 10.34.3 (see `.mise.toml`):

```bash
cd frontend
corepack enable
pnpm install --frozen-lockfile
pnpm dev
```

Vite serves on port `8443` by default; set `PORT` to override it. The app
currently has no API integration. When API calls are added, point them at the
gateway origin and keep the frontend Nginx CSP and service CORS allowlist in
sync. The production image is built from `frontend/Dockerfile`.

## Testing

```bash
cd services/security-and-authentication
go test ./...

cd ../user
go test ./...

cd ../course
go test ./...

cd ../enrollment
go test ./...

cd ../assessment
go test ./...

cd ../analytics
go test ./...
```

Integration tests use `TEST_DATABASE_URL` and skip when it is unset:

```powershell
$env:TEST_DATABASE_URL='postgres://auth_app:password@127.0.0.1:5432/auth_test?sslmode=disable'
go test ./internal/...
```

```powershell
$env:TEST_DATABASE_URL='postgres://user_app:password@127.0.0.1:5432/users_test?sslmode=disable'
go test ./internal/...
```

```powershell
$env:TEST_DATABASE_URL='postgres://course_app:password@127.0.0.1:5432/courses_test?sslmode=disable'
go test ./internal/...
```

```powershell
$env:TEST_DATABASE_URL='postgres://enrollment_app:password@127.0.0.1:5432/enrollments_test?sslmode=disable'
go test ./internal/...
```

## Local Docker Compose

The stack lives under `infrastructure/docker`:

- `compose.yaml` defines the base stack.
- `compose.dev.yaml` sets `LOG_LEVEL=debug` for the six services.
- `infrastructure/environments/dev/.env.example` is the tracked Compose-level
  environment template.

It runs six PostgreSQL containers, the private object store, six migration
jobs, the MinIO bootstrap job, the six services, the gateway, and the
frontend. Only frontend port `3000` and gateway port `8080` are published.

1. Create the ignored environment files:

```bash
cp infrastructure/environments/dev/.env.example infrastructure/docker/.env
cp services/security-and-authentication/.env.example services/security-and-authentication/.env
cp services/user/.env.example services/user/.env
cp services/course/.env.example services/course/.env
cp services/enrollment/.env.example services/enrollment/.env
cp services/assessment/.env.example services/assessment/.env
cp services/analytics/.env.example services/analytics/.env
```

Set the database passwords and object-store credentials in
`infrastructure/docker/.env`. Keep the shared `INTERNAL_SERVICE_SECRET`, JWT
issuer/audience, and `AUTH_JWT_PUBLIC_KEYS_JSON` aligned across the service
`.env` files. Auth's generated key directory is mounted read-only at
`/app/var/keys`; keep `AUTH_JWT_PRIVATE_KEY_FILE` pointed at the generated
file under that directory.

2. Validate the combined configuration:

```bash
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev config --quiet
```

3. Build and start:

```bash
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev up -d --build
```

4. Check readiness and logs:

```bash
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev ps
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev logs -f gateway
```

The frontend is at `http://localhost:3000`; the gateway health endpoint is
`http://localhost:8080/health`. The gateway uses Compose service-name
upstreams and re-resolves Docker DNS every 10 seconds. Scale a stateless
service with:

```bash
docker compose -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml -p platform-dev up -d --scale auth=2
```

Stop the stack with `docker compose ... down`; add `-v` only when
intentionally deleting the database and object-store volumes.

The object store uses the maintained `pgsty/silo` fork because MinIO
community-edition images are no longer published. The frontend currently has
no API calls; `VITE_API_BASE_URL` and the frontend CSP are pre-wired for the
gateway at `http://localhost:8080`. Analytics has no CORS configuration and
no frontend analytics calls yet.

This is a development integration stack, not a production TLS deployment. The
full stack built and reached readiness on 2026-11-22; that does not establish
production deployment readiness. See
[Infrastructure Architecture](infrastructure/architecture.md) for the topology
and validation evidence.

## Code quality

```bash
gofmt -w .
go vet ./...
```

## Troubleshooting

- `eligibility_unavailable` on login: check the User Service is running and
  `USER_SERVICE_INTERNAL_URL` is correct.
- `invalid internal service authentication`: the shared secrets differ, clocks
  are skewed, or the caller ID is not allowed.
- `AUTH_JWT_PUBLIC_KEYS_JSON must be configured`: the user service has no
  public key JSON or the value is not valid JSON.
- Migrations dirty: inspect the schema, fix the cause, then use
  `-direction force -force-version <version>` only after verifying the state.
