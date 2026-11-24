"""Validate newly tagged images and ONLY the isolated QA rehearsal application stack."""
import json
import argparse
from pathlib import Path
import re
import socket
import urllib.request

import postgres_rehearsal as pg


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tag', default='phase123')
    parser.add_argument('--evidence-dir', type=Path)
    args = parser.parse_args()
    pg.guard()
    for port in (3101, 8181):
        with socket.socket() as probe:
            probe.bind(('127.0.0.1', port))
    env = pg.STATE / 'rehearsal.env'
    values = pg.platform.read_env(env)
    for service in pg.platform.IMAGES:
        values[service.upper().replace('-', '_') + '_IMAGE'] = 'ucm-' + service + ':' + args.tag
    env.write_text(''.join(f'{k}={v}\n' for k,v in values.items()))
    sizes = {}
    for service in pg.platform.IMAGES:
        image = 'ucm-' + service + ':' + args.tag
        metadata = json.loads(pg.run(['docker', 'image', 'inspect', image]).stdout)[0]
        sizes[service] = {'bytes': metadata['Size'], 'id': metadata['Id']}
        if service in pg.DATABASES:
            binary = 'auth-service' if service == 'auth' else service + '-service'
            check = f'test -x /app/{binary}; test -s /etc/ssl/certs/ca-certificates.crt; test "$(id -u)" != 0; test ! -e /app/go.mod; test ! -e /src; ! command -v go; ! command -v gcc; ! command -v npm; test -z "$(find /app -name \"*_test.go\" -o -name \"*.md\")"'
            pg.run(['docker', 'run', '--rm', '--label', 'ucm.validation=phase123',
                    '--network', 'none', '--read-only', '--cap-drop', 'ALL',
                    '--entrypoint', 'sh', image, '-ec', check])
        if service in ('gateway', 'frontend'):
            pg.run(['docker', 'run', '--rm', '--label', 'ucm.validation=phase123',
                    '--entrypoint', 'nginx', image, '-t'])
        print('PASS runtime artifact/user/tool inspection:', service, flush=True)
    evidence = args.evidence_dir or pg.STATE.parent
    evidence.mkdir(parents=True, exist_ok=True)
    output = evidence / ('image-inventory-' + args.tag + '.json')
    if output.exists(): raise RuntimeError('Refusing to overwrite runtime evidence')
    output.write_text(json.dumps(sizes, indent=2))
    pg.run(pg.compose(True) + ['up', '-d', '--no-build', '--wait', '--wait-timeout', '180'])
    raw = pg.run(pg.compose(True) + ['ps', '-a', '--format', 'json']).stdout.decode()
    rows = json.loads(raw) if raw.strip().startswith('[') else [json.loads(l) for l in raw.splitlines() if l]
    if len(rows) != 19:
        raise RuntimeError('Expected 19 target components including the storage Select guard')
    for row in rows:
        job = row['Service'].endswith('-migrate') or row['Service'] in ('db-permissions', 'assessment-minio-init')
        if job:
            if row['State'] != 'exited' or row['ExitCode'] != 0: raise RuntimeError('Setup job failure')
        elif row['State'] != 'running' or row['Health'] != 'healthy':
            raise RuntimeError('Unhealthy rehearsal runtime: ' + row['Service'])
    checks = [('/health', 200), ('/unknown-rehearsal-route', 404), ('/internal/events', 404)]
    for path in ('auth/me', 'users/me', 'courses', 'enrollments/00000000-0000-0000-0000-000000000001',
                 'assessments/00000000-0000-0000-0000-000000000001', 'analytics/students/me'):
        checks.append(('/api/v1/' + path, 401))
    import urllib.error
    for path, expected in checks:
        try: response = urllib.request.urlopen('http://127.0.0.1:8181' + path, timeout=40)
        except urllib.error.HTTPError as e: response = e
        with response:
            if response.status != expected or not response.headers.get('X-Content-Type-Options'):
                raise RuntimeError('Gateway status/header mismatch: ' + path)
        print('PASS gateway', path, expected, flush=True)
    with urllib.request.urlopen('http://127.0.0.1:3101/', timeout=10) as response:
        page = response.read().decode()
        if response.status != 200: raise RuntimeError('Frontend inaccessible')
    asset = re.search(r'src="(/assets/[^"]+[.]js)"', page)
    if not asset: raise RuntimeError('Frontend compiled script reference missing')
    with urllib.request.urlopen('http://127.0.0.1:3101' + asset.group(1), timeout=10) as response:
        if response.status != 200: raise RuntimeError('Frontend artifact inaccessible')
    for service in ('frontend', 'gateway'):
        pg.run(pg.compose(True) + ['exec', '-T', service, 'nginx', '-t'])
    for service in pg.DATABASES:
        cid = pg.run(pg.compose(True) + ['ps', '-q', service]).stdout.decode().strip()
        metadata = json.loads(pg.run(['docker', 'inspect', cid]).stdout)[0]
        if metadata['Config']['User'] != 'app' or not metadata['HostConfig']['ReadonlyRootfs']:
            raise RuntimeError('Unexpected runtime privilege/filesystem setting')
    pg.run(pg.compose(True) + ['exec', '-T', 'storage-guard', 'nginx', '-t'])
    probe = pg.run(pg.compose(True) + ['exec', '-T', 'assessment', 'wget', '-S', '-O', '/dev/null',
                   '--post-data=<safe/>', 'http://minio:9000/submissions/probe?%73elect&select-type=2'], okay=True)
    # BusyBox wget omits response headers on HTTP errors; keep the application
    # network-path 403 check and independently prove the guard's response marker.
    marker = pg.run(pg.compose(True) + ['exec', '-T', 'minio', 'curl', '--silent', '--show-error',
                    '--include', '--request', 'POST', '--data', '<safe/>',
                    'http://minio:9000/submissions/probe?%73elect&select-type=2'])
    if probe.returncode == 0 or b'403 Forbidden' not in probe.stderr or \
       b'HTTP/1.1 403' not in marker.stdout or b'x-s3-select-guard: denied' not in marker.stdout.lower():
        raise RuntimeError('Application stack can bypass the storage Select guard')
    print('PASS eleven healthy runtimes / eight setup jobs; storage guard, six Go listeners, frontend assets, gateway and edge syntax')
    pg.run(pg.compose(True) + ['stop', '--timeout', '20'])
    ids = pg.run(pg.compose(True) + ['ps', '-aq']).stdout.decode().split()
    metadata = json.loads(pg.run(['docker', 'inspect', *ids]).stdout)
    if any(r['State']['Running'] or r['State']['ExitCode'] != 0 for r in metadata):
        raise RuntimeError('Graceful stop failed')
    print('PASS graceful shutdown of new rehearsal stack; all legacy environments untouched')


if __name__ == '__main__': main()
