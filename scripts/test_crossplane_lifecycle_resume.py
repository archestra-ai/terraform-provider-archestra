"""Integrity checks for carrying completed runtime evidence across a fixture outage."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('lifecycle', Path(__file__).parent / 'integration/crossplane-lifecycle.py')
lifecycle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lifecycle)


class ResumeEvidenceTests(unittest.TestCase):
    def test_different_binary_context_or_api_cannot_reuse_results(self):
        proof = {'terraform_sha256': 'native', 'controller_sha256': 'controller',
                 'kubernetes_context': 'kind-private', 'api_url': 'http://127.0.0.1:19000',
                 'api_spec_sha256': 'snapshot', 'cleanup': 'passed', 'results': []}
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'proof.json'
            path.write_text(json.dumps(proof))
            for field in ('terraform_sha256', 'controller_sha256', 'kubernetes_context', 'api_url', 'api_spec_sha256'):
                with self.subTest(field=field), self.assertRaises(ValueError):
                    lifecycle.resume_results(path, {**proof, field: 'different'})
            self.assertEqual(lifecycle.resume_results(path, proof), [])
            path.write_text(json.dumps({**proof, 'cleanup': 'blocked'}))
            with self.assertRaises(ValueError):
                lifecycle.resume_results(path, proof)

    def test_partial_group_does_not_skip_remaining_resource_or_other_scope(self):
        records = [{'resource': 'ProviderKey', 'scope': 'cluster'}]
        self.assertNotIn('llm', lifecycle.completed_cases(records, 'cluster'))
        records.append({'resource': 'ModelOverride', 'scope': 'cluster'})
        self.assertIn('llm', lifecycle.completed_cases(records, 'cluster'))
        self.assertNotIn('llm', lifecycle.completed_cases(records, 'namespaced'))

    def test_default_policy_variants_must_all_be_proven(self):
        records = [{'resource': 'RuntimeCredentialInitialGrantDefaults', 'scope': 'cluster',
                    'parameter_source': source} for source in ('default', 'empty')]
        self.assertNotIn('initial-grant-defaults', lifecycle.completed_cases(records, 'cluster'))
        records.append({'resource': 'RuntimeCredentialInitialGrantDefaults', 'scope': 'cluster',
                        'parameter_source': 'init-empty'})
        self.assertIn('initial-grant-defaults', lifecycle.completed_cases(records, 'cluster'))


if __name__ == '__main__':
    unittest.main()
