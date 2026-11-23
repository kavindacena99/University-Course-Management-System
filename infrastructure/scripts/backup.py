"""Non-destructive logical backups. Restore is deliberately a documented manual operation."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import uuid

import platformctl as local


def restrict_directory(path):
    path.chmod(0o700)
    if os.name == 'nt':
        identity = subprocess.check_output(['whoami'], text=True).strip()
        subprocess.run(['icacls', str(path), '/inheritance:r', '/grant:r',
                        f'{identity}:(OI)(CI)F', '/grant:r', 'SYSTEM:(OI)(CI)F'],
                       check=True, capture_output=True)


def digest(path):
    result = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(block)
    return result.hexdigest()


def object_command(env, config, script, destination=None, data=None):
    values = config['services']['assessment']['environment']
    process_env = os.environ.copy()
    for key in ('OBJECT_STORAGE_ACCESS_KEY', 'OBJECT_STORAGE_SECRET_KEY'):
        process_env[key] = str(values[key])
    image = config['services']['assessment-minio-init']['image']
    command = ['docker', 'run', '--rm', '--read-only', '--cap-drop', 'ALL',
               '--security-opt', 'no-new-privileges', '--tmpfs', '/tmp:rw,noexec,nosuid,size=16m',
               '--network', config['networks']['data']['name'],
               '--env', 'OBJECT_STORAGE_ACCESS_KEY', '--env', 'OBJECT_STORAGE_SECRET_KEY']
    if destination:
        command += ['--mount', f'type=bind,source={destination},target=/backup']
    if data is not None:
        command += ['-i']
    command += ['--entrypoint', '/bin/sh', image, '-c',
                'set -eu; export MC_CONFIG_DIR=/tmp/mc; '
                'mc alias set local http://minio:9000 "$OBJECT_STORAGE_ACCESS_KEY" '
                '"$OBJECT_STORAGE_SECRET_KEY" >/dev/null; ' + script]
    result = subprocess.run(command, env=process_env, input=data, capture_output=True)
    if result.returncode:
        raise RuntimeError('Scoped object-store operation failed; credential-bearing output suppressed')


def create(env, sentinel=False, output_root=None):
    if sentinel and env == 'dev':
        raise RuntimeError('Synthetic object validation is restricted to new rehearsal environments')
    config = local.model(env)
    local.verify_model(env, config)
    local.health(env)
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '-' + uuid.uuid4().hex[:8]
    if output_root is None:
        # Keep database exports out of Git AND synced working directories.
        output_root = (Path(os.environ['LOCALAPPDATA']) / 'UCM/backups' if os.name == 'nt'
                       else Path(os.environ.get('XDG_STATE_HOME', Path.home() / '.local/state')) / 'ucm/backups')
    target = Path(output_root).expanduser().resolve() / env / stamp
    target.mkdir(parents=True, exist_ok=False, mode=0o700)
    restrict_directory(target)
    marker = '.infrastructure-validation/' + uuid.uuid4().hex + '.txt'
    payload = b'local synthetic backup validation only\n'
    if sentinel:
        object_command(env, config, f'mc pipe local/submissions/{marker} >/dev/null', data=payload)
    try:
        for service in local.SERVICES:
            output = target / f'{service}.dump'
            command = local.compose(env) + ['exec', '-T', service + '-db', 'sh', '-c',
                       'exec pg_dump -Fc -U "$POSTGRES_USER" "$POSTGRES_DB"']
            with output.open('xb') as stream:
                result = subprocess.run([str(x) for x in command], cwd=local.ROOT,
                                        stdout=stream, stderr=subprocess.PIPE)
            if result.returncode:
                raise RuntimeError(f'{service} backup failed; partial backup retained, not marked complete')
            output.chmod(0o600)
            with output.open('rb') as stream:
                result = subprocess.run([str(x) for x in local.compose(env) + ['exec', '-T',
                                        service + '-db', 'pg_restore', '--list']], cwd=local.ROOT,
                                        stdin=stream, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
            if result.returncode:
                raise RuntimeError(f'{service} archive integrity check failed')
        object_command(env, config, 'mkdir -p /backup/objects; mc mirror local/submissions /backup/objects >/dev/null', target)
        if sentinel and (target / 'objects' / marker).read_bytes() != payload:
            raise RuntimeError('Synthetic object was not recovered byte-for-byte into the backup')
        public = target / 'configuration'
        public.mkdir(mode=0o700)
        override = 'compose.' + local.ENVIRONMENTS[env][1] + '.yaml'
        for name in ('compose.yaml', override):
            shutil.copyfile(local.ROOT / 'infrastructure/docker' / name, public / name)
        for service in ('frontend', 'gateway'):
            runtime = local.env_path(env).parent / 'runtime' / f'{service}.conf'
            source = runtime if runtime.exists() else local.ROOT / (
                'frontend/nginx.conf' if service == 'frontend' else 'infrastructure/nginx/nginx.conf')
            shutil.copyfile(source, public / f'{service}.conf')
        normal = {service: {key: value for key, value in config['services'][service]['environment'].items()
                           if not any(word in key for word in ('PASSWORD', 'SECRET', 'TOKEN', 'KEY'))}
                  for service in local.SERVICES}
        (public / 'non-secret-settings.json').write_text(json.dumps(normal, indent=2), encoding='utf-8')
        images = {}
        for entry in local.states(env):
            images[entry['Service']] = local.run(['docker', 'inspect', entry['ID'],
                                                 '--format', '{{.Image}}'], capture=True).strip()
        manifest = {'environment': env, 'project': config['name'], 'created_utc': stamp,
                    'git_head': local.run(['git', '-c', f'safe.directory={local.ROOT.as_posix()}',
                                           'rev-parse', 'HEAD'], capture=True).strip(),
                    'images': images, 'secret_recovery': 'Separate environment Vault/key backup REQUIRED',
                    'consistency': 'Per-database MVCC snapshots; not a cross-service/object atomic snapshot',
                    'synthetic_object_verified': sentinel,
                    'files': {p.relative_to(target).as_posix(): {'sha256': digest(p), 'bytes': p.stat().st_size}
                              for p in target.rglob('*') if p.is_file()}}
        (target / 'manifest.json').write_text(json.dumps(manifest, indent=2), encoding='utf-8')
        (target / 'manifest.json').chmod(0o600)
        verify(target)
        print(f'PASS backup created: {target}; six valid PG archives, objects, non-secret config and checksums')
        print('Secrets are intentionally separate. Encrypt before off-device storage; no production backup schedule is active.')
        return target
    finally:
        if sentinel:
            # Delete only the newly generated UUID object, never a pre-existing key.
            object_command(env, config, f'mc rm local/submissions/{marker} >/dev/null')


def verify(target):
    target = Path(target).resolve()
    manifest = json.loads((target / 'manifest.json').read_text())
    if len([name for name in manifest['files'] if name.endswith('.dump')]) != 6:
        raise RuntimeError('Expected six database archives')
    for name, expected in manifest['files'].items():
        path = (target / name).resolve()
        if not path.is_relative_to(target) or path.is_symlink():
            raise RuntimeError('Unsafe backup manifest path')
        if path.stat().st_size != expected['bytes'] or digest(path) != expected['sha256']:
            raise RuntimeError('Backup checksum/size mismatch')
    print(f'PASS integrity: {len(manifest["files"])} files verified; no restore executed')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['create', 'verify'])
    parser.add_argument('--environment', choices=local.ENVIRONMENTS)
    parser.add_argument('--path', type=Path)
    parser.add_argument('--validate-object', action='store_true')
    parser.add_argument('--output-root', type=Path, help='Protected persistent host directory, outside cloud-synced workspaces')
    args = parser.parse_args()
    if args.action == 'verify':
        if not args.path:
            parser.error('verify requires --path')
        verify(args.path)
    else:
        if not args.environment:
            parser.error('create requires --environment')
        create(args.environment, args.validate_object, args.output_root)


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, ValueError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        sys.exit(1)
