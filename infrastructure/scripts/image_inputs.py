"""Report whether a GitHub event changed application image build inputs."""
import os
from pathlib import Path
import subprocess

PREFIXES = ('services/', 'frontend/', 'infrastructure/nginx/', 'infrastructure/postgres/')


def changed(paths):
    return any(path.replace('\\', '/').startswith(PREFIXES) for path in paths)


def main():
    event = os.environ.get('GITHUB_EVENT_NAME')
    if event != 'push':
        result = True
    else:
        before = os.environ.get('GITHUB_EVENT_BEFORE', '')
        after = os.environ.get('GITHUB_SHA', '')
        if len(before) != 40 or before == '0' * 40 or len(after) != 40:
            result = True
        else:
            paths = subprocess.check_output(['git', 'diff', '--name-only', before, after], text=True).splitlines()
            result = changed(paths)
    with Path(os.environ['GITHUB_OUTPUT']).open('a', encoding='utf-8') as stream:
        stream.write('changed=' + str(result).lower() + '\n')
    print('Application image inputs changed:', result)


if __name__ == '__main__':
    main()
