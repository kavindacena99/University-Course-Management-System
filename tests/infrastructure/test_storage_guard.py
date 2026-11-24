import copy
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'infrastructure/scripts'))
from storage_rehearsal import validate_model


class StorageBoundary(unittest.TestCase):
    def fixture(self):
        return {'name': 'storage-test', 'services': {
            'minio': {'command': ['server', '--address', '127.0.0.1:19000', '--console-address', '127.0.0.1:19001', '/data']},
            'storage-guard': {'network_mode': 'service:minio'},
            'assessment-minio-init': {'depends_on': {'storage-guard': {}}}},
            'volumes': {'object-data': {'name': 'storage-test_object-data'}},
            'networks': {'data': {'name': 'storage-test_data'}}}

    def test_valid_private_boundary(self):
        validate_model(self.fixture(), 'storage-test')

    def test_public_port_is_rejected(self):
        model = self.fixture(); model['services']['minio']['ports'] = [{'published':'9000'}]
        with self.assertRaisesRegex(RuntimeError, 'publish'):
            validate_model(model, 'storage-test')

    def test_direct_backend_listener_is_rejected(self):
        model = self.fixture(); model['services']['minio']['command'][2] = ':19000'
        with self.assertRaisesRegex(RuntimeError, 'Unguarded'):
            validate_model(model, 'storage-test')

    def test_guard_namespace_bypass_is_rejected(self):
        model = self.fixture(); model['services']['storage-guard']['network_mode'] = 'host'
        with self.assertRaisesRegex(RuntimeError, 'namespace'):
            validate_model(model, 'storage-test')


if __name__ == '__main__': unittest.main()
