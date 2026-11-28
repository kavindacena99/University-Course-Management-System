# Infrastructure implementation: Phases 1 and 2

Audit and execution date: 2026-11-23, Windows PowerShell and Docker Desktop.

The local frontend, six Go/Gin services, and Nginx gateway run successfully.
All 15 long-running containers are healthy; six migration jobs and the object
store bootstrap job exit successfully. Replica discovery, distribution,
replacement, and recovery were exercised. Removal can briefly return 504 during
DNS convergence; this is not an uninterrupted-failover guarantee.

## Phase 1: verified architecture and starting state

[Architecture](architecture.md) contains the service/module/port/route matrix,
Mermaid topology, environment requirements, network and storage decisions,
health semantics, and scaling restrictions. Exact API registration remains in
the six `internal/handler.go` files and the six `docs/openapi` specifications.

- Actual services: `security-and-authentication`, `user`, `course`, `enrollment`,
  `assessment`, and `analytics`; Compose names are `auth`, `user`, `course`,
  `enrollment`, `assessment`, and `analytics`. Ports are 8081 through 8086.
- Each is an independent Go 1.27.2 module with root `main.go`, Gin routing,
  PostgreSQL, migrations, signed internal HTTP, and graceful signal shutdown.
- Frontend: React 19, React Router 7, TypeScript, Vite 8, Tailwind 4, Figma
  plugins, Node 22, pnpm 10.34.3. Production build is `pnpm build`; Nginx serves
  `dist`. Development/preview use port 8443 unless `PORT` overrides it. There
  are currently no API calls in frontend source. `VITE_API_BASE_URL` is an
  unused public build-time integration setting, not a runtime secret channel.
- Supporting components were already established: six PostgreSQL databases
  and private S3-compatible Assessment storage. No Redis, broker, or additional
  domain service was added. The resumed stack already used the `pgsty/silo`
  object-store image and its `mc` bootstrap client; that implementation was
  retained and validated. General image-registry availability claims in older
  documentation were not independently audited in this turn.
- Existing host and Docker upstreams were correct and separate. Dockerfile
  copies only Docker upstream definitions, shared routing, global config, and
  snippets. Prefix preservation, nested offering route ownership, Docker DNS,
  least-connections, passive failure handling, security/rate/body limits,
  forwarding header overwrite, health and default 404 remain intact.
- The interrupted work already supplied Compose relocation, frontend images,
  dependency lock repairs, object-store bootstrap changes, documentation, and
  a small Analytics import correction. They were preserved and validated.
  Six backend Dockerfiles already used appropriate module-directory contexts,
  multi-stage builds, CA certificates, migration binaries, and non-root users.
- No repository CI workflow was found. Later deployment automation was not added.
- Docker client/engine 29.9.3, Desktop 4.99.0, Compose v5.7.0, Linux/amd64 are
  available through the Windows `desktop-linux` context. Sandbox access to its
  named pipe was denied; validation succeeded using approved external execution.
  Ubuntu and docker-desktop are running under WSL2. Ubuntu's CLI cannot reach
  `/var/run/docker.sock`; Ubuntu Docker integration remains an environment gap.

## Phase 2: final implementation

```text
frontend/
  Dockerfile
  .dockerignore
  nginx.conf
services/{security-and-authentication,user,course,enrollment,assessment,analytics}/
  Dockerfile
  .dockerignore
  go.mod / go.sum / main.go / cmd/migrate / migrations
services/assessment/
  MinIOInit.Dockerfile / minio-init.sh / minio-policy.json
infrastructure/
  docker/
    compose.yaml
    compose.dev.yaml
    validate.ps1
    validate-scaling.ps1
    .env                    # ignored, existing local credentials
  environments/dev/
    .env.example
  nginx/                    # established location preserved
    Dockerfile / .dockerignore / nginx.conf / mime.types
    conf.d/{00-upstreams.conf,10-default.conf}
    compose/00-upstreams.conf
    snippets/{proxy-headers.conf,security-headers.conf}
docs/infrastructure/
  architecture.md
  phase-1-2-report.md
```

The complete common configuration is
[`compose.yaml`](../../infrastructure/docker/compose.yaml); the only environment
override is [`compose.dev.yaml`](../../infrastructure/docker/compose.dev.yaml).
There is no duplicate root Compose file. Common configuration defines the six
databases, six migrations, object store and setup job, six services, frontend,
and gateway. Service images are built through the migration service build
definitions and reused by the corresponding application containers.

Services have bounded JSON logs, restart policies, readiness checks,
`no-new-privileges`, and 15-second shutdown grace. One-shot jobs do not restart.
Gateway and frontend inherit image healthchecks. No scalable service has a
fixed container name or published backend port.

Networks `edge`, `backend`, and `data` separate frontend, API, and persistence
membership. They retain outbound connectivity. Gateway bridges `edge` and
`backend`; applications join `backend` and `data`; databases/object store only
join `data`. Frontend joins only `edge`. Only host ports 3000 and 8080 are
published; current bindings permit access from other machines, subject to the
Windows firewall. Browser calls must use a browser-reachable gateway origin,
not a Compose hostname.

Six PostgreSQL named volumes and one object-store named volume persist data.
Auth keys use the existing ignored host directory mounted read-only. Auth's
file notifications remain ephemeral and must be retrieved before replacing its
container. No volumes, user data, or credentials were deleted or rotated.

Compose requires local password variables, overrides DB host/port/SSL settings,
and uses service DNS for internal calls. It now explicitly fixes internal
listener ports and object-store HTTP mode. Public Auth links are configurable
through `PUBLIC_BASE_URL`. DEV explicitly selects development mode and debug
logging. Existing service `.env` files retain shared secrets, JWT keys and
service-specific settings; all remain ignored. The example is a configuration
template requiring replacement credentials and real generated JWT keys.

Nginx frontend and gateway now use a digest pinned to tested Nginx 1.30.7,
supporting open-source dynamic upstream `resolve`. Existing routing was not
rewritten. The frontend CSP allows its existing `static.figma.com` font source;
script and API policies remain scoped. Changing gateway/browser origins still
requires aligning CORS and frontend CSP, then rebuilding static frontend assets.

Analytics was selected for scaling because it has database-backed nonce
protection and no background worker. Docker DNS returned both backend IPs.
Both original replicas served 10/20 correlated requests. The surviving replica
served all eight checks after convergence; replacement again distributed 10/10,
with the gateway container unchanged. Both-down produced 504 and recovery
returned 401. The final full-outage recovery needed four polling attempts that
returned 502/504 before succeeding; a single 15-second wait was insufficient.
The script restores one replica in `finally`. The final script exited 0, and
the subsequent smoke run passed against all eight current application image
IDs. Recent application logs contained zero ERROR-level entries after the
five stale application containers were replaced.

Auth/User/Course/Enrollment replay caches are process-local. Auth/User rate
counters are also local. User's outbox uses PostgreSQL row claiming; Assessment
polls pending outbox events and needs a duplicate-delivery/concurrency review
before production scaling. Compose scaling compatibility is not evidence that
all stateful security or worker behavior is safe across replicas.

## Commands and operation

Run from the repository root. The PowerShell array avoids machine-specific
paths and repeating the full Compose invocation:

```powershell
$dc = @('compose', '-p', 'platform-dev', '-f', 'infrastructure/docker/compose.yaml',
        '-f', 'infrastructure/docker/compose.dev.yaml')

# First-time setup only: never overwrite existing local environment files.
# Copy infrastructure/environments/dev/.env.example to infrastructure/docker/.env.
# Copy six service .env.example files to .env; fill in credentials and shared keys.
# Generate Auth keys using docs/development-setup.md and distribute PUBLIC keys.

docker @dc config --quiet
docker @dc up -d --build --wait --wait-timeout 180
docker @dc ps -a
docker @dc logs -f --tail 100 gateway
docker @dc build
docker @dc build frontend
docker @dc up -d --no-deps --force-recreate --wait frontend
# After rebuilding all images, explicitly replace application containers:
docker @dc up -d --no-deps --force-recreate --wait auth user course enrollment assessment analytics gateway frontend
docker @dc exec -T gateway nginx -t
docker @dc exec -T gateway nginx -T
docker @dc exec course sh
docker @dc up -d --scale analytics=2 --wait
docker @dc up -d --scale analytics=1 --wait
docker @dc stop
docker @dc start
docker @dc down

powershell -NoProfile -File infrastructure/docker/validate.ps1
# Deliberately interrupts Analytics; use only for this local DEV exercise.
powershell -NoProfile -File infrastructure/docker/validate-scaling.ps1
```

`down` retains volumes. Do not add `-v` for ordinary shutdown. The validation
scripts accept `-Project` and `-GatewayUrl`; the smoke script also accepts
`-FrontendUrl`. They do not write credentials or runtime logs into the repo.
They test unauthenticated routing and local infrastructure, not business flows.

## Testing report

In this table `docker @dc` expands to the exact common/DEV/project arguments
above. Tests were actually executed; historical November 22 claims were not
used as substitutes for this run.

| Test | Command executed | Result | Remaining issues |
| --- | --- | --- | --- |
| Docker client/server | `docker version` | Passed | Approved external execution required for sandbox pipe access |
| Engine readiness | `docker info`; `docker info --format '{{.OSType}} {{.ServerVersion}}'` | Passed: Linux 29.9.3 | None for Windows CLI |
| Compose version | `docker compose version` | Passed: v5.7.0 | None |
| WSL inventory | `wsl --list --verbose` | Passed: Ubuntu and docker-desktop, WSL2 running | None |
| Ubuntu Docker | `wsl -d Ubuntu -- sh -lc 'docker version --format "{{.Server.Os}} {{.Server.Version}}"'` | Blocked by environment: socket missing | Enable/review Desktop Ubuntu integration later |
| Base + DEV model | `docker @dc config --quiet` | Passed | Requires existing ignored local env files |
| Nine images | `docker @dc build` | Passed | BuildKit reused cached layers where available |
| Final pinned frontend/gateway | `docker @dc build gateway frontend` | Passed | Vite warned about two unresolved SVG references |
| Deploy current image IDs | `docker @dc up -d --no-deps --force-recreate --wait --wait-timeout 180 auth user course enrollment assessment`; compare `docker image inspect ... --format '{{.Id}}'` with container `docker inspect ... --format '{{.Image}}'` | Passed on final verification | External builds initially left five application containers on older IDs; checked Auth notification file before replacement, retained all DB/object volumes |
| Startup / migrations / init | `docker @dc up -d --wait --wait-timeout 180` | Passed | Uses retained local data, not empty-volume initialization |
| Container state | `docker @dc ps -a`; smoke script JSON status assertions | Passed | 15 healthy runtimes; seven jobs exited 0 |
| Docker Nginx syntax/version | `docker @dc exec -T gateway nginx -t`; `nginx -v` | Passed: 1.30.7 | Host and Docker upstreams remain separate |
| Loaded upstreams and graceful shutdown | `docker @dc exec -T gateway nginx -T`; `docker @dc logs --since 20m analytics` filtered for `server_stopped` | Passed | Six Docker DNS upstreams loaded, no host loopback peers; two graceful-stop log entries observed |
| Host-mode Nginx syntax | `docker run --rm --mount "type=bind,source=$auditNginxPath,target=/audit,readonly" <pinned-nginx-image> sh -c 'cp -R /audit /tmp/nginx; nginx -t -p /tmp/nginx/ -c nginx.conf'` | Passed | Syntax checked in Linux; native Windows host listener not launched |
| Smoke routes and headers | `powershell -NoProfile -File infrastructure/docker/validate.ps1` | Passed | 16 requests: health/UI 200, protected APIs 401, unknown/internal/missing assets 404 |
| Correct upstream ownership | Requests with unique `X-Request-ID`, followed by `docker @dc logs --since 1m --no-color <service>` | Passed | Auth/User/Course/Enrollment/Assessment correlated once each; Analytics correlated in scaling test |
| All backend readiness | Gateway `wget -qO- http://<service>:<port>/health/ready` for all six | Passed on final rerun | A concurrent intentional Analytics outage earlier returned DNS failure, as expected |
| Frontend-to-gateway network | `docker @dc exec -T frontend wget -qO- http://gateway:8080/health` | Passed: `ok` | Browser uses localhost origin, not this internal hostname |
| CORS | `curl.exe -s -D - -o NUL -X OPTIONS http://localhost:8080/api/v1/users/me -H 'Origin: http://localhost:3000' -H 'Access-Control-Request-Method: GET'` | Passed: 204 and exact allowed origin | Analytics has no CORS implementation |
| Login rate limit | Ten `curl.exe` POSTs with JSON `{}` to `/api/v1/auth/login` | Passed: seven 401 then three 429 | No valid credentials or user records used |
| Initial zero-error failover expectation | Initial `validate-scaling.ps1` run | Failed: one request returned 504 after stopping a replica | No zero-error claim; final script records transition errors separately from recovery |
| Fixed-delay total-outage recovery expectation | Second `validate-scaling.ps1` run | Failed: a request after a fixed 15-second wait still returned 504 | Final smoke checks subsequently passed; use bounded recovery polling, not a single TTL-based wait |
| Replica discovery/replacement/outage/recovery | Final `powershell -NoProfile -File infrastructure/docker/validate-scaling.ps1` | Passed with observed transient 504 | Checks two IPs, 10/10 distribution twice, healthy survivor, unchanged gateway, both-down 504, restored routing |
| Go suites | In each of the six service directories: `go test ./...`, Go 1.27.2, workspace `GOCACHE` | Passed for all six | Database integration tests without dedicated test DB configuration may skip |
| Diff whitespace | `git -c safe.directory=<repository> diff --check` | Passed | Git emitted existing CRLF conversion warnings, no whitespace errors |
| Secret/runtime tracking | `git ... ls-files '*.env' '*.pem' '*.key' '*.log' '*.exe'`; `git ... check-ignore <seven-env-files> <auth-private-key>` | Passed | No matching sensitive/runtime tracked files; existing env/key paths ignored |
| Authenticated full workflows | Not executed | Not executed | No account or business-data mutation needed for infrastructure validation |
| Empty-volume destructive reset | Not executed | Not executed | Existing user data intentionally retained |
| Native browser visual QA | Not executed | Not executed | HTTP/static bundle checks do not prove visual correctness |
| QA/PROD, Ansible, cloud, CI/CD, TLS/secrets integrations | Not executed | Not executed: outside scope | Phase 3 or later |

## File change inventory

`Carried in` means already modified/untracked when this resumed session began;
ownership is not inferred and those changes were preserved. `Updated now` or
`Added now` identifies this continuation's writes. No commit was created.

| File | Status / reason |
| --- | --- |
| `.gitattributes` | Carried in, added: Linux shell LF endings |
| `README.md` | Carried in, modified: current frontend and Compose setup |
| `compose.yaml` | Carried in, removed: relocated common stack; no duplicate root config |
| `docs/architecture.md` | Carried in, modified: current topology |
| `docs/deployment-readiness.md` | Carried in, modified: deployment gaps and local validation |
| `docs/development-setup.md` | Carried in, modified: Docker/frontend setup |
| `docs/enrollment-discovery.md` | Carried in, modified: updated discovery/deployment context |
| `docs/implementation-report.md` | Carried in, modified: existing implementation summary |
| `docs/infrastructure/architecture.md` | Carried in, added; updated now: module/dependency/env audit, Mermaid, health/scaling/failure boundaries |
| `docs/infrastructure/phase-1-2-report.md` | Added now: complete report, commands, tests and inventory |
| `frontend/.dockerignore` | Carried in, added: omit dependencies/build caches/env files |
| `frontend/Dockerfile` | Carried in, added; updated now: production build/static serving, pin tested Nginx digest |
| `frontend/nginx.conf` | Carried in, added; updated now: SPA serving/security/cache/health and allow existing Figma font source |
| `frontend/pnpm-lock.yaml` | Carried in, modified: lock matches existing React Router dependency and Linux dependency metadata |
| `infrastructure/docker/compose.yaml` | Carried in, added; updated now: explicit container ports, local DB port/SSL, object-store SSL, configurable Auth public URL |
| `infrastructure/docker/compose.dev.yaml` | Carried in, added; updated now: development mode with debug logs |
| `infrastructure/docker/validate.ps1` | Added now: repeatable HTTP/security/status/setup-job checks |
| `infrastructure/docker/validate-scaling.ps1` | Added now: correlated replica/failure/replacement checks with restoration |
| `infrastructure/environments/dev/.env.example` | Carried in, added; updated now: document configurable public Auth URL |
| `infrastructure/nginx/Dockerfile` | Updated now: pin tested 1.30.7 image digest |
| `infrastructure/nginx/compose/00-upstreams.conf` | Updated now: comment identifies pinned version; directives preserved |
| `services/analytics/.dockerignore` | Carried in, added: exclude secrets/runtime files |
| `services/analytics/Dockerfile` | Carried in, modified: correct Go builder version |
| `services/analytics/go.mod` | Carried in, modified: resolved indirect dependencies |
| `services/analytics/go.sum` | Carried in, added: dependency checksums |
| `services/analytics/main.go` | Carried in, modified: alias actual `analytics` package, resolving compilation; no business change |
| `services/assessment/.dockerignore` | Carried in, modified: build context exclusions |
| `services/assessment/Dockerfile` | Carried in, modified: correct Go builder version |
| `services/assessment/MinIOInit.Dockerfile` | Carried in, modified: source available mc client for retained storage image |
| `services/assessment/README.md` | Carried in, modified: current storage/bootstrap instructions |
| `services/assessment/go.mod` | Carried in, modified: resolved indirect dependencies |
| `services/assessment/go.sum` | Carried in, added: dependency checksums |
| `services/assessment/minio-init.sh` | Carried in, modified: repeatable bucket/policy/scoped-user initialization |
| `services/enrollment/README.md` | Carried in, modified: current local Compose instructions |

Auth, User, Course, and Enrollment Dockerfiles and ignore files were kept and
validated. Existing Nginx main routing, host upstreams, and security snippets
were preserved. No schema or API contract was changed in this continuation.
Generated Go test caches remain under ignored `.go-cache/`. No real env files,
private keys, generated binaries, runtime logs, or persistent volumes were added
to version control.

## Remaining work and Phase 3 handoff

1. Define separate DEV/QA/PROD-like local configuration inputs and overrides
   around this common Compose model. Use distinct project names, host ports,
   secrets/key paths, and image tags. Current shared local tags and host key
   mount are not a completed multi-environment deployment.
2. Install and validate Ansible in an appropriate Linux/WSL control environment;
   design repository roles, local inventories, idempotent preflight/deploy/health
   checks, and rollback before creating playbooks in the next phase.
3. Automate local deployment using those reviewed inventories and common
   Compose model, including migrations, retained data, and failure recovery.
4. Design environment-specific secret delivery and explicit rotation procedures.
   Do not reuse DEV signing keys, database credentials, or shared internal
   service secrets across environments.
5. Review process-local replay/rate state, Assessment outbox concurrency, health
   removal/draining and transient gateway failures before production scaling.
6. Frontend team: add API integration, Analytics CORS if required, and repair
   unresolved `/assets/0c1d9.svg` and `/assets/5b43a.svg` references. Existing
   source did not contain those assets; the Vite build succeeded with warnings.

Cloud provisioning, Ansible implementation, GitHub Actions deployment,
centralized production secrets, Kubernetes, Terraform, and QA/PROD deployment
were deliberately not implemented.
