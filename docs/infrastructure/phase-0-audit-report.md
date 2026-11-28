# Phase 0 infrastructure audit and implementation baseline

Audit date: 2026-11-23. Repository baseline: `main`, commit
`a2df85f2bd655ccbe155ce781a2c95b9d8b41f2d` (merged PR #12). This is a new
roadmap baseline; “Phase 1” below means the forthcoming PostgreSQL/isolation
phase, not the historical Phase 1/2 work. Historical reports were not rewritten.

## 1. Executive summary

**Verdict: READY WITH CONDITIONS to implement Phase 1, not ready for release or
data cutover.** The six-service architecture, project isolation, gateway,
multi-stage images and deployment helpers are substantial existing assets.
Preserve them. The approved database topology is **not** implemented: there are
six PostgreSQL containers and six database volumes per environment, or eighteen
clusters across three environments. Consolidation needs logical migration into
three new cluster volumes, not physical volume reuse or directory merging.

Current runtime inspection found **all 66 project containers stopped**, each
with exit code 0. Docker Desktop is available. This audit did not start them;
live role queries, gateway requests and readiness could not be verified. A
stopped container's previous health metadata is not present-tense readiness.

GitHub execution has advanced beyond the historical reports: three runs exist
and failed. On merged `main`, all six backend jobs, frontend and nine image
build/scan jobs succeeded, but infrastructure Compose validation failed and
publishing/manifest jobs were skipped. No successful publication is evidenced
by those runs; package enumeration is separately blocked by missing
`read:packages` authorization. Final-tag publishing would rebuild artifacts,
contrary to the newly approved candidate-to-final digest reuse policy.

Security debt remains. Applications and migrations share bootstrap identities.
The retained PostgreSQL scan actually contains **26 HIGH and one CRITICAL**,
not the historical prose's 27 HIGH. Its image ID matches the installed image.
This is a recount of retained evidence, not a new vulnerability scan. A storage
finding has vendor backport evidence, but no repository VEX decision exists.

## 2. Scope, evidence and constraints

Inspected tracked structure, all nine deployable Dockerfiles, all four Compose
files, eight relevant ignore files (root, six services, frontend; gateway ignore
also reviewed), all three workflows, inventories/playbooks/roles, lifecycle,
validation/scanner/backup scripts, six modules/routes/migrations/tests, frontend
runtime/API references, Nginx variants/snippets and infrastructure/developer/
database/security documentation. Read local configuration **keys**, safe DB
connection metadata and secret-presence checks; no credentials are reproduced.

Evidence levels:

| ID | Evidence obtained in Phase 0 | Limit |
| --- | --- | --- |
| E1 | Git status clean at start; `main`/HEAD/branches/tags/tracked-file inventory | No fetch, checkout, commit, tag or push |
| E2 | Docker version/info/Compose, 66 sanitized container inspections, image metadata, networks and volumes | No start/stop/build/pull/run or database query |
| E3 | Three resolved Compose models checked in memory with existing inputs and `verify_model` | Proves configuration, not running connectivity or role catalog state |
| E4 | Dockerfiles and stopped PostgreSQL entrypoint copied to stdout and inspected in memory | No image extraction to the repository; no new container |
| E5 | GitHub runs/jobs/logs/artifacts, releases/tags, environments and permissions APIs | Read only; some administrative/package APIs returned 403 |
| E6 | Eleven synthetic Python tests and `gofmt -l services` executed | No Go suite, integration suite or build executed locally |
| E7 | Retained Trivy JSON reports and image IDs recounted; vendor advisory read | No new scanner run; retained reports are ignored, host-local evidence |
| E8 | Existing QA backup manifests/ACLs and latest 12-file checksum verification | No new backup, object write, archive restore or archive-listing container |
| E9 | Ubuntu already running; Docker CLI/socket check and host tool inventory | No Desktop/WSL setting changed |

Git has 366 tracked files, no root `compose.yaml`, no current Git tags and no
GitHub release/tag entries. Local branches include `main`, the merged
`infrastructure/phases-3-5` and `infra-fix-1`, service feature branches and older
development branches. Observed feature-branch/PR history is not an enforced
branch/release policy. Initial Git state was clean; implementation files remain
unchanged. Only the five Phase 0 Markdown deliverables are authorized outputs.

## 3. Actual repository architecture

```text
frontend/                 React/Vite application, Dockerfile, nginx.conf
services/
  security-and-authentication/  user/  course/  enrollment/  assessment/  analytics/
infrastructure/
  docker/                 common + dev/qa/prod overrides, two validation PS scripts
  environments/           dev example; qa/prod examples and ignored local inputs
  nginx/                  global config, host/Docker upstreams, routing, snippets
  ansible/                controller, three inventories, four playbooks, five roles
  scripts/                lifecycle/model/security/backup/Vault/ACL tooling
tests/infrastructure/     two synthetic safety/integrity suites
.github/workflows/        ci.yml, publish-images.yml, prepare-deployment.yml
docs/                    application contracts, historical reports, operating guide
```

Every service has its own `go.mod`/`go.sum`, Go 1.27.3 directive, root `main.go`,
Gin router in `internal/handler.go`, startup/readiness/shutdown in
`internal/app.go`, DB connection layer and `cmd/migrate/main.go`. No root Go
workspace or additional microservice was found. Internal HTTP uses Docker DNS
and signed service calls; no broker/cache is required by this implementation.

| Domain / Compose name | Actual directory under `services/` | Port | Public routes under `/api/v1` | Database / app identity |
| --- | --- | --- | --- | --- |
| Authentication / `auth` | `security-and-authentication` | 8081 | `/auth/**`, `/admin/auth/**` | `authentication_and_security` / `auth_app` |
| User / `user` | `user` | 8082 | `/users`, `/users/**`, `/admin/users[/**]` | `users` / `user_app` |
| Course / `course` | `course` | 8083 | `/courses[/**]`, `/offerings[/**]`, `/admin/courses[/**]`, `/admin/offerings[/**]`, excluding nested routes below | `courses` / `course_app` |
| Enrollment / `enrollment` | `enrollment` | 8084 | `/enrollments[/**]`, `/students/:id/enrollments`, `/offerings/:id/enrollments` | `enrollments` / `enrollment_app` |
| Assessment / `assessment` | `assessment` | 8085 | `/assessments/**`, `/submissions/**`, nested offering `/assessments[/**]` and `/results[/**]` | `assessments` / `assessment_app` |
| Analytics / `analytics` | `analytics` | 8086 | `/analytics/**` | `analytics` / `analytics_app` |

Service-to-service routes under `/internal` are not gateway-public. Auth talks
to User; User to Auth; Course to Auth/User; Enrollment to Course/Auth/User;
Assessment to Enrollment/Analytics/object storage; Analytics to Course/Assessment.
User and Assessment have outbox processing. Auth/User/Course/Enrollment contain
process-local replay state; Auth/User also have local rate state. Assessment
outbox duplicate-delivery/concurrency needs review before scaling. Analytics
uses DB-backed nonce protection. These findings agree with source and the
historical scaling restrictions; infrastructure replica support is not blanket
application scaling certification.

## 4. Current environment architecture

| Environment | Project / override | Frontend / gateway host bindings | Existing persistent resources | State observed |
| --- | --- | --- | --- | --- |
| DEV | `platform-dev` / `compose.dev.yaml` | all interfaces: 3000 / 8080 | six DB volumes + one object volume | 22 stopped containers |
| QA | `platform-qa` / `compose.qa.yaml` | 127.0.0.1:3001 / 8081 | separate six DB volumes + object volume | 22 stopped containers |
| PROD-like | `platform-prodlike` / `compose.prod.yaml` | 127.0.0.1:3002 / 8082 | separate six DB volumes + object volume | 22 stopped containers |

Each project has `edge`, `backend`, `data` bridges, without explicit external
resources or global resource-name overrides. All 21 project volumes and nine
networks exist. An unrelated anonymous volume and `ucm-infra-trivy-cache` also
exist; neither is an application data volume or a cleanup target. Defined edge
ports are distinct; a stopped stack is not proof ports will be available at its
next deployment. No PostgreSQL, object-store or service host ports are published.

Frontends join `edge`; gateways join `edge` and `backend`; services join
`backend`/`data`; stores join `data`. All services can reach the persistence
network: credential/SQL privilege separation must enforce data ownership.
No `internal: true` blocks needed outbound access. Project resources and
credentials resolve separately; no cross-environment DB hostname was found.
Browser integration can still bypass intended isolation if future frontend code
uses a baked-in DEV API URL; see section 14.

`platformctl.py init` generates only new QA/PROD-like local credentials/keys and
refuses existing/partial input replacement and DEV regeneration. Existing DEV
uses `infrastructure/docker/.env` plus six service `.env` files; rehearsals use
`infrastructure/environments/{qa,prod}/.env` and `runtime/*.env`. QA/PROD-like
password uniqueness and cross-environment matching guards passed. No values
were regenerated. Rehearsal issuers/audiences/cookie prefixes differ because
browser cookies do not isolate by port. Current CORS generation uses the exact
`http://localhost:<frontend-port>` origin; use of `127.0.0.1` as a browser origin
needs corresponding CORS/CSP review.

An additional value-masked audit confirmed each environment's six services use
one consistent internal service credential, and the three environments use
distinct credentials and JWT issuers. This verifies existing input consistency,
not live authentication or authorization behavior.

## 5. Docker and Compose assessment

`infrastructure/docker/compose.yaml` contains 22 components: six databases,
six migrations, object store/setup job, six services and two edges. Application
builds reside on migration service definitions and are reused by their runtime
services. No duplicate root stack exists. `compose.dev.yaml` supplies app mode
and logs. QA/PROD-like overrides are nearly identical hardened overlays with
only QA vs production application mode differing; keep explicit environments,
and consider a common hardening fragment only when it makes maintenance easier.

Go images use service-directory contexts, Go 1.27.3 builders, cached module/build
mounts, static stripped binaries and Alpine 3.22 runtime with CA certificates
and `USER app`. Final copies contain service/migration binaries and SQL; User
also includes the bootstrap admin utility. They do not copy Go source, tests,
docs or workflows into the runtime. Migration/admin tools are useful shared
artifacts, though separate runtime/migration image splitting is optional future
minimization, not a reason to redesign now. Backend ignore files exclude keys,
env, var, logs and binaries; some still permit test/docs into the **builder**
context, not the final runtime.

Frontend builds with Node 22/pnpm 10.34.3 and serves only `dist` plus Nginx
configuration. Both frontend and gateway pin their Nginx base digest and install
a fixed libxml2 package floor. Gateway runtime copies one Docker upstream file,
routing/global configuration and snippets; host upstreams are not copied too.
Storage setup image contains the vendor `mc`, shell script and scoped policy.
The controller is tooling, not a seventh business service. Its root build
context allowlists only the two requirements files it needs.

Pinning is mixed: Nginx/control/storage-helper bases and rehearsal storage are
digest pinned; Go/Node/Alpine builders and PostgreSQL use tags. Patching packages
with `apk` at build time also changes outputs over time. A source SHA tag alone
cannot replace immutable published digest selection. DEV retains older `:local`
images while rehearsals use `:phase345`; `:candidate` occurs in examples, but no
such local images were observed. Do not overwrite old tags as a migration shortcut.

Long-running components restart `unless-stopped`; jobs do not restart. Logs are
rotated JSON (10 MB × 3). Go shutdown grace is 15s. Six readiness checks call
`/health/ready`; edge images have `/health` checks; object health is liveness.
Dependencies use healthy/successful-job conditions. All six service readiness
handlers ping the DB and read schema_migrations version/dirty state; Assessment
also verifies object storage. They do not test every downstream business flow.

QA/PROD-like Go containers have read-only roots, all capabilities dropped,
`no-new-privileges`, 384 MiB/1 CPU/128 PID limits and no-exec scratch tmpfs.
Nginx retains startup privilege capabilities with 128 MiB/64 PID limits and
bounded writable scratch; object storage has 512 MiB/128 PID limits; each DB
has 512 MiB/128 PID limits, drops NET_RAW, and retains required bootstrap
permissions. Sanitized stopped-container settings match this configuration.
DEV lacks these rehearsal memory/readonly/capability restrictions; do not
silently recreate it during this audit to apply them.

## 6. PostgreSQL assessment

### Topology, versions and persistence

**Actual:** six PostgreSQL 18.7 clusters per project, one business database per
cluster, with maintenance/template databases in addition. The configured tag is
`postgres:18-alpine`; all eighteen containers reference the installed image:

`postgres@sha256:0115267ab5a509022df9319c77be365cef335adb71556c75f60f5360ee247e71`.

**Approved target:** one PostgreSQL cluster and one new DB volume per project,
six independent logical databases within it: three clusters/three DB volumes
in steady state. Keep the separate three object volumes. Existing physical
cluster directories cannot be merged. The new design must not point one server
at six old volumes or repurpose the Auth cluster as all-service storage.

Existing volumes are `<project>_{auth,user,course,enrollment,assessment,analytics}-db-data`,
mounted at `/var/lib/postgresql`, appropriate to PostgreSQL 18's major-specific
PGDATA layout. The installed entrypoint invokes `initdb --username=$POSTGRES_USER`
and processes `/docker-entrypoint-initdb.d` only for a new data directory.
No repository SQL initializer is mounted there currently. Changing environment
values on a populated volume does not rotate roles/passwords or create new DBs.
These semantics are also documented by the [official PostgreSQL image](https://hub.docker.com/_/postgres).

### Identities, access and ownership

Every app and its migrator use the same `<service>_app` identity listed in
section 3. That identity is also the cluster's `POSTGRES_USER`, which the official
entrypoint initializes as a superuser. The historical Phase 5 report documents
read-only role inspection and a failed attempt to demote the PostgreSQL 18
bootstrap role. No later role-separation implementation was found. Treat all
six bootstrap/runtime identities as excessively privileged until safely queried.
No separate migration logins, limited runtime grants or cross-DB revocations
exist in repository initialization/migration SQL.

**Fresh catalog verification is BLOCKED:** all DB containers are stopped and
starting them is prohibited. Thus actual `rolsuper`, object owners, ACLs,
additional manually created users, row counts and applied migration versions
are not asserted as freshly queried facts. The old report's temporary QA admin
role removal is historical evidence, not a new catalog observation. Current
app/migration credentials and configured usernames match the bootstrap model;
network isolation alone does not remediate the role risk. Separate physical
clusters currently limit cross-service DB reach, but consolidation will increase
the blast radius unless least privilege is implemented **before** app cutover.

### Logical schema and migrations

| Database | Migration directory under `services/` | Latest source migration | Principal tables |
| --- | --- | --- | --- |
| `authentication_and_security` | `security-and-authentication/migrations` | 000006 | users/roles/permissions, sessions/refresh/action tokens, inbox, audit |
| `users` | `user/migrations` | 000004 | users, user_status_history, outbox_events |
| `courses` | `course/migrations` | 000001 | courses, course_offerings, course_audit_events |
| `enrollments` | `enrollment/migrations` | 000001 | enrollments, enrollment_offering_guards |
| `assessments` | `assessment/migrations` | 000003 | assessments/submissions/grades/revisions, extensions/file types, final-result imports, outbox, nonces |
| `analytics` | `analytics/migrations` | 000001 | programme/grade-band configuration, student assignment, grade/result projections, inbox/projection state, nonces |

Numbers above are **source versions**, not queried applied versions. All six
`cmd/migrate/main.go` tools use service-specific DB variables and golang-migrate;
Compose runs `-path /app/migrations -direction up` and waits before app startup.
Each logical DB retains its own migration history. No SQL role/database/extension
provisioning was found in those business migrations. Normal schema creation,
constraints and permission catalogue seeding exist and must be preserved byte
for byte. Do not collapse all six schemas into one database.

TCP `pg_isready -h 127.0.0.1` avoids the historical bootstrap-socket race, but
proves neither six DB readiness nor restricted-user authentication. Five service
pools explicitly allow 20 connections each; Analytics uses pgxpool defaults.
Those five alone can consume 100 connections, before Analytics, migrations,
backups and operator sessions. Shared-cluster sizing must budget all of them.

Readiness needs runtime SELECT on `schema_migrations` after privilege separation,
but runtime must not mutate it. Auth readiness accepts version >=4 although six
source migrations exist; Assessment accepts >=1 although three exist. User
requires >=4 and Course/Enrollment/Analytics >=1. Release-specific validation
must compare actual versions to the selected release, not rely on the weaker
minimum readiness threshold alone.

### Backup and restore

`infrastructure/scripts/backup.py` produces six custom-format `pg_dump -Fc`
archives, private object mirrors, safe configuration copies, image IDs and
SHA-256/size manifest. It assumes six `*-db` services and `POSTGRES_USER` per DB;
it must change for consolidation. Logical dumps do not back up cluster role
passwords/global grants; secrets/role definitions need separate recovery custody.

Three retained QA snapshots are under `%LOCALAPPDATA%/UCM/backups/qa/`, including
`20261123T115640Z-85dc94fd`. Phase 0 **verified that snapshot's 12 file checksums**;
no new dump, archive-listing container, synthetic object or restoration ran.
Restricted ACL inspection found user/SYSTEM/privileged admin/owner entries;
QA runtime credential ACLs grant user/SYSTEM. Backups remain plaintext locally,
without an active encrypted off-site schedule or purge/retention automation.
No DEV or PROD-like snapshot was identified in this inspected backup root.
Historical archive/synthetic-object tests are not a full restore drill. The
guide describes restoring to a verified empty target with ownership review;
that drill is required before DEV cutover. Backups are per-DB snapshots, not
cross-service/object atomic recovery points.

## 7. Ansible and manual deployment

`infrastructure/ansible/` contains three local inventories (`dev`, `qa`,
`prodlike`), four playbooks (`deploy`, `validate`, `health-check`, `rollback`),
and `preflight`, `secrets`, `application`, `nginx`, `health-check` roles. Directory
mapping for PROD-like is deliberately `environments/prod`, not a fourth target.
Preflight checks identity, local connection, Linux engine and Compose >=2.24.4.

Manual PowerShell entry point **exists**: `infrastructure/ansible/invoke.ps1`.
It creates a disposable controller with host-equivalent Docker-socket access,
not a self-hosted GitHub runner. It handles inventory, build/existing/pull modes,
manifest and external Vault password file. The Python lifecycle tool can also
be called from PowerShell. Do not replace working Ansible simply because the
approved operator interface is manual PowerShell.

All inventories default to `deployment_mode: existing`, not mandatory builds.
`pull` requires a nine-digest manifest; `build` with a manifest is refused.
Project lock prevents overlapping helper deployments within one environment;
peer-rehearsal checks refuse QA and PROD-like running together on this PC. Direct
Compose bypasses these checks. Lock and previous-manifest handling are basic
local mechanisms, not transactional, cluster-wide release coordination.

Deploy runs automatic migration/setup dependencies, readiness, image ID checks,
Nginx syntax and unauthenticated HTTP checks. Stale-image fallback/recovery and
`PLATFORM_CHANGED`/container-ID comparison support repeat execution. Historical
QA/PROD-like runs reported `changed=0` on repeats. **No deployment or idempotency
run was executed in Phase 0.** There is no atomic multi-service rollout or
automatic data rollback; partially failed deploys can leave mixed state, and
current/previous manifests are recorded only after successful release deployment.
The DEV Auth replacement guard occurs after the initial `up`, so it must not be
treated as a transactional guarantee against notification/container replacement.

`validate.yml` includes the secrets role and can render Vault files when actual
Vault inputs are present; its name does not make it universally read-only.
Similarly `ci_models.py` initializes missing rehearsal files. Neither was run
as a generic validation shortcut in this audit. The Windows wrapper does not
forward private-registry credentials; native Linux with operator credentials or
a reviewed ephemeral login mechanism is needed before registry-mode validation.

Native Ubuntu still has no `/var/run/docker.sock`; Docker reports the missing
Desktop integration. Host Go is 1.27.2 (modules need 1.27.3). No host
`ansible-playbook`, `gitleaks`, `trivy` or `pg_restore` executable was discovered.
The controller fallback is historically tested but creating it is prohibited in
this phase. No competing WSL Docker Engine or Desktop setting was installed.

## 8. GitHub Actions assessment

| Workflow | Actual responsibilities | Trigger / privilege / retention |
| --- | --- | --- |
| `.github/workflows/ci.yml` | six Go jobs, frozen pnpm install/available checks/build/audit, infrastructure tests/history scan/models/Ansible syntax, nine image build/scans | PR; non-main/non-tag pushes; reusable call. `contents: read`, full-SHA official actions, per-ref cancellation, bounded job timeouts |
| `.github/workflows/publish-images.yml` | reusable CI prerequisite; trusted-ref nine-image GHCR build/push, exact-digest scan, manifest aggregation | main, `v*`, manual; `packages: write` only publisher, GITHUB_TOKEN; ref serialization; per-image artifacts 30 days, release manifest 90 days |
| `.github/workflows/prepare-deployment.yml` | inspect successful publish run, validate/download nine-image manifest, environment-selected approved artifact | manual QA/PROD-like; contents/actions read; artifact 30 days; no deploy/SSH/PC runner |

No `pull_request_target` publish path exists. Checkout disables persistent
credentials; actions and scanner images are pinned. Release aggregation has no
explicit job timeout and preparation has no deployment concurrency lock; these
are bounded operational improvements for later implementation. Published images
are scanned **after push**, so a failed scan can leave unapproved package blobs
but no approved manifest. Selection must use only approved manifests.

**Observed GitHub results, not triggered by this audit:**

| Run | Source / event | Result |
| --- | --- | --- |
| [45186313447](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/45186313447) | 334d412, branch push | CI failure; infrastructure model step failed |
| [45186338309](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/45186338309) | 334d412, PR #12 | CI failure; same failing step |
| [45186478945](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/45186478945) | a2df85f, merged main push | verification failed; publish and release-manifest skipped |

Main run jobs explicitly report success for six Go matrices, frontend, all nine
image build/scans, Python safety tests and history secret scan. Infrastructure
job `137733297828` fails in `ci_models.py:15` calling `platformctl.py:model/run`
with `RuntimeError: docker failed (exit 1)` immediately after QA initialization.
Ansible syntax and Git diff steps are skipped. The helper suppresses captured
stderr to avoid leaking secrets, so **the underlying Compose diagnostic is not
known**. Do not invent an env/Compose-version cause. Local models passing with
already initialized files did not exercise the fresh-checkout runner failure.

The main run has nine Docker build-record artifacts, **no `immutable-release`
artifact**. A build record is not a deployed release manifest. Owner settings:
Actions enabled, all actions allowed, no environments configured; branch
protection GET returns 403 requiring GitHub Pro or public-repository eligibility.
No settings were changed. Historical “GitHub execution unverified” statements
must now be read as pre-merge history, not current pipeline status.

Workflow defaults API reports read-only default permissions and no ability for
Actions to approve pull-request reviews. All three repository workflows are
active; repository-wide SHA pinning is not enforced even though these workflow
action references are pinned.

## 9. GHCR and artifact readiness

Workflow namespace is `ghcr.io/kavindacena99/university-course-management-system/`
with `auth`, `user`, `course`, `enrollment`, `assessment`, `analytics`, `frontend`,
`gateway`, `assessment-minio-init`. Exactly nine deployable image keys are
expected. Supporting PostgreSQL/storage are external vendor images, not extra
microservices. All required Dockerfiles exist and their contexts are correct.

Observed local application tags are `:local` and `:phase345`; no local GHCR app
images were observed. Workflow tags include full `sha-<commit>`, branch and
metadata-action SemVer tags; `latest` is disabled. Tags remain mutable, including
SHA-looking tags. `validate_release` insists on exactly nine sha256 references,
but does not bind them to a particular source commit, registry namespace,
candidate version, approval or config digest. No actual current/previous
manifest exists under `.infra-state/{dev,qa,prodlike}`.

All nine local `:phase345` images lack an OCI source-revision label, so their tags
alone do not establish a source commit. Their retained vulnerability reports
were correlated to the exact installed image IDs below (not asserted to be
GHCR registry digests or newly scanned images):

| Local image (`:phase345`) | Installed image ID, sha256 |
| --- | --- |
| ucm-auth | `8e998332e3b65e5441f0ecccbf51217ad4d1c92ac1f080de641c59c7621aac41` |
| ucm-user | `426287a95a8401718d991072fec162cb6e9fccdd91c93e46bdf73c7bab956496` |
| ucm-course | `4e2d4ad62e81a3a59d84da6f86c1ab0390a693adbf2e2fb279c10c6743c566d7` |
| ucm-enrollment | `a6ce93ebf43ad1abf51d2064ca99277cfa3e12d2a8b2c7db8119107bd8a28a9a` |
| ucm-assessment | `b69206553291877c8d889f934d493354ff768a546911897430a0d3a5d110b662` |
| ucm-analytics | `6a3500a1d4a46b5ece3c1991f606419f6879caa03c046fdeed34695e480bfb1b` |
| ucm-gateway | `be4ad2fc3559112d9e838100ca65dd82964f33badcf0f198cf1d279c95c4f497` |
| ucm-frontend | `519a8077287d5fc6787b1ec61b42a1eb9e1d38ce289c3ba764c237568526090d` |
| ucm-assessment-minio-init | `549a05fdae3331eb06e6205fd0e86e8bad675b18dafa60d4876b60b937969473` |

Edge image metadata explicitly reports `NGINX_VERSION=1.30.7`. Six Go image
users are `app`; edge/helper images use the default root startup model discussed
above. No runtime executable was launched to obtain these facts.

GHCR package-list API returned 403 requiring `read:packages`. Existence,
visibility and access of other remote packages are **BLOCKED**, not presumed
absent/private. The inspected runs did not reach publication. No login refresh,
package modification, registry pull/push or workflow dispatch was performed.
The current nine-image env manifest format could select historical images if
available, but does not preserve a complete deployment's supporting-image pins,
config version, migration state, source/approval metadata and secret versions.

## 10. Versioning and release management

No Git tags or GitHub releases were observed. Frontend `package.json` is 1.0.0;
this is package metadata, not an application release bill of materials. No
candidate/QA approval record or tag-to-approved-manifest mapping was found.

| Approved step | Existing behavior | Assessment |
| --- | --- | --- |
| Dev image `dev-<sha>` | SHA/branch tags, examples `:candidate`, local validation `:phase345` | Tag convention not aligned |
| `vX.Y.Z-rc.N` candidate | `v*` tag triggers publisher; metadata supports SemVer-shaped refs | No strict RC policy or approved source binding |
| Candidate to QA | manual selected manifest + `pull` supported | No successful published manifest/pull deployment evidenced |
| QA release validation and approval | health/unauthenticated probes exist; preparation uses GitHub environment | No full QA suite/evidence/approver record; environments not configured |
| Final tag on same source | Git accepts a final ref; no implementation checks equality to approved candidate | Missing enforcement |
| Exact digest reuse for final | every final `v*` push runs another build/push | **Conflicts with target; caching is not promotion** |
| Historical release deployment | validated digest file accepted | Manifests expire after 90 days; no durable release catalogue/assets |
| PROD-like smoke only | current Ansible does health/Nginx/401 checks, not full tests | Useful base, DB connectivity indirect; explicit release smoke profile missing |

Preserve one tested candidate manifest through QA approval/final promotion;
tag aliases must not cause builds. Persist source SHA, candidate/final version,
nine application digests, supporting-image/config/migration references and QA
approval/test evidence durably. Final and candidate source equality and identical
image digest sets need machine verification. This belongs to subsequent release
implementation; it was not silently fixed in Phase 0.

## 11. Testing and validation inventory

**Status convention:** Phase 0 executed checks are marked executed. “Remote
success observed” means existing GitHub job results inspected, not a local test
rerun. Historical pass claims alone are not current verification. Go files with
database integration tests skip when `TEST_DATABASE_URL` is unset; current CI
does not provision those DBs or set it. A green Go job therefore does not prove
repository integration tests ran.

| Suite / location | Scope | Environment and exact execution command | Phase 0 status / dependencies |
| --- | --- | --- | --- |
| Auth: `services/security-and-authentication/internal/{config/config_test.go,internal_caller_test.go,internal_student_test.go,passwords_test.go,service_test.go,serviceauth_test.go,test_helpers_test.go,tokens_test.go}` | 8 files / 43 test functions; credentials, JWT/session/permissions, HMAC, configuration; some DB integration | Go 1.27.3, service directory: `go test ./...`; CI also `go mod verify`, `go mod tidy -diff`, `go vet ./...` | NOT EXECUTED locally; remote job success observed. DB portions require migrated `auth_test`, are skipped without TEST_DATABASE_URL |
| User: `services/user/internal/{config/config_test.go,handler_test.go,jwt_test.go,outbox_test.go,repository_test.go,service_test.go,serviceauth_test.go}` | 7 files / 71 functions; lifecycle/handler/auth/outbox/repository | Same command in `services/user` | NOT EXECUTED locally; remote job success observed. DB integration writes/cleans `users_test` |
| Course: `services/course/internal/*_test.go` and `internal/config/config_test.go` | 12 files / 48 functions; lifecycle, HTTP/JWT/HMAC, clients, shutdown, DB concurrency | Same command in `services/course` | NOT EXECUTED locally; remote job success observed. Repository tests require migrated `courses_test` |
| Enrollment: `services/enrollment/internal/*_test.go` and `internal/config/config_test.go` | 6 files / 17 functions; clients/config/handlers/model/service/capacity concurrency | Same command in `services/enrollment` | NOT EXECUTED locally; remote job success observed. Repository tests require `enrollments_test` |
| Assessment: `services/assessment/internal/final_results_test.go` | 1 file / 1 function; final-result import parsing | `cd services/assessment; go test ./...` | NOT EXECUTED locally; remote job success observed. Real storage/DB/auth flows not covered by this small suite |
| Analytics: `services/analytics/internal/model_test.go` | 1 file / 3 functions; report/grade calculations | `cd services/analytics; go test ./...` | NOT EXECUTED locally; remote job success observed. No discovered real projection/DB integration suite |
| Formatting: all six service trees | Go format compliance, read only | `gofmt -l services` | EXECUTED: empty output |
| `tests/infrastructure/test_platform.py` | 8 synthetic target, overwrite, isolation, release-input and deployment-lock guards | `.infra-venv/Scripts/python.exe -B -m unittest discover -s tests/infrastructure -v` | EXECUTED together with backup tests: 11 passed; temporary fixtures, ROOT redirected outside checkout |
| `tests/infrastructure/test_backup.py` | 3 synthetic checksum/tamper/path traversal tests | Same unittest command | EXECUTED; no real database restore |
| `infrastructure/docker/validate.ps1` | 16 unauthenticated HTTP/header probes, gateway syntax, 22-component assertions | `powershell -NoProfile -File infrastructure/docker/validate.ps1` | IMPLEMENTED_UNVERIFIED in this audit; requires running DEV. Hardcodes DEV override despite adjustable project name; not safe to treat as QA selector |
| `infrastructure/docker/validate-scaling.ps1` | scale Analytics, correlate logs/DNS, stop/remove replica, replace, outage/recovery | `powershell -NoProfile -File infrastructure/docker/validate-scaling.ps1` | NOT EXECUTED: intentionally mutates containers; explicitly prohibited in Phase 0 and inappropriate for normal PROD-like smoke |
| `platformctl.py health` / Ansible health-check | 22-service state set, 15 healthy runtimes, seven jobs, 12 unauthenticated probes | `.infra-venv/Scripts/python.exe -B infrastructure/scripts/platformctl.py health --environment qa` (or dev/prodlike) | BLOCKED: all stacks stopped. No claim of authentication or end-to-end success |
| Three Compose models | identity, ports, persistence/credential isolation | In-memory `platformctl.model(e)` then `verify_model(e,c)` for all three existing environments | EXECUTED: all passed; no init/deploy calls |
| `ci_models.py` | fresh-checkout rehearsal generation + models and DEV examples | `python infrastructure/scripts/ci_models.py` | NOT EXECUTED locally because it may create env/key files; existing remote job FAILED |
| Ansible inventories / syntax | three graphs × four playbooks | `ansible-inventory -i inventories/<env>/hosts.yml --graph`; `ansible-playbook -i inventories/<env>/hosts.yml playbooks/<name>.yml --syntax-check` | IMPLEMENTED_UNVERIFIED in Phase 0; host Ansible absent, controller creation prohibited; main CI step skipped |
| Vault synthetic test | encrypt/decrypt/0600 + validate role | `python3 ../scripts/validate_vault.py` inside controller with explicit ANSIBLE_CONFIG | NOT EXECUTED: controller creation prohibited; test can run secrets-role materialization logic; historical evidence only |
| `scan_secrets.py` | redacted history + non-ignored tree using pinned Gitleaks | `.infra-venv/Scripts/python.exe infrastructure/scripts/scan_secrets.py` | NOT EXECUTED: creates scanner containers. Remote main history scan success observed; no new full-tree security pass claimed |
| `scan_images.py` | nine app/helper Trivy HIGH/CRITICAL gates | `.infra-venv/Scripts/python.exe infrastructure/scripts/scan_images.py --skip-db-update` | NOT EXECUTED: creates containers/reports. Cached nine-image zero results inspected; remote nine scan steps successful |
| Existing QA backup integrity | size and SHA-256 of six dumps/object/config | `.infra-venv/Scripts/python.exe -B infrastructure/scripts/backup.py verify --path "$env:LOCALAPPDATA/UCM/backups/qa/20261123T115640Z-85dc94fd"` | EXECUTED: 12 files verified; no archive restore |
| Backup generation/restore | dump/list/object mirror; separate manual recovery | `backup.py create --environment qa`; restore guide in `docs/infrastructure/deployment-guide.md` | NOT EXECUTED; generation creates client container, optional sentinel writes; restore is state-changing and untested here |
| Frontend build/audit | frozen dependency install, `dist`, prod dependency audit | In `frontend`: `pnpm install --frozen-lockfile`; `pnpm build`; `pnpm audit --prod --audit-level high` | NOT EXECUTED locally to avoid build outputs/installation; remote job success observed |
| Frontend lint/unit/browser E2E | expected QA release behavior | No lint/test/e2e scripts or browser test suite found in tracked frontend/tests | MISSING; `format` is mutating formatting, not lint/test |
| End-to-end auth/authorization/API/DB QA release suite | authenticated multi-service scenarios and approval evidence | No single release-validation entry point exists | MISSING/PARTIAL: service tests/contracts exist, unauthenticated smoke is insufficient |
| PROD-like release smoke profile | exact release IDs, DB connectivity, gateway/non-destructive smoke only | Existing health-check playbook is a base | PARTIALLY IMPLEMENTED; no version-bound smoke/approval contract |

DB test helpers explicitly refuse other database names and run DELETE cleanup;
these tests must never be aimed at the six application databases. Isolated
throwaway QA test databases should be outside the six persistent business DB
set. No such DBs were provisioned in Phase 0. Do not add the full QA suite to
PROD-like as a shortcut.

## 12. Security findings

1. **Database privileges:** bootstrap/migration/runtime separation missing.
   Consolidating before isolation/grant tests would materially increase impact.
2. **Retained PostgreSQL findings:** `.infra-state/security/postgres-vulnerabilities.json`,
   timestamp 2026-11-23T09:36:47Z, matches installed digest. Recount: 26 HIGH,
   one CRITICAL. Twenty-six stdlib findings are in `/usr/local/bin/gosu`
   (Go v1.24.6), plus one libxml2 finding. The CRITICAL entry is
   `CVE-2025-68121` (TLS certificate validation during resumption). Binary
   inclusion/severity does not establish reachable exploitation; no reachability
   test was executed. Retain the evidence, triage usage with vendor provenance,
   and scan a compatible candidate in an implementation phase. Do not blindly
   switch major version, base distribution, PGDATA directory or physical volume.
3. **Storage finding:** retained November image report has HIGH
   `CVE-2026-39414` against upstream module identity. Installed OCI revision is
   `44e848fedf8cec3d8c0da4b1aa8422390a2b7a8b`. The [vendor advisory](https://silo.pgsty.com/blog/security/cve-2026-39414/)
   describes completed June fixes; historical report records ancestry evidence.
   This supports backport review, not automatic scanner suppression or a fresh
   exploitability verdict. A signed/reviewed VEX decision is not present.
4. **Secrets:** actual env/keys/Vault/passwords/dumps/logs are ignored; zero
   tracked runtime credential paths found. Safe examples and `roles/secrets`
   source are included. No actual `inventories/*/vault.yml` exists: templates
   and integration are present, real environment Vault enrollment is not.
   Auth uses the existing file loader via a Compose file secret; others remain
   env-based and Docker-admin visible. Compose secrets are not encrypted storage.
5. **Host/sync:** credential ACLs restrict ordinary access; the checkout is under
   OneDrive, which `.gitignore`/ACLs do not stop syncing as the current user.
   Backup defaults are outside sync, but sensitive data is plaintext locally.
6. **Networks/containers:** private ports and rehearsal hardening are present;
   DEV is all-interface on two edge ports and less restricted. App proxy trust
   uses broad Docker 172.16/12 assumptions; actual allocator subnet changes or
   future load balancers need explicit trusted-network review.
7. **GitHub/GHCR:** least-privilege workflow tokens exist, but no environment
   reviewers and no verified enforced branch protection. Package permissions
   remain unknown. Current token inspection is not package access verification.
8. **TLS/cloud:** local HTTP and `AUTH_COOKIE_SECURE=false` are explicit rehearsal
   exceptions. No live certificate, renewal, SSH baseline, workload identity or
   cloud secret manager exists. Do not describe documented plans as active.

## 13. Nginx/API Gateway assessment

Keep `infrastructure/nginx/nginx.conf`, `conf.d/00-upstreams.conf` (host),
`compose/00-upstreams.conf` (Docker), `conf.d/10-default.conf`, and
`snippets/{proxy-headers,security-headers}.conf` separated. Dockerfile installs
only its Docker upstreams at the active name. Six upstream zones use
`least_conn`, DNS `127.0.0.11 valid=10s ipv6=off`, `resolve`, keepalive 32,
`max_fails=3 fail_timeout=10s`. The pinned Nginx 1.30.7 supports this architecture.

Proxy paths have no URI rewrite; full `/api/v1` paths remain intact. Nested
offering ownership uses specific regex locations before Course fallback.
All six service registrations were compared with routing. `/internal` and
unknown routes fall to 404. Gateway `/health` returns a constant 200 and does
not verify upstreams. API rate limit is 30/s, login/forgot 12/min, registration
6/min with scoped bursts; exact `/api/v1/users` limiting affects every method,
not only registration. Connect timeout 5s and send/read 30s allow transient
502/504 during DNS convergence; no zero-error failover guarantee exists.

Public edge overwrites forwarded headers, API security headers apply on errors,
and optional real-IP trust is commented until an actual trusted LB exists.
CORS is handled by service middleware/environment, not a gateway-wide permissive
wildcard. Assessment routes allow 21 MiB uploads and 6 MiB result imports while
applications retain their own limits. Static inspection supports the intended
gateway boundary; live `nginx -t`, route/DNS/capability/failure tests were **not**
run because every gateway is stopped and audit cannot start containers.

## 14. Frontend integration dependencies

React 19, Vite 8, Tailwind 4, React Router 7, Node 22, pnpm 10.34.3. Build is
`pnpm build`; runtime is static Nginx, not Vite's dev/preview server. Figma dev
plugins in `frontend/vite.config.ts` are serve-only. `frontend/AGENTS.md`'s
already-running Figma server claim is authoring-environment guidance, not
evidence of a running deployment here.

No `fetch`, Axios, VITE API-base usage or direct backend-port call was found
in tracked frontend source. This means API integration is absent, not proved
correct. Frontend Nginx serves SPA/assets and does not proxy `/api`; the browser
must call a reachable gateway origin. Its CSP initially permits localhost:8080;
rehearsal initialization renders the appropriate 8081/8082 CSP. Internal Docker
hostnames are not browser API addresses.

`VITE_API_BASE_URL` remains a build ARG and is unused today. Once consumed by
frontend code it would be baked into the shared assets: changing only rehearsal
CSP/env would not update it. That conflicts with immutable cross-environment
promotion. Frontend engineers need an agreed runtime API-origin mechanism or
reviewed same-origin gateway proxy before enabling integration. Preserve the
single API gateway and never add public microservice ports. Existing missing
SVG warnings, absent browser suite and complete authentication/CSRF flows remain
application-team dependencies; no React or Nginx changes were made here.

## 15. Documentation assessment

| Source | Useful baseline | Conflict / follow-up |
| --- | --- | --- |
| `docs/infrastructure/architecture.md` | current three-project section, ports, topology, routes and historical details | later historical module audit says Go 1.27.2 while current modules use 1.27.3; six clusters per env conflicts with new target |
| `docs/infrastructure/phase-1-2-report.md` | dated Docker/scaling evidence and limitations | DEV-only/no-CI statements belong to that historical scope, not current state |
| `phase-3-report.md`, `phase-4-report.md`, `phase-5-report.md`, `validation-ledger.md` in same directory | idempotency/Vault/build/backup evidence, manual prerequisites | pre-merge “no GitHub execution/no commit” and DEV running are historical; current runs exist and all stacks are now stopped; PostgreSQL severity prose disagrees with retained JSON |
| `docs/infrastructure/deployment-guide.md` | existing operational commands, local secrets, backup/restore and controller | six-DB-service assumptions and pre-merge runtime claims must change after implementation; no RC/final immutable promotion procedure |
| `docs/database.md` | four established schemas and guarded test DB procedure | begins “Four PostgreSQL databases”; lacks Assessment/Analytics schema and new target cluster/role design |
| `docs/development-setup.md` | native/Docker setup and migration commands | Go 1.27.2 prerequisite stale; Auth scaling example needs process-local-state caution |
| `docs/deployment-readiness.md` | dated service readiness and Docker observations | three-service/Go 1.27.2/no-CI statements are not current infrastructure facts |
| `README.md`, `docs/architecture.md`, `docs/implementation-report.md`, `docs/service-integration.md`, `docs/security.md` | responsibilities and signed HTTP/security behavior | older structure/data descriptions and pre-implementation claims need a future canonical-current-state pass, preserving dated history |
| `infrastructure/environments/{qa,prod}/.env.example` | safe placeholders and image/origin inputs | header still names nonexistent `platform.py`; actual helper is `platformctl.py` |

No historical document was overwritten. The implementation plan specifies which
living documents should be updated only after validation supports new claims.

## 16. Technical debt and files to preserve

Preserve all six module directories, migration SQL/history, gateway architecture,
private object policy/data, independent environment credentials/key material,
existing named volumes, env-file conventions and current route/API contracts.
The legacy validation scripts are not duplicates of release QA: they are DEV
smoke/failure exercises, hardcode the DEV override and 22-component layout, and
must be explicitly adapted or labelled rather than blindly reused for QA.
`platformctl.py` and `backup.py` also encode six DB service names. Shared-image
migration reuse, generated Nginx files and basic manifest rollback are existing
choices with known limits, not unused code to delete. The empty ignored root
`backups/` directory is not evidence of an active backup system.

Missing operational pieces include safe multi-cluster-to-one migration,
ownership/grant transition, connection budgeting, verified recovery, durable
release manifests/approval binding, authenticated QA tests and frontend runtime
API configuration. No extra infrastructure product is needed to start this work.

## 17. Risks and blockers

See [risk register](phase-0-risk-register.md) for priorities and ownership.
Immediate risks are data loss during consolidation, bootstrap superuser scope,
cached critical supporting-image finding, failed CI, final-tag rebuilds and
unverified restore. External limits are WSL integration, GitHub plan/package
permissions, shared-PC capacity and unavailable real cloud targets. Stopped
environments are a deliberate audit boundary, not an invitation to restart them.

## 18. Verification commands, results and readiness

Executed read-only commands (repository root; all Git invocations used
`git -c safe.directory=C:/Users/user/OneDrive/Desktop/ESD/project/University-Course-Management-System`):

```powershell
git status --porcelain=v1 --untracked-files=all
git branch -avv
git log -7 --oneline
git tag --list
git ls-files
docker version
docker info --format '{{json .}}' # parsed in memory; only platform/CPU/RAM printed
docker compose version
docker ps -a --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.Ports}}'
docker volume ls --format '{{.Name}}'
docker network ls --format '{{.Name}}'
docker image ls --format '{{.Repository}}:{{.Tag}}|{{.ID}}|{{.Size}}'
docker inspect <existing-project-container-ids> # parsed in memory, credentials never printed
docker image inspect postgres:18-alpine
docker cp platform-dev-auth-db-1:/usr/local/bin/docker-entrypoint.sh - # tar parsed in memory
.infra-venv/Scripts/python.exe -B -m unittest discover -s tests/infrastructure -v
gofmt -l services
go version
wsl --list --verbose
wsl -d Ubuntu -- sh -lc 'command -v docker; test -S /var/run/docker.sock && echo SOCKET_PRESENT || echo SOCKET_ABSENT; docker version --format "{{.Server.Version}}" 2>&1 | head -n 5'
.infra-venv/Scripts/python.exe -B infrastructure/scripts/backup.py verify --path "$env:LOCALAPPDATA/UCM/backups/qa/20261123T115640Z-85dc94fd"
git check-ignore infrastructure/docker/.env infrastructure/environments/qa/.env infrastructure/environments/prod/.env infrastructure/environments/qa/runtime/auth.env infrastructure/ansible/inventories/qa/vault.yml
Get-Acl infrastructure/environments/qa/runtime/course.env
icacls "$env:LOCALAPPDATA/UCM/backups/qa/20261123T115640Z-85dc94fd"
```

The in-memory Compose audit used the following exact Python operations with
`python -B` (no initialization or state-writing helper action):

```python
import sys
sys.dont_write_bytecode = True
sys.path.insert(0, 'infrastructure/scripts')
import platformctl as p
for env in p.ENVIRONMENTS:
    config = p.model(env)
    p.verify_model(env, config)
```

This invokes `docker compose --env-file <existing-env> -p <project> -f
infrastructure/docker/compose.yaml -f infrastructure/docker/compose.<dev|qa|prod>.yaml
config --format json` and captures output in memory. Only safe fields were
printed. Result: three models passed. No fresh-checkout initialization was run.

GitHub reads used `gh api` with repository prefix
`repos/kavindacena99/University-Course-Management-System/` and these exact suffixes:
`actions/runs?per_page=30`, `actions/runs/45186478945/jobs?per_page=100`, the same
jobs endpoint for runs 45186338309 and 45186313447,
`actions/jobs/137733297828/logs`, `actions/runs/45186478945/artifacts`,
`actions/workflows`, `actions/permissions`, `actions/permissions/workflow`,
`environments`, `branches/main/protection`, `releases`, `tags`.
`users/kavindacena99/packages?package_type=container&per_page=100` returned 403.
An exploratory repository `check-suites` URL returned 404; jobs/run endpoints
provided the required evidence instead. The first job-log reader hit Windows
CP1252 decoding; a byte read followed by UTF-8 decoding recovered the traceback.
Some initial `rg` wildcard paths were invalid on Windows and were replaced with
directory/glob or Python traversal. No failed probe is recorded as a passed test.

Final boundary verification passed: branch `main`, full baseline HEAD, empty tag
set, Git index and every tracked file remain unchanged. Exactly the five Phase 0
Markdown deliverables are new. All 66 project containers remain stopped with
exit code 0; all 23 existing volumes (21 project data volumes) and 12 networks
(nine project networks) remain. Local Markdown link/UTF-8/whitespace checks passed.
An in-memory check against sensitive values in 21 existing env inputs found no
credential values in the audit documents; no values were printed. No commit,
tag, push, publication, workflow trigger, container start/stop/create, volume
mutation, database data/role write or implementation/configuration edit occurred.

**Implementation decision summary:**

- Ready: known service boundaries, module/Docker contexts, gateway routing,
  project-scoped resources, safe synthetic tests, current Compose model checks,
  manual controller interface and existing backup format.
- Change: six clusters per env to one through fresh volumes/logical migration;
  separate runtime/migrator/admin privileges; adapt helpers/checks/backups and
  budget shared connections; repair fresh-checkout CI diagnostic/root cause.
- Manual GitHub: protection-plan decision, required checks/reviewers/environments,
  package-read/access/visibility review and authorization for future publication.
- External blocks: native WSL socket, current stopped services for catalog/health,
  unavailable cloud host/identity/TLS, private-package inventory authorization.
- Safest order: recoverability and role/image baseline → isolated QA target →
  restore/privilege/migration/connectivity validation → sequential PROD-like
  rehearsal → approved DEV maintenance cutover → immutable release/QA approval
  automation. Do not migrate DEV first or rebuild the final QA-approved release.
- Before Phase 1 runtime work: agree the migration/recovery window, acquire a
  safe live inventory and verified backup, select a reviewed compatible image,
  confirm least-privilege design and capacity, and accept the rollback/write
  divergence rules in the [file-level plan](phase-1-implementation-plan.md).

No infrastructure implementation begins automatically from this report.
