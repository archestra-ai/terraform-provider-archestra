import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('sync_plan', Path(__file__).with_name('provider-sync-plan.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SyncTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git('init', '-q')
        self.git('config', 'user.name', 'Test')
        self.git('config', 'user.email', 'test@example.invalid')
        self.first = self.commit('first')
        self.inventory = {'resources': [{'terraform_resource': 'archestra_agent', 'crossplane_configured': True,
                                        'endpoints': [{'present_in_upstream': True}],
                                        'acceptance_execution': 'passed', 'crossplane_reconciliation': 'passed'}]}

    def git(self, *args):
        return module.git(self.root, *args)

    def commit(self, value):
        (self.root / 'api.json').write_text(value)
        self.git('add', 'api.json')
        self.git('commit', '-qm', value)
        return self.git('rev-parse', 'HEAD')

    def test_first_run_audits_everything_and_never_advances_checkpoint(self):
        result = module.plan(self.root, 'HEAD', {}, self.inventory)
        self.assertTrue(result['full_audit_required'])
        self.assertFalse(result['checkpoint_advanced'])
        self.assertEqual(result['action'], 'reconcile')

    def test_missed_days_compare_against_last_success_not_yesterday(self):
        self.commit('second')
        self.commit('third')
        result = module.plan(self.root, 'HEAD', {'last_successful_commit': self.first}, self.inventory)
        self.assertEqual(result['changed_paths'], ['api.json'])
        self.assertEqual(result['last_successful_commit'], self.first)

    def test_no_change_needs_same_inventory_and_verified_runtime(self):
        initial = module.plan(self.root, 'HEAD', {}, self.inventory)
        ledger = {'last_successful_commit': self.first, 'inventory_fingerprint': initial['inventory_fingerprint']}
        self.assertEqual(module.plan(self.root, 'HEAD', ledger, self.inventory)['action'], 'no_change')
        self.inventory['resources'][0]['crossplane_reconciliation'] = 'unverified'
        self.assertEqual(module.plan(self.root, 'HEAD', ledger, self.inventory)['action'], 'reconcile')

    def test_pending_pr_prevents_duplicate_creation(self):
        result = module.plan(self.root, 'HEAD', {'pending_pr': 'https://github.com/archestra-ai/terraform-provider-archestra/pull/1'}, self.inventory)
        self.assertEqual(result['action'], 'inspect_pending_pr')
        self.assertTrue(result['manual_review_required'])

    def test_unknown_checkpoint_recovers_with_full_audit(self):
        result = module.plan(self.root, 'HEAD', {'last_successful_commit': 'f' * 40}, self.inventory)
        self.assertTrue(result['full_audit_required'])

    def test_rejects_ledger_injection(self):
        with self.assertRaises(ValueError):
            module.plan(self.root, 'HEAD', {'last_successful_commit': '--help'}, self.inventory)
        with self.assertRaises(ValueError):
            module.plan(self.root, 'HEAD', {'pending_pr': 'https://evil.example/pull/1'}, self.inventory)


if __name__ == '__main__':
    unittest.main()
