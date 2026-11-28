import sys
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infrastructure/scripts'))
from image_inputs import changed


class ImageInputContract(unittest.TestCase):
    def test_application_sources_require_new_images(self):
        for path in ('services/auth/main.go', 'frontend/src/App.tsx',
                     'infrastructure/nginx/Dockerfile', 'infrastructure/postgres/Dockerfile'):
            with self.subTest(path=path):
                self.assertTrue(changed([path]))

    def test_release_metadata_does_not_rebuild_images(self):
        self.assertFalse(changed(['docs/infrastructure/release-management.md',
                                  'release-manifests/v0.1.0-rc.1/release-manifest.json',
                                  'infrastructure/scripts/targetctl.py',
                                  '.github/workflows/ci.yml']))


if __name__ == '__main__':
    unittest.main()
