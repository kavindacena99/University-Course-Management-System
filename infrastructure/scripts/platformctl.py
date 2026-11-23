"""Local Compose lifecycle. No shell interpolation, volume deletion or implicit target.

Run with the Python requirements in requirements.txt installed. Secret contents
and rendered Compose configuration are never printed.
"""
import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
ENVIRONMENTS = {
    'dev': ('platform-dev', 'dev', 3000, 8080),
    'qa': ('platform-qa', 'qa', 3001, 8081),
    'prodlike': ('platform-prodlike', 'prod', 3002, 8082),
}
SERVICES = {'auth': 'security-and-authentication', 'user': 'user', 'course': 'course',
            'enrollment': 'enrollment', 'assessment': 'assessment', 'analytics': 'analytics'}
IMAGES = [*SERVICES, 'gateway', 'frontend', 'assessment-minio-init']


@contextmanager
def deployment_lock(env):
    directory = ROOT / '.infra-state' / env
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    lock = directory / 'deploy.lock'
    try:
        descriptor = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    except FileExistsError as error:
        raise RuntimeError('Deployment lock exists; inspect the owning process before retrying') from error
    try:
        os.write(descriptor, f'pid={os.getpid()}\n'.encode())
        yield
    finally:
        os.close(descriptor)
        lock.unlink()


def run(args, capture=False, **kwargs):
    result = subprocess.run([str(a) for a in args], cwd=ROOT, text=True,
                            capture_output=capture, **kwargs)
    if result.returncode:
        # Commands may contain resolved config or env validation errors. Do not
        # include captured stdout/stderr (which can contain secrets) in exceptions.
        raise RuntimeError(f'{Path(str(args[0])).name} failed (exit {result.returncode})')
    return result.stdout if capture else ''


def env_path(env):
    return (ROOT / 'infrastructure/docker/.env' if env == 'dev' else
            ROOT / 'infrastructure/environments' / ENVIRONMENTS[env][1] / '.env')


def read_env(path):
    values = {}
    for line in path.read_text(encoding='utf-8-sig').splitlines():
        if line.strip() and not line.lstrip().startswith('#') and '=' in line:
            key, value = line.split('=', 1)
            values[key] = value
    return values


def private_write(path, data, mode=0o600):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    # Exclusive creation: initialization never replaces existing credentials.
    with path.open('x', encoding='utf-8', newline='\n') as stream:
        stream.write(data)
    path.chmod(mode)


def write_env(path, values):
    private_write(path, ''.join(f'{k}={v}\n' for k, v in values.items()))


def initialize(env):
    if env == 'dev':
        raise RuntimeError('Existing DEV configuration is managed separately; never regenerate it')
    path = env_path(env)
    runtime = path.parent / 'runtime'
    if path.exists() or runtime.exists():
        raise RuntimeError('Environment already initialized or partially initialized; inspect before continuing')
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    values = read_env(path.parent / '.env.example')
    for key in [*(f'{s.upper()}_DB_PASSWORD' for s in SERVICES),
                'MINIO_ROOT_PASSWORD',
                'ASSESSMENT_OBJECT_PASSWORD']:
        values[key] = secrets.token_hex(32)
    values['MINIO_ROOT_USER'] = f'local-{env}-admin'
    values['ASSESSMENT_OBJECT_USER'] = f'assessment-{env}'
    private = Ed25519PrivateKey.generate()
    private_write(runtime / 'keys/jwt-private.pem', private.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption()).decode(), 0o444)
    # Parent is restricted; the individual read-only secret bind must be readable
    # by the existing non-root Go UID. Compose file secrets do not remap ownership.
    public = private.public_key().public_bytes(serialization.Encoding.PEM,
                                              serialization.PublicFormat.SubjectPublicKeyInfo).decode()
    public_json = json.dumps({f'local-{env}': public}, separators=(',', ':'))
    shared = secrets.token_hex(32)
    for service, directory in SERVICES.items():
        service_env = read_env(ROOT / 'services' / directory / '.env.example')
        service_env.pop('TEST_DATABASE_URL', None)
        service_env.update({
            'APP_ENV': 'production' if env == 'prodlike' else 'qa',
            'LOG_LEVEL': 'info', 'DB_PASSWORD': values[f'{service.upper()}_DB_PASSWORD'],
            'INTERNAL_SERVICE_SECRET': shared,
            'AUTH_JWT_ISSUER': f'university-auth-local-{env}',
            'AUTH_JWT_AUDIENCE': f'university-api-local-{env}',
            'CORS_ALLOWED_ORIGINS': f'http://localhost:{ENVIRONMENTS[env][2]}',
            'ACCESS_TOKEN_COOKIE_NAME': f'{env}_access_token',
            'CSRF_COOKIE_NAME': f'{env}_csrf_token',
        })
        if service == 'auth':
            service_env.update({'AUTH_JWT_KEY_ID': f'local-{env}',
                                'AUTH_JWT_PRIVATE_KEY_FILE': '/run/secrets/auth_signing_key',
                                'NOTIFICATIONS_MODE': 'none', 'AUTH_COOKIE_SECURE': 'false',
                                'REFRESH_TOKEN_COOKIE_NAME': f'{env}_refresh_token',
                                'PUBLIC_BASE_URL': values['PUBLIC_BASE_URL']})
        else:
            service_env['AUTH_JWT_PUBLIC_KEYS_JSON'] = public_json
        if service == 'user':
            service_env['ADMIN_BOOTSTRAP_TOKEN'] = secrets.token_hex(32)
        if service == 'assessment':
            service_env['OBJECT_STORAGE_ACCESS_KEY'] = values['ASSESSMENT_OBJECT_USER']
            service_env['OBJECT_STORAGE_SECRET_KEY'] = values['ASSESSMENT_OBJECT_PASSWORD']
        write_env(runtime / f'{service}.env', service_env)
    # Render public edge configuration per environment using one canonical source.
    frontend = (ROOT / 'frontend/nginx.conf').read_text(encoding='utf-8')
    private_write(runtime / 'frontend.conf', frontend.replace('http://localhost:8080',
                                                             values['PUBLIC_BASE_URL']), 0o644)
    # Config per project; shared audited snippets/routes remain in the image.
    gateway = (ROOT / 'infrastructure/nginx/nginx.conf').read_text(encoding='utf-8')
    private_write(runtime / 'gateway.conf', gateway.replace('pid logs/nginx.pid;', 'pid /tmp/nginx.pid;'), 0o644)
    write_env(path, values)
    print(f'Initialized isolated local {env}; no credentials printed. Protect its directory ACL.')


def compose(env, release=None):
    project, directory, _, _ = ENVIRONMENTS[env]
    if not env_path(env).is_file():
        raise RuntimeError(f'Missing local {env} environment file; initialize it first')
    args = ['docker', 'compose', '--env-file', env_path(env), '-p', project,
            '-f', 'infrastructure/docker/compose.yaml',
            '-f', f'infrastructure/docker/compose.{directory}.yaml']
    if release:
        args += ['--env-file', validate_release(release)]
    return args


def validate_release(path):
    path = Path(path).resolve()
    values = read_env(path)
    expected = {image.upper().replace('-', '_') + '_IMAGE' for image in IMAGES}
    if set(values) != expected or any(not re.fullmatch(
            r'[a-z0-9][a-z0-9./:_-]+@sha256:[0-9a-f]{64}', value) for value in values.values()):
        raise RuntimeError('Release must contain exactly nine IMAGE variables pinned by sha256 digest')
    return path


def model(env, release=None):
    return json.loads(run(compose(env, release) + ['config', '--format', 'json'], capture=True))


def verify_model(env, config):
    project, _, frontend, gateway = ENVIRONMENTS[env]
    if config['name'] != project:
        raise RuntimeError('Compose project identity mismatch')
    if any('name' in v and not v['name'].startswith(project + '_') or v.get('external')
           for kind in ('networks', 'volumes') for v in config[kind].values()):
        raise RuntimeError('Shared or external Docker resource rejected')
    if any(v.get('driver_opts') for v in config['volumes'].values()):
        raise RuntimeError('Host-backed volume driver aliases are not allowed in local isolated environments')
    for service, definition in config['services'].items():
        if definition.get('container_name'):
            raise RuntimeError('Fixed container name rejected')
        for port in definition.get('ports', []):
            if service not in ('frontend', 'gateway'):
                raise RuntimeError('Private service publishes a port')
            expected = frontend if service == 'frontend' else gateway
            if int(port['published']) != expected:
                raise RuntimeError('Environment port allocation mismatch')
    for service in [*(s + '-db' for s in SERVICES), 'minio']:
        mounts = config['services'].get(service, {}).get('volumes', [])
        if not mounts or any(v['type'] != 'volume' or v.get('source') not in config['volumes'] for v in mounts):
            raise RuntimeError('Database/object persistence must use project-owned named volumes')
    if env != 'dev':
        expected_root = (ROOT / 'infrastructure/environments' / ENVIRONMENTS[env][1]).resolve()
        # Resolved env files are flattened by Compose; inspect auth mount source
        # and compare credentials to DEV without disclosing their values.
        secret_source = Path(config['secrets']['auth_signing_key']['file']).resolve()
        if not secret_source.is_relative_to(expected_root):
            raise RuntimeError('Signing key does not belong to the selected environment')
        dev = read_env(env_path('dev')) if env_path('dev').exists() else {}
        current = read_env(env_path(env))
        passwords = [value for key, value in current.items() if key.endswith('_PASSWORD')]
        if len(passwords) != len(set(passwords)):
            raise RuntimeError('Rehearsal password reuse within an environment is not allowed')
        other = env_path('prodlike' if env == 'qa' else 'qa')
        peer = read_env(other) if other.exists() else {}
        for key, value in current.items():
            if key.endswith('_PASSWORD') and (value == dev.get(key) or value == peer.get(key)):
                raise RuntimeError('Credentials reused across environments')
        for service in SERVICES:
            password = current[f'{service.upper()}_DB_PASSWORD']
            if config['services'][service]['environment']['ACCESS_TOKEN_COOKIE_NAME'] != f'{env}_access_token':
                raise RuntimeError('Browser cookie namespace is not isolated')
            if (config['services'][service]['environment']['DB_PASSWORD'] != password or
                config['services'][service + '-db']['environment']['POSTGRES_PASSWORD'] != password or
                config['services'][service]['environment']['AUTH_JWT_ISSUER'] != f'university-auth-local-{env}'):
                raise RuntimeError('Resolved environment credentials/issuer mismatch; check exported shell variables')
    print(f'PASS {project}: ports, resource scope, private services and credential isolation')


def states(env):
    raw = run(compose(env) + ['ps', '-a', '--format', 'json'], capture=True)
    return json.loads(raw) if raw.strip().startswith('[') else [json.loads(s) for s in raw.splitlines() if s]


def health(env):
    entries = states(env)
    expected = set(SERVICES) | {s + '-db' for s in SERVICES} | {s + '-migrate' for s in SERVICES} | {'frontend', 'gateway', 'minio', 'assessment-minio-init'}
    if {entry['Service'] for entry in entries} != expected:
        raise RuntimeError('Missing or unexpected stack components')
    for entry in entries:
        job = entry['Service'].endswith('-migrate') or entry['Service'] == 'assessment-minio-init'
        if job and (entry['State'] != 'exited' or entry['ExitCode'] != 0):
            raise RuntimeError(f'Setup job failed: {entry["Service"]}')
        if not job and (entry['State'] != 'running' or entry['Health'] != 'healthy'):
            raise RuntimeError(f'Unhealthy component: {entry["Service"]}')
    project, _, frontend, gateway = ENVIRONMENTS[env]
    # Run HTTP probes inside the gateway so the same checks work from an Ansible
    # control container (whose localhost is not the Windows host).
    probes = [('http://127.0.0.1:8080/health', 200),
              ('http://frontend/health', 200), ('http://frontend/', 200),
              ('http://frontend/courses', 200),
              ('http://127.0.0.1:8080/not-a-route', 404),
              ('http://127.0.0.1:8080/internal/events', 404)]
    for path in ('auth/me', 'users/me', 'courses',
                 'enrollments/00000000-0000-0000-0000-000000000001',
                 'assessments/00000000-0000-0000-0000-000000000001', 'analytics/students/me'):
        probes.append((f'http://127.0.0.1:8080/api/v1/{path}', 401))
    for url, expected in probes:
        # BusyBox wget returns 1 for expected 401/404; capture headers only.
        result = subprocess.run([str(x) for x in compose(env) + ['exec', '-T', 'gateway',
                                 'wget', '-S', '-O', '/dev/null', url]],
                                cwd=ROOT, text=True, capture_output=True)
        codes = re.findall(r'HTTP/1\.[01] (\d{3})', result.stderr)
        if not codes or int(codes[-1]) != expected:
            raise RuntimeError(f'HTTP probe failed: {url} (expected {expected})')
    jobs = sum(entry['Service'].endswith('-migrate') or entry['Service'] == 'assessment-minio-init' for entry in entries)
    print(f'PASS {project}: {len(entries) - jobs} healthy containers, {jobs} successful jobs, 12 HTTP checks')


def deploy(env, mode, release=None):
    run(['docker', 'info', '--format', '{{.OSType}}'], capture=True)
    # Inspect existing state before mutation, and avoid three stacks contending
    # on this 8 GiB Desktop engine. DEV is never stopped by this helper.
    run(['docker', 'network', 'ls', '--format', '{{.Name}}'], capture=True)
    run(['docker', 'volume', 'ls', '--format', '{{.Name}}'], capture=True)
    if env != 'dev':
        peer = 'prodlike' if env == 'qa' else 'qa'
        if run(['docker', 'ps', '-q', '--filter',
                f'label=com.docker.compose.project={ENVIRONMENTS[peer][0]}'], capture=True).strip():
            raise RuntimeError('Stop the other local rehearsal first; DEV remains running')
    verify_model(env, model(env, release))
    cmd = compose(env, release)
    if mode == 'pull' and not release:
        raise RuntimeError('Registry mode requires an immutable release manifest')
    if release and mode == 'build':
        raise RuntimeError('Do not rebuild a promoted release')
    if mode == 'build':
        run(cmd + ['build'])
    elif mode == 'pull':
        run(cmd + ['pull', '--policy', 'always'])
    before = sorted((s['Service'], s['ID']) for s in states(env))
    run(cmd + ['up', '-d', '--no-build', '--wait', '--wait-timeout', '180'], capture=True)
    # A shared migration/app image can leave an existing app on an old ID even
    # after a separate build. Recreate only stale application containers.
    desired = model(env, release)
    stale = []
    for service in [*SERVICES, 'frontend', 'gateway']:
        wanted = run(['docker', 'image', 'inspect', desired['services'][service]['image'],
                      '--format', '{{.Id}}'], capture=True).strip()
        ids = run(cmd + ['ps', '-q', service], capture=True).split()
        if any(run(['docker', 'inspect', cid, '--format', '{{.Image}}'], capture=True).strip()
               != wanted for cid in ids):
            stale.append(service)
    if stale:
        if 'auth' in stale and env == 'dev':
            raise RuntimeError('DEV Auth image changed: preserve file notifications before explicitly updating it')
        run(cmd + ['up', '-d', '--no-deps', '--force-recreate', '--wait',
                   '--wait-timeout', '180', *stale], capture=True)
    run(cmd + ['exec', '-T', 'gateway', 'nginx', '-t'], capture=True)
    health(env)
    after = sorted((s['Service'], s['ID']) for s in states(env))
    state_dir = ROOT / '.infra-state' / env
    state_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
    if release:
        current = state_dir / 'current.env'
        previous = state_dir / 'previous.env'
        data = validate_release(release).read_text()
        if current.exists() and current.read_text() != data:
            previous.write_text(current.read_text(), encoding='utf-8')
        current.write_text(data, encoding='utf-8')
    print('PLATFORM_CHANGED=' + str(before != after).lower())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['init', 'config', 'deploy', 'health', 'nginx', 'stop', 'rollback'])
    parser.add_argument('--environment', required=True, choices=ENVIRONMENTS)
    parser.add_argument('--mode', choices=['existing', 'build', 'pull'], default='existing')
    parser.add_argument('--release', type=Path)
    args = parser.parse_args()
    if args.action == 'init':
        initialize(args.environment)
    elif args.action == 'config':
        verify_model(args.environment, model(args.environment, args.release))
    elif args.action == 'deploy':
        with deployment_lock(args.environment):
            deploy(args.environment, args.mode, args.release)
    elif args.action == 'health':
        health(args.environment)
    elif args.action == 'nginx':
        for service in ('gateway', 'frontend'):
            run(compose(args.environment) + ['exec', '-T', service, 'nginx', '-t'], capture=True)
        print('PASS gateway and frontend Nginx syntax')
    elif args.action == 'stop':
        verify_model(args.environment, model(args.environment))
        run(compose(args.environment) + ['stop'])
    elif args.action == 'rollback':
        previous = ROOT / '.infra-state' / args.environment / 'previous.env'
        if not previous.is_file():
            raise RuntimeError('No previous immutable release; rollback refused')
        with deployment_lock(args.environment):
            deploy(args.environment, 'pull', previous)


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, ValueError) as error:
        print(f'ERROR: {error}', file=sys.stderr)
        sys.exit(1)
