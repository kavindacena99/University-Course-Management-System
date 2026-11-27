# Updated infrastructure validation ledger — roadmap Phases 1–3

Date: 2026-11-23. Commands run from the repository root on Windows Docker
Desktop unless noted. This ledger retains failures and a verbatim snapshot of
the old ledger below; it does not rewrite historical failures as passes.

`PY` below means `.infra-venv/Scripts/python.exe -B`. `PG` means
`infrastructure/scripts/postgres_rehearsal.py`. In the command table these are
readability abbreviations, not installed aliases. Programmatic operations are
marked as Python calls rather than represented as shell commands that did not
run. No real secret values, database row contents or private keys are included.

## New execution evidence

| Validation | Executed command / exact orchestration operation | Result | Evidence / limits |
| --- | --- | --- | --- |
| Git baseline | `git -c safe.directory=C:/Users/user/OneDrive/Desktop/ESD/project/University-Course-Management-System status --short`; `git ... rev-parse HEAD` | PASSED | main at `a2df85f`; five pre-existing untracked audit files preserved |
| Engine/context | `docker version`; `docker info`; `docker compose version`; `docker context show` | PASSED | Linux 29.9.3, desktop-linux, Compose 5.7.0, 8 CPUs/~7.63 GiB |
| Runtime/data baseline | `docker ps -a`; `docker network ls`; `docker volume ls`; `docker inspect <66-baseline-IDs>` | PASSED | Original 66 stopped; original 23 volumes captured |
| Source QA backup | Python `backup.verify(snapshot)` for `%LOCALAPPDATA%/UCM/backups/qa/20261123T115640Z-85dc94fd` | PASSED | 12 file checksums, recorded QA/project identity |
| Rehearsal initialization | `PY PG init`; `PY PG start` | PASSED | Only new project `platform-qa-pg-rehearsal-20261123`, fresh PostgreSQL volume |
| Dump preflight/restore | Python `postgres_rehearsal.restore(snapshot)`; per dump `pg_restore --list`, then `pg_restore --no-owner --no-acl --exit-on-error -h 127.0.0.1 -U <service>_migrator -d <actual-DB>` in new container | PASSED | All six source dumps PostgreSQL 18, restored into empty new schemas; no legacy mounts |
| Restored migrations | `PY PG migrate` / six `compose.target.yaml run --rm --no-deps <service>-migrate` | PASSED | Auth6/User4/Course1/Enrollment1/Assessment3/Analytics1, clean migration state |
| Real SQL security | `PY PG verify`; Python `sql()` positive/negative queries over TCP for 12 roles | PASSED | Own DB/schema ownership, DML/SELECT, no elevated flags, no runtime DDL/admin/history writes, cross-DB denial, no role escalation |
| Additional DROP denial | Python `sql('DROP DATABASE ...', ..., expect_failure=True)` for all 12 roles against an unrelated empty admin-owned probe | PASSED | Permission denied, probe removed by admin; restored fingerprints unchanged |
| Database persistence | `PY PG recreate` | PASSED | New container/new-volume reuse only; all six per-table row counts/content hashes match |
| Failed initialization | Disposable new PostgreSQL container with invalid SQL and tmpfs data | PASSED negative test | Deliberate initialization exited 3; no healthy success marker; no named legacy volume |
| Six fresh fixture migrations | `PY infrastructure/scripts/rehearsal_tests.py` | Migrations PASSED; aggregate test FAILED | Fresh disposable DBs on new cluster; Auth application test fails; see below |
| Auth database-backed suite | Go container `go test -count=1 ./...` with its disposable TEST_DATABASE_URL | **FAILED** | `TestServicePermissionsAndAudit`, `internal/service_test.go:504`: expected11/got23; unchanged application code |
| Auth baseline CI/unit path | Same pinned Go container with TEST_DATABASE_URL unset; `go mod download; go mod verify; go mod tidy -diff; go test -count=1 ./...; go vet ./...; go build -trimpath -o /tmp/service .` | PASSED, DB tests skipped | Separate baseline check does not erase the integration failure |
| User/Course/Enrollment/Assessment DB-backed tests | Same module/test/vet/build command with service-local disposable DB | PASSED | Actual available integration/unit tests; logs under local tests evidence |
| Intermediate Analytics fixture | Full rehearsal test attempt while grant helper was being refined | FAILED | Grant-helper mismatch; finally removed disposable fixtures; not counted as pass |
| Analytics isolated retry | `PY infrastructure/scripts/rehearsal_tests.py --only analytics` | PASSED | Fresh migration, tests, modules, vet and build; all six business fingerprints unchanged |
| All six Go formatting | Pinned Go1.27.3 container, `test -z "$(gofmt -l services/security-and-authentication services/user services/course services/enrollment services/assessment services/analytics)"` | PASSED | Read-only source mount, no network |
| Nine application/helper image builds | Python `run(compose(True) + ['build'])` with `COMPOSE_PARALLEL_LIMIT=2`, new rehearsal input image tags `:phase123` | PASSED | Correct independent module contexts; frontend build included |
| Helper final rebuild | `docker build -t ucm-assessment-minio-init:phase123 -f services/assessment/MinIOInit.Dockerfile services/assessment` | PASSED | CA trust, USER app; final runtime rerun after rebuild |
| Derived PostgreSQL build | `docker build -t ucm-postgres:phase123 infrastructure/postgres` | PASSED | Same PostgreSQL18.7, gosu1.20 rebuilt Go1.27.3, libxml2 fixed |
| Original PostgreSQL scan | Trivy0.76.2 `image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 --format json` against original pinned image | **FAILED vulnerability gate** | Fresh 26 HIGH / 1 CRITICAL, raw `security/postgres-current.json` |
| Derived PostgreSQL scan | Same strict Trivy command against `ucm-postgres:phase123` | PASSED | 0 HIGH / 0 CRITICAL, `security/postgres-candidate.json` |
| Same-major image compatibility | `PY PG select-image --image ucm-postgres:phase123` | PASSED | New cluster recreated with local digest; row/hash persistence and SQL tests retained |
| Nine final application scans | `PY infrastructure/scripts/scan_images.py --tag phase123 --skip-db-update --output-dir .infra-state/phase123/security/application` | PASSED after documented retry | All nine final ImageIDs matched reports with 0 HIGH/CRITICAL |
| Enrollment scanner execution | Initial full scan attempt | FAILED tooling | Exit1/no JSON report; not interpreted as clean |
| Enrollment scan retry | Same scan command with `--only enrollment` | PASSED | Valid JSON, 0 HIGH / 0 CRITICAL |
| Helper final scan | Same scan command with `--only assessment-minio-init` after final rebuild | PASSED | Valid JSON matches non-root helper image, 0 HIGH / 0 CRITICAL |
| Object-store scan | Strict Trivy scan of pinned Silo image | **FAILED vulnerability gate** | One HIGH CVE-2026-39414, no allowlist/suppression |
| Vendor backport evidence | Read-only vendor advisory and upstream GitHub compare API for fix `fd69c89` to OCI revision `44e848f` | Review incomplete | Ahead805/behind0 supports backport review; owner binary/VEX disposition still required |
| New SQL backups | Python `postgres_rehearsal.backup_cluster()` | PASSED | Six custom dumps, all TOCs; seven manifest files verified; external protected snapshot `20261123T163755Z-21699eee` |
| Password-free globals backup | `PY PG start`; `PY PG backup`; `PY PG stop` after adding `pg_dumpall --globals-only --no-role-passwords` | PASSED | New snapshot `20261123T165903Z-8c294f57`: eight checksummed files; only new database briefly started, then stopped; no second restore claimed |
| Full isolated application | `PY infrastructure/scripts/validate_runtime.py` (including final helper rebuild rerun) | PASSED | 18 components: 10 healthy runtimes, eight successful jobs |
| Image content/security | Runtime script: `docker image inspect`; network-none/read-only `docker run` artifact/user checks | PASSED | Six binaries executable/non-root; CA trust; no source/tests/docs/go/gcc/npm in six app runtimes |
| Nginx syntax | `nginx -t` in both new edge containers and isolated candidate image checks | PASSED | Existing gateway routes/DNS/snippets preserved |
| HTTP checks | Runtime script GET frontend `/` and referenced compiled JS; gateway `/health`, unknown/internal and six actual `/api/v1/...` protected routes | PASSED | 200 HTML/JS/health;404 unknown/internal;401 each service; security headers; not authenticated E2E |
| Graceful shutdown | Runtime script `compose.target.yaml stop --timeout 20` and inspect all18 | PASSED | All stopped/exited0, no volume deletion |
| Three independent targets | `PY infrastructure/scripts/prepare_postgres_target.py --environment dev` (also qa, prodlike) | PASSED, no deployment | Inactive generated inputs; 39 separate DB secrets, own signing/config/storage; three models have onePG/two volumes/all images pinned |
| Safety unit tests | `PY -m unittest discover -s tests/infrastructure -v` | PASSED | 18 tests: backup, deployment and new target isolation/credentials/barrier guards |
| Eight model validation | `PY infrastructure/scripts/ci_models.py` | PASSED | Both config JSON/quiet, resource/secret/path/build-context checks; synthetic temp inputs only |
| Original runner failure | Exact downloaded/checksummed Compose5.1.3 plus clean HEAD source/fresh fixtures | FAILED as expected | Duplicate `frontend.security_opt` items reproduced original three CI failures |
| Older/current Linux CLI validation | Controller container, copied Compose5.1.3 CLI plugin then built-in5.7.1, each running `python -B infrastructure/scripts/ci_models.py` | PASSED | All eight models passed both versions, no Docker socket |
| Workflow lint | `docker run --rm --network none --mount "type=bind,source=$PWD,target=/repo,readonly" --workdir /repo rhysd/actionlint:1.7.12 -color .github/workflows/ci.yml .github/workflows/publish-images.yml .github/workflows/prepare-deployment.yml` | PASSED | All three final workflows, exit0; no remote execution |
| Ansible syntax | Linux controller: `ansible-inventory -i inventories/<env>/hosts.yml --graph`; `ansible-playbook -i inventories/<env>/hosts.yml playbooks/<playbook>.yml --syntax-check` | PASSED | Three inventories, four playbooks each (12); no deployment/data operation |
| Final preservation/image/target checks | `PY .infra-state/phase123/final_checks.py` | PASSED | 66 old IDs/start timestamps unchanged,23 original volumes present,10 document hashes unchanged,52 unique new DB secrets, nine final scan/ImageIDs match |

## Final review checks

| Validation | Executed command | Actual outcome |
| --- | --- | --- |
| Redacted secret scan | `PY infrastructure/scripts/scan_secrets.py` | PASSED: 21 commits, 392 tracked/non-ignored working-tree files, no leaks found |
| Whitespace | `git -c safe.directory=C:/Users/user/OneDrive/Desktop/ESD/project/University-Course-Management-System diff --check` | PASSED, no output |
| Application/config preservation | `git ... diff --exit-code -- 'services/**/*.go' 'services/**/go.mod' 'services/**/go.sum' 'services/**/migrations/**' infrastructure/nginx frontend/src infrastructure/docker/compose.yaml infrastructure/docker/compose.dev.yaml infrastructure/ansible` | PASSED, no diff |
| Full tracked implementation diff | `git ... diff -- .github/workflows .gitignore frontend/Dockerfile infrastructure/docker/compose.qa.yaml infrastructure/docker/compose.prod.yaml infrastructure/scripts services` | Reviewed; only inventoried implementation changes |

Ignored generated secret data is deliberately excluded from the scanning snapshot
and separately checked against documentation for leakage. Documentation edits
after the scan correct migration-binary wording and add these non-secret outcomes;
the final generated-credential leakage/preservation check covers completed reports.

## Not executed / blocked / deferred

- New GitHub-hosted CI execution: **NOT EXECUTED**, requires later authorized
  commit/push; historical failed runs remain failed. No remote trigger.
- GHCR access/publishing, release tags/manifests/promotion: **DEFERRED**; previous
  access403 is not repaired by a read-only CI token. No registry login/publication.
- DEV/existing QA/PROD-like cutover: **NOT EXECUTED**, explicitly unauthorized.
- Auth permission-catalog correction: **BLOCKED ON APPLICATION OWNER**, outside
  infrastructure scope. Existing failing assertion retained.
- Object-store HIGH binary/vendor disposition: **REQUIRES OWNER REVIEW**.
- Full object-store/config/secrets restore and disaster-recovery drill:
  **NOT EXECUTED**; SQL restoration/backup verification alone is not full DR.
- New target Ansible deployment/idempotency, full authenticated E2E, browser
  visual QA, fresh replica/failure load tests: **NOT EXECUTED** in this phase.
- WSL Docker deployment: **BLOCKED** by unavailable socket integration; no
  Desktop settings or competing Docker Engine were changed.
- Cloud/TLS/SSH/remote deployment: **DEFERRED**, no servers/providers selected.

## Historical ledger snapshot — preserved verbatim below

The following is a snapshot of the original `validation-ledger.md` as it stood
before this implementation. Its dated results and changed-file inventory refer
to the earlier roadmap phases, not newly executed evidence. The source file
itself remains unchanged and its hash is verified against the baseline.

# Phases 3–5 validation ledger

Validation date: 2026-11-23. Commands below ran from the repository root unless
their working directory is shown. Commands use the existing ignored local inputs;
no credential values are reproduced. Repeated commands are listed once. This is
the acceptance-command record, not a transcript of every read-only file inspection,
registry metadata query, dependency download or diagnostic retry.

| Test | Executed command | Outcome |
| --- | --- | --- |
| Engine readiness | `docker version`; `docker info`; `docker compose version` | PASSED: Linux engine 29.9.3, Compose 5.7.0 |
| Starting-state inventory | `docker ps -a`; `docker network ls`; `docker volume ls`; `git status --short` | PASSED: existing DEV volumes preserved; initially no running stack |
| Compose isolation | `.infra-venv/Scripts/python.exe infrastructure/scripts/ci_models.py` | PASSED: DEV/QA/PROD-like project, ports, credential and resource guards |
| Final application image builds | `docker compose --env-file infrastructure/environments/qa/.env -p platform-qa -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.qa.yaml build` with `COMPOSE_PARALLEL_LIMIT=2` | PASSED: nine images; separate `:phase345` validation tags |
| Control image | `docker build -t ucm-ansible-control:local -f infrastructure/ansible/Controller.Dockerfile .` | PASSED: Linux Ansible 2.20.11 controller |
| QA validation | `powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment qa -Playbook validate` | PASSED |
| QA deployment/repeat | `powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment qa -Playbook deploy` | See final runtime evidence below |
| PROD-like deployment/repeat | `powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment prodlike -Playbook deploy` | See final runtime evidence below |
| Environment-only stop | `.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py stop --environment qa` and same for `prodlike` | No volume deletion; DEV excluded |
| Safety/integrity tests | `.infra-venv/Scripts/python.exe -m unittest discover -s tests/infrastructure -v` | PASSED: 11 tests |
| Go formatting | `gofmt -l services` in Go toolchain container | PASSED: empty output after formatting |
| Six Go modules | In each actual service directory, Go 1.27.3: `go mod verify`; `go mod tidy -diff`; `go test ./...`; `go vet ./...` | PASSED for all six services |
| Business-source preservation | `git diff -w --ignore-blank-lines --exit-code -- ':(glob)services/**/*.go'` with repository safe-directory option | PASSED: no non-whitespace Go source changes |
| Frontend production build | Production `frontend/Dockerfile` build, frozen pnpm lockfile | PASSED; two existing unresolved SVG warnings |
| Frontend audit | `pnpm audit --prod --audit-level high` in frontend build environment | PASSED: no known production dependency vulnerabilities |
| Frontend lint/unit tests | Inspected `frontend/package.json` scripts | NOT EXECUTED: scripts do not exist |
| Workflow syntax | `docker run --rm --mount "type=bind,source=${PWD},target=/workspace" --workdir /workspace rhysd/actionlint:1.7.12 -color` | PASSED: all three workflows |
| Image security gate | `.infra-venv/Scripts/python.exe infrastructure/scripts/scan_images.py --skip-db-update` | PASSED: nine rebuilt images, each zero HIGH/CRITICAL |
| Supporting-image security | Trivy image scans of pinned rehearsal storage and existing PostgreSQL image | FAILED raw gate: PostgreSQL 27 HIGH; storage one raw upstream-module match with vendor-backport evidence; see Phase 5 |
| Redacted secret scan | `.infra-venv/Scripts/python.exe infrastructure/scripts/scan_secrets.py` | PASSED: all 20 commits and 366 tracked/non-ignored files including this ledger |
| Vault | `python3 ../scripts/validate_vault.py` inside trusted controller, explicit `ANSIBLE_CONFIG`, repository and Docker socket mounts | PASSED: encrypt/decrypt, 0600 temporary inputs, encrypted-variable role execution |
| Ansible inventory/syntax | In controller `infrastructure/ansible`: `ansible-inventory -i inventories/<env>/hosts.yml --graph`; `ansible-playbook -i inventories/<env>/hosts.yml playbooks/<name>.yml --syntax-check` | PASSED: 3 inventories × 4 playbooks |
| Backup validation | `.infra-venv/Scripts/python.exe infrastructure/scripts/backup.py create --environment qa --validate-object` | PASSED: six archive listings, unique synthetic object recovery, checksum manifest; no restore |
| Ignore policy | `git check-ignore` on actual env/key/Vault/dump paths and safe examples/role source | PASSED: sensitive inputs ignored, examples and source included |
| Whitespace | `git diff --check` with safe-directory option | PASSED |
| Native WSL deployment | Ubuntu Docker endpoint/socket inspection | BLOCKED BY ENVIRONMENT: Desktop integration/socket unavailable; controller fallback tested |
| GitHub-hosted execution/GHCR publication | No dispatch, push or publication performed | NOT EXECUTED; authorization/review and owner setup required |
| GitHub protection settings | Read-only GitHub API Actions/environment/protection queries | Actions enabled, no environments; branch protection GET 403 due private-repository plan restriction |
| Live TLS/cloud/SSH | No remote target provisioned | DEFERRED; no fake certificate or risky local host modification |
| Destructive restore/full business E2E/browser visual QA | Not run | NOT EXECUTED; restore requires separate empty-target drill; frontend integration remains application work |

The first QA startup hit premature database readiness; TCP health checks fixed it
without changing migrations or clearing volumes. The first vulnerability gate
failed and drove dependency/package upgrades. The attempted bootstrap-role
demotion failed and was withdrawn, retaining that security limitation explicitly.
One manual Vault invocation omitted explicit `ANSIBLE_CONFIG` on a Windows mount;
the corrected invocation passed. No failed attempt is presented as a success.

## Final runtime evidence

Final deployment and cleanup results are recorded here after the patched-image
rehearsals. Existing DEV images are deliberately retained pending a controlled,
backed-up update; a green patched-image scan does not certify old DEV artifacts.

- Final QA deploy: `ok=13 changed=1 failed=0`; repeat: `ok=13 changed=0 failed=0`.
  Fifteen healthy runtimes, seven successful setup jobs and 12 HTTP probes passed.
  Host UI/gateway ports 3001/8081 returned 200. Effective read-only/no-new-privileges
  settings were inspected on all eight application/edge containers.
- The patched QA backup at local UTC `20261123T115640Z-85dc94fd` passed all six
  archive listings, synthetic object verification and the 12-file integrity
  manifest. It remains under LocalAppData, outside the synced checkout.
- Final PROD-like initial deploy: `ok=13 changed=1 failed=0`; host ports 3002/8082
  returned 200, and all eight application/edge restrictions were inspected.
  Repeat deployment: `ok=13 changed=0 failed=0`; both Nginx configurations,
  15 healthy runtimes, seven successful jobs and all 12 HTTP probes passed.
- Final cleanup inspection passed: only the original 15 healthy DEV runtimes
  remain running. QA and PROD-like containers are stopped. Each project's seven
  data volumes and three networks remain, totaling 21 volumes and nine networks.
  No data/volume prune or destructive restore was run. Git HEAD remains
  `159cf79eb027ff0765f88a59babb767b848b6c3d`; no commit or push was performed.

## Changed-file inventory

The generated inventory below lists every changed or added non-ignored file.
No files were committed and no previously tracked files were removed.

<!-- generated-file-inventory -->

| Change | File | Purpose |
| --- | --- | --- |
| MODIFIED | `.gitattributes` | Consistent line endings, build-context hygiene, sensitive-file exclusion or narrow scanner configuration. |
| MODIFIED | `.gitignore` | Consistent line endings, build-context hygiene, sensitive-file exclusion or narrow scanner configuration. |
| MODIFIED | `README.md` | Architecture, operating instructions and actual validation/status evidence. |
| MODIFIED | `docs/infrastructure/architecture.md` | Architecture, operating instructions and actual validation/status evidence. |
| MODIFIED | `frontend/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `infrastructure/docker/compose.yaml` | Shared Compose fixes or small isolated hardened environment override. |
| MODIFIED | `infrastructure/nginx/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/analytics/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/analytics/cmd/migrate/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/database/postgres.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/go.mod` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/analytics/go.sum` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/analytics/internal/app.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/clients.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/config/config.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/handler.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/model.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/model_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/repository.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/security.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/analytics/internal/service.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/assessment/MinIOInit.Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/assessment/cmd/migrate/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/database/postgres.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/go.mod` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/assessment/go.sum` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/assessment/internal/app.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/config/config.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/errors.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/filetypes.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/final_results.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/final_results_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/handler.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/httpx.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/internal_auth.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/logger.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/model.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/outbox.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/repository.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/security.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/service.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/internal/storage.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/assessment/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/course/go.mod` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/course/go.sum` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/course/internal/app.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/config/config.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/handler.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/handler_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/httpx.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/internal_offering_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/middleware.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/model.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/service.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/course/internal/serviceverify.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/enrollment/cmd/migrate/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/database/postgres.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/go.mod` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/enrollment/go.sum` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/enrollment/internal/app.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/clients.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/clients_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/config/config.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/config/config_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/errors.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/handler.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/handler_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/httpx.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/jwt.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/logger.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/middleware.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/model.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/model_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/repository.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/repository_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/service.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/service_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/internal/serviceauth.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/enrollment/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/security-and-authentication/cmd/keygen/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/cmd/migrate/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/database/postgres.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/go.mod` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/security-and-authentication/go.sum` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/security-and-authentication/internal/app.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/config/config.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/config/config_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/errors.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/handler.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/httpx.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/internal_caller_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/internal_student_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/logger.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/middleware.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/model.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/notifications.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/passwords.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/passwords_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/repository.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/service.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/service_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/serviceauth.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/serviceauth_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/test_helpers_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/tokens.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/tokens_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/internal/userclient.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/security-and-authentication/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| MODIFIED | `services/user/cmd/bootstrap-admin/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/cmd/migrate/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/database/postgres.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/go.mod` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/user/go.sum` | Go 1.27.3 / patched dependency versions and verified checksums. |
| MODIFIED | `services/user/internal/app.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/authclient.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/config/config.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/config/config_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/errors.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/handler.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/handler_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/httpx.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/jwt.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/jwt_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/logger.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/middleware.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/model.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/outbox.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/outbox_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/repository.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/repository_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/service.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/service_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/serviceauth.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/internal/serviceauth_test.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| MODIFIED | `services/user/main.go` | Go formatting and LF normalization only; semantic whitespace comparison passed. |
| ADDED | `.dockerignore` | Consistent line endings, build-context hygiene, sensitive-file exclusion or narrow scanner configuration. |
| ADDED | `.github/workflows/ci.yml` | CI, immutable GHCR publication or controlled deployment preparation. |
| ADDED | `.github/workflows/prepare-deployment.yml` | CI, immutable GHCR publication or controlled deployment preparation. |
| ADDED | `.github/workflows/publish-images.yml` | CI, immutable GHCR publication or controlled deployment preparation. |
| ADDED | `.gitleaks.toml` | Consistent line endings, build-context hygiene, sensitive-file exclusion or narrow scanner configuration. |
| ADDED | `docs/infrastructure/deployment-guide.md` | Architecture, operating instructions and actual validation/status evidence. |
| ADDED | `docs/infrastructure/phase-3-report.md` | Architecture, operating instructions and actual validation/status evidence. |
| ADDED | `docs/infrastructure/phase-4-report.md` | Architecture, operating instructions and actual validation/status evidence. |
| ADDED | `docs/infrastructure/phase-5-report.md` | Architecture, operating instructions and actual validation/status evidence. |
| ADDED | `docs/infrastructure/validation-ledger.md` | Architecture, operating instructions and actual validation/status evidence. |
| ADDED | `infrastructure/ansible/Controller.Dockerfile` | Validated image build, patched runtime/toolchain or trusted controller build. |
| ADDED | `infrastructure/ansible/ansible.cfg` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/dev/group_vars/all.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/dev/hosts.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/dev/vault.yml.example` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/prodlike/group_vars/all.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/prodlike/hosts.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/prodlike/vault.yml.example` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/qa/group_vars/all.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/qa/hosts.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/inventories/qa/vault.yml.example` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/invoke.ps1` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/playbooks/deploy.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/playbooks/health-check.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/playbooks/rollback.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/playbooks/validate.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/requirements.txt` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/requirements.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/roles/application/tasks/main.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/roles/health-check/tasks/main.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/roles/nginx/tasks/main.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/roles/preflight/tasks/main.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/ansible/roles/secrets/tasks/main.yml` | Explicit local inventory, safe deployment role/playbook, Vault template or controller wrapper. |
| ADDED | `infrastructure/docker/compose.prod.yaml` | Shared Compose fixes or small isolated hardened environment override. |
| ADDED | `infrastructure/docker/compose.qa.yaml` | Shared Compose fixes or small isolated hardened environment override. |
| ADDED | `infrastructure/environments/prod/.env.example` | Safe independent local rehearsal configuration example. |
| ADDED | `infrastructure/environments/qa/.env.example` | Safe independent local rehearsal configuration example. |
| ADDED | `infrastructure/scripts/backup.py` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/ci_models.py` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/platformctl.py` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/protect-local-secrets.ps1` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/requirements.txt` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/scan_images.py` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/scan_secrets.py` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `infrastructure/scripts/validate_vault.py` | Validated environment lifecycle, security scan, secrets, backup or model tooling. |
| ADDED | `tests/infrastructure/test_backup.py` | Deployment/backup safety and integrity regression checks. |
| ADDED | `tests/infrastructure/test_platform.py` | Deployment/backup safety and integrity regression checks. |


## 2026-11-24 shutdown recovery and fresh revalidation

The shutdown recovery found the application remediation, new reports and raw
2026-11-23 evidence already present as uncommitted work. All were preserved.
The earlier summary claimed phase/ledger addenda had been appended, but inspection
found them absent; this dated addendum completes that unfinished documentation.
Earlier failure statements above remain historical, not the current verdict.

Fresh evidence root: `.infra-state/remediation/recovery-20261124T020340Z`. Exact process arguments and exit codes are in
`results.json`; sanitized initial resource identities/file hashes are in
`initial.json`; final baseline preservation is in `final-safety.json`.

| Phase | Current acceptance | Remaining condition |
| --- | --- | --- |
| 1 | **COMPLETED AND VERIFIED** | Accepts isolated target/rehearsal only; legacy cutovers remain unauthorized. |
| 2 | **IMPLEMENTED — OWNER APPROVAL PENDING** | Owner must explicitly approve the exact-digest, guard-mandatory local-only Silo exception, or select a separately evaluated vendor option. |
| 3 | **IMPLEMENTED — REMOTE VERIFICATION PENDING** | GitHub-hosted execution has not been authorized or triggered. |

| Test area | Executed command | Expected | Actual | Status | Evidence |
| --- | --- | --- | --- | --- | --- |
| postgres-start | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py start` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-start.log` |
| auth-targeted | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/rehearsal_tests.py --only auth --test-run ^TestServicePermissionsAndAudit$ --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\auth-targeted` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/auth-targeted.log` |
| backend-tests | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/rehearsal_tests.py --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\backend-tests` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/backend-tests.log` |
| postgres-privileges | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py verify` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-privileges.log` |
| postgres-restore | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/revalidate_postgres.py --snapshot %LOCALAPPDATA%/UCM\backups\qa-rehearsal\20261123T165903Z-8c294f57 --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\postgres-restore` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-restore.log` |
| postgres-persistence | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py recreate --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\postgres-persistence` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-persistence.log` |
| storage-start | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py start` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-start.log` |
| storage-test | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py test` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-test.log` |
| storage-stop | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py stop` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-stop.log` |
| runtime | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/validate_runtime.py --tag remediation-20261123 --evidence-dir .infra-state\remediation\recovery-20261124T020340Z\runtime` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/runtime.log` |
| infrastructure-tests | `.infra-venv\Scripts\python.exe -B -m unittest discover -s tests/infrastructure -v` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/infrastructure-tests.log` |
| compose-models | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/ci_models.py` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/compose-models.log` |
| actionlint | `docker run --rm --network none --mount type=bind,source=C:\Users\user\OneDrive\Desktop\ESD\project\University-Course-Management-System,target=/repo,readonly --workdir /repo rhysd/actionlint:1.7.12 -color .github/workflows/ci.yml .github/workflows/publish-images.yml .github/workflows/prepare-deployment.yml` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/actionlint.log` |
| secret-scan | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/scan_secrets.py` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/secret-scan.log` |
| diff-check | `git diff --check` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/diff-check.log` |
| storage-final-stop | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/storage_rehearsal.py stop` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/storage-final-stop.log` |
| postgres-final-stop | `.infra-venv\Scripts\python.exe -B infrastructure/scripts/postgres_rehearsal.py stop` | Exit 0 | Exit 0 | PASSED | `.infra-state/remediation/recovery-20261124T020340Z/postgres-final-stop.log` |

Fresh Trivy 0.76.2 scans on 2026-11-24 retained strict HIGH/CRITICAL gates:
PostgreSQL candidate 0 HIGH / 0 CRITICAL, legacy PostgreSQL 26 HIGH / 1 CRITICAL,
and Silo 1 HIGH / 0 CRITICAL (CVE-2026-39414). Image IDs matched exact inspected
digests. Evidence: `.infra-state/remediation/security/recovery-ghcr-20261124/summary.json`
and three raw JSON reports. The first attempt downloaded the updated vulnerability
database but exceeded its scan deadline before image inspection completed; its
failure log remains in `security/recovery-20261124/postgres-candidate.log`.
The successful retry used `.infra-state/recovery_scan.py --output-dir
.infra-state/remediation/security/recovery-ghcr-20261124`, which delegates to the
existing scanner with `TRIVY_DB_REPOSITORY=ghcr.io/aquasec/trivy-db:2`.
No vulnerability suppression, severity change or false clean result was used.

Final safety: all 66 legacy container IDs/start timestamps are unchanged and
stopped. All 23 original volumes and the additional baseline rehearsal/cache
volumes remain present (27 baseline names checked). Only authorized isolated
rehearsals/tool containers ran; all rehearsal runtimes were stopped afterward.
No commit, tag, push, GHCR publication, remote workflow trigger, legacy volume
deletion/recreation, legacy database write, DEV/QA/PROD-like cutover or Phase 4
work occurred. No secret was disclosed. Existing application/migration/Compose
changes were preserved. No application, SQL migration, Compose or policy file
was further changed during recovery; nine-image builds/scans from 2026-11-23
remain historical and were not newly repeated. Runtime execution was repeated.

Evidence-retention limitation from the earlier session remains disclosed: its
first Auth rerun overwrote `.infra-state/phase123/tests/auth.log`. The old failure
survives in reports/transcript; that raw log cannot be claimed byte-preserved.
This recovery did not overwrite prior validation evidence. Phase 0 is unchanged.


## 2026-11-24 owner approval — local storage exception

Approval recorded at 2026-11-24 02:22 UTC from the project owner's explicit
conversation instruction: "Approve the local-only exception with the guard required".
This is owner approval, not an exception self-approved by Codex.

Approved image:
`pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329`.

The exception accepts the residual HIGH CVE-2026-39414 scanner finding for
isolated local rehearsal use only, with the tested Select guard mandatory.
Silo's backend and console must remain bound to loopback, with no network-facing
unguarded alternate endpoint. Existing IAM restrictions remain required. The
image is not declared scanner-clean; raw findings and strict scan exits remain
unchanged. A different digest or removal/bypass of the guard is outside this
approval and requires a new security assessment and owner decision.

Acceptance relies on the already executed 2026-11-24 validation: all six backend
suites and named database-backed Authentication test passed; PostgreSQL candidate
scan, privileges, restores and persistence passed; normal S3/multipart/presigned/
metadata/IAM and object recovery tests passed; 54 Select probes were denied and
backend/console bypass ports were unreachable; full runtime, ten Compose models,
24 infrastructure tests, actionlint, secret scans and whitespace checks passed.
Evidence: `.infra-state/remediation/recovery-20261124T020340Z/` and
`.infra-state/remediation/security/recovery-ghcr-20261124/summary.json`.
No runtime test or scan was repeated merely to record this documentation approval.

| Phase | Current acceptance after owner approval |
| --- | --- |
| Phase 1 | **COMPLETED AND VERIFIED** — isolated target/rehearsal scope |
| Phase 2 | **COMPLETED AND VERIFIED** — isolated local scope with this owner-approved, guard-mandatory storage exception |
| Phase 3 | **IMPLEMENTED — REMOTE VERIFICATION PENDING** |

The earlier owner-pending statements are retained as historical evidence and
superseded by this dated approval for the scope above. This approval does not
authorize DEV, existing QA or PROD-like cutover, legacy component replacement,
cloud/production use, commits, tags, pushes, GHCR publishing, remote workflow
execution or Phase 4. Phase 3 remote verification still needs separate
authorization; Phase 4 remains deferred. Legacy storage remains unchanged.

This approval-recording operation changes documentation only. Previous report
bytes, application/configuration changes and validation evidence are preserved.
No Docker resource or database was started or modified.

## 2026-11-27 Phase 4 protected-main continuation

The historical Phase 1–3 entries above remain unchanged. PR #15 merged at
`92fce6b02c15def11c9194137eca5387bfefa6dd`; the repository is now public
and active ruleset 28252583 protects `~DEFAULT_BRANCH` against deletion and
non-fast-forward updates. Fresh hosted
[CI run 46047552300](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46047552300)
reported `GITHUB_REF_PROTECTED=true`. Six backend, frontend, infrastructure,
nine application image and PostgreSQL candidate jobs passed. The publication
prerequisite failed on the old registry-access policy and `publish` skipped;
this is remote Phase 3 validation, not GHCR publication evidence.

Full-history and non-ignored working-tree secret scans passed (25 commits,
418 files). Tracked-path history inspection found no runtime `.env`, private
key or database dump. GitHub secret scanning is disabled; the current CLI
package API returns 403 for missing `read:packages`. The new registry bootstrap
and safe-rerun tests passed locally (39 infrastructure tests total). The
separate PGSTY Silo HIGH CVE-2026-39414 still needs an explicit Phase 4
publication-scope owner decision; the 2026-11-24 approval above remains
local-only. No environment deployment, database operation or volume mutation
was performed in this continuation.
