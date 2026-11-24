# PostgreSQL supporting security revalidation

Date: 2026-11-23. Result: **COMPLETED AND VERIFIED** for the isolated PostgreSQL
candidate and Phase 1 compatibility requirements. No legacy upgrade or cutover.

## Images and fresh raw scans

Scanner: Trivy 0.76.2, digest
`sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb`.
New scan batch started 2026-11-23T17:33:18Z. Vulnerability database update checks
were enabled; HIGH/CRITICAL exit gates remained strict.

| Image | Fresh raw result | Disposition |
| --- | --- | --- |
| `ucm-postgres@sha256:cb90d02d2e03c00889b7d8c08e1bf522ef62891b121c02d7b0a4239170661a4c` | Exit0; 0 HIGH / 0 CRITICAL | Candidate remains clean at the scanned severities/time |
| `postgres:18-alpine@sha256:0115267ab5a509022df9319c77be365cef335adb71556c75f60f5360ee247e71` | Exit1; 26 HIGH / 1 CRITICAL | Legacy finding remains unresolved in stopped legacy deployments; not suppressed |

The existing `infrastructure/postgres/Dockerfile` was inspected and preserved:
same PostgreSQL 18.7 server/base, libxml2 package floor `2.13.9-r3`, same upstream
gosu 1.20 source rebuilt with Go 1.27.3 and checksum-verified source. No major
version, PGDATA format or extension upgrade was performed. The legacy critical
finding remains CVE-2025-68121 in the old gosu Go standard library; libxml2 and
other old stdlib findings remain in the raw report. Inclusion/severity alone is
not a claim of reachable exploitation, and no risk exception was self-approved.

Actual server SQL returned **18.7**. All six databases contain only the standard
`plpgsql` extension version 1.0. No CREATE EXTENSION or external UUID-extension
requirement was found in application migrations. Compatibility is established
for these inspected schemas/services, not arbitrary future extensions.

## Database and identity revalidation

Only retained project `platform-qa-pg-rehearsal-20261123` was used. Its database
container and named volume identities were checked before start. The six names
remain `authentication_and_security`, `users`, `courses`, `enrollments`,
`assessments`, `analytics`. New-cluster admin is `platform_admin`; each service
uses its own `<service>_migrator` owner and limited `<service>_app` runtime login.

`postgres_rehearsal.verify()` passed real SQL checks for all six databases:
database/schema/object ownership; runtime read/DML and sequences; no elevated
role flags; no runtime CREATE DATABASE/ROLE/TABLE/SCHEMA, migration-history writes
or migrator SET ROLE; no cross-service DB connection for runtime/migrator;
migrator own-schema DDL succeeds but cluster administration is denied. These are
positive/negative SQL tests, not assertions inferred only from YAML.

All six backend suites passed with fresh isolated migration fixtures, including
the remediated Authentication catalogue and audit tests. Retained business DB
fingerprints were unchanged after fixture cleanup. Migration versions remain
Auth6, User4, Course1, Enrollment1, Assessment3, Analytics1, all clean.

All six migrators were then executed **twice** using the new local application
images and service-specific identities. Both repetitions succeeded; data
fingerprints matched before/after. SQL separately reconfirmed 23 Auth permissions
and seeded default role counts ADMIN19, LECTURER8, STAFF4, STUDENT2. No old SQL
migration was edited and no forward data migration was necessary.

## Restore and persistence drill

Executed:

```powershell
.infra-venv/Scripts/python.exe -B infrastructure/scripts/revalidate_postgres.py --snapshot "$env:LOCALAPPDATA/UCM/backups/qa-rehearsal/20261123T165903Z-8c294f57" --evidence-dir .infra-state/remediation/postgres-restore
.infra-venv/Scripts/python.exe -B infrastructure/scripts/postgres_rehearsal.py recreate --evidence-dir .infra-state/remediation/postgres-persistence
```

The source eight-file manifest (six custom dumps, password-free globals SQL and
recovery metadata) was checksum-verified. Every archive TOC confirmed PostgreSQL
18 source/tool compatibility. Each was restored into a **new disposable**
`remediation_<service>_restore` database on the new rehearsal cluster, owned and
restored by its own migrator, using `--no-owner --no-acl --exit-on-error`.

Each restored database matched the backup's per-table counts/content hashes,
had the correct object owner and clean migration state, and denied runtime DDL
and migration-history writes. Only databases created by that invocation were
dropped afterward. The six retained business databases were not reset or
restored over. Global role SQL was retained as recovery metadata; it was not
blindly replayed over existing roles. Secret recovery remains separate.

The retained PostgreSQL container was recreated against the **same rehearsal
volume**. All six business database fingerprints survived; SQL privilege checks
passed again. New exclusively created restore/persistence evidence is under
`.infra-state/remediation`, leaving the old restore/persistence records intact.

Evidence: `security/supporting/{postgres-candidate.json,postgres-legacy.json,summary.json}`,
`postgres-restore/restore.json`, `postgres-persistence/persistence-evidence.json`
and `postgres-final.json`. The latter records server/extensions, role counts and
the two successful no-change migration repeats.

## Cutover restrictions remain

All 66 legacy containers stayed stopped; no legacy users, passwords, grants,
data, images or volumes were changed. Their bootstrap privileges and vulnerable
images are not fixed by a successful new-cluster rehearsal. DEV/existing QA/
PROD-like cutover still requires separate authorization, live source inventory,
current verified environment backups, write fencing, reconciliation and rollback
planning. No physical legacy PGDATA volume may be attached to this target.
Full environment disaster recovery, production HA and cloud deployment are not
claimed by this local SQL recovery exercise.


## 2026-11-24 shutdown recovery and fresh revalidation

The exact PostgreSQL candidate remains
`ucm-postgres@sha256:cb90d02d2e03c00889b7d8c08e1bf522ef62891b121c02d7b0a4239170661a4c`.
Fresh SQL returned server 18.7. All six role/ownership, runtime DML, denied DDL,
administration, migration-history mutation and cross-database checks passed.
All six verified backup archives were restored into new disposable databases;
content/ownership checks passed and only those fixtures were removed. Recreating
the isolated PostgreSQL container reused the retained rehearsal volume and
preserved all six data fingerprints; privilege checks passed again. Fresh fixture
migrations passed for all six services. The earlier extension inventory and two
no-change migration repetitions remain 2026-11-23 evidence, not newly repeated
claims. No legacy database was started, upgraded or modified.

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
