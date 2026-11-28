#!/bin/sh
set -eu
# This short-lived job holds only the six migrator credentials, never bootstrap.
cp /run/secrets/pg_migration_passwords /tmp/pgpass
chmod 600 /tmp/pgpass
export PGPASSFILE=/tmp/pgpass
for pair in auth:authentication_and_security user:users course:courses enrollment:enrollments assessment:assessments analytics:analytics; do
    service=${pair%%:*}
    database=${pair#*:}
    psql -X -q -v ON_ERROR_STOP=1 -v "runtime=${service}_app" -h postgres \
        -U "${service}_migrator" -d "$database" -f /provision/permissions.sql
done
