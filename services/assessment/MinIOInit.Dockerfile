FROM minio/mc:latest AS mc

FROM alpine:3.22

COPY --from=mc /usr/bin/mc /usr/bin/mc
COPY minio-init.sh /usr/local/bin/minio-init.sh
COPY minio-policy.json /app/minio-policy.json

ENTRYPOINT ["/bin/sh", "/usr/local/bin/minio-init.sh"]
