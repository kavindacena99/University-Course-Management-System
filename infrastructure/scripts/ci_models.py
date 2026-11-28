"""Fresh, isolated Compose fixtures: never read or modify developer inputs."""
import json
import os
from pathlib import Path
import subprocess
import sys
import shutil
import tempfile
import re

import platformctl as local
import postgres_rehearsal as rehearsal
import storage_rehearsal as storage


def validate_target(config, project):
    if config.get('name') != project:
        raise RuntimeError('Target project identity mismatch')
    services = config['services']
    if 'postgres' not in services or any(s.endswith('-db') for s in services):
        raise RuntimeError('Target must contain one PostgreSQL service and no legacy DBs')
    if any(s not in ('frontend', 'gateway') and d.get('ports') for s, d in services.items()):
        raise RuntimeError('Private component publishes a port')
    for kind in ('networks', 'volumes'):
        for d in config[kind].values():
            if d.get('external') or d.get('driver_opts') or not d.get('name', '').startswith(project + '_'):
                raise RuntimeError('Shared/aliased target resource rejected')
    for s, db in rehearsal.DATABASES.items():
        app, migrate = services[s]['environment'], services[s + '-migrate']['environment']
        if (app['DB_HOST'], app['DB_NAME'], app['DB_USER']) != ('postgres', db, s + '_app'):
            raise RuntimeError('Runtime database mapping mismatch: ' + s)
        if (migrate['DB_HOST'], migrate['DB_NAME'], migrate['DB_USER']) != ('postgres', db, s + '_migrator'):
            raise RuntimeError('Migration database mapping mismatch: ' + s)
        if app['DB_PASSWORD'] == migrate['DB_PASSWORD'] or 'db-permissions' not in services[s]['depends_on']:
            raise RuntimeError('Credential separation or permission readiness barrier missing')
    for definition in config.get('secrets', {}).values():
        if not Path(definition['file']).is_file():
            raise RuntimeError('Missing synthetic secret input file')


def checked_model(root, command, label, env=None):
    r = subprocess.run([str(v) for v in command] + ['config', '--format', 'json'],
                       cwd=root, env=env, capture_output=True, text=True)
    if r.returncode:
        match = re.search(r'services\.[a-zA-Z0-9_.-]+ items at \d+ and \d+ are equal', r.stderr)
        detail = match.group(0) if match else 'check CLI version, schema and required fixture paths'
        raise RuntimeError(f'{label}: Compose failed ({r.returncode}): {detail}')
    subprocess.run([str(v) for v in command] + ['config', '--quiet'], cwd=root,
                   env=env, capture_output=True, check=True)
    print('PASS ' + label + ': syntax and resolved model')
    config = json.loads(r.stdout)
    for service, definition in config['services'].items():
        for mount in definition.get('volumes', []):
            if mount['type'] == 'bind' and not Path(mount['source']).exists():
                raise RuntimeError(label + ': missing bind source for ' + service)
        if definition.get('build'):
            build = definition['build']
            if not (Path(build['context']) / build.get('dockerfile', 'Dockerfile')).is_file():
                raise RuntimeError(label + ': missing Dockerfile for ' + service)
    return config


def main():
    source = local.ROOT
    print(subprocess.check_output(['docker', 'compose', 'version'], text=True).strip())
    with tempfile.TemporaryDirectory(prefix='ucm-compose-ci-') as directory:
        root = Path(directory)
        for pattern in ('infrastructure/docker/*.yaml', 'infrastructure/environments/*/*.example',
                        'services/*/.env.example', 'frontend/nginx.conf',
                        'infrastructure/nginx/nginx.conf', 'infrastructure/postgres/**/*.sh',
                        'infrastructure/postgres/*.sql', 'infrastructure/object-storage/*.conf', 'services/*/Dockerfile',
                        'services/assessment/MinIOInit.Dockerfile', 'frontend/Dockerfile',
                        'infrastructure/nginx/Dockerfile', 'infrastructure/postgres/Dockerfile'):
            for p in source.glob(pattern):
                target = root / p.relative_to(source)
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(p, target)
        original = (local.ROOT, rehearsal.ROOT, rehearsal.STATE, rehearsal.PROJECT)
        try:
            local.ROOT = rehearsal.ROOT = root
            for e in ('qa', 'prodlike'):
                local.initialize(e)
                local.verify_model(e, checked_model(root, local.compose(e), 'legacy-' + e))
            keydir = root / 'services/security-and-authentication/var/keys'
            keydir.mkdir(parents=True)
            shutil.copyfile(root / 'infrastructure/environments/qa/runtime/keys/jwt-private.pem', keydir / 'jwt-private.pem')
            variables = os.environ.copy()
            for s, d in local.SERVICES.items():
                variables[s.upper() + '_ENV_FILE'] = str(root / 'services' / d / '.env.example')
            dc = ['docker', 'compose', '--env-file', root / 'infrastructure/environments/dev/.env.example',
                  '-p', 'platform-dev', '-f', root / 'infrastructure/docker/compose.yaml']
            local.verify_model('dev', checked_model(root, dc, 'legacy-base', variables))
            local.verify_model('dev', checked_model(root, dc + ['-f', root / 'infrastructure/docker/compose.dev.yaml'], 'legacy-dev', variables))
            projects = set()
            for e, mode in (('dev', 'development'), ('qa', 'qa'), ('prodlike', 'production')):
                rehearsal.STATE = root / '.fixtures' / e
                rehearsal.PROJECT = 'platform-' + e + '-target'
                rehearsal.init()
                p = rehearsal.STATE / 'rehearsal.env'
                t = p.read_text().replace('TARGET_APP_ENV=qa', 'TARGET_APP_ENV=' + mode)
                front, gateway = local.ENVIRONMENTS[e][2:]
                p.write_text(t.replace('FRONTEND_PORT=3101', 'FRONTEND_PORT=' + str(front)).replace('GATEWAY_PORT=8181', 'GATEWAY_PORT=' + str(gateway)))
                c = checked_model(root, rehearsal.compose(True), 'consolidated-' + e)
                validate_target(c, rehearsal.PROJECT)
                storage.validate_boundary(c['services'])
                if c['name'] in projects: raise RuntimeError('Target project collision')
                projects.add(c['name'])
            c = checked_model(root, rehearsal.compose(), 'standalone-QA-postgres')
            if set(c['services']) != {'postgres'} or c['services']['postgres'].get('ports'):
                raise RuntimeError('Unsafe standalone rehearsal')
            for project in ('platform-storage-ci', 'platform-storage-restore-ci'):
                settings = local.read_env(p)
                settings.update(STORAGE_PROJECT=project, MINIO_ROOT_PASSWORD=rehearsal.secrets.token_hex(32),
                                ASSESSMENT_OBJECT_PASSWORD=rehearsal.secrets.token_hex(32))
                fixture = root / '.fixtures' / (project + '.env')
                fixture.write_text(''.join(f'{k}={v}\n' for k,v in settings.items()))
                command = ['docker', 'compose', '--env-file', fixture, '-p', project,
                           '-f', root / 'infrastructure/docker/compose.storage-rehearsal.yaml']
                storage.validate_model(checked_model(root, command, project), project)
        finally:
            local.ROOT, rehearsal.ROOT, rehearsal.STATE, rehearsal.PROJECT = original
    print('PASS ten configurations; no repository inputs or runtime resources changed')


if __name__ == '__main__':
    main()
