"""Generate NEW inactive local target inputs. This command cannot deploy/cut over."""
import argparse
import json
from pathlib import Path
import postgres_rehearsal as pg


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--environment', required=True, choices=pg.platform.ENVIRONMENTS)
    p.add_argument('--tag', default='phase123', help='Locally built application image tag; PostgreSQL pin is independent')
    a = p.parse_args()
    env = a.environment
    pg.STATE = pg.ROOT / '.infra-state/phase123/targets' / env
    pg.PROJECT = 'platform-' + env + '-target'
    pg.init()  # Exclusive creation, never replaces any legacy/partially existing input.
    path = pg.STATE / 'rehearsal.env'
    values = pg.platform.read_env(path)
    front, gateway = pg.platform.ENVIRONMENTS[env][2:]
    values.update({'TARGET_APP_ENV': 'production' if env == 'prodlike' else ('development' if env == 'dev' else 'qa'),
                   'FRONTEND_PORT': str(front), 'GATEWAY_PORT': str(gateway),
                   'PUBLIC_BASE_URL': f'http://localhost:{gateway}'})
    for key, image in list(values.items()):
        if key.endswith('_IMAGE') and image.startswith('ucm-'):
            image = image.replace(':phase345', ':phase123')
            if key != 'POSTGRES_IMAGE': image = image.rsplit(':', 1)[0] + ':' + a.tag
            metadata = json.loads(pg.run(['docker', 'image', 'inspect', image]).stdout)[0]
            if not metadata.get('RepoDigests'): raise RuntimeError('Build local images before preparing targets')
            values[key] = metadata['RepoDigests'][0]
    path.write_text(''.join(f'{k}={v}\n' for k,v in values.items()))
    (pg.STATE / 'frontend.conf').write_text((pg.ROOT / 'frontend/nginx.conf').read_text().replace('http://localhost:8080', values['PUBLIC_BASE_URL']))
    for s in pg.DATABASES:
        file = pg.STATE / (s + '.env')
        e = pg.platform.read_env(file)
        e['CORS_ALLOWED_ORIGINS'] = f'http://localhost:{front}'
        file.write_text(''.join(f'{k}={v}\n' for k,v in e.items()))
    print('Prepared inactive target inputs:', env, '; no container or volume operation')


if __name__ == '__main__': main()
