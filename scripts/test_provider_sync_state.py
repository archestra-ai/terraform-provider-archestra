import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('state', Path(__file__).with_name('provider-sync-state.py'))
state = importlib.util.module_from_spec(spec)
spec.loader.exec_module(state)
SHA = 'a' * 40
URL = 'https://github.com/archestra-ai/terraform-provider-archestra/pull/123'
INITIAL = dict(format_version=1, last_successful_commit=None, pending_pr=None, inventory_fingerprint=None)


def verified(action='no_change'):
    return dict(action=action, upstream_commit=SHA, changes_required=False,
                validation={k: True for k in ('full_inventory', 'terraform_lifecycle', 'crossplane_both_scopes', 'state_and_crd_migrations', 'examples', 'packaging')},
                inventory_fingerprint='b' * 64, evidence=[URL], pr_url=URL,
                github_state='merged', verified_head='c' * 40, merge_commit='d' * 40, required_checks_passed=True)


class Transitions(unittest.TestCase):
    def pending(self):
        return state.transition(INITIAL, dict(action='pending', pr_url=URL, upstream_commit=SHA))

    def test_pending_does_not_advance(self):
        self.assertIsNone(self.pending()['last_successful_commit'])

    def test_duplicate_reuses_pending(self):
        first = self.pending()
        self.assertEqual(first, state.transition(first, dict(action='pending', pr_url=URL, upstream_commit=SHA)))
        with self.assertRaises(ValueError):
            state.transition(first, dict(action='pending', pr_url=URL+'4', upstream_commit=SHA))

    def test_failure_cannot_advance(self):
        for check in verified()['validation']:
            event = verified()
            event['validation'][check] = False
            with self.assertRaises(ValueError):
                state.transition(INITIAL, event)

    def test_no_change(self):
        self.assertEqual(state.transition(INITIAL, verified())['last_successful_commit'], SHA)
        with self.assertRaises(ValueError):
            state.transition(self.pending(), verified())

    def test_merged_exact_pending(self):
        self.assertEqual(state.transition(self.pending(), verified('merged'))['last_successful_commit'], SHA)
        for key, value in [('upstream_commit', 'e'*40), ('github_state', 'open'), ('required_checks_passed', False)]:
            event = verified('merged'); event[key] = value
            with self.assertRaises(ValueError):
                state.transition(self.pending(), event)

    def test_closed_unmerged_preserves_checkpoint(self):
        result = state.transition(self.pending(), dict(action='closed_unmerged', pr_url=URL, github_state='closed', merged=False))
        self.assertIsNone(result['last_successful_commit'])
        self.assertIsNone(result['pending_pr'])

    def test_untrusted_targets(self):
        for url in ['https://evil.test/123', URL+';echo x']:
            with self.assertRaises(ValueError):
                state.transition(INITIAL, dict(action='pending', pr_url=url, upstream_commit=SHA))
        event = verified(); event['evidence'] = []
        with self.assertRaises(ValueError):state.transition(INITIAL, event)

    def test_git_publication_cas_and_crash_recovery(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); remote=root/'state.git'; repo=root/'runner'
            subprocess.run(['git','init','--bare',str(remote)], check=True, capture_output=True)
            subprocess.run(['git','init',str(repo)], check=True, capture_output=True)
            for key,value in [('user.name','Sync test'),('user.email','sync@example.invalid')]:
                state.git(repo,'config',key,value)
            hook=repo/'.git/hooks/pre-push'; hook.write_text('#!/bin/sh\nexit 99\n'); hook.chmod(0o700)
            # Publication must not execute hooks from a potentially modified checkout.
            first=state.publish(repo,str(remote),None,INITIAL)
            pending=state.publish(repo,str(remote),first,self.pending())
            self.assertEqual(state.remote_head(repo,str(remote)),pending)
            # A stale runner cannot overwrite a successful concurrent publication.
            with self.assertRaises(ValueError):state.publish(repo,str(remote),first,INITIAL)
            body=state.git(repo,'show',pending+':state.json')
            self.assertEqual(json.loads(body)['pending_pr'],URL)
            completed=state.publish(repo,str(remote),pending,state.transition(self.pending(),verified('merged')))
            self.assertEqual(json.loads(state.git(repo,'show',completed+':state.json'))['last_successful_commit'],SHA)
