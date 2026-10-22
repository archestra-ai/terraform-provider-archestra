"""The lifecycle job summary reports the outcome and records, and survives a missing evidence file."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('summary', Path(__file__).parent / 'integration/crossplane-lifecycle-summary.py')
summary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summary)


class LifecycleSummaryTests(unittest.TestCase):
    def test_reports_outcome_error_and_records(self):
        evidence = {'outcome': 'failed', 'cleanup': 'passed', 'api_version': '1.5.0-rc.3', 'error': 'Team drift was not repaired',
                    'results': [{'resource': 'Team', 'scope': 'cluster', 'create': 'passed', 'delete': 'soft-delete passed', 'execution': 'not_invoked'},
                                {'resource': 'Team', 'scope': 'namespaced', 'create': 'passed'}]}
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'evidence.json'
            path.write_text(json.dumps(evidence))
            text = summary.summary(path)
        self.assertIn('- Outcome: failed', text)
        self.assertIn('- Records: 2', text)
        self.assertIn('- Error: Team drift was not repaired', text)
        self.assertIn('| Team | cluster | 3 |', text)
        self.assertIn('| Team | namespaced | 1 |', text)

    def test_missing_evidence_is_reported_not_raised(self):
        with tempfile.TemporaryDirectory() as directory:
            text = summary.summary(Path(directory) / 'absent.json')
        self.assertIn('No evidence was written', text)


if __name__ == '__main__':
    unittest.main()
