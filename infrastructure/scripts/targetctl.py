"""Guarded lifecycle for the prepared consolidated QA and PROD-like targets."""
import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid

import release_pipeline

ROOT = Path(__file__).resolve().parents[2]
TARGETS = {'qa': ('platform-qa-target', 3001, 8081),
           'prodlike': ('platform-prodlike-target', 3002, 8082)}
SERVICES = ('auth', 'user', 'course', 'enrollment', 'assessment', 'analytics')
DB_NAMES = ('authentication_and_security', 'users', 'courses', 'enrollments', 'assessments', 'analytics')
SILO = 'pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329'
EXPECTED = set(SERVICES) | {s + '-migrate' for s in SERVICES} | {'postgres', 'db-permissions', 'minio', 'storage-guard', 'assessment-minio-init', 'frontend', 'gateway'}


def state(env):
    return ROOT / '.infra-state/phase123/targets' / env


def run(args, *, input=None, check=True):
    result = subprocess.run([str(x) for x in args], cwd=ROOT, input=input,
                            capture_output=True, text=True)
    if check and result.returncode:
        raise RuntimeError(f'{Path(str(args[0])).name} failed (exit {result.returncode}); output suppressed')
    return result


def read_env(path):
    values = {}
    for line in path.read_text(encoding='utf-8-sig').splitlines():
        if line.strip() and not line.lstrip().startswith('#') and '=' in line:
            key, value = line.split('=', 1)
            values[key] = value
    return values


def release(path):
    path = Path(path).resolve()
    if not path.is_relative_to(ROOT / 'release-manifests'):
        raise ValueError('Release manifest must be in release-manifests')
    manifest = release_pipeline.read(path)
    release_pipeline.validate(manifest, path)
    return manifest


def compose(env, manifest):
    project = TARGETS[env][0]
    return ['docker', 'compose', '--env-file', state(env) / 'rehearsal.env',
            '--env-file', Path(manifest).resolve().parent / 'images.env',
            '-p', project, '-f', 'infrastructure/docker/compose.target.yaml']


def model(env, manifest):
    return json.loads(run(compose(env, manifest) + ['config', '--format', 'json']).stdout)


def verify_model(env, manifest, config):
    m = release(manifest)
    project, frontend, gateway = TARGETS[env]
    values = read_env(state(env) / 'rehearsal.env')
    if values.get('PG_PROJECT') != project or config.get('name') != project:
        raise ValueError('Target project identity mismatch')
    if (values.get('POSTGRES_IMAGE') != 'ucm-postgres@sha256:cb90d02d2e03c00889b7d8c08e1bf522ef62891b121c02d7b0a4239170661a4c'
            or values.get('OBJECT_STORE_IMAGE') != SILO):
        raise ValueError('PostgreSQL or storage image differs from reviewed target pin')
    services = config['services']
    if set(services) != EXPECTED:
        raise ValueError('Consolidated target service inventory mismatch')
    for row in m['images']:
        name = row['service']
        if services[name]['image'] != row['image'] + '@' + row['digest']:
            raise ValueError(f'Release image mismatch: {name}')
    if services['postgres']['image'] != values['POSTGRES_IMAGE'] or services['minio']['image'] != SILO:
        raise ValueError('Supporting image mismatch')
    if services['postgres'].get('container_name') != project + '-postgres':
        raise ValueError('PostgreSQL container identity mismatch')
    if services['minio']['command'] != ['server', '--address', '127.0.0.1:19000', '--console-address', '127.0.0.1:19001', '/data']:
        raise ValueError('Silo loopback binding mismatch')
    if services['storage-guard'].get('network_mode') != 'service:minio' or 'storage-guard' not in services['assessment-minio-init']['depends_on']:
        raise ValueError('Mandatory storage guard missing')
    guard_mounts = services['storage-guard'].get('volumes', [])
    if not any(Path(v.get('source', '')).resolve() == (ROOT / 'infrastructure/object-storage/select-guard.conf').resolve()
               and v.get('read_only') for v in guard_mounts):
        raise ValueError('Storage guard configuration mismatch')
    for name, expected_port in [('frontend', frontend), ('gateway', gateway)]:
        ports = services[name].get('ports', [])
        if len(ports) != 1 or int(ports[0]['published']) != expected_port or ports[0]['host_ip'] != '127.0.0.1':
            raise ValueError('Target edge port mismatch')
    if any(services[n].get('ports') for n in EXPECTED - {'frontend', 'gateway'}):
        raise ValueError('Private service publishes a host port')
    for item in config['volumes'].values():
        if item.get('external') or item.get('driver_opts') or not item['name'].startswith(project + '_'):
            raise ValueError('External, legacy, or aliased volume rejected')
    if set(config['volumes']) != {'postgres-data', 'minio-data'}:
        raise ValueError('Target must have exactly two persistent volumes')
    for item in config['networks'].values():
        if item.get('external') or not item['name'].startswith(project + '_'):
            raise ValueError('External or shared network rejected')
    return m


@contextmanager
def lock(env):
    root = ROOT / '.infra-state/releases/locks'
    root.mkdir(parents=True, exist_ok=True)
    path = root / (env + '.lock')
    try:
        descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    except FileExistsError as error:
        raise RuntimeError('Target deployment lock exists; inspect before retry') from error
    try:
        os.write(descriptor, str(os.getpid()).encode())
        yield
    finally:
        os.close(descriptor)
        path.unlink()


def engine():
    if run(['docker', 'context', 'show']).stdout.strip() != 'desktop-linux':
        raise RuntimeError('Verified desktop-linux Docker context required')
    if run(['docker', 'info', '--format', '{{.OSType}}']).stdout.strip() != 'linux':
        raise RuntimeError('Docker Desktop Linux engine required')
    if int(run(['docker', 'info', '--format', '{{.MemTotal}}']).stdout.strip()) < 6 * 1024 ** 3:
        raise RuntimeError('Docker Desktop memory below 6 GiB local target floor')


def snapshot(env, manifest):
    m = release(manifest)
    project = TARGETS[env][0]
    containers = run(['docker', 'ps', '-a', '--filter', f'label=com.docker.compose.project={project}',
                      '--format', '{{.ID}} {{.Names}} {{.Status}}']).stdout.splitlines()
    volumes = run(['docker', 'volume', 'ls', '--format', '{{.Name}}']).stdout.splitlines()
    networks = run(['docker', 'network', 'ls', '--format', '{{.Name}}']).stdout.splitlines()
    return {'environment': env, 'project': project, 'release': m['tag'],
            'source_commit': m['source_commit'], 'containers': containers,
            'volumes': [v for v in volumes if v.startswith(project + '_')],
            'networks': [n for n in networks if n.startswith(project + '_')],
            'captured_at': datetime.now(timezone.utc).isoformat()}


def approval(env):
    path = ROOT / '.infra-state/releases/storage-runtime-approval.json'
    if not path.is_file():
        raise RuntimeError('Separate owner storage runtime approval is absent')
    record = release_pipeline.read(path)
    if (record.get('image') != SILO or env not in record.get('environments', []) or
            not record.get('owner') or not record.get('approved_at') or
            record.get('guard_required') is not True):
        raise RuntimeError('Storage runtime approval does not cover exact image and environment')


def verified_backup(env, tag):
    marker = ROOT / '.infra-state/releases' / tag / (env + '-backup-verified.json')
    if not marker.is_file():
        raise RuntimeError('Existing target volume requires verified predeployment backup')
    record = release_pipeline.read(marker)
    directory = Path(record['path']).resolve()
    if directory.is_relative_to(ROOT) or not directory.is_dir():
        raise RuntimeError('Backup location is missing or inside repository')
    raw = (directory / 'manifest.json').read_bytes()
    if hashlib.sha256(raw).hexdigest() != record['manifest_sha256']:
        raise RuntimeError('Backup manifest checksum mismatch')
    manifest = json.loads(raw)
    if manifest['environment'] != env or manifest['release'] != tag:
        raise RuntimeError('Backup target identity mismatch')
    for name, item in manifest['files'].items():
        path = (directory / name).resolve()
        if not path.is_relative_to(directory) or path.stat().st_size != item['bytes'] or hashlib.sha256(path.read_bytes()).hexdigest() != item['sha256']:
            raise RuntimeError('Backup file verification failed')
    return record


def preflight(env, manifest):
    engine()
    m = verify_model(env, manifest, model(env, manifest))
    peer = 'prodlike' if env == 'qa' else 'qa'
    if run(['docker', 'ps', '-q', '--filter', f'label=com.docker.compose.project={TARGETS[peer][0]}']).stdout.strip():
        raise RuntimeError('Other consolidated target is running; constrained engine permits one')
    info = snapshot(env, manifest)
    directory = ROOT / '.infra-state/releases' / m['tag']
    directory.mkdir(parents=True, exist_ok=True)
    (directory / (env + '-preflight.json')).write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8')
    print(f"PASS {env} preflight: {info['project']}, {len(info['containers'])} existing containers, {len(info['volumes'])} target volumes")
    return m, info


def sync_config(env):
    """Update only the generated non-secret frontend routing file while stopped."""
    engine()
    project = TARGETS[env][0]
    if run(['docker', 'ps', '-q', '--filter', f'label=com.docker.compose.project={project}']).stdout.strip():
        raise RuntimeError('Stop target before changing its frontend routing file')
    target = state(env) / 'frontend.conf'
    if not target.is_file() or not (state(env) / 'rehearsal.env').is_file():
        raise RuntimeError('Prepared target inputs missing; never regenerate them here')
    current = target.read_bytes()
    desired = (ROOT / 'frontend/nginx.conf').read_bytes()
    if current == desired:
        print('PASS frontend routing already current:', project)
        return
    backup_dir = ROOT / '.infra-state/releases/config-backups' / env
    backup_dir.mkdir(parents=True, exist_ok=True)
    backup_path = backup_dir / ('frontend-' + hashlib.sha256(current).hexdigest() + '.conf')
    with backup_path.open('xb') as stream:
        stream.write(current)
    target.write_bytes(desired)
    if target.read_bytes() != desired:
        raise RuntimeError('Frontend routing update verification failed')
    print('PASS frontend routing updated with original retained:', project)


def pull(env, manifest):
    m, _ = preflight(env, manifest)
    for row in m['images']:
        reference = row['image'] + '@' + row['digest']
        run(['docker', 'pull', '--platform', 'linux/amd64', reference])
        if run(['docker', 'image', 'inspect', reference, '--format', '{{.Id}}']).stdout.strip() == '':
            raise RuntimeError('Pulled image not inspectable: ' + row['service'])
        print('PULLED ' + row['service'] + ' ' + row['digest'])
    postgres = model(env, manifest)['services']['postgres']['image']
    if run(['docker', 'image', 'inspect', postgres, '--format', '{{.Id}}']).stdout.strip() != postgres.split('@')[1]:
        raise RuntimeError('Reviewed local PostgreSQL candidate ID mismatch')
    run(['docker', 'pull', '--platform', 'linux/amd64', SILO])
    print('PASS release and supporting image pulls; no container started')


def start_support(env, manifest):
    with lock(env):
        preflight(env, manifest)
        approval(env)
        run(compose(env, manifest) + ['up', '-d', '--no-build', '--wait', '--wait-timeout', '180',
                                      'postgres', 'minio', 'storage-guard'])
        print('PASS target PostgreSQL and guarded storage started for backup; applications remain stopped')


def ps(env, manifest):
    raw = run(compose(env, manifest) + ['ps', '-a', '--format', 'json']).stdout.strip()
    return json.loads(raw) if raw.startswith('[') else [json.loads(line) for line in raw.splitlines()]


def backup(env, manifest):
    import backup as backup_support
    with lock(env):
        m, _ = preflight(env, manifest)
        approval(env)
        rows = {r['Service']: r for r in ps(env, manifest)}
        if any(rows.get(s, {}).get('State') != 'running' for s in ('postgres', 'minio', 'storage-guard')):
            raise RuntimeError('Database and guarded object store must be running for a consistent predeployment backup')
        base = Path(os.environ.get('LOCALAPPDATA', str(Path.home() / '.local/state'))) / 'UCM/backups' / (env + '-target')
        destination = base / (datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex[:8])
        if destination.resolve().is_relative_to(ROOT):
            raise RuntimeError('Backup destination must be outside synced repository')
        destination.mkdir(parents=True, exist_ok=False)
        backup_support.restrict_directory(destination)
        project = TARGETS[env][0]
        for service, database in zip(SERVICES, DB_NAMES):
            result = subprocess.run(['docker', 'exec', project + '-postgres', 'pg_dump', '-Fc',
                                     '-U', 'platform_admin', database], cwd=ROOT, capture_output=True)
            if result.returncode:
                raise RuntimeError('PostgreSQL backup failed: ' + service)
            check = subprocess.run(['docker', 'exec', '-i', project + '-postgres', 'pg_restore', '--list'],
                                   cwd=ROOT, input=result.stdout, capture_output=True)
            if check.returncode:
                raise RuntimeError('PostgreSQL archive TOC failed: ' + service)
            (destination / (service + '.dump')).write_bytes(result.stdout)
        values = read_env(state(env) / 'rehearsal.env')
        temporary = '/tmp/ucm-backup-' + uuid.uuid4().hex
        credential = 'http://' + values['ASSESSMENT_OBJECT_USER'] + ':' + values['ASSESSMENT_OBJECT_PASSWORD'] + '@127.0.0.1:9000'
        script = 'IFS= read -r MC_HOST_target; export MC_HOST_target; exec mc mirror target/submissions "$1"'
        run(['docker', 'exec', '-i', project + '-minio-1', 'sh', '-c', script, 'mc', temporary], input=credential + '\n')
        run(['docker', 'cp', project + '-minio-1:' + temporary, destination / 'objects'])
        # Remove only the uniquely named temporary copy inside the container.
        run(['docker', 'exec', project + '-minio-1', 'rm', '-rf', temporary])
        files = {p.relative_to(destination).as_posix(): {'bytes': p.stat().st_size,
                    'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
                 for p in destination.rglob('*') if p.is_file()}
        record = {'environment': env, 'project': project, 'release': m['tag'],
                  'captured_at': datetime.now(timezone.utc).isoformat(), 'files': files,
                  'scope': 'Six logical PostgreSQL dumps and guarded submissions; secret escrow separate'}
        (destination / 'manifest.json').write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')
        for name, metadata in files.items():
            if hashlib.sha256((destination / name).read_bytes()).hexdigest() != metadata['sha256']:
                raise RuntimeError('Backup checksum verification failed')
        marker = ROOT / '.infra-state/releases' / m['tag'] / (env + '-backup-verified.json')
        marker.write_text(json.dumps({'path': str(destination), 'manifest_sha256': hashlib.sha256((destination / 'manifest.json').read_bytes()).hexdigest(),
                                      'captured_at': record['captured_at']}, indent=2) + '\n', encoding='utf-8')
        print('PASS protected external backup:', destination.name, len(files), 'checksummed files')


def migrate(env, manifest):
    import postgres_rehearsal as pg
    with lock(env):
        m, info = preflight(env, manifest)
        approval(env)
        if info['volumes']:
            verified_backup(env, m['tag'])
        original = (pg.STATE, pg.PROJECT)
        try:
            pg.STATE, pg.PROJECT = state(env), TARGETS[env][0]
            run(compose(env, manifest) + ['up', '-d', '--no-build', '--wait', 'postgres'])
            for service in SERVICES:
                run(compose(env, manifest) + ['run', '--rm', '--no-deps', service + '-migrate'])
                print('PASS migration:', service)
            run(compose(env, manifest) + ['up', '-d', '--no-build', 'db-permissions'])
            pg.verify()
        finally:
            pg.STATE, pg.PROJECT = original
        print('PASS restricted migrations and SQL privilege verification')


def health(env, manifest):
    m = release(manifest)
    rows = ps(env, manifest)
    if {r['Service'] for r in rows} != EXPECTED:
        raise RuntimeError('Target container inventory incomplete')
    for row in rows:
        job = row['Service'].endswith('-migrate') or row['Service'] in ('db-permissions', 'assessment-minio-init')
        if job:
            if row['State'] != 'exited' or row['ExitCode'] != 0:
                raise RuntimeError('Setup job failed: ' + row['Service'])
        elif row['State'] != 'running' or row.get('Health') != 'healthy':
            raise RuntimeError('Runtime unhealthy: ' + row['Service'])
    for image in m['images']:
        name = image['service']
        container_id = next(r['ID'] for r in rows if r['Service'] == name)
        actual = run(['docker', 'inspect', container_id, '--format', '{{.Image}}']).stdout.strip()
        wanted = run(['docker', 'image', 'inspect', image['image'] + '@' + image['digest'],
                      '--format', '{{.Id}}']).stdout.strip()
        if actual != wanted:
            raise RuntimeError('Running image ID mismatch: ' + name)
    print('PASS target health:', len(rows), 'components')


def smoke(env, manifest):
    health(env, manifest)
    for path, expected in [('/health', 200), ('/api/v1/auth/me', 401),
                           ('/api/v1/courses', 401), ('/not-a-route', 404)]:
        result = run(compose(env, manifest) + ['exec', '-T', 'gateway', 'wget', '-S', '-O', '/dev/null',
                                               'http://127.0.0.1:8080' + path], check=False)
        if f'HTTP/1.1 {expected}' not in result.stderr and f'HTTP/1.0 {expected}' not in result.stderr:
            raise RuntimeError('Gateway smoke HTTP status mismatch: ' + path)
    run(compose(env, manifest) + ['exec', '-T', 'gateway', 'nginx', '-t'])
    run(compose(env, manifest) + ['exec', '-T', 'frontend', 'nginx', '-t'])
    print('PASS gateway and frontend smoke probes')


def validate(env, manifest):
    import postgres_rehearsal as pg
    smoke(env, manifest)
    config = model(env, manifest)
    verify_model(env, manifest, config)
    select = run(compose(env, manifest) + ['exec', '-T', 'minio', 'curl', '-sS', '-o', '/dev/null',
                                          '-w', '%{http_code}', '-X', 'POST',
                                          'http://127.0.0.1:9000/submissions?select&select-type=2'])
    if select.stdout.strip() != '403':
        raise RuntimeError('S3 Select was not denied by guard')
    bypass = run(compose(env, manifest) + ['exec', '-T', 'gateway', 'wget', '-q', '-O', '/dev/null',
                                          'http://minio:19000/minio/health/live'], check=False)
    if bypass.returncode == 0:
        raise RuntimeError('Unguarded Silo listener reachable from application network')
    if env == 'qa':
        original = (pg.STATE, pg.PROJECT)
        try:
            pg.STATE, pg.PROJECT = state(env), TARGETS[env][0]
            pg.verify()
        finally:
            pg.STATE, pg.PROJECT = original
    print('PASS consolidated model, guard denial, alternate-listener probe and SQL checks')


def deploy(env, manifest):
    with lock(env):
        m, info = preflight(env, manifest)
        approval(env)
        if info['volumes']:
            verified_backup(env, m['tag'])
        current = ROOT / '.infra-state/releases/targets' / env / 'current-deployment.json'
        previous = current.parent / 'previous-deployment.json'
        if current.exists() and previous.exists():
            raise RuntimeError('Previous deployment record exists; preserve it before new promotion')
        pull(env, manifest)
        run(compose(env, manifest) + ['up', '-d', '--no-build', '--wait', '--wait-timeout', '240'])
        health(env, manifest)
        validate(env, manifest)
        evidence = {'environment': env, 'project': TARGETS[env][0], 'release': m['tag'],
                    'manifest_sha256': hashlib.sha256(Path(manifest).read_bytes()).hexdigest(),
                    'source_commit': m['source_commit'], 'deployed_at': datetime.now(timezone.utc).isoformat(),
                    'images': {r['service']: r['digest'] for r in m['images']}}
        out = ROOT / '.infra-state/releases' / m['tag'] / (env + '-deployment.json')
        out.write_text(json.dumps(evidence, indent=2) + '\n', encoding='utf-8')
        current.parent.mkdir(parents=True, exist_ok=True)
        if current.exists():
            previous.write_bytes(current.read_bytes())
        current.write_text(json.dumps(evidence, indent=2) + '\n', encoding='utf-8')
        print('PASS target deployed and smoke checked; full QA validation remains separate')


def stop(env, manifest):
    with lock(env):
        preflight(env, manifest)
        run(compose(env, manifest) + ['stop', '--timeout', '30'])
        if run(['docker', 'ps', '-q', '--filter', f'label=com.docker.compose.project={TARGETS[env][0]}']).stdout.strip():
            raise RuntimeError('Target still has running containers')
        print('PASS target stopped; volumes and networks preserved')


def rollback(env, manifest):
    previous = release(manifest)
    current_path = ROOT / '.infra-state/releases/targets' / env / 'current-deployment.json'
    compatibility = ROOT / '.infra-state/releases' / previous['tag'] / (env + '-rollback-compatibility.json')
    if not current_path.is_file() or not compatibility.is_file():
        raise RuntimeError('Previous deployment and reviewed migration compatibility are required')
    current = release_pipeline.read(current_path)
    review = release_pipeline.read(compatibility)
    if (current['release'] == previous['tag'] or review.get('approved') is not True or
            review.get('from_release') != current['release'] or review.get('to_release') != previous['tag']):
        raise RuntimeError('Rollback reference or schema compatibility review mismatch')
    backup_marker = ROOT / '.infra-state/releases' / current['release'] / (env + '-backup-verified.json')
    if not backup_marker.is_file():
        raise RuntimeError('Verified current backup required before image rollback')
    deploy(env, manifest)
    record = {'environment': env, 'previous_release': previous['tag'],
              'previous_source_commit': previous['source_commit'],
              'previous_images': {r['service']: r['digest'] for r in previous['images']},
              'current_release': current['release'], 'current_source_commit': current['source_commit'],
              'current_images': current['images'], 'deployed_at': datetime.now(timezone.utc).isoformat(),
              'rollback_command': f'targetctl.py rollback --environment {env} --release {manifest}',
              'validation': 'guarded deploy, health, smoke and SQL checks passed'}
    (ROOT / '.infra-state/releases' / previous['tag'] / (env + '-rollback.json')).write_text(
        json.dumps(record, indent=2) + '\n', encoding='utf-8')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['config', 'preflight', 'pull', 'backup', 'migrate', 'deploy',
                                          'health', 'validate', 'smoke', 'stop', 'status', 'rollback', 'sync-config',
                                          'start-support'])
    parser.add_argument('--environment', required=True, choices=TARGETS)
    parser.add_argument('--release', required=True, type=Path)
    args = parser.parse_args()
    env, manifest = args.environment, args.release
    if args.action == 'config':
        verify_model(env, manifest, model(env, manifest))
        print('PASS consolidated target model:', TARGETS[env][0])
    elif args.action == 'sync-config':
        sync_config(env)
    elif args.action == 'preflight':
        preflight(env, manifest)
    elif args.action == 'pull':
        pull(env, manifest)
    elif args.action == 'start-support':
        start_support(env, manifest)
    elif args.action == 'backup':
        backup(env, manifest)
    elif args.action == 'migrate':
        migrate(env, manifest)
    elif args.action == 'deploy':
        deploy(env, manifest)
    elif args.action == 'health':
        health(env, manifest)
    elif args.action == 'smoke':
        smoke(env, manifest)
    elif args.action == 'status':
        engine()
        info = snapshot(env, manifest)
        print(json.dumps(info, indent=2))
    elif args.action == 'stop':
        stop(env, manifest)
    elif args.action == 'validate':
        validate(env, manifest)
        print('Full business and browser validation require the QA suite')
    elif args.action == 'rollback':
        rollback(env, manifest)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, RuntimeError, OSError, KeyError) as error:
        print('ERROR:', error, file=sys.stderr)
        sys.exit(1)
