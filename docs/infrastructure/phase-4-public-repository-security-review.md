# Phase 4 public repository security review

Date: 2026-11-27. Repository:
`kavindacena99/University-Course-Management-System` (public), baseline
`main` commit `92fce6b02c15def11c9194137eca5387bfefa6dd`.

## Scope and results

The existing `infrastructure/scripts/scan_secrets.py` ran against all 25
reachable commits and the tracked/non-ignored working tree (418 files on the
latest scan). Its
Gitleaks history and working-tree scans both reported **no leaks**. The fresh
GitHub-hosted run
[46047552300](https://github.com/kavindacena99/University-Course-Management-System/actions/runs/46047552300)
also passed its full-history secret-scan step. No credential value was copied
into this report.

Tracked-path inspection found `.env.example` templates for the three local
environments and six services, but no tracked runtime `.env`, key, database
dump or backup file. The templates use illustrative placeholders. Application
images use final runtime stages; their Docker build contexts exclude `.env`
files through the existing `.dockerignore` rules. The publication workflow
passes `GITHUB_TOKEN` through standard input to temporary runner credentials;
no source file or image build argument receives it. The frontend's publication
build argument is empty and source currently makes no direct microservice
request; the Nginx gateway remains the documented API boundary.

The read-only GitHub secret-scanning alerts API returned HTTP 404 because
GitHub secret scanning is disabled for this repository. The local and CI
Gitleaks results are the available automated history checks; they do not prove
that every piece of public project material is non-sensitive. The repository
contains architecture, infrastructure and security reports intended for
project review. No real user dataset or private connection string was found in
the tracked-path and scanner review.

## Remaining custody boundaries

Ignored local runtime credentials, JWT signing keys, Docker credentials and
backups remain on the workstation outside tracked Git content. Their continued
exclusion depends on the ignore rules and operator custody. This review did not
print, rotate, upload or deploy them. A newly discovered live secret would
require immediate revocation/rotation and a separate incident review; this
scan found none requiring such action.
