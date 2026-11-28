"""Target lifecycle must stop before runtime mutation without owner approval."""
from pathlib import Path
import os
import sys
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infrastructure/scripts'))
import targetctl


class TargetSafety(unittest.TestCase):
    def test_qa_and_prodlike_are_distinct(self):
        self.assertNotEqual(targetctl.TARGETS['qa'], targetctl.TARGETS['prodlike'])
        self.assertEqual(targetctl.TARGETS['qa'], ('platform-qa-target', 3001, 8081))
        self.assertEqual(targetctl.TARGETS['prodlike'], ('platform-prodlike-target', 3002, 8082))

    def test_missing_runtime_approval_blocks_deploy_before_pull(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(targetctl, 'ROOT', Path(directory)):
            with patch.object(targetctl, 'preflight', return_value=({'tag': 'v0.1.0-rc.1'}, {'volumes': []})), \
                 patch.object(targetctl, 'pull') as pull, \
                 self.assertRaisesRegex(RuntimeError, 'approval is absent'):
                targetctl.deploy('qa', 'candidate.json')
            pull.assert_not_called()

    def test_missing_previous_release_refuses_rollback(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(targetctl, 'ROOT', Path(directory)):
            with patch.object(targetctl, 'release', return_value={'tag': 'v0.1.0-rc.1'}), \
                 self.assertRaisesRegex(RuntimeError, 'Previous deployment'):
                targetctl.rollback('qa', 'candidate.json')

    def test_frontend_config_sync_preserves_original(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(targetctl, 'ROOT', Path(directory)):
            root = Path(directory)
            state = root / '.infra-state/phase123/targets/qa'
            state.mkdir(parents=True)
            (state / 'rehearsal.env').write_text('PG_PROJECT=platform-qa-target\n')
            (state / 'frontend.conf').write_bytes(b'old frontend config')
            source = root / 'frontend/nginx.conf'
            source.parent.mkdir()
            source.write_bytes(b'new same-origin proxy config')
            with patch.object(targetctl, 'engine'), \
                 patch.object(targetctl, 'run', return_value=SimpleNamespace(stdout='')):
                targetctl.sync_config('qa')
            self.assertEqual((state / 'frontend.conf').read_bytes(), source.read_bytes())
            backups = list((root / '.infra-state/releases/config-backups/qa').glob('frontend-*.conf'))
            self.assertEqual(len(backups), 1)
            self.assertEqual(backups[0].read_bytes(), b'old frontend config')

    def test_empty_submissions_bucket_still_produces_verified_backup(self):
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryDirectory() as external:
            root = Path(directory)
            (root / '.infra-state/releases/v0.1.0-rc.2').mkdir(parents=True)
            rows = [{'Service': service, 'State': 'running'}
                    for service in ('postgres', 'minio', 'storage-guard')]

            def docker(args, **kwargs):
                if 'test' in args and '-d' in args:
                    return SimpleNamespace(returncode=1, stdout='')
                if args[:2] == ['docker', 'cp']:
                    self.fail('Empty bucket must not invoke docker cp')
                return SimpleNamespace(returncode=0, stdout='')

            with patch.object(targetctl, 'ROOT', root), \
                 patch.object(targetctl, 'preflight', return_value=({'tag': 'v0.1.0-rc.2'}, {})), \
                 patch.object(targetctl, 'approval'), \
                 patch.object(targetctl, 'ps', return_value=rows), \
                 patch.object(targetctl, 'read_env', return_value={
                     'ASSESSMENT_OBJECT_USER': 'fixture', 'ASSESSMENT_OBJECT_PASSWORD': 'fixture'}), \
                 patch.object(targetctl, 'run', side_effect=docker), \
                 patch('subprocess.run', return_value=SimpleNamespace(returncode=0, stdout=b'archive')), \
                 patch('backup.restrict_directory'), \
                 patch.dict(os.environ, {'LOCALAPPDATA': external}):
                targetctl.backup('qa', 'candidate.json')
                targetctl.verified_backup('qa', 'v0.1.0-rc.2')
            marker = root / '.infra-state/releases/v0.1.0-rc.2/qa-backup-verified.json'
            self.assertTrue(marker.is_file())


if __name__ == '__main__':
    unittest.main()
