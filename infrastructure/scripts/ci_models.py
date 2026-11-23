"""Validate fresh-checkout models using disposable local-only inputs, never DEV files."""
import json
import os
from pathlib import Path
import subprocess
import sys

import platformctl as local


def main():
    for env in ('qa', 'prodlike'):
        if not local.env_path(env).exists():
            local.initialize(env)
        local.verify_model(env, local.model(env))
    variables = os.environ.copy()
    for service, directory in local.SERVICES.items():
        variables[f'{service.upper()}_ENV_FILE'] = str(
            local.ROOT / 'services' / directory / '.env.example')
    result = subprocess.run(['docker', 'compose', '--env-file',
                             'infrastructure/environments/dev/.env.example',
                             '-p', 'platform-dev', '-f', 'infrastructure/docker/compose.yaml',
                             '-f', 'infrastructure/docker/compose.dev.yaml',
                             'config', '--format', 'json'], cwd=local.ROOT,
                            env=variables, capture_output=True, text=True, check=True)
    local.verify_model('dev', json.loads(result.stdout))
    print('PASS fresh-checkout Compose models; DEV input files were not created or changed')


if __name__ == '__main__':
    main()
