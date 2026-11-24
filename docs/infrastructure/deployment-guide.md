# Local deployment and delivery guide

## Current safety gate and PostgreSQL target selection

The [new implementation integration report](phase-1-2-3-integration-report.md)
and [updated ledger](updated-infrastructure-validation-ledger.md) describe the
current state. All legacy environments are stopped and preserved. Commands in
the older sections below describe the **legacy** deployment interface; they do
not deploy the new consolidated target. Do not run them as a cutover shortcut.
Publishing and deployment-preparation GitHub workflows are currently held;
their historical instructions below are deferred until the release phase.

The new rehearsal uses only project `platform-qa-pg-rehearsal-20261123` and its
new data volumes. Existing generated inputs must not be overwritten. From the
repository root, inspect before any operation:

```powershell
git status --short
docker context show
docker ps -a
docker volume ls
docker network ls
```

For a genuinely new rehearsal only: build the ten local images described in the
Phase 2 report, then use `postgres-rehearsal.ps1 -Action init` once. The init
command refuses an existing input directory. The executed rehearsal inputs and
volumes already exist, so **do not reinitialize this machine's rehearsal**.
Supported guarded commands are:

```powershell
# Syntax/model only; no container start.
powershell -NoProfile -File infrastructure/docker/postgres-rehearsal.ps1 -Action config
# These act ONLY on the new rehearsal, when explicitly needed:
powershell -NoProfile -File infrastructure/docker/postgres-rehearsal.ps1 -Action start
powershell -NoProfile -File infrastructure/docker/postgres-rehearsal.ps1 -Action verify
powershell -NoProfile -File infrastructure/docker/postgres-rehearsal.ps1 -Action backup
powershell -NoProfile -File infrastructure/docker/postgres-rehearsal.ps1 -Action stop
```

`migrate` runs six service-specific migration jobs and finalizes business-table
grants. `restore -Snapshot <verified-QA-backup-directory>` is allowed only on an
empty new target after checksum/TOC/source/version checks; it refuses nonempty
schemas. Never retry restore by deleting data. `recreate` verifies content hashes
around replacement of only the new database container. `select-image -Image
ucm-postgres:phase123` restricts selection to a local PostgreSQL 18 image and pins
its local digest after compatibility checks. A PostgreSQL-only backup does not
replace object-storage, secret escrow or full recovery procedures.

Inactive target preparation is separate:

```powershell
# Run once per genuinely new target; refuses existing inputs; does not deploy.
.infra-venv/Scripts/python.exe -B infrastructure/scripts/prepare_postgres_target.py --environment dev
.infra-venv/Scripts/python.exe -B infrastructure/scripts/prepare_postgres_target.py --environment qa
.infra-venv/Scripts/python.exe -B infrastructure/scripts/prepare_postgres_target.py --environment prodlike
# Example configuration inspection ONLY, never implicit cutover:
docker compose --env-file .infra-state/phase123/targets/qa/rehearsal.env -p platform-qa-target -f infrastructure/docker/compose.target.yaml config --quiet
```

All three target inputs already exist locally following this work. They contain
independent credentials, signing issuers/keys, cookie names, ports, resource names
and pinned local images. They are ignored and ACL-restricted. Do not commit or
copy them across environments. Administrative and migration secrets are distinct
from runtime DB passwords. Applications still receive passwords through env;
Docker file secrets are local mounts, not encrypted external secret storage.

Before any future cutover, obtain explicit authorization and satisfy the Phase 1
report's backup, write-fence, Auth permission-catalog, migration, role, data
reconciliation and rollback gates. Existing Ansible remains a legacy interface;
it has not silently been redirected. Do not use target Compose files with a
legacy project name or a legacy volume. No Phase 4 release/promotion begins here.

This repository has DEV, QA and a local **PROD-like rehearsal**. None is a live
cloud Production deployment. Existing DEV data must not be reset to troubleshoot
deployment. Use `stop`; never use volume deletion or Docker prune shortcuts.

## Prerequisites and first setup

Use Docker Desktop's Linux engine, Compose >=2.24.4 and Python with
`infrastructure/scripts/requirements.txt`. Go 1.27.3 and Node 22/pnpm 10.34.3
are needed for host builds; container builds use their existing Dockerfiles.

```powershell
docker version
docker info
docker compose version
python -m venv .infra-venv
.infra-venv/Scripts/python.exe -m pip install -r infrastructure/scripts/requirements.txt
```

DEV keeps its existing `infrastructure/docker/.env`, six service `.env` files,
Auth key mount, project `platform-dev`, data volumes and ports 3000/8080. For a
new developer only, follow the Phase 1/2 setup for keys and matching service
configuration. Do not copy templates over existing files.

Generate independent credentials for new local rehearsal environments:

```powershell
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py init --environment qa
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py init --environment prodlike
```

On first setup, build the shared candidate image set once through QA with
`platformctl.py deploy --environment qa --mode build`, validate it, then stop
QA and deploy PROD-like in `existing` mode to reuse those images. Templates use
`:candidate` local tags, separate from DEV's existing `:local` tags. Local tags
are mutable build conveniences; select digest manifests for auditable registry
promotion. This task's isolated validation used separately named `:phase345`
image tags and verified the actual running image IDs.

Initialization refuses existing or partial inputs. If it fails midway, inspect
and recover the newly created local files; do not delete/recreate data volumes.
Runtime inputs are ignored under `infrastructure/environments/{qa,prod}`.
These are locally generated secrets, not production credentials. Move actual
live secrets to a reviewed external secret store when a provider is selected.

## Start, check and stop one target

```powershell
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py config --environment dev
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py deploy --environment dev
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py health --environment dev

.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py deploy --environment qa
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py health --environment qa
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py stop --environment qa

.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py deploy --environment prodlike
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py health --environment prodlike
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py stop --environment prodlike
```

Default `existing` mode uses already built images. `--mode build` builds locally;
`--mode pull --release <manifest>` pulls a reviewed immutable release. A promoted
release cannot be rebuilt through this helper. The helper checks existing Docker
resources, configuration and credential boundaries before deploying. On this
8 GiB Desktop engine it refuses to start QA while PROD-like runs or vice versa;
DEV is left running. Stop only the rehearsal you started. Its seven named volumes
remain available for the next deployment.

| Target | Frontend | Gateway health |
| --- | --- | --- |
| DEV | http://localhost:3000 | http://localhost:8080/health |
| QA | http://localhost:3001 | http://localhost:8081/health |
| PROD-like | http://localhost:3002 | http://localhost:8082/health |

QA/PROD-like edge bindings are loopback-only. Backend, database and object ports
are not published. HTTP is a local rehearsal exception: secure cookies and TLS
must be enabled together before live deployment. PROD-like Auth disables the
unimplemented external notification provider; this does not prove activation or
password-reset delivery works.

Equivalent direct commands require the correct environment file:

```powershell
docker compose --env-file infrastructure/environments/qa/.env -p platform-qa `
  -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.qa.yaml up -d --wait
docker compose --env-file infrastructure/environments/prod/.env -p platform-prodlike `
  -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.prod.yaml up -d --wait
docker compose --env-file infrastructure/docker/.env -p platform-dev `
  -f infrastructure/docker/compose.yaml -f infrastructure/docker/compose.dev.yaml ps -a
```

Use the same arguments followed by `logs --tail 100 gateway`, `exec -T gateway
nginx -t` or `stop`. Do not publish rendered `compose config` output: it contains
resolved secrets. Prefer helper `config` or Compose `config --quiet`.

## Ansible: native Linux or Windows fallback

With functioning Linux/WSL Docker access:

```bash
python3 -m venv .infra-venv
.infra-venv/bin/pip install -r infrastructure/ansible/requirements.txt
source .infra-venv/bin/activate
cd infrastructure/ansible
export ANSIBLE_CONFIG="$PWD/ansible.cfg"
ansible-playbook -i inventories/dev/hosts.yml playbooks/validate.yml
ansible-playbook -i inventories/qa/hosts.yml playbooks/deploy.yml
ansible-playbook -i inventories/prodlike/hosts.yml playbooks/deploy.yml
ansible-playbook -i inventories/qa/hosts.yml playbooks/health-check.yml
```

Ubuntu's Docker socket is currently unavailable on this PC. No native WSL
runtime success is claimed. Review Desktop **Resources → WSL Integration →
Ubuntu** and apply changes only when a Desktop restart is safe for DEV.

The tested Windows alternative uses a disposable Ansible control image:

```powershell
docker build -t ucm-ansible-control:local -f infrastructure/ansible/Controller.Dockerfile .
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment dev -Playbook validate
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment qa -Playbook deploy
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment prodlike -Playbook deploy
```

This control container mounts the local Docker socket and has host-equivalent
Docker access. Use only trusted checkout/image contents. No internet-facing
runner or new daemon is installed. The wrapper preserves Desktop's Linux host
path for Compose mounts. Run repeated deployments using the same execution
method to avoid recreations caused solely by Windows-versus-Linux path strings.

## CI, publication and immutable release selection

PR CI checks all six Go modules, frontend, models, Ansible syntax, image builds,
secret history and image vulnerabilities. Frontend lint/unit scripts are absent
and explicitly reported. Main/version publishing requires the same CI first,
uses `GITHUB_TOKEN` and outputs `immutable-release/release.env` with nine pinned
image digests. That artifact is promoted unchanged between QA and PROD-like.
No cloud/PC deployment runs on GitHub-hosted runners.

After the owner enables and authorizes real publication, download a successful
publication artifact and run the optional protected preparation workflow. Store
the selected non-secret manifest under ignored `.infra-state/releases/`.

```powershell
gh run download <successful-publish-run-id> --name immutable-release --dir .infra-state/releases/selected
.infra-venv/Scripts/python.exe infrastructure/scripts/platformctl.py deploy --environment qa `
  --mode pull --release .infra-state/releases/selected/release.env
powershell -NoProfile -File infrastructure/ansible/invoke.ps1 -Environment qa -Playbook deploy `
  -DeploymentMode pull -ReleaseManifest .infra-state/releases/selected/release.env
```

For native Ansible use `-e deployment_mode=pull -e release_manifest=/absolute/path/release.env`.
Authentication for private GHCR pulls belongs in the operator's Docker credential
store, outside Git. The control-container wrapper does not automatically forward
host registry credentials; use native Linux Ansible with its credential store or
a reviewed ephemeral registry-login mechanism before private pulls. Local image
reuse is the runtime-tested mode; private registry promotion is not claimed tested.

Successful registry deployment records current/previous manifests. After checking
schema compatibility, `playbooks/rollback.yml` or helper `rollback --environment
qa` can redeploy the previous image set. With no previous immutable release,
rollback refuses rather than guessing tags. Database migrations are not reversed
and backups must be available before incompatible schema changes.

## Operating limits

Nginx discovers replicas through Docker DNS. A service/container replacement or
complete outage may surface transient 502/504 responses while DNS and passive
failure handling converge. Health scripts prove readiness and routing, not
authenticated business workflows or zero-error failover. Keep replicas at one
for services with process-local replay/rate state or unreviewed outbox concurrency.

Neither local database passwords nor an existing scoped object user are rotated
by editing `.env` alone. Preserve keys/credentials needed for current data;
coordinate explicit database/object-user rotation and application restart.
Backing up data does not make rollback across incompatible schema versions safe.

## Configuration and secret management

Normal configuration includes URLs, ports, logging, issuer/audience, caller
identities, cookie names and image references. Passwords, shared internal
authentication secrets, Auth private keys and bootstrap tokens are secret.
Never print resolved Compose configuration or place secrets in image arguments,
frontend Vite variables, GitHub workflow inputs or command-line passwords.

Each rehearsal has its own credentials and JWT issuer/audience. Cookie names
are also prefixed by environment: browser cookies are scoped by host/path,
not TCP port. This prevents localhost QA/PROD-like from overwriting DEV cookies.
The frontend currently has no API client; a future client must read the correct
environment CSRF cookie through a public runtime configuration mechanism.

Local environment files are acceptable for these rehearsals, but Docker
administrators can inspect container environment values. Only Auth's existing
private-key loader uses a mounted Compose secret file. Compose file secrets are
read-only host binds, not an encrypted secret service. Other Go secrets still
use environment variables; no new `_FILE` support is claimed.

On Windows, restrict the local files with:

```powershell
powershell -NoProfile -File infrastructure/scripts/protect-local-secrets.ps1 -Environment qa
powershell -NoProfile -File infrastructure/scripts/protect-local-secrets.ps1 -Environment prodlike
```

The same script supports DEV for a controlled credentials-permission update.
It does not rotate values or change application containers. It uses direct file
ACEs and checks current-user access. ACLs do not prevent a privileged host
administrator or a sync program running as the current user from reading data.
This checkout is under OneDrive: never use it to hold live production secrets.
Use external protected secret paths/Vault before live deployment; local
rehearsal values are not production secrets.

Native Ansible Vault setup, with passwords outside the repository:

```bash
cd infrastructure/ansible
# Start from the documented vault.yml.example; do not deploy placeholders.
ansible-vault create --vault-id qa@prompt inventories/qa/vault.yml
ansible-vault edit --vault-id qa@prompt inventories/qa/vault.yml
ansible-vault create --vault-id prodlike@prompt inventories/prodlike/vault.yml
ansible-playbook -i inventories/qa/hosts.yml playbooks/deploy.yml --vault-id qa@prompt
# Alternatively use a protected external password file, chmod 600 on Linux.
ansible-playbook -i inventories/qa/hosts.yml playbooks/deploy.yml --vault-password-file /protected/outside-repo/qa-vault-password
```

For the Windows wrapper pass `-VaultPasswordFile <external path>`; it refuses
password files inside the checkout and mounts the file read-only for that run.
The secrets role refuses plaintext `vault.yml`, loads encrypted variables with
`no_log`, and permits only `.env`/`runtime/*` outputs within the selected rehearsal.
Actual encrypted files remain ignored by default. Separate Vault IDs/passwords
per environment; do not store the passwords beside encrypted content. Vault
encryption does not protect rendered environment values while applications run.

Synthetic Vault encryption/decryption and encrypted-variable role validation can
be repeated by running `infrastructure/scripts/validate_vault.py` inside the
trusted control container, mounted as in `invoke.ps1`. No real secret data is
needed, and temporary password/plaintext test material stays inside the
disposable container.

## Backups, retention and recovery

```powershell
.infra-venv/Scripts/python.exe infrastructure/scripts/backup.py create --environment qa
.infra-venv/Scripts/python.exe infrastructure/scripts/backup.py verify --path <snapshot-directory>
# Synthetic write/readback is restricted to QA/PROD-like, never DEV.
.infra-venv/Scripts/python.exe infrastructure/scripts/backup.py create --environment qa --validate-object
```

Run backup commands on the host, where their output is persistent. Defaults are
`%LOCALAPPDATA%/UCM/backups/<environment>/<UTC-unique-id>` on Windows and
`$XDG_STATE_HOME/ucm/backups` (or `~/.local/state/ucm/backups`) on Linux. An explicit
`--output-root` must be a protected persistent host directory outside cloud-sync
and Git. Do not use an ephemeral control-container filesystem for backup output.

The script takes six PostgreSQL custom-format logical dumps, checks each with
`pg_restore --list`, mirrors the existing private submissions bucket using only
the scoped Assessment object credentials, and records byte counts, SHA-256
checksums, image IDs, configuration copies and non-secret recovery settings.
UUID/timestamp paths prevent overwriting earlier snapshots. The optional
synthetic object has a unique test key; only that newly created source key is
removed after verification. No database restoration is executed.

Passwords and signing keys are intentionally excluded: back them up separately
through the environment Vault/password-management policy. Without that material,
the data snapshot is not a complete secret-recovery package. Dumps and objects
can themselves contain sensitive data and remain plaintext locally; encrypt
before off-device storage. Checksums detect corruption, not malicious replacement
by someone who can rewrite both the backup and manifest.

Each DB has its own consistent MVCC snapshot, not one distributed transaction
across all databases and objects. Quiesce writes in an approved maintenance
window when a cross-service recovery point is required. The script does not stop
DEV to manufacture consistency. Suggested retention for an approved scheduled
backup policy is 7 daily / 4 weekly / 3 monthly copies, with capacity monitoring
and encrypted off-device copies; retention deletion/scheduling is not active.
No backup purge operation is included.

Restore only into a verified empty, isolated recovery target after operator
approval. First run `backup.py verify`, inspect `pg_restore --list`, provision
matching database roles/version, then use `pg_restore --exit-on-error --no-owner`
against the empty target. Restore objects with the scoped client to the intended
private bucket, recover Vault/keys separately, validate migrations/readiness and
authenticated workflows, and only then consider cutover. Do not point restore
commands at DEV or add destructive `--clean` flags. No destructive restore drill
was executed in this task.

## Future host, TLS and cloud prerequisites

Do not enable SSH or change the personal PC firewall for this pipeline. Future
Linux servers need a dedicated deployment user, verified key-based access,
host-key verification, a tested recovery path before disabling root/password
login, restricted inbound traffic, security updates, and no public backend/DB
ports. Real remote inventories and server credentials are deliberately absent.

Terminate HTTPS at the selected cloud ingress or gateway using valid certificates
and a reviewed renewal mechanism. Protect private keys, validate configuration
before reload, enable redirects and secure Auth cookies together, and enable
HSTS only after HTTPS/domain behavior is verified. Trust forwarded client/proto
headers only from actual load-balancer CIDRs. Local HTTP is not production TLS.
No fake production certificate, private key or untested TLS listener was added.

Azure integration should use Key Vault and managed identity; AWS should use
Secrets Manager and IAM roles. Grant each service/environment only its required
secret versions, avoid static cloud keys, audit access and implement rotation
with application restarts/reloads as appropriate. Neither provider is selected
or configured here. GitHub environment reviewers, main/tag protection and GHCR
package visibility/access also require owner action; the current private-repo
plan returned 403 for branch protection and no environments were configured.

## Validated handoff state

QA and PROD-like were deployed sequentially through the Windows Ansible
controller, each with 15 healthy runtimes, seven successful jobs and 12 HTTP
probes. Repeated deployments returned `changed=0`. Both rehearsal stacks were
then stopped with all volumes retained. DEV's original 15 containers remain
healthy. Nine patched application/helper images passed the HIGH/CRITICAL gate;
supporting-image and database-role limitations remain in the Phase 5 report.
The [validation ledger](validation-ledger.md) contains the exact acceptance
commands, outcomes, failures/retries and all 187 changed/added files.
