"""Prevent independent services and SQL seeds drifting from Auth's catalogue."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
AUTH = ROOT / 'services/security-and-authentication'
CONSTANT = re.compile(r'^\s*(?:Perm\w*|Permission\w*)\s*=\s*"([a-z]+(?::[a-z]+)+)"', re.M)


def catalogue():
    source = (AUTH / 'internal/model.go').read_text()
    definitions = dict(re.findall(r'^\s*(Perm\w*)\s*=\s*"([^"]+)"', source, re.M))
    array = re.search(r'var AllPermissions = \[\]string\{([^}]+)\}', source).group(1)
    return {definitions[name] for name in re.findall(r'\bPerm\w+\b', array)}


class PermissionContract(unittest.TestCase):
    def test_migration_catalogue_matches_domain(self):
        seeded = set()
        for path in (AUTH / 'migrations').glob('*.up.sql'):
            for values in re.findall(r'INSERT INTO auth_permissions\s*\(code, description\)\s*VALUES\s*(.*?)(?:ON CONFLICT|;)', path.read_text(), re.S):
                seeded.update(re.findall(r"\('([^']+)',", values))
        self.assertTrue(seeded, 'No permission seed definitions found')
        self.assertEqual(seeded, catalogue())

    def test_service_permission_constants_are_registered(self):
        for name in ('security-and-authentication', 'user', 'course', 'enrollment', 'assessment', 'analytics'):
            required = set()
            for path in (ROOT / 'services' / name).rglob('*.go'):
                if not path.name.endswith('_test.go'):
                    required.update(CONSTANT.findall(path.read_text(encoding='utf-8')))
            self.assertTrue(required, name + ': expected real permission definitions')
            self.assertFalse(required - catalogue(), name + ': permissions absent from Auth catalogue')


if __name__ == '__main__': unittest.main()
