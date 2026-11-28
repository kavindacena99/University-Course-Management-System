"""Restore verified rehearsal SQL backups into NEW disposable DBs, never reset data."""
import argparse
import hashlib
import json
from pathlib import Path

import backup
import postgres_rehearsal as pg


def fingerprint(service, database):
    tables = pg.sql("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;", service, database=database).splitlines()
    query = ' UNION ALL '.join(f"SELECT '{t}',count(*)::text,coalesce(md5(string_agg(row_to_json(t)::text,'' ORDER BY row_to_json(t)::text)),'empty') FROM public.\"{t}\" t" for t in tables)
    return pg.sql(query, service, database=database)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--snapshot', type=Path, required=True)
    p.add_argument('--evidence-dir', type=Path, required=True)
    a = p.parse_args()
    pg.guard()
    a.evidence_dir.mkdir(parents=True, exist_ok=False)
    backup.verify(a.snapshot)
    manifest = json.loads((a.snapshot / 'manifest.json').read_text())
    if manifest.get('project') != pg.PROJECT or manifest.get('environment') != 'qa-rehearsal':
        raise RuntimeError('Only a verified backup of the retained NEW rehearsal is allowed')
    recovery = json.loads((a.snapshot / 'recovery-metadata.json').read_text())
    before = pg.data_fingerprint()
    fixtures = {s: 'remediation_' + s + '_restore' for s in pg.DATABASES}
    for db in fixtures.values():
        if pg.sql(f"SELECT count(*) FROM pg_database WHERE datname='{db}';") != '0':
            raise RuntimeError('Existing restore fixture rejected')
    created, results = [], {}
    try:
        for service, fixture in fixtures.items():
            pg.sql(f'CREATE DATABASE {fixture} OWNER {service}_migrator; REVOKE ALL ON DATABASE {fixture} FROM PUBLIC; GRANT CONNECT ON DATABASE {fixture} TO {service}_app;')
            created.append(fixture)
            pg.sql(f'ALTER SCHEMA public OWNER TO {service}_migrator; REVOKE ALL ON SCHEMA public FROM PUBLIC;', service, True, fixture)
            archive = (a.snapshot / (service + '.dump')).read_bytes()
            toc = pg.run(['docker', 'exec', '-i', pg.PROJECT + '-postgres', 'pg_restore', '--list'], archive).stdout.decode()
            if 'Dumped from database version: 18.' not in toc or 'Dumped by pg_dump version: 18.' not in toc:
                raise RuntimeError('Archive version compatibility not established')
            temporary = '/tmp/remediation-' + service + '.dump'
            pg.run(['docker', 'exec', '-i', pg.PROJECT + '-postgres', 'sh', '-c', 'cat > "$1"', 'restore', temporary], archive)
            script = 'IFS= read -r PGPASSWORD; export PGPASSWORD; exec pg_restore -h 127.0.0.1 -U "$1" -d "$2" --no-owner --no-acl --exit-on-error "$3"'
            pg.run(['docker', 'exec', '-i', pg.PROJECT + '-postgres', 'sh', '-c', script, 'restore', service + '_migrator', fixture, temporary], (pg.settings()['migration'][service]+'\n').encode())
            grants = (pg.ROOT / 'infrastructure/postgres/permissions.sql').read_text().replace(':"runtime"', '"'+service+'_app"')
            pg.sql(grants, service, True, fixture)
            actual = fingerprint(service, fixture)
            expected = recovery['data_fingerprints'][pg.DATABASES[service]]
            if actual != expected: raise RuntimeError('Restored data checksum/count mismatch: ' + service)
            if pg.sql("SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND pg_get_userbyid(c.relowner) <> current_user;", service, True, fixture) != '0':
                raise RuntimeError('Restored ownership mismatch')
            pg.sql('CREATE TABLE public.forbidden_restore(id int);', service, database=fixture, expect_failure=True)
            pg.sql('UPDATE schema_migrations SET dirty=false;', service, database=fixture, expect_failure=True)
            results[service] = {'restored_counts_and_hashes_match':True, 'owner':service+'_migrator',
                                'migration_version':pg.sql("SELECT version::text||':'||dirty::text FROM schema_migrations;", service, database=fixture)}
            print('PASS isolated restore/ownership/runtime/history checks:', service, flush=True)
    finally:
        # ONLY fixture DBs created in this invocation; six retained DBs are untouched.
        for db in created: pg.sql(f'DROP DATABASE {db};')
    if before != pg.data_fingerprint(): raise RuntimeError('Retained rehearsal business data changed')
    pg.verify()
    (a.evidence_dir / 'restore.json').write_text(json.dumps({'snapshot':a.snapshot.name,
        'manifest_sha256':hashlib.sha256((a.snapshot/'manifest.json').read_bytes()).hexdigest(),
        'databases':results,'retained_business_fingerprints_unchanged':True},indent=2))
    print('PASS all six backup restores; disposable DBs removed; no retained DB reset')


if __name__ == '__main__': main()
