# Phase 3 — local environments and Ansible

The common Compose architecture now supports explicitly selected DEV, QA and
PROD-like projects. PROD-like is a local production rehearsal, not live
Production. No cloud, SSH service, firewall, Git commit, data reset or volume
deletion was performed.

## Verified starting state

The working tree was clean. Contrary to the supplied running-state description,
Docker had no application containers or project networks at the start. Seven
existing `platform-dev_*` data volumes remained. DEV was restored with its
existing ignored environment files and retained volumes before implementation;
it reached 15 healthy runtimes and seven successful setup jobs. No existing
credentials were regenerated. Docker Desktop is Linux/amd64, engine 29.9.3,
8 CPUs and about 7.63 GiB available to the engine.

## Environment boundaries

| Target | Project | Frontend / gateway | Configuration inputs |
| --- | --- | --- | --- |
| DEV | `platform-dev` | 3000 / 8080, existing bindings retained | Existing `infrastructure/docker/.env` and six service `.env` files |
| QA | `platform-qa` | 127.0.0.1:3001 / 8081 | `environments/qa/.env`, independent `runtime/*.env` and signing key |
| PROD-like | `platform-prodlike` | 127.0.0.1:3002 / 8082 | `environments/prod/.env`, independent `runtime/*.env` and signing key |

Paths above are under `infrastructure` unless qualified. The six internal
ports remain 8081–8086. Each project owns three networks, six PostgreSQL volumes
and one object volume, without external/shared resource names. Each has its own
containers, logs, database credentials, object credentials, JWT signing key,
issuer/audience and internal service secret. Identical database names are safe
inside separate database containers; real data is not copied between projects.
Separate generated frontend CSP and global gateway configurations use the same
audited sources/snippets/upstreams. Backend DNS resolves only within each
project's networks. Only the two edge ports are published.

`compose.yaml` remains the common architecture. Small `compose.qa.yaml` and
`compose.prod.yaml` overrides select application mode, private key file secret,
edge configuration and loopback ports. `!override` replaces port lists and
`!reset` removes DEV's key-directory mount; Compose >=2.24.4 is required.
[Docker's merge rules](https://docs.docker.com/reference/compose-file/merge/)
explain why simply adding another port or secret mount would not isolate it.

`platformctl.py init` generates only local QA/PROD-like credentials from OS
cryptographic randomness. It refuses DEV and refuses to overwrite any existing
or partially initialized environment. Templates contain placeholders, not usable
keys. `config` refuses resource sharing, incorrect project/port assignments,
exported credential/issuer mismatches and password reuse across environments.
Eleven safety tests cover target identity, overwrite prevention, resource/port
isolation and release-manifest injection/mutable-tag rejection.

## Automation

```text
infrastructure/ansible/
  ansible.cfg / requirements.yml / requirements.txt / Controller.Dockerfile
  invoke.ps1
  inventories/{dev,qa,prodlike}/
    hosts.yml / group_vars/all.yml / vault.yml.example
  playbooks/{deploy,validate,health-check,rollback}.yml
  roles/{preflight,application,nginx,secrets,health-check}/tasks/main.yml
```

All inventories use local connection and explicit environment identity. Roles
use `ansible.builtin`, so no external collection is required. Preflight checks
the inventory/directory, Linux engine and Compose version. Secrets validates
the selected inputs and optionally renders Vault-encrypted files with `no_log`.
Application validates, builds/pulls or reuses images, starts with readiness
waiting, handles stale application image IDs, and records whether container
identities changed. Nginx validates both edge server configurations; health
checks assert 15 healthy runtimes, seven completed jobs and 12 HTTP checks.
Repeated deployment does not recreate unchanged containers or remove volumes.
Migrations/init jobs may execute again, using their existing idempotent behavior.

The helper supports `existing`, `build` and `pull` modes. Registry deployment
requires an immutable nine-image manifest; environment/config injection through
that manifest is rejected. Successful registry releases record current/previous
references under ignored `.infra-state/<environment>`. Rollback refuses without
a previous immutable release and never reverses a schema or deletes data.
Operators must review schema compatibility before rolling images back.

## WSL and Windows execution

Ubuntu 24.04 uses Python 3.12.3 but has no pip/Ansible installed. Its normal
Docker endpoint has no socket. An inspected existing Desktop proxy socket also
rejected access. No permissions, symlinks, daemon, GUI settings or competing
Docker Engine were changed.

The supported alternative here is a disposable Linux Ansible control container
built from the official Docker CLI image and ansible-core 2.20.11. It uses the
existing Desktop engine through the mounted Docker socket; this grants
host-equivalent Docker access and is intended only for this trusted local
checkout. It is not an application service or internet-exposed runner.
`invoke.ps1` mounts the checkout at its corresponding Desktop Linux host path,
so Compose resolves host bind sources correctly. Do not alternate Windows and
Linux Compose paths during idempotency tests: differing mount strings can
cause legitimate container recreation.

For native WSL later, review Docker Desktop **Settings → Resources → WSL
Integration → Ubuntu**, enable the intended integration and apply/restart when
DEV downtime is acceptable; then verify `docker version` and `docker compose
version` in Ubuntu. These instructions follow
[Docker's WSL documentation](https://docs.docker.com/desktop/features/wsl/use-wsl/).
No GUI setting is claimed to have been changed.

## Validation and operating commands

Executed from the repository root:

```powershell
python -m venv .infra-venv
.infra-venv/Scripts/python.exe -m pip install -r infrastructure/scripts/requirements.txt
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py init --environment qa
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py init --environment prodlike
# config --environment dev, qa and prodlike all passed.
docker build -t ucm-ansible-control:local -f infrastructure/ansible/Controller.Dockerfile .
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment qa -Playbook validate
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment qa -Playbook deploy
# Repeat the same deploy command to verify idempotency.
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py stop --environment qa
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment prodlike -Playbook deploy
# Repeat, then stop only PROD-like; named volumes remain.
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py stop --environment prodlike
.infra-venv/Scripts/python.exe -m unittest discover -s tests/infrastructure -v
```

All three inventory graphs and all four playbooks for each inventory were
syntax-checked in the control image. QA validation reported `ok=10 changed=0
failed=0`; its deployment reported `ok=13 changed=1 failed=0`, and repeat
deployment reported `ok=13 changed=0 failed=0`. PROD-like first deployment
reported `ok=13 changed=1 failed=0`. The final security-updated QA and PROD-like
deployments each again reported `ok=13 changed=1 failed=0`; each repeat reported
`ok=13 changed=0 failed=0`. Both passed 15 healthy runtimes, seven successful
setup jobs, 12 HTTP checks and both host edge ports. See the
[validation ledger](validation-ledger.md) for the commands, final cleanup evidence
and complete changed-file list.

The initial QA migration failed with a TCP connection refusal while the fresh
PostgreSQL cluster was switching from its bootstrap socket server to its final
listener. Existing Unix-socket `pg_isready` had marked it healthy prematurely.
The shared health checks now explicitly use `-h 127.0.0.1`; QA retry passed and
fresh PROD-like deployment passed. No migration SQL/schema was changed, no DB
reset was used, and DEV's running containers were not recreated for this fix.

## Status

- **COMPLETED AND VERIFIED:** models, local credentials/key generation, explicit
  isolation guards, eleven safety tests, inventory/playbook syntax, control-node
  execution, QA and PROD-like deployment/idempotency, independent networks and
  all 21 retained data volumes.
- **BLOCKED BY ENVIRONMENT:** native Ubuntu Docker socket/integration.
- **IMPLEMENTED BUT NOT RUNTIME-VERIFIED:** GHCR pull/promotion and rollback to
  a previous real immutable release; no registry publication exists from this task.
- **REQUIRES MANUAL USER ACTION:** native WSL integration if desired; ongoing
  credential/Vault-password/backup custody as described in the security report.
- **DEFERRED UNTIL CLOUD SERVERS ARE AVAILABLE:** remote inventories, SSH access,
  remote host baselines and live Production deployment.
