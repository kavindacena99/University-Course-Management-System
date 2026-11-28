# Phase 5 — local security, secrets and recovery

This is a local security baseline and a documented cloud handoff, not a claim of
live Production security. No production secret manager, real TLS certificate,
remote host, SSH service, firewall change, Git commit or data reset was introduced.

## Secret classification and isolation

Normal inputs include ports, service URLs, log level, image references, JWT
issuer/audience, caller IDs and cookie names. Secret inputs include DB passwords,
the internal HMAC secret, object-store credentials,
Auth private key and User bootstrap token. There is no verified SMTP/payment
integration requiring invented secrets.

QA and PROD-like receive independently generated local credentials and signing
keys. Their issuers, audiences, CORS origins and cookie names are distinct.
Cookies are host/path scoped, not port scoped; environment prefixes prevent
localhost rehearsals overwriting DEV cookies. All six services use the matching
environment prefix. Future frontend integration must use the correct CSRF cookie.

Each Go application receives its own DB password and the existing shared internal
service credential needed by its signed HTTP integration. Object credentials go
only to Assessment; Auth signing material goes only to Auth. Migration jobs now
receive only DB settings: they do not need JWT, HMAC or bootstrap secrets.
The object initialization job retains the admin/scoped credentials it needs for
bucket/user setup. Private backend, database and object ports are not published.

The existing Auth file-based private-key loader is used through a Compose file
secret in rehearsals. This is a read-only host-file mount, not an encrypted
secret service. File-source secrets do not guarantee UID/mode remapping; the
individual key file is readable by the existing non-root UID while its host
parent/ACL is restricted. Other Go secret inputs remain environment-based and
are readable by a Docker administrator. Local protected env files are explicitly
permitted for this phase; no false `_FILE` support or Swarm encryption is claimed.
A new cross-module secret-loader abstraction was not needed for these local
targets; future provider integration can render protected inputs or add reviewed
configuration-layer support without rewriting business code.

## Vault and host-file permissions

Ansible inventories have safe `vault.yml.example` templates. Actual `vault.yml`
and vault password files are ignored. The secrets role requires an encrypted
Vault envelope, loads with `no_log`, rejects cross-environment/path-traversal
outputs and renders only the selected rehearsal's `.env`/`runtime/*` paths.
DEV input replacement through Vault is refused to preserve existing user inputs.
Separate Vault IDs/passwords belong outside the checkout and outside rendered
files; the Windows wrapper rejects in-repo Vault password files.

`validate_vault.py` used temporary random test passwords and synthetic variables
inside the disposable control container. Encryption produced a Vault envelope;
decryption matched the original; mode 0600 was checked; the encrypted-variable
validation playbook executed successfully without printing the synthetic value.
No actual secret set or fake production password was committed or activated.
The activation commands are in the deployment guide. Actual Vault enrollment
and password custody require the owner's chosen secret-handling procedure.

New rehearsal files have current-user/SYSTEM NTFS ACLs. Validation caught a
recursive-directory-ACE mistake that temporarily denied access to rehearsal env
files; the script was corrected to assign direct file ACEs, re-applied and
verified by successful Compose reads and backup execution. DEV files/containers
were unaffected by that failure. Protected backup files were also checked to
exclude ordinary-user access; privileged local administrators/owner rights can
still read them, as expected for host administration.

The checkout resides under OneDrive. Ignore rules and ACLs do not stop a sync
program running as the current user. Rehearsal values are local-only, not live
production secrets; live material must move to protected external paths/Vault
and eventually provider-managed delivery. Backups default outside this synced
checkout rather than treating `.gitignore` as encryption.

## Git and secret scanning

Ignore rules cover env inputs, runtime/secret directories, Vault/password files,
private/TLS key formats, dumps, backup paths, logs, caches and temporary tooling
state. Safe examples remain included. The legitimate Ansible `roles/secrets`
source directory is explicitly included, rather than accidentally hidden by a
generic `secrets/` ignore rule. Actual ignore-policy checks confirmed both secret
exclusion and source/example inclusion. SQL migrations are not broadly ignored.

Gitleaks scanned all 20 existing commits and the non-ignored working tree with
redacted output. The first pass reported one non-secret Phase 1/2 route-status
prose fragment; the default generic-key heuristic had interpreted test-result
words as a key. Its reviewed exception applies only to that exact fragment in
that exact report file, with all default rules retained. Corrected history and
working-tree rescans passed. No actual credential exposure was identified by
these scans; this does not prove no secret could ever have existed elsewhere.
No history rewrite or credential value was printed.

The final scan covered 366 tracked/non-ignored files, including the
[validation ledger and full changed-file inventory](validation-ledger.md).

## Container, database and network controls

QA/PROD-like Go applications run as the existing non-root user with all
capabilities dropped, read-only root filesystems, bounded no-exec scratch tmpfs,
`no-new-privileges`, PID and memory limits. The existing official Nginx root
master/worker model retains only CHOWN/SETUID/SETGID, plus NET_BIND_SERVICE for
the frontend's port 80. Cache/run/PID scratch paths are writable tmpfs; routing,
headers, Docker DNS zones, rate limits and default 404 behavior are preserved.
Global gateway PID moves to `/tmp` only in generated rehearsal config, keeping
host-run and existing DEV files intact.

Database bootstrap privileges and writable data directories remain where
required; raw-network capability is removed. The object store retains its
existing root-owned volume behavior with capabilities removed rather than
blindly changing existing data ownership. Logs, restarts, readiness and stop
grace remain bounded. Local memory/PID limits were runtime-tested for startup
and health, not certified under production load.

The audit confirmed that database application identities currently use the
PostgreSQL bootstrap superuser. An attempted QA-only restriction was rejected by
PostgreSQL 18 because its bootstrap role must retain SUPERUSER. That experiment
was withdrawn, and the temporary QA admin role was removed. No schema or data
was changed. DEV roles were read only. Separating bootstrap/migration/runtime
roles requires a reviewed privilege and ownership migration; it remains an
explicit security blocker before live Production. Network separation does not
remove this privilege risk.

All 21 application-data volumes and nine project networks are independent and
retained. The deployment helper rejects external/shared/host-aliased persistent
storage. Actual tests confirmed DEV cannot directly reach a QA backend IP
across project bridges and the QA frontend cannot resolve private Auth DNS.
Normal in-project gateway routes and frontend-to-gateway connectivity passed.
Outbound access is retained intentionally; `internal: true` was not applied
blindly. The Docker control socket remains a host-trust boundary.

## Image security work and remaining vendor findings

The first strict HIGH/CRITICAL scan failed for all nine application/helper images:
64 HIGH occurrences, zero CRITICAL, including duplicate occurrences across
service/migration binaries. The policy was not weakened or configured to ignore
unfixed issues.

The Nginx runtime libxml2 finding was repaired using the vendor's fixed package
floor while keeping Nginx 1.30.7 pinned. All six service modules/builders receive
Go 1.27.3, `x/net` 0.62.0, its required `x/crypto` 0.58.0 / `x/text` 0.43.0 and
compatible transitive dependencies. Assessment's compression dependency moves
to 1.18.7. Original requested minimum combinations were rejected by Go module
version constraints, then corrected using x/net's actual requirements. All six
Go suites/vet/module checks passed after the security updates. Business Go source
changes are formatting only; no API/schema rewrite was made.

The final rebuilt nine application/helper images each passed Trivy with **zero
HIGH and zero CRITICAL** findings using the downloaded current database. The
initial download timeout was retried; cached-database rescans used
`scan_images.py --skip-db-update`. Supporting-image findings below are separate
and remain visible.

The old image-bundled storage client used Go 1.27.1. Rather than invent a custom
fork build, the initialization helper uses the actual vendor November release,
pinned by its published and pulled digest. Rehearsal servers use the same vendor
security release. Its upgrade checklist was inspected; rehearsals are stopped
single-server test stores, with verified backups and no concurrent/batch jobs
created by this task. DEV storage is intentionally not upgraded in place.

Supporting-image scans are recorded even though they are not the nine built
application artifacts. The existing PostgreSQL image reported 27 HIGH matches:
one libxml2 finding and 26 stdlib matches in its `gosu` bootstrap helper. These
are retained for vendor remediation/reachability review; no blanket scanner
suppression or unsafe replacement of six database images was introduced.

The newer object-store server reports one upstream module match,
`CVE-2026-39414`. The vendor documents complete fixes in June and the pulled
image revision `44e848f...` is 805 commits ahead of the complete-fix commit with
zero behind. The image's OCI revision matches the inspected release source.
This is evidence of a maintained-fork backport, not permission to hide arbitrary
findings. The raw report remains for owner/VEX review. See the
[vendor's advisory](https://silo.pgsty.com/blog/security/cve-2026-39414/) and
[upgrade notes](https://silo.pgsty.com/blog/release/silo-20261109/#upgrade).
No claim of zero raw vulnerabilities in every supporting image is made.

Existing DEV is still on its retained older artifacts/privileges. A controlled
DEV update needs backup, storage checklist review and a maintenance window;
security remediation was built/tested in isolated rehearsals instead of silently
restarting or migrating existing DEV data.

## Backup and recovery

`backup.py` takes six PostgreSQL custom-format logical backups, checks each
archive with `pg_restore --list`, mirrors private submissions using only the
scoped object identity, and records checksums, byte counts, image IDs and
non-secret recovery settings. UUID/UTC directories never overwrite earlier
snapshots. Windows defaults to LocalAppData, Linux to persistent state storage;
output is outside Git/sync by default, with restricted permissions.

QA validation recovered a unique synthetic object byte-for-byte into the backup,
then removed only that newly created source object. Twelve files passed manifest
integrity verification. The initial test snapshot was moved out of the synced
workspace and verified again; a second snapshot exercised the external default
location and non-overwrite behavior. Six real archive listings were checked;
no destructive database restoration was executed. Eleven deployment/backup safety
tests passed, including damaged-archive-manifest and path-traversal rejection.

Snapshots are independently consistent per DB, not an atomic distributed backup
of all DBs and objects. Keys/passwords require separate Vault recovery. Dumps
are sensitive plaintext locally and need approved encryption before off-device
storage. Checksums protect against corruption, not a malicious backup owner.
Retention proposals, restore steps, required empty-target verification and
maintenance-window requirements are in the deployment guide. No production
schedule, purge, cloud copy or full disaster-recovery drill is claimed active.

## SSH, TLS and future cloud integration

No Windows firewall or SSH-service change was made. Future servers need a
dedicated deploy identity, verified key access and recovery before disabling
password/root login, strict SSH host-key checks, limited public ingress, security
updates and private backends. No fake IP, SSH key or remote inventory is provided.

Local HTTP continues. Live HTTPS needs a real certificate/renewal design at the
gateway or cloud ingress, protected private keys, validated reloads, reviewed
redirects/HSTS and secure cookie settings. Only actual load-balancer CIDRs may
be trusted for forwarded IP/proto restoration. No fake production certificate
or unexecuted TLS configuration test is represented as working.

Local rehearsal initialization explicitly sets `AUTH_COOKIE_SECURE=false` for
HTTP, including PROD-like. This is a local exception, not a live Production
setting; enabling actual HTTPS and secure cookies must be coordinated.

Azure's future path is Key Vault plus managed identity; AWS's is Secrets Manager
plus IAM roles. Use environment/service-scoped access, auditable secret versions,
rotation/revocation procedures and workload identity instead of static cloud
keys. No provider is selected and neither integration is active.

## Acceptance classification

- **COMPLETED AND VERIFIED:** local secret generation/isolation, ignore policy,
  redacted scans, encrypted synthetic Vault execution, corrected NTFS ACLs,
  hardened rehearsal runtime/network checks, safe backups and integrity tests.
- **IMPLEMENTED BUT NOT RUNTIME-VERIFIED:** real environment Vault enrollment,
  cloud secret delivery, live HTTPS, private GHCR promotion/rollback and full
  authenticated business/restore workflows.
- **BLOCKED BY ENVIRONMENT:** native Ubuntu Docker socket and current private
  repository protection-plan restrictions; control-container execution works.
- **REQUIRES MANUAL USER ACTION:** live-secret/password custody, GitHub/package
  policy, supporting vendor finding review and controlled existing DEV upgrade.
- **DEFERRED UNTIL CLOUD SERVERS ARE AVAILABLE:** remote host baseline, workload
  identities, certificate renewal/ingress, encrypted off-site scheduled backups
  and live Production recovery/capacity certification.
