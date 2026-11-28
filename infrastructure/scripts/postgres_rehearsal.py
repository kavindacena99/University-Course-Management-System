"""Isolated QA-only PostgreSQL rehearsal. Never mounts or starts legacy resources."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
from datetime import datetime, timezone

import platformctl as platform

ROOT = Path(__file__).resolve().parents[2]
STATE = ROOT / '.infra-state/phase123/qa-rehearsal'
PROJECT = 'platform-qa-pg-rehearsal-20261123'
DATABASES = {'auth': 'authentication_and_security', 'user': 'users', 'course': 'courses',
             'enrollment': 'enrollments', 'assessment': 'assessments', 'analytics': 'analytics'}
VERSIONS = {'auth': 6, 'user': 4, 'course': 1, 'enrollment': 1, 'assessment': 3, 'analytics': 1}
POSTGRES = 'ucm-postgres:phase123'  # Build locally; runtime selection is pinned below.


def run(args, data=None, okay=False):
    result = subprocess.run([str(a) for a in args], input=data, capture_output=True, cwd=ROOT)
    if result.returncode and not okay:
        # SQL/password-bearing errors must not be echoed verbatim.
        raise RuntimeError(f'{args[0]} failed ({result.returncode}); sensitive output suppressed')
    return result


def settings():
    d = json.loads((STATE / 'credentials.json').read_text())
    if d['project'] != PROJECT:
        raise RuntimeError('Unexpected rehearsal project')
    return d


def compose(full=False):
    return ['docker', 'compose', '--env-file', STATE / 'rehearsal.env', '-p', PROJECT,
            '-f', 'infrastructure/docker/compose.target.yaml' if full else 'infrastructure/docker/compose.postgres.yaml']


def guard():
    if run(['docker', 'context', 'show']).stdout.decode().strip() != 'desktop-linux':
        raise RuntimeError('This local rehearsal requires the verified desktop-linux context')
    config = json.loads(run(compose() + ['config', '--format', 'json']).stdout)
    if config['name'] != PROJECT or set(config['services']) != {'postgres'}:
        raise RuntimeError('Unexpected rehearsal model')
    db = config['services']['postgres']
    if db.get('ports') or db['container_name'] != PROJECT + '-postgres':
        raise RuntimeError('Unsafe database exposure or container identity')
    vols = [v for v in db['volumes'] if v['type'] == 'volume']
    if len(vols) != 1 or config['volumes'][vols[0]['source']]['name'] != PROJECT + '_postgres-data':
        raise RuntimeError('Legacy/shared database volume rejected')
    if any(v.get('external') or v.get('driver_opts') for v in config['volumes'].values()):
        raise RuntimeError('External/aliased persistence rejected')
    return config


def sql(statement, service=None, migration=False, database=None, expect_failure=False):
    d = settings()
    if service:
        user = service + ('_migrator' if migration else '_app')
        password = d['migration' if migration else 'runtime'][service]
        database = database or DATABASES[service]
    else:
        user, password, database = 'platform_admin', d['admin'], database or 'postgres'
    # Read password from stdin in the new DB container; not argv or Docker config.
    script = 'IFS= read -r PGPASSWORD; export PGPASSWORD; exec psql -X -qAt -v ON_ERROR_STOP=1 -h 127.0.0.1 -U "$1" -d "$2"'
    r = run(['docker', 'exec', '-i', PROJECT + '-postgres', 'sh', '-c', script, 'psql', user, database],
            (password + '\n' + statement).encode(), okay=expect_failure)
    if expect_failure and r.returncode == 0:
        raise RuntimeError('Forbidden SQL unexpectedly succeeded')
    if expect_failure and not any(x in r.stderr.decode(errors='replace') for x in ('permission denied', 'must be owner')):
        raise RuntimeError('Negative privilege test failed for an unrelated reason')
    return r.stdout.decode().strip()


def init():
    if STATE.exists():
        raise RuntimeError('Rehearsal inputs already exist; refusing overwrite')
    STATE.mkdir(parents=True, mode=0o700)
    d = {'project': PROJECT, 'admin': secrets.token_hex(32),
         'runtime': {s: secrets.token_hex(32) for s in DATABASES},
         'migration': {s: secrets.token_hex(32) for s in DATABASES}}
    (STATE / 'credentials.json').write_text(json.dumps(d))
    (STATE / 'admin-password').write_text(d['admin'])
    statements = ['\\set ON_ERROR_STOP on', 'REVOKE CONNECT, TEMPORARY ON DATABASE postgres FROM PUBLIC;',
                  'REVOKE CONNECT, TEMPORARY ON DATABASE template1 FROM PUBLIC;']
    for s, db in DATABASES.items():
        for kind, suffix in (('runtime', 'app'), ('migration', 'migrator')):
            statements.append(f"CREATE ROLE {s}_{suffix} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD '{d[kind][s]}';")
        statements += [f'CREATE DATABASE {db} OWNER {s}_migrator;',
                       f'REVOKE ALL ON DATABASE {db} FROM PUBLIC;',
                       f'GRANT CONNECT ON DATABASE {db} TO {s}_app;', f'\\connect {db}',
                       f'ALTER SCHEMA public OWNER TO {s}_migrator;',
                       'REVOKE ALL ON SCHEMA public FROM PUBLIC;',
                       f'GRANT USAGE ON SCHEMA public TO {s}_app;',
                       f'ALTER DEFAULT PRIVILEGES FOR ROLE {s}_migrator IN SCHEMA public GRANT SELECT ON TABLES TO {s}_app;',
                       f'ALTER DEFAULT PRIVILEGES FOR ROLE {s}_migrator IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO {s}_app;',
                       f'ALTER DEFAULT PRIVILEGES FOR ROLE {s}_migrator IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;',
                       '\\connect postgres']
    (STATE / 'bootstrap.sql').write_text('\n'.join(statements) + '\n')
    (STATE / 'migration.pgpass').write_text(''.join(f"postgres:5432:{db}:{s}_migrator:{d['migration'][s]}\n" for s, db in DATABASES.items()))
    values = {'PG_PROJECT': PROJECT, 'POSTGRES_IMAGE': POSTGRES,
              'PG_ADMIN_PASSWORD_FILE': (STATE / 'admin-password').as_posix(),
              'PG_BOOTSTRAP_SQL_FILE': (STATE / 'bootstrap.sql').as_posix(),
              'PG_MIGRATION_PASSWORD_FILE': (STATE / 'migration.pgpass').as_posix(),
              'TARGET_APP_ENV': 'qa', 'FRONTEND_PORT': '3101', 'GATEWAY_PORT': '8181', 'PUBLIC_BASE_URL': 'http://localhost:8181',
              'MINIO_ROOT_USER': 'rehearsal-admin', 'MINIO_ROOT_PASSWORD': secrets.token_hex(32),
              'ASSESSMENT_OBJECT_USER': 'rehearsal-assessment', 'ASSESSMENT_OBJECT_PASSWORD': secrets.token_hex(32),
              'OBJECT_STORE_IMAGE': 'pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329'}
    # All app credentials/config belong exclusively to this rehearsal.
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    key = Ed25519PrivateKey.generate()
    (STATE / 'jwt-private.pem').write_bytes(key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
    public = key.public_key().public_bytes(serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo).decode()
    shared = secrets.token_hex(32)
    for s, directory in platform.SERVICES.items():
        e = platform.read_env(ROOT / 'services' / directory / '.env.example')
        e.pop('TEST_DATABASE_URL', None)
        e.update({'INTERNAL_SERVICE_SECRET': shared, 'DB_PASSWORD': d['runtime'][s],
                  'AUTH_JWT_ISSUER': 'university-auth-' + PROJECT, 'AUTH_JWT_AUDIENCE': 'university-api-' + PROJECT,
                  'CORS_ALLOWED_ORIGINS': 'http://localhost:3101', 'ACCESS_TOKEN_COOKIE_NAME': PROJECT.replace('-', '_') + '_access',
                  'CSRF_COOKIE_NAME': PROJECT.replace('-', '_') + '_csrf'})
        if s == 'auth':
            e.update({'AUTH_JWT_PRIVATE_KEY_FILE': '/run/secrets/auth_signing_key', 'AUTH_JWT_KEY_ID': PROJECT,
                      'NOTIFICATIONS_MODE': 'none', 'AUTH_COOKIE_SECURE': 'false', 'REFRESH_TOKEN_COOKIE_NAME': PROJECT.replace('-', '_') + '_refresh'})
        else:
            e['AUTH_JWT_PUBLIC_KEYS_JSON'] = json.dumps({PROJECT: public}, separators=(',', ':'))
        if s == 'user': e['ADMIN_BOOTSTRAP_TOKEN'] = secrets.token_hex(32)
        path = STATE / (s + '.env')
        path.write_text(''.join(f'{k}={v}\n' for k, v in e.items()))
        values[s.upper() + '_ENV_FILE'] = path.as_posix()
        values[s.upper() + '_DB_PASSWORD'] = d['runtime'][s]
        values[s.upper() + '_MIGRATION_DB_PASSWORD'] = d['migration'][s]
        values[s.upper() + '_IMAGE'] = 'ucm-' + s + ':phase123'
    (STATE / 'frontend.conf').write_text((ROOT / 'frontend/nginx.conf').read_text().replace('http://localhost:8080', 'http://localhost:8181'))
    (STATE / 'gateway.conf').write_text((ROOT / 'infrastructure/nginx/nginx.conf').read_text().replace('pid logs/nginx.pid;', 'pid /tmp/nginx.pid;'))
    values.update({'AUTH_PRIVATE_KEY_FILE': (STATE / 'jwt-private.pem').as_posix(),
                   'FRONTEND_CONFIG_FILE': (STATE / 'frontend.conf').as_posix(),
                   'GATEWAY_CONFIG_FILE': (STATE / 'gateway.conf').as_posix(),
                   'GATEWAY_IMAGE': 'ucm-gateway:phase123', 'FRONTEND_IMAGE': 'ucm-frontend:phase123',
                   'ASSESSMENT_MINIO_INIT_IMAGE': 'ucm-assessment-minio-init:phase123'})
    (STATE / 'rehearsal.env').write_text(''.join(f'{k}={v}\n' for k, v in values.items()))
    for p in STATE.iterdir(): p.chmod(0o600)
    (STATE / 'jwt-private.pem').chmod(0o444)
    if os.name == 'nt':
        identity = subprocess.check_output(['whoami'], text=True).strip()
        for p in [STATE, *STATE.iterdir()]:
            inheritance = '(OI)(CI)' if p.is_dir() else ''
            run(['icacls', p, '/inheritance:r', '/grant:r', f'{identity}:{inheritance}F', '/grant:r', f'SYSTEM:{inheritance}F'])
    print('PASS new rehearsal inputs initialized; credentials masked')


def grants():
    for s, db in DATABASES.items():
        statement = (ROOT / 'infrastructure/postgres/permissions.sql').read_text().replace('\\set ON_ERROR_STOP on', '').replace(':"runtime"', '"' + s + '_app"').replace(":'runtime'", "'" + s + "_app'")
        sql(statement, s, True)


def migrate():
    guard()
    for s in DATABASES:
        run(compose(True) + ['run', '--rm', '--no-deps', s + '-migrate'])
        print('PASS migration identity:', s)
    grants()


def restore(snapshot, evidence=None):
    guard()
    import backup
    backup.verify(snapshot)
    manifest = json.loads((snapshot / 'manifest.json').read_text())
    if manifest.get('environment') != 'qa' or manifest.get('project') != 'platform-qa':
        raise RuntimeError('Only verified existing QA snapshots are accepted')
    # Preflight every database/archive before any restoration; no clean/drop/reset.
    for s, db in DATABASES.items():
        if sql("SELECT count(*) FROM pg_tables WHERE schemaname='public';", database=db) != '0':
            raise RuntimeError('Restore requires completely empty service schemas')
        run(['docker', 'cp', snapshot / (s + '.dump'), PROJECT + '-postgres:/tmp/' + s + '.dump'])
        toc = run(['docker', 'exec', PROJECT + '-postgres', 'pg_restore', '--list', '/tmp/' + s + '.dump']).stdout.decode()
        if 'Dumped from database version: 18.' not in toc or 'Dumped by pg_dump version: 18.' not in toc:
            raise RuntimeError('Backup PostgreSQL major/version compatibility not established')
    for s, db in DATABASES.items():
        script = 'IFS= read -r PGPASSWORD; export PGPASSWORD; exec pg_restore -h 127.0.0.1 -U "$1" -d "$2" --no-owner --no-acl --exit-on-error "$3"'
        run(['docker', 'exec', '-i', PROJECT + '-postgres', 'sh', '-c', script, 'restore', s + '_migrator', db, '/tmp/' + s + '.dump'], (settings()['migration'][s] + '\n').encode())
        print('PASS verified QA archive restored as non-superuser:', s)
    grants()
    record_evidence('restore-evidence.json', {'snapshot': snapshot.name, 'manifest_sha256': hashlib.sha256((snapshot / 'manifest.json').read_bytes()).hexdigest(), 'databases': list(DATABASES.values())}, evidence)


def record_evidence(name, data, directory=None):
    directory = directory or (STATE / 'evidence' / datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
    directory.mkdir(parents=True, exist_ok=True)
    with (directory / name).open('x', encoding='utf-8') as file:
        json.dump(data, file, indent=2)


def verify():
    guard()
    if sql('SELECT count(*) FROM pg_database WHERE datname IN (' + ','.join("'" + x + "'" for x in DATABASES.values()) + ');') != '6':
        raise RuntimeError('Missing logical databases')
    for s, db in DATABASES.items():
        assert sql('SELECT current_user;', s) == s + '_app'
        assert sql('SELECT current_user;', s, True) == s + '_migrator'
        assert sql('SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database();', s) == s + '_migrator'
        assert sql("SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='public';", s) == s + '_migrator'
        flags = sql(f"SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls FROM pg_roles WHERE rolname IN ('{s}_app','{s}_migrator');")
        assert flags.splitlines() == ['f', 'f']
        for migration in (False, True):
            for other in DATABASES.values():
                if other != db: sql('SELECT 1;', s, migration, other, True)
        for forbidden in ('CREATE DATABASE forbidden_rehearsal;', 'CREATE ROLE forbidden_rehearsal;', 'CREATE TABLE public.forbidden_rehearsal(id int);', 'CREATE SCHEMA forbidden_rehearsal;', 'UPDATE schema_migrations SET dirty=false;'):
            sql(forbidden, s, expect_failure=True)
        sql('CREATE DATABASE forbidden_rehearsal;', s, True, expect_failure=True)
        sql('CREATE ROLE forbidden_rehearsal;', s, True, expect_failure=True)
        sql(f'SET ROLE {s}_migrator;', s, expect_failure=True)
        # Entire probe including DDL/data is rolled back; business rows untouched.
        assert sql('BEGIN; CREATE TABLE public.permission_probe(id serial PRIMARY KEY, value text); INSERT INTO permission_probe(value) VALUES (\'migration\'); ROLLBACK; SELECT 1;', s, True).endswith('1')
        table = sql("SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename <> 'schema_migrations' ORDER BY tablename LIMIT 1;", s)
        sql(f'SELECT count(*) FROM public."{table}";', s)
        assert sql(f"SELECT bool_and(has_table_privilege(current_user,'public.{table}',p)) FROM unnest(ARRAY['INSERT','UPDATE','DELETE']) p;", s) == 't'
        assert sql(f"SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND pg_get_userbyid(c.relowner)<>'{s}_migrator';", s) == '0'
        probe = 'rehearsal_probe_' + secrets.token_hex(4)
        sql(f'CREATE TABLE public.{probe}(id serial PRIMARY KEY,value text); GRANT INSERT,UPDATE,DELETE ON {probe} TO {s}_app;', s, True)
        try:
            answer = sql(f"BEGIN; INSERT INTO {probe}(value) VALUES ('runtime'); UPDATE {probe} SET value='updated'; SELECT value FROM {probe}; DELETE FROM {probe}; ROLLBACK;", s)
            assert 'updated' in answer
        finally:
            sql(f'DROP TABLE public.{probe};', s, True)
        assert sql('SELECT version::text || \':\' || dirty::text FROM schema_migrations;', s) == f'{VERSIONS[s]}:false'
        print('PASS SQL access/ownership/DDL/admin/cross-DB/migration checks:', s)
    print('PASS all six logical database security and migration contracts')


def data_fingerprint():
    result = {}
    for s, db in DATABASES.items():
        tables = sql("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;", s).splitlines()
        query = ' UNION ALL '.join(f"SELECT '{t}',count(*)::text,coalesce(md5(string_agg(row_to_json(t)::text,'' ORDER BY row_to_json(t)::text)),'empty') FROM public.\"{t}\" t" for t in tables)
        result[db] = sql(query, s)
    return result


def backup_cluster():
    import backup
    base = Path(os.environ.get('LOCALAPPDATA', str(Path.home() / '.local/state'))) / 'UCM/backups/qa-rehearsal'
    target = base / (datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + secrets.token_hex(4))
    target.mkdir(parents=True, mode=0o700)
    backup.restrict_directory(target)
    for s, db in DATABASES.items():
        data = run(['docker', 'exec', PROJECT + '-postgres', 'pg_dump', '-Fc', '-U', 'platform_admin', db]).stdout
        run(['docker', 'exec', '-i', PROJECT + '-postgres', 'pg_restore', '--list'], data)
        file = target / (s + '.dump')
        file.write_bytes(data); file.chmod(0o600)
    # Retain restorable global role definitions/memberships, never password hashes.
    globals_sql = run(['docker', 'exec', PROJECT + '-postgres', 'pg_dumpall',
                       '-U', 'platform_admin', '--globals-only', '--no-role-passwords']).stdout
    if b' PASSWORD ' in globals_sql:
        raise RuntimeError('Global backup unexpectedly contains role passwords')
    (target / 'globals-no-passwords.sql').write_bytes(globals_sql)
    (target / 'globals-no-passwords.sql').chmod(0o600)
    metadata = {'scope': 'PostgreSQL only; object storage/secrets require separate backup',
                'project': PROJECT, 'server': sql('SELECT version();'),
                'roles_without_passwords': sql("SELECT rolname,rolsuper,rolcreatedb,rolcreaterole FROM pg_roles WHERE rolname LIKE '%_app' OR rolname LIKE '%_migrator' OR rolname='platform_admin' ORDER BY rolname;"),
                'memberships': sql('SELECT roleid::regrole,member::regrole,admin_option FROM pg_auth_members ORDER BY 1,2;'),
                'database_owners_and_acl': sql('SELECT datname,pg_get_userbyid(datdba),datacl FROM pg_database ORDER BY datname;'),
                'data_fingerprints': data_fingerprint()}
    (target / 'recovery-metadata.json').write_text(json.dumps(metadata, indent=2))
    manifest = {'environment': 'qa-rehearsal', 'project': PROJECT,
                'files': {p.name: {'bytes': p.stat().st_size, 'sha256': hashlib.sha256(p.read_bytes()).hexdigest()} for p in target.iterdir() if p.is_file()}}
    (target / 'manifest.json').write_text(json.dumps(manifest, indent=2))
    backup.verify(target)
    print('PASS six consolidated-cluster logical backups/TOC/checksums:', target)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['init', 'config', 'start', 'migrate', 'restore', 'verify', 'recreate', 'stop', 'select-image', 'backup'])
    p.add_argument('--snapshot', type=Path)
    p.add_argument('--evidence-dir', type=Path, help='New evidence location; records are exclusively created')
    p.add_argument('--image', help='Local PostgreSQL candidate; never a legacy deployment target')
    a = p.parse_args()
    if a.action == 'init': return init()
    guard()
    if a.action == 'config': print('PASS isolated PostgreSQL model')
    elif a.action == 'start': run(compose() + ['up', '-d', '--wait', '--wait-timeout', '180'])
    elif a.action == 'migrate': migrate()
    elif a.action == 'restore':
        if not a.snapshot: p.error('--snapshot required')
        restore(a.snapshot.resolve(), a.evidence_dir)
    elif a.action == 'verify': verify()
    elif a.action == 'backup': backup_cluster()
    elif a.action in ('recreate', 'select-image'):
        before = data_fingerprint()
        if a.action == 'select-image':
            if not a.image: p.error('--image required')
            metadata = json.loads(run(['docker', 'image', 'inspect', a.image]).stdout)[0]
            if 'PG_MAJOR=18' not in metadata['Config']['Env']:
                raise RuntimeError('Only the compatible PostgreSQL 18 candidate is permitted')
            refs = metadata.get('RepoDigests', [])
            if not refs: raise RuntimeError('Candidate lacks a local immutable reference')
            env = STATE / 'rehearsal.env'
            lines = env.read_text().splitlines()
            env.write_text('\n'.join('POSTGRES_IMAGE=' + refs[0] if line.startswith('POSTGRES_IMAGE=') else line for line in lines) + '\n')
        run(compose() + ['up', '-d', '--force-recreate', '--wait', '--wait-timeout', '180', 'postgres'])
        assert before == data_fingerprint()
        record_evidence('persistence-evidence.json', {'row_counts_and_hashes_match': True, 'databases': list(before)}, a.evidence_dir)
        verify()
        print('PASS new volume data/schema survive database container recreation')
    elif a.action == 'stop': run(compose(True) + ['stop'])


if __name__ == '__main__':
    try: main()
    except (RuntimeError, AssertionError, OSError, ValueError) as e:
        print('ERROR:', str(e) or 'SQL contract assertion failed', file=sys.stderr)
        sys.exit(1)
