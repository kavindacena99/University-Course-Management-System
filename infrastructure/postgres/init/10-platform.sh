#!/bin/sh
# First initialization only. No reconciliation or credential reset on restart.
set -eu
test -s /run/secrets/pg_bootstrap_sql
psql -X -q -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
    --file /run/secrets/pg_bootstrap_sql
# A partial initialization must never become healthy on a subsequent restart.
touch "$PGDATA/.platform-initialized"
