FROM pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329 AS mc

FROM alpine:3.22@sha256:49340b346798875b5592e5d420241f2f9cfba5f47e62ebb1446a1b34734e898f

RUN apk add --no-cache ca-certificates 'libcrypto3>=3.5.9-r2' 'libssl3>=3.5.9-r2' \
    && addgroup -S app && adduser -S -G app app

COPY --from=mc /usr/bin/mc /usr/bin/mc
COPY minio-init.sh /usr/local/bin/minio-init.sh
COPY minio-policy.json /app/minio-policy.json

ENV MC_CONFIG_DIR=/tmp/mc
USER app
ENTRYPOINT ["/bin/sh", "/usr/local/bin/minio-init.sh"]
