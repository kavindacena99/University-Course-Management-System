import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parents[2] / 'infrastructure/scripts'
sys.path.insert(0, str(SCRIPTS))
import backup


class BackupIntegrity(unittest.TestCase):
    def fixture(self, root):
        files = {}
        for service in backup.local.SERVICES:
            path = root / f'{service}.dump'
            path.write_bytes(b'synthetic integrity fixture')
            files[path.name] = {'sha256': backup.digest(path), 'bytes': path.stat().st_size}
        return {'files': files}

    def test_valid_checksums_pass(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'manifest.json').write_text(json.dumps(self.fixture(root)))
            backup.verify(root)

    def test_changed_backup_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'manifest.json').write_text(json.dumps(self.fixture(root)))
            (root / 'auth.dump').write_bytes(b'damaged')
            with self.assertRaisesRegex(RuntimeError, 'checksum/size'):
                backup.verify(root)

    def test_manifest_cannot_read_outside_backup(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            manifest = self.fixture(root)
            manifest['files']['../outside'] = {'sha256': '', 'bytes': 1}
            (root / 'manifest.json').write_text(json.dumps(manifest))
            with self.assertRaisesRegex(RuntimeError, 'Unsafe backup'):
                backup.verify(root)


if __name__ == '__main__':
    unittest.main()
