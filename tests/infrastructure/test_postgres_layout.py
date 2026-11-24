import copy
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'infrastructure/scripts'))
from ci_models import validate_target
import postgres_rehearsal as pg


class ConsolidatedLayout(unittest.TestCase):
    def fixture(self):
        services = {'postgres': {}}
        for s, db in pg.DATABASES.items():
            services[s] = {'environment': {'DB_HOST': 'postgres', 'DB_NAME': db,
                           'DB_USER': s + '_app', 'DB_PASSWORD': 'runtime-fixture'},
                           'depends_on': {'db-permissions': {}}}
            services[s + '-migrate'] = {'environment': {'DB_HOST': 'postgres', 'DB_NAME': db,
                           'DB_USER': s + '_migrator', 'DB_PASSWORD': 'migration-fixture'}}
        return {'name': 'test-target', 'services': services,
                'networks': {'data': {'name': 'test-target_data'}},
                'volumes': {'postgres-data': {'name': 'test-target_postgres-data'}}}

    def test_valid_split_roles(self):
        validate_target(self.fixture(), 'test-target')

    def test_private_database_port(self):
        c = self.fixture(); c['services']['postgres']['ports'] = [{'published': '5432'}]
        with self.assertRaisesRegex(RuntimeError, 'publishes'):
            validate_target(c, 'test-target')

    def test_legacy_database_cannot_join_target(self):
        c = self.fixture(); c['services']['auth-db'] = {}
        with self.assertRaisesRegex(RuntimeError, 'one PostgreSQL'):
            validate_target(c, 'test-target')

    def test_legacy_volume_cannot_be_reused(self):
        c = self.fixture(); c['volumes']['postgres-data']['name'] = 'platform-qa_auth-db-data'
        with self.assertRaisesRegex(RuntimeError, 'Shared'):
            validate_target(c, 'test-target')

    def test_runtime_cannot_use_migration_secret(self):
        c = self.fixture(); c['services']['auth']['environment']['DB_PASSWORD'] = 'migration-fixture'
        with self.assertRaisesRegex(RuntimeError, 'Credential'):
            validate_target(c, 'test-target')

    def test_application_cannot_bypass_permissions(self):
        c = self.fixture(); c['services']['auth']['depends_on'] = {}
        with self.assertRaisesRegex(RuntimeError, 'barrier'):
            validate_target(c, 'test-target')

    def test_missing_secret_reference_is_rejected(self):
        c = self.fixture(); c['secrets'] = {'admin': {'file': '/this-secret-does-not-exist'}}
        with self.assertRaisesRegex(RuntimeError, 'secret input'):
            validate_target(c, 'test-target')


if __name__ == '__main__':
    unittest.main()
