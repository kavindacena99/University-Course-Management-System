"""Safety gates for target selection and immutable image promotion."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    'ucm_platform', Path(__file__).resolve().parents[2] / 'infrastructure/scripts/platformctl.py')
platform = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(platform)


class DeploymentSafety(unittest.TestCase):
    def test_dev_credentials_cannot_be_regenerated(self):
        with self.assertRaisesRegex(RuntimeError, 'never regenerate'):
            platform.initialize('dev')

    def test_existing_environment_cannot_be_overwritten(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(platform, 'ROOT', Path(tmp)):
            path = platform.env_path('qa')
            path.parent.mkdir(parents=True)
            path.write_text('DO_NOT_REPLACE=this-file\n')
            with self.assertRaisesRegex(RuntimeError, 'already initialized'):
                platform.initialize('qa')
            self.assertEqual(path.read_text(), 'DO_NOT_REPLACE=this-file\n')

    def test_mutable_release_is_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'release.env'
            path.write_text('AUTH_IMAGE=ghcr.io/owner/repo/auth:latest\n')
            with self.assertRaisesRegex(RuntimeError, 'pinned by sha256'):
                platform.validate_release(path)

    def test_release_cannot_override_environment_or_credentials(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'release.env'
            data = ''.join(f'{s.upper().replace("-", "_")}_IMAGE=ghcr.io/org/repo/{s}@sha256:{"a" * 64}\n'
                           for s in platform.IMAGES)
            path.write_text(data)
            self.assertEqual(platform.validate_release(path), path)
            path.write_text(data + 'COMPOSE_PROJECT_NAME=platform-dev\n')
            with self.assertRaises(RuntimeError):
                platform.validate_release(path)

    def test_cross_environment_project_is_refused(self):
        with self.assertRaisesRegex(RuntimeError, 'identity mismatch'):
            platform.verify_model('qa', {'name': 'platform-dev'})

    def test_external_persistent_volume_is_refused(self):
        with self.assertRaisesRegex(RuntimeError, 'Shared or external'):
            platform.verify_model('dev', {'name': 'platform-dev', 'networks': {},
                                         'volumes': {'db': {'external': True}}})

    def test_private_backend_publication_is_refused(self):
        with self.assertRaisesRegex(RuntimeError, 'Private service'):
            platform.verify_model('dev', {'name': 'platform-dev', 'networks': {}, 'volumes': {},
                                         'services': {'auth': {'ports': [{'published': '8081'}]}}})

    def test_deployment_lock_excludes_overlap_and_releases_after_failure(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(platform, 'ROOT', Path(tmp)):
            with self.assertRaisesRegex(RuntimeError, 'synthetic failure'):
                with platform.deployment_lock('qa'):
                    with self.assertRaisesRegex(RuntimeError, 'lock exists'):
                        with platform.deployment_lock('qa'):
                            self.fail('Overlapping deployment acquired the same target')
                    raise RuntimeError('synthetic failure')
            self.assertFalse((Path(tmp) / '.infra-state/qa/deploy.lock').exists())


if __name__ == '__main__':
    unittest.main()
