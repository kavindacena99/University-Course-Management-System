# Phase 0 executive decision summary

Baseline: `main` / `a2df85f`, 2026-11-23. This is the fifth requested audit
document; Section 4 supplied four explicit file paths, so this separate summary
uses the stated default path. The same decision summary is included at the end
of the main report. No infrastructure implementation was performed.

## Verdict

**READY WITH CONDITIONS** to implement Phase 1's isolated QA database rehearsal.
**Not ready** for a DEV data cutover or approved RC-to-final release today.

## What is ready

- Six real Go/Gin services and their existing API/DB boundaries are understood.
- One shared Compose stack plus DEV/QA/PROD-like overlays, unique projects/ports,
  private services, nine networks and 21 project data volumes already exist.
- All three current Compose models passed Phase 0 configuration/isolation checks.
- Eleven synthetic infrastructure tests and read-only Go formatting checks
  passed in Phase 0. Existing QA snapshot's 12 file checksums also verified.
- Multi-stage runtime images exclude development source/docs/tests. Nginx's
  host/Docker upstream split and six-service gateway architecture should remain.
- Manual PowerShell-to-Ansible deployment interface, image manifest validation,
  local Vault integration and backups are implemented, with important limits.

## What requires changes

1. **Database topology:** eighteen existing clusters/eighteen DB volumes must
   become three clusters/three new DB volumes, retaining six logical databases
   per environment and the three existing object volumes. Use logical restore
   and a staged migration; never merge/reuse physical PGDATA directories.
2. **Privileges:** bootstrap identities still serve applications and migrations.
   New target admin/migrator/runtime separation and negative cross-DB access
   checks are prerequisites to consolidation cutover.
3. **Security evidence:** retained same-image PostgreSQL JSON reports 26 HIGH and
   one CRITICAL, correcting the historical 27-HIGH prose. The CRITICAL is in
   bundled gosu's Go stdlib. This is cached evidence, not a fresh scan or proven
   reachable exploit. Object-store upstream finding has vendor backport evidence
   but needs formal disposition. Do not blindly jump PostgreSQL major versions.
4. **CI:** all three observed GitHub runs failed infrastructure Compose validation.
   Main-run six backend jobs, frontend and nine image build/scans succeeded;
   publisher and release-manifest jobs were skipped. Root Compose diagnostic is
   suppressed, so its specific underlying cause is not yet established.
5. **Releases:** final version tags currently rebuild. No durable RC/QA approval
   record/source binding or no-rebuild final promotion exists. Exactly reuse
   approved QA image digests; build caching does not satisfy this rule.
6. **QA:** authenticated integration/authorization/DB/E2E release acceptance is
   incomplete. CI omits real repository integration DB fixtures. Keep full QA
   validation separate from PROD-like non-destructive smoke.
7. **Frontend:** no API calls are implemented today. Before using the Vite build
   API variable, agree a runtime-origin mechanism compatible with one immutable
   image across environments and the gateway-only backend access rule.

## Manual configuration and external limits

- Zero GitHub environments are configured; main protection API returns a private
  repository plan restriction. Owner must decide protection/required checks/
  reviewers, not assume YAML alone enforces approval.
- GHCR package enumeration requires read:packages authorization; actual package
  visibility/access is unknown. No successful publication is evidenced by the
  inspected runs. Do not refresh credentials or publish during the audit.
- All 66 project containers were **already stopped** when inspected. The engine
  is available, but live DB catalog/route/health validation is blocked by the
  explicit no-start audit rule. Their stop cause was not investigated as a
  failure claim; exit codes were 0.
- Native WSL Docker socket remains unavailable. Existing Windows controller is
  a usable historical fallback, not a reason to install another Docker daemon.
- The PC has 8 CPUs/~7.63 GiB and a common host-failure domain. Keep rehearsals
  sequential until combined workload/connection capacity is measured.
- Real Vault enrollment, protected password/key custody, compatible image
  remediation, a restoration drill and DEV maintenance approval remain needed.
- Cloud hosts/provider/domain/identities are absent; live TLS, cloud secrets,
  remote hardening and Production HA/DR are not active local capabilities.

## Safest implementation order

1. Obtain a later authorized live read-only DB/role/ownership/migration inventory,
   verified backup and recovery/maintenance agreement. Triage supporting images.
2. Stage a new QA PostgreSQL volume/cluster with six databases, separated roles,
   logical restore, privilege/ownership checks and connection/resource budget.
3. Adapt existing helpers, backups, health checks and CI to the target topology;
   prove recovery and repeat deployment in QA. Preserve existing DEV resources.
4. Rehearse sequentially on PROD-like with non-destructive smoke only.
5. Cut DEV over last under the agreed write-fence/rollback policy; retain old data.
6. Implement immutable candidate approval/final promotion and complete QA
   integration gates before relying on the release pipeline.

**Exact first implementation task:** implement the staged QA-only PostgreSQL
initializer and explicit backup/migration preflight on a new project-scoped
volume, with six logical DBs and restricted runtime/migrator identities, while
ordinary DEV deployment remains fail-closed until an approved data cutover.
Do not start this automatically from the audit.

## Deliverables and safety

- [Main audit and evidence/command/test inventory](phase-0-audit-report.md)
- [Capability gap matrix](phase-0-gap-analysis.md)
- [File-level Phase 1 implementation plan](phase-1-implementation-plan.md)
- [Risk and dependency register](phase-0-risk-register.md)
- This executive summary.

Only these five Markdown files are added. No commits, tags, pushes, GHCR
publication, workflow triggers, container creation/start/stop, volume mutation,
DB data/role changes, application/infrastructure/configuration edits or branch
switches are part of this phase. Historical reports remain unchanged.
