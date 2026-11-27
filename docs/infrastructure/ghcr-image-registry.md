# GHCR application image registry

Date: 2026-11-24. Repository: `kavindacena99/University-Course-Management-System`
(private). Publication is currently blocked by prerequisites. The names below
are implemented destinations, not claims that packages exist.

## Inventory and names

Registry prefix: `ghcr.io/kavindacena99/university-course-management-system/`.
The existing historical publisher used this same namespace. Names are lowercase.
All nine images target `linux/amd64`, matching the local Linux Docker deployment.
The final unnamed Dockerfile stage is the runtime; no development stage is exported.

| Service | Dockerfile | Build context | Local image | GHCR image |
| --- | --- | --- | --- | --- |
| Authentication | `services/security-and-authentication/Dockerfile` | `services/security-and-authentication` | `ucm-auth` | `ghcr.io/kavindacena99/university-course-management-system/auth` |
| User | `services/user/Dockerfile` | `services/user` | `ucm-user` | `ghcr.io/kavindacena99/university-course-management-system/user` |
| Course | `services/course/Dockerfile` | `services/course` | `ucm-course` | `ghcr.io/kavindacena99/university-course-management-system/course` |
| Enrollment | `services/enrollment/Dockerfile` | `services/enrollment` | `ucm-enrollment` | `ghcr.io/kavindacena99/university-course-management-system/enrollment` |
| Assessment | `services/assessment/Dockerfile` | `services/assessment` | `ucm-assessment` | `ghcr.io/kavindacena99/university-course-management-system/assessment` |
| Analytics | `services/analytics/Dockerfile` | `services/analytics` | `ucm-analytics` | `ghcr.io/kavindacena99/university-course-management-system/analytics` |
| Gateway | `infrastructure/nginx/Dockerfile` | `infrastructure/nginx` | `ucm-gateway` | `ghcr.io/kavindacena99/university-course-management-system/gateway` |
| Frontend | `frontend/Dockerfile` | `frontend` | `ucm-frontend` | `ghcr.io/kavindacena99/university-course-management-system/frontend` |
| Storage setup | `services/assessment/MinIOInit.Dockerfile` | `services/assessment` | `ucm-assessment-minio-init` | `ghcr.io/kavindacena99/university-course-management-system/assessment-minio-init` |

The setup helper is independently deployed, so it is a ninth application-owned
image. Six migration jobs reuse their service images and dedicated migration
binaries; they are not six additional images. The storage Select guard reuses
the gateway image with a separate configuration; it is not a tenth image.

Third-party/supporting images are excluded from publication: PostgreSQL vendor
image, derived `ucm-postgres` candidate, and PGSTY Silo. The derived database
image remains supporting infrastructure despite having a repository Dockerfile.
It stays in the existing CI build/scan matrix without becoming a GHCR package.
Go/Node/Alpine builders, Trivy, Gitleaks, actionlint, BuildKit/SBOM generator and
the Ansible controller are build/test/control tools, not application packages.

## Build and runtime configuration

Only frontend has a public build argument: publication builds explicitly set
`VITE_API_BASE_URL=`. Local Compose's existing development argument remains
unchanged. Current frontend source contains no implemented backend API calls;
this phase does not implement that integration. Future browser calls must use
the API Gateway. No per-environment image rebuild is introduced for promotion.

Authentication uses runtime DB, JWT signing-key, session/notification and internal
service configuration; its signing key remains a runtime secret mount. The other
five Go services use their existing runtime DB, public JWT verification and signed
internal-service configuration. Assessment additionally needs runtime S3 endpoint,
bucket and scoped credentials. The setup helper receives object-store root and
application provisioning credentials at runtime only. Gateway uses existing
Docker DNS/upstream routing and runtime Nginx configuration; frontend serves
static assets with runtime Nginx configuration. None of these credentials is a
build argument. Service ports remain 8081-8086, gateway 8080, frontend 80; setup
publishes no listener. Seven Go/helper runtimes use `app`; the existing Nginx
entrypoint/master privilege convention is retained rather than falsely called
fully non-root. `.dockerignore` exclusions and final-image content checks passed.

## Immutable identity and consumption

The only publication tag is `sha-<full-40-character-git-sha>`. No latest, branch,
environment, semantic version or release-candidate tags are generated. Existing
SHA tags cause publication to fail rather than overwrite them. GHCR tags are
not a registry-enforced write-once mechanism; an external privileged actor could
still change one. The authoritative reference is the actual returned digest.

`image-manifest.json` records the complete verified publication. `images.env`
contains exactly the nine existing `*_IMAGE=image@sha256:...` variables, compatible
with `platformctl.validate_release()` and the existing Compose interpolation.
Neither file is generated as a successful publication artifact without actual
upload, registry resolution, pull and post-upload scan results. Existing DEV,
QA, PROD-like and inactive target inputs/volumes are unchanged; no deployment
command is run. Local development can continue using existing source builds.

## Private package access

Keep packages private by default, consistent with the private repository. No
visibility changes were performed. Workflow publication uses ephemeral
`GITHUB_TOKEN` with `packages: write` only in the publish job; it does not use a PAT.
For existing packages, the owner must verify the repository is linked and has
write access under **Package settings → Manage Actions access**, and verify any
organization policy permits package creation. A 403 is a stop condition.

The existing CLI token has repository/workflow access but package enumeration
returned HTTP 403: `You need at least read:packages scope to list packages`.
This does not establish the future workflow token's permissions. The owner can
inspect package settings through GitHub or authorize read-only package inspection;
do not replace publication authentication with a personal token to bypass access.

For later private pulls on the local PC, provision a separately approved reader
identity with package read access, following GitHub's supported authentication
model. GitHub documents a classic token with `read:packages` for external CLI
pulls; this is a deployment reader, not a publishing-token workaround. Use the
Docker credential store and standard input, never Compose or committed files:

```powershell
$env:GHCR_READ_TOKEN | docker login ghcr.io --username <approved-reader> --password-stdin
docker pull --platform linux/amd64 <image@sha256:actual-digest-from-manifest>
```

These are instructions only; no credential was created or login/pull performed
against GHCR. Anonymous pulls require public visibility, which is not assumed.

References: [GitHub package permissions](https://docs.github.com/en/packages/learn-github-packages/about-permissions-for-github-packages),
[package access configuration](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility).

## 2026-11-27 status

The nine names and `linux/amd64` inventory above remain the application release
set. Six migration jobs reuse their service images; the storage setup job needs
`assessment-minio-init`. PostgreSQL and PGSTY Silo are supporting images and
remain outside GHCR application publication. `VITE_API_BASE_URL` is explicitly
empty in the publication build; runtime frontend policy permits the gateway
origin and no application code currently makes a direct service call. Runtime
DB, JWT and S3 secrets remain external to image build arguments.

GitHub-hosted run 46038430324 built all ten matrix entries. The PostgreSQL
support image scan passed. The nine application image scans failed: seven
Alpine-based Go/helper runtimes reported HIGH CVE-2026-124709 in `libcrypto3`
and `libssl3` 3.5.9-r0; frontend and gateway reported six HIGH libxml2 findings
on 2.13.9-r3. Scanner records identify fixed floors of OpenSSL 3.5.9-r2 and
libxml2 2.13.9-r5. The Dockerfiles now request those floors; all nine then built
and passed local strict scans with zero HIGH/CRITICAL findings. A clean hosted
scan of a reviewed commit is still required before publication. The raw
reports are retained in run artifacts and ignored local evidence.

No package enumeration or private package access claim is possible with the
current CLI token: the package API returns 403 for missing `read:packages`.
There are **no verified published references or registry digests** for this
Phase 4 attempt. The `sha-<full source SHA>` tags and digest-based `images.env`
remain implemented contracts, not evidence of publication.
