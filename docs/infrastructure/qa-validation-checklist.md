# QA validation checklist

Record each result as PASS, FAIL, or NOT RUN in the machine-readable QA report.
NOT RUN is not a pass.

| Area | Required evidence |
| --- | --- |
| Source and images | RC tag, source SHA, Phase 4 run, nine registry and running image digests |
| Target identity | Project, 3001/8081 ports, one PostgreSQL, private networks, two separate named data volumes |
| Database | Six logical DBs, six restricted migrators, six runtime roles, migration versions, no cross-DB access |
| Backup | Predeployment external protected snapshot and checksum if target data existed |
| Storage | Loopback Silo, guard on 9000, no alternate listener, normal scoped upload/read/delete, S3 Select denied |
| Services | Six readiness endpoints, DB connections, gateway route/status behavior, service-to-service authentication |
| Business API | Auth, user, course, enrollment, assessment/file and analytics flows; validation and 401/403/404/500 handling |
| Browser | Load/static assets, login/logout, cookies/CSRF, courses, enrollment, assessment/file, analytics, gateway-only requests |
| Security | Secret-free frontend assets/logs, private backend ports, scanner records including raw Silo HIGH |
| Stability | Safe restart/persistence, bounded smoke response times/errors and Docker resource usage |
| Final state | QA stopped cleanly, target volumes preserved, DEV identities unchanged |
