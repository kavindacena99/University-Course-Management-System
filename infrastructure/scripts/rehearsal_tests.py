"""Fresh migration and Go integration tests on disposable DBs in the NEW rehearsal."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import shlex
import subprocess
import tempfile

import postgres_rehearsal as pg

GO = 'golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--only', choices=pg.DATABASES, help='Run one isolated fixture; does not erase other recorded failures')
    parser.add_argument('--evidence-dir', type=Path, help='New output directory; existing logs are never overwritten')
    parser.add_argument('--test-run', help='Optional Go -run filter for a targeted isolated regression')
    args = parser.parse_args()
    pg.guard()
    original = pg.data_fingerprint()
    logs = args.evidence_dir or (pg.STATE.parent / 'tests' / datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
    logs.mkdir(parents=True, exist_ok=False)
    for name in ('go-mod-cache', 'go-build-cache'):
        pg.run(['docker', 'volume', 'create', '--label', 'ucm.validation=phase123',
                '--label', 'com.docker.compose.project=' + pg.PROJECT, pg.PROJECT + '_' + name])
    created, failures = [], []
    try:
        for s, db in pg.DATABASES.items():
            if args.only and s != args.only: continue
            test = 'auth_test' if s == 'auth' else db + '_test'
            if pg.sql(f"SELECT count(*) FROM pg_database WHERE datname='{test}';") != '0':
                raise RuntimeError('Refusing to replace an existing test fixture database')
            pg.sql(f'CREATE DATABASE {test} OWNER {s}_migrator; REVOKE ALL ON DATABASE {test} FROM PUBLIC; GRANT CONNECT ON DATABASE {test} TO {s}_app;')
            created.append(test)
            pg.run(pg.compose(True) + ['run', '--rm', '--no-deps', '-e', 'DB_NAME=' + test, s + '-migrate'])
            statement = (pg.ROOT / 'infrastructure/postgres/permissions.sql').read_text().replace('\\set ON_ERROR_STOP on', '').replace(':"runtime"', '"' + s + '_app"').replace(":'runtime'", "'" + s + "_app'")
            pg.sql(statement, s, True, test)
            print('PASS fresh migration fixture:', s, flush=True)
            with tempfile.TemporaryDirectory(prefix='ucm-go-source-') as temporary:
                source = Path(temporary)
                directory = pg.platform.SERVICES[s]
                prefix = f'services/{directory}/'
                files = subprocess.check_output(['git', '-c', f'safe.directory={pg.ROOT.as_posix()}', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=pg.ROOT).decode().split('\0')
                for file in files:
                    if file.startswith(prefix):
                        target = source / file[len(prefix):]
                        target.parent.mkdir(parents=True, exist_ok=True)
                        shutil.copyfile(pg.ROOT / file, target)
                env = os.environ.copy()
                env['TEST_DATABASE_URL'] = f"postgres://{s}_app:{pg.settings()['runtime'][s]}@postgres:5432/{test}?sslmode=disable"
                command = ['docker', 'run', '--rm', '--label', 'ucm.validation=phase123',
                           '--name', pg.PROJECT + '-go-' + s, '--network', pg.PROJECT + '_data',
                           '--cpus', '2', '--memory', '2g', '--env', 'TEST_DATABASE_URL',
                           '--mount', f'type=bind,source={source},target=/src,readonly',
                           '--mount', f'type=volume,source={pg.PROJECT}_go-mod-cache,target=/go/pkg/mod',
                           '--mount', f'type=volume,source={pg.PROJECT}_go-build-cache,target=/root/.cache/go-build',
                           '--workdir', '/src', GO, 'sh', '-ec',
                           'go mod download; go mod verify; go mod tidy -diff; go test -count=1 ' +
                           ('-v -run ' + shlex.quote(args.test_run) + ' ' if args.test_run else '') +
                           './...; go vet ./...; go build -trimpath -o /tmp/service .']
                result = subprocess.run(command, env=env, capture_output=True)
                output = (result.stdout + result.stderr).decode('utf-8', 'replace')
                d = pg.settings()
                for secret in [d['admin'], *d['runtime'].values(), *d['migration'].values()]:
                    output = output.replace(secret, '[REDACTED]')
                (logs / (s + '.log')).write_text(output, encoding='utf-8')
                if result.returncode:
                    print(output[-5000:])
                    failures.append(s)
                    # Preserve real DB failure; independently check the existing CI/unit path.
                    env.pop('TEST_DATABASE_URL', None)
                    unit = subprocess.run(command, env=env, capture_output=True)
                    (logs / (s + '-unit.log')).write_text((unit.stdout + unit.stderr).decode('utf-8', 'replace'), encoding='utf-8')
                    if unit.returncode: raise RuntimeError('Unit/vet/build also failed: ' + s)
                    print('FAILED real database suite, baseline unit/vet/build passed:', s, flush=True)
                else:
                    print('PASS Go tests including available DB integration, vet, modules and build:', s, flush=True)
    finally:
        # Only the six test DBs created by THIS invocation; no business DB/drop/clean.
        for test in created:
            pg.sql(f'DROP DATABASE {test};')
    if original != pg.data_fingerprint():
        raise RuntimeError('Rehearsal business-data fingerprint changed during fixture tests')
    print('PASS disposable fixtures removed; all six restored business DB fingerprints unchanged')
    if failures:
        raise RuntimeError('Database integration failures retained (not skipped/weakened): ' + ','.join(failures))


if __name__ == '__main__':
    main()
