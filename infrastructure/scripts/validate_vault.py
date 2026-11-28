"""Run inside the trusted Ansible control container; synthetic data only."""
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def main():
    with tempfile.TemporaryDirectory(prefix='ucm-vault-test-') as directory:
        tmp = Path(directory)
        tmp.chmod(0o700)
        password = tmp / 'password'
        password.write_text(secrets.token_hex(32))
        password.chmod(0o600)
        inventory = tmp / 'inventories/qa'
        (inventory / 'group_vars').mkdir(parents=True)
        source = ROOT / 'infrastructure/ansible/inventories/qa'
        shutil.copyfile(source / 'hosts.yml', inventory / 'hosts.yml')
        shutil.copyfile(source / 'group_vars/all.yml', inventory / 'group_vars/all.yml')
        vault = inventory / 'vault.yml'
        data = 'vault_environment_files: []\nvault_validation_marker: synthetic-only\n'
        vault.write_text(data)
        vault.chmod(0o600)
        subprocess.run(['ansible-vault', 'encrypt', '--vault-password-file', str(password), str(vault)],
                       check=True, capture_output=True)
        assert vault.read_text().startswith('$ANSIBLE_VAULT;')
        plaintext = subprocess.check_output(['ansible-vault', 'view', '--vault-password-file',
                                            str(password), str(vault)], text=True)
        assert plaintext == data
        assert password.stat().st_mode & 0o777 == 0o600
        result = subprocess.run(['ansible-playbook', '-i', str(inventory / 'hosts.yml'),
                                 'playbooks/validate.yml', '--vault-password-file', str(password)],
                                cwd=ROOT / 'infrastructure/ansible', text=True, capture_output=True)
        assert result.returncode == 0, 'Vault-enabled validation playbook failed'
        assert 'synthetic-only' not in result.stdout + result.stderr
        print('PASS Vault encrypt/decrypt, restrictive test permissions, and encrypted-variable role execution')


if __name__ == '__main__':
    main()
