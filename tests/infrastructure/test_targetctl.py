"""Target lifecycle must stop before runtime mutation without owner approval."""
from pathlib import Path
import sys
import tempfile
import unittest
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


if __name__ == '__main__':
    unittest.main()
