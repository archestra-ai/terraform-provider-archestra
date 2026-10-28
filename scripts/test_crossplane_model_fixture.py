"""Model-provider fixture secrets must use the generated Crossplane selectors."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('lifecycle', Path(__file__).parent / 'integration/crossplane-lifecycle.py')
lifecycle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lifecycle)


class ModelFixtureTests(unittest.TestCase):
    def test_both_scopes_use_secret_references_and_owned_cleanup_identity(self):
        for scope in ('cluster', 'namespaced'):
            with self.subTest(scope=scope):
                secret, fields = lifecycle.model_provider_fixture('crossplane-system', scope, 'fixture-run')
                self.assertEqual(set(fields), {'apiKeySecretRef'})
                ref = fields['apiKeySecretRef']
                self.assertEqual(ref['name'], secret['metadata']['name'])
                self.assertEqual(secret['stringData'][ref['key']], 'synthetic-local-only')
                self.assertEqual(secret['metadata']['namespace'], 'crossplane-system')
                self.assertEqual(secret['metadata']['labels'], {'provider-test-run': 'fixture-run'})
                self.assertEqual(ref.get('namespace'), 'crossplane-system' if scope == 'cluster' else None)


if __name__ == '__main__':
    unittest.main()
