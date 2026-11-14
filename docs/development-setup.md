# Development Setup

## Prerequisites

- Go 1.27.2 or later
- PostgreSQL 16 or later
- NGINX (optional for gateway testing)
- Git

## 1. Create databases and roles

Create separate databases and least-privilege users:

```sql
CREATE USER user_app WITH PASSWORD 'change-me';
CREATE USER auth_app WITH PASSWORD 'change-me';
CREATE USER course_app WITH PASSWORD 'change-me';
CREATE USER enrollment_app WITH PASSWORD 'change-me';
CREATE DATABASE users OWNER user_app;
CREATE DATABASE authentication_and_security OWNER auth_app;
CREATE DATABASE users_test OWNER user_app;
CREATE DATABASE auth_test OWNER auth_app;
CREATE DATABASE courses OWNER course_app;
CREATE DATABASE courses_test OWNER course_app;
CREATE DATABASE enrollments OWNER enrollment_app;
CREATE DATABASE enrollments_test OWNER enrollment_app;
```

Do not reuse production credentials in development.

## 2. Configure environment files

```bash
cp services/security-and-authentication/.env.example services/security-and-authentication/.env
cp services/user/.env.example services/user/.env
cp services/course/.env.example services/course/.env
cp services/enrollment/.env.example services/enrollment/.env
```

Set at minimum:

- database host, port, name, user, and password for each service
- a shared `INTERNAL_SERVICE_SECRET` of at least 32 characters
- `AUTH_JWT_KEY_ID` and the auth service private key path
- `AUTH_JWT_PUBLIC_KEYS_JSON` in User, Course, and Enrollment with the matching public key
- `enrollment-service` in Auth and User `INTERNAL_ALLOWED_CALLERS`; Course allows it on the signed Offering facts route
- Enrollment `INTERNAL_ALLOWED_CALLERS=assessment-service` for future Assessment verification
- `ADMIN_BOOTSTRAP_TOKEN` if you plan to bootstrap an administrator

`.env` files are gitignored and must never be committed.

## 3. Generate JWT keys

```bash
cd services/security-and-authentication
go run ./cmd/keygen --out-dir ./var/keys --key-id dev-1
```

The command writes an Ed25519 private key and public key. Keep the private key
server-side and put the public key into the User, Course, and Enrollment configurations. Newlines
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

The services bind to loopback ports `8081`, `8082`, `8083`, and `8084`
by default.

## 6. Start the gateway

From the repository root, run NGINX with this configuration. On Windows:

```powershell
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

`compose.yaml` runs four separate PostgreSQL containers, one migration job
per service, the four services, and an NGINX gateway. Only gateway port 8080
is published. Configure all four ignored service `.env` files and set Auth's
`AUTH_JWT_PRIVATE_KEY_FILE` to the generated file under `./var/keys`. Export
`AUTH_DB_PASSWORD`, `USER_DB_PASSWORD`, `COURSE_DB_PASSWORD`, and
`ENROLLMENT_DB_PASSWORD` with the values for the local database containers,
then run:

```sh
docker compose config --quiet
docker compose up --build
```

This is a development integration stack, not a production TLS deployment.
An isolated stack was built and reached readiness on 2026-11-14; that does not
establish production deployment readiness.

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
