#!/bin/sh
set -eu

mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"

attempt=0
until mc ready local; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
        echo "MinIO did not become ready before the timeout" >&2
        exit 1
    fi
    sleep 2
done

mc mb --ignore-existing local/submissions
if ! mc admin policy info local assessment-submissions; then
    mc admin policy create local assessment-submissions /app/minio-policy.json
fi
if ! mc admin user info local "$ASSESSMENT_OBJECT_USER"; then
    mc admin user add local "$ASSESSMENT_OBJECT_USER" "$ASSESSMENT_OBJECT_PASSWORD"
fi
mc admin policy attach local assessment-submissions --user "$ASSESSMENT_OBJECT_USER"
