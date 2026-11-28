"""Two new private synthetic S3 stores. Never selects legacy storage or volumes."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile

import postgres_rehearsal as pg
import backup

STATE = pg.ROOT / '.infra-state/remediation/storage'
PROJECTS = ('platform-storage-remediation-20261123', 'platform-storage-restore-remediation-20261123')
VENDOR = 'pgsty/silo:RELEASE.2026-11-12T00-00-00Z@sha256:c20d112390b9a9ce8c11b8af95ad35bcaa62f4759663c09c936cd8d2b511f329'


def compose(project):
    if project not in PROJECTS:
        raise RuntimeError('Unknown storage rehearsal project')
    return ['docker', 'compose', '--env-file', STATE / (project + '.env'), '-p', project,
            '-f', 'infrastructure/docker/compose.storage-rehearsal.yaml']


def validate_model(model, project):
    if model['name'] != project or set(model['services']) != {'minio', 'storage-guard', 'assessment-minio-init'}:
        raise RuntimeError('Unexpected storage rehearsal selection')
    if any(s.get('ports') for s in model['services'].values()):
        raise RuntimeError('Storage rehearsal must not publish ports')
    for kind in ('volumes', 'networks'):
        for resource in model[kind].values():
            if resource.get('external') or resource.get('driver_opts') or not resource['name'].startswith(project + '_'):
                raise RuntimeError('Legacy/shared storage resources rejected')
    validate_boundary(model['services'])


def validate_boundary(services):
    store, guard = services['minio'], services['storage-guard']
    if store['command'] != ['server', '--address', '127.0.0.1:19000', '--console-address', '127.0.0.1:19001', '/data']:
        raise RuntimeError('Unguarded storage listener rejected')
    if guard.get('network_mode') != 'service:minio' or guard.get('networks'):
        raise RuntimeError('Guard must share only the store network namespace')
    if 'storage-guard' not in services['assessment-minio-init']['depends_on']:
        raise RuntimeError('Setup can bypass guard readiness')


def guard():
    if pg.run(['docker', 'context', 'show']).stdout.decode().strip() != 'desktop-linux':
        raise RuntimeError('Wrong Docker context')
    for project in PROJECTS:
        model = json.loads(pg.run(compose(project) + ['config', '--format', 'json']).stdout)
        validate_model(model, project)
        pg.run(compose(project) + ['config', '--quiet'])


def initialize():
    if STATE.exists():
        raise RuntimeError('Storage inputs already exist; refusing overwrite')
    STATE.mkdir(parents=True, mode=0o700)
    backup.restrict_directory(STATE)
    images = {}
    for name, image in [('GATEWAY_IMAGE', 'ucm-gateway:phase123'), ('ASSESSMENT_MINIO_INIT_IMAGE', 'ucm-assessment-minio-init:phase123')]:
        metadata = json.loads(pg.run(['docker', 'image', 'inspect', image]).stdout)[0]
        images[name] = metadata['RepoDigests'][0]
    for project in PROJECTS:
        values = dict(images, STORAGE_PROJECT=project, OBJECT_STORE_IMAGE=VENDOR,
                      MINIO_ROOT_USER='rehearsal-admin', MINIO_ROOT_PASSWORD=secrets.token_hex(32),
                      ASSESSMENT_OBJECT_USER='assessment-rehearsal', ASSESSMENT_OBJECT_PASSWORD=secrets.token_hex(32))
        file = STATE / (project + '.env')
        file.write_text(''.join(f'{k}={v}\n' for k,v in values.items())); file.chmod(0o600)
    print('PASS separate protected synthetic storage inputs; no credentials printed')


def test():
    guard()
    values = [pg.platform.read_env(STATE / (p + '.env')) for p in PROJECTS]
    env = os.environ.copy()
    for prefix, d in zip(('STORAGE', 'RESTORE'), values):
        env.update({prefix + '_ROOT_USER': d['MINIO_ROOT_USER'], prefix + '_ROOT_PASSWORD': d['MINIO_ROOT_PASSWORD'],
                    prefix + '_APP_USER': d['ASSESSMENT_OBJECT_USER'], prefix + '_APP_PASSWORD': d['ASSESSMENT_OBJECT_PASSWORD']})
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    env.update(STORAGE_REHEARSAL='isolated-synthetic-only', STORAGE_BACKUP_DIR='/evidence/backup-' + stamp)
    logs = STATE / 'results'; logs.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='ucm-storage-source-') as directory:
        source = Path(directory)
        files = subprocess.check_output(['git', '-c', f'safe.directory={pg.ROOT.as_posix()}', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=pg.ROOT).decode().split('\0')
        prefix = 'services/assessment/'
        for name in set(files):
            if name.startswith(prefix):
                target = source / name[len(prefix):]; target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(pg.ROOT / name, target)
        command = ['docker', 'run', '--rm', '--label', 'ucm.validation=remediation', '--cpus', '1', '--memory', '1g',
                   '--network', PROJECTS[0] + '_data', '--network', PROJECTS[1] + '_data',
                   '--mount', f'type=bind,source={source},target=/src,readonly',
                   '--mount', f'type=bind,source={logs},target=/evidence',
                   '--mount', f'type=volume,source={pg.PROJECT}_go-mod-cache,target=/go/pkg/mod',
                   '--mount', f'type=volume,source={pg.PROJECT}_go-build-cache,target=/root/.cache/go-build',
                   '--workdir', '/src']
        for key in ('STORAGE_ROOT_USER', 'STORAGE_ROOT_PASSWORD', 'STORAGE_APP_USER', 'STORAGE_APP_PASSWORD',
                    'RESTORE_ROOT_USER', 'RESTORE_ROOT_PASSWORD', 'RESTORE_APP_USER', 'RESTORE_APP_PASSWORD',
                    'STORAGE_REHEARSAL', 'STORAGE_BACKUP_DIR'):
            command += ['--env', key]
        command += ['golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18',
                    'go', 'test', '-count=1', '-v', '-run', '^TestObjectStorageRehearsal$', './internal']
        result = subprocess.run(command, env=env, capture_output=True)
    output = (result.stdout + result.stderr).decode('utf-8', 'replace')
    for d in values:
        for key in ('MINIO_ROOT_PASSWORD', 'ASSESSMENT_OBJECT_PASSWORD'): output = output.replace(d[key], '[REDACTED]')
    (logs / ('test-' + stamp + '.log')).write_text(output)
    print(output[-4000:])
    if result.returncode:
        raise RuntimeError('Storage test failed; separate redacted evidence retained')
    print('PASS synthetic S3 backup/restore and guard validation; no legacy access')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['init', 'config', 'start', 'test', 'stop'])
    args = parser.parse_args()
    if args.action == 'init': return initialize()
    guard()
    if args.action == 'config': return print('PASS two isolated guarded storage models')
    if args.action == 'test': return test()
    for project in PROJECTS:
        operation = ['up', '-d', '--wait', '--wait-timeout', '120'] if args.action == 'start' else ['stop', '--timeout', '20']
        pg.run(compose(project) + operation)
    print('PASS storage rehearsal ' + args.action + '; named volumes retained')


if __name__ == '__main__': main()
