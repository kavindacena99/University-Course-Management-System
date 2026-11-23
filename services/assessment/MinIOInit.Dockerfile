FROM pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329 AS mc

FROM alpine:3.22

COPY --from=mc /usr/bin/mc /usr/bin/mc
COPY minio-init.sh /usr/local/bin/minio-init.sh
COPY minio-policy.json /app/minio-policy.json

ENTRYPOINT ["/bin/sh", "/usr/local/bin/minio-init.sh"]
