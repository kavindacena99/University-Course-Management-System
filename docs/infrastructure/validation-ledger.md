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
