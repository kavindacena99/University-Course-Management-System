"""Semantic release boundaries against the retained real publication manifest."""
from copy import deepcopy
import json
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infrastructure/scripts'))
import release_pipeline


class ReleaseContract(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.path = ROOT / 'release-manifests/v0.1.0-rc.1/release-manifest.json'
        cls.manifest = json.loads(cls.path.read_text(encoding='utf-8'))

    def test_actual_candidate_matches_publication(self):
        source = release_pipeline.validate(self.manifest, self.path)
        self.assertEqual(len(source['images']), 9)

    def test_rc_number_and_channel_cannot_drift(self):
        for version, number in [('0.1.0-rc.2', 1), ('0.1.0-rc.1', 2)]:
            with self.subTest(version=version, number=number), self.assertRaises(ValueError):
                changed = deepcopy(self.manifest)
                changed['version'] = version
                changed['tag'] = 'v' + version
                changed['rc_number'] = number
                release_pipeline.validate(changed)

    def test_stable_requires_qa_and_same_published_source(self):
        stable = deepcopy(self.manifest)
        stable.update(version='0.1.0', tag='v0.1.0', channel='stable', rc_number=None)
        with self.assertRaises(ValueError):
            release_pipeline.validate(stable)
        stable['qa_validation'] = {'status': 'passed', 'reference': 'qa-report-example'}
        with self.assertRaises(ValueError):
            release_pipeline.validate(stable)
        stable['source_commit'] = 'f' * 40
        with self.assertRaises(ValueError):
            release_pipeline.validate(stable)

    def test_image_or_publication_identity_cannot_change(self):
        for field in ('digest', 'runtime_digest'):
            with self.subTest(field=field), self.assertRaises(ValueError):
                changed = deepcopy(self.manifest)
                changed['images'][0][field] = 'sha256:' + '0' * 64
                release_pipeline.validate(changed)

    def test_existing_candidate_cannot_be_overwritten(self):
        with self.assertRaises(FileExistsError):
            release_pipeline.create_rc(
                '0.1.0-rc.1',
                ROOT / self.manifest['publication']['image_manifest_path'],
                self.manifest['publication']['manifest_artifact_url'],
                self.manifest['publication']['evidence_artifact_url'],
            )


if __name__ == '__main__':
    unittest.main()
