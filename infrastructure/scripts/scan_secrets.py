"""Scan full Git history and the non-ignored working tree with redacted output."""
from pathlib import Path
import subprocess
import tempfile
import shutil
import sys

ROOT = Path(__file__).resolve().parents[2]
IMAGE = 'ghcr.io/gitleaks/gitleaks:v8.30.1@sha256:c00b6bd0aeb3071cbcb79009cb16a60dd9e0a7c60e2be9ab65d25e6bc8abbb7f'


def main():
    history = subprocess.run(['docker', 'run', '--rm', '--mount',
                              f'type=bind,source={ROOT},target=/repo,readonly', IMAGE,
                              'git', '/repo', '--config', '/repo/.gitleaks.toml',
                              '--redact', '--no-banner', '--log-opts=--all'])
    listing = subprocess.check_output(['git', '-c', f'safe.directory={ROOT.as_posix()}',
                                      'ls-files', '-z', '--cached', '--others', '--exclude-standard'],
                                     cwd=ROOT).decode().split('\0')
    with tempfile.TemporaryDirectory(prefix='ucm-secret-scan-') as tmp:
        snapshot = Path(tmp)
        count = 0
        for name in set(listing):
            if not name:
                continue
            source = ROOT / name
            if source.is_file() and not source.is_symlink():
                target = snapshot / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(source, target)
                count += 1
        tree = subprocess.run(['docker', 'run', '--rm', '--mount',
                               f'type=bind,source={snapshot},target=/scan,readonly', IMAGE,
                               'dir', '/scan', '--config', '/scan/.gitleaks.toml',
                               '--redact', '--no-banner'])
    print(f'Working-tree scan covered {count} tracked/non-ignored files; ignored secrets were not copied.')
    if history.returncode or tree.returncode:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
