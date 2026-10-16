#!/usr/bin/env python3
"""Validate synchronization ledger transitions and publish with fast-forward CAS.

Run only from the pinned trusted publication phase. Verification input must come
from checked GitHub/CI results, never from a model's completion message.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess

BRANCH = 'refs/heads/automation/provider-sync-state'
SHA = re.compile(r'^[0-9a-f]{40}$')
PR = re.compile(r'^https://github.com/archestra-ai/terraform-provider-archestra/pull/[1-9][0-9]*$')


def require_sha(value):
    if not isinstance(value, str) or not SHA.fullmatch(value):
        raise ValueError('Expected an immutable SHA-1 commit')
    return value


def transition(previous, event):
    if previous.get('format_version') != 1:
        raise ValueError('Unsupported ledger version')
    if previous.get('last_successful_commit') is not None:
        require_sha(previous['last_successful_commit'])
    pending = previous.get('pending_pr')
    if pending is not None and not PR.fullmatch(pending):
        raise ValueError('Invalid pending PR')
    result = dict(previous)
    action = event.get('action')
    if action == 'pending':
        url = event.get('pr_url', '')
        if not PR.fullmatch(url) or (pending is not None and pending != url):
            raise ValueError('Refusing a second pending PR')
        result.update(pending_pr=url, pending_upstream_commit=require_sha(event.get('upstream_commit')))
    elif action == 'closed_unmerged':
        if not pending or event.get('pr_url') != pending or event.get('github_state') != 'closed' or event.get('merged') is not False:
            raise ValueError('Closure must be verified for the exact pending PR')
        result.update(pending_pr=None, pending_upstream_commit=None)
    elif action in ('merged', 'no_change'):
        target = require_sha(event.get('upstream_commit'))
        validation = event.get('validation', {})
        required = ('full_inventory', 'terraform_lifecycle', 'crossplane_both_scopes',
                    'state_and_crd_migrations', 'examples', 'packaging')
        if any(validation.get(check) is not True for check in required):
            raise ValueError('Incomplete validation; success checkpoint remains unchanged')
        evidence = event.get('evidence', [])
        if not evidence or any(not isinstance(e, str) or not e.startswith('https://github.com/archestra-ai/terraform-provider-archestra/') for e in evidence):
            raise ValueError('Durable repository evidence is required')
        fingerprint = event.get('inventory_fingerprint', '')
        if not re.fullmatch(r'[0-9a-f]{64}', fingerprint):
            raise ValueError('Invalid inventory fingerprint')
        if action == 'merged':
            if not pending or event.get('pr_url') != pending or event.get('github_state') != 'merged':
                raise ValueError('Pending PR must be verified merged')
            if previous.get('pending_upstream_commit') != target:
                raise ValueError('Merged PR does not cover the proposed checkpoint')
            require_sha(event.get('verified_head'))
            require_sha(event.get('merge_commit'))
            if event.get('required_checks_passed') is not True:
                raise ValueError('Exact-head required checks must pass')
        elif pending or event.get('changes_required') is not False:
            raise ValueError('No-change cannot bypass a pending PR or required changes')
        result.update(last_successful_commit=target, inventory_fingerprint=fingerprint,
                      pending_pr=None, pending_upstream_commit=None, evidence=evidence)
    else:
        raise ValueError('Unknown transition')
    return result


def git(repo, *args, data=None):
    return subprocess.run(['git', '-c', 'core.hooksPath=/dev/null', '-c', 'protocol.ext.allow=never', '-C', str(repo), *args], input=data, text=True,
                          capture_output=True, check=True).stdout.strip()


def remote_head(repo, remote):
    found = git(repo, 'ls-remote', '--refs', remote, BRANCH)
    return found.split()[0] if found else None


def publish(repo, remote, expected, ledger):
    """A concurrent push rejects this sibling commit; never force or retry blindly."""
    if expected is not None:
        require_sha(expected)
    if remote_head(repo, remote) != expected:
        raise ValueError('State branch moved; refetch and reconcile before publication')
    blob = git(repo, 'hash-object', '-w', '--stdin', data=json.dumps(ledger, indent=2, sort_keys=True) + '\n')
    tree = git(repo, 'mktree', data='100644 blob ' + blob + '\tstate.json\n')
    parent = ['-p', expected] if expected else []
    commit = git(repo, 'commit-tree', tree, *parent, data='Update reviewed provider synchronization state\n')
    git(repo, 'push', remote, commit + ':' + BRANCH)
    if remote_head(repo, remote) != commit:
        raise ValueError('Published head changed; inspect the winning state before proceeding')
    return commit


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ledger', type=Path, required=True)
    parser.add_argument('--event', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--publish-repo', type=Path)
    parser.add_argument('--remote', default='origin')
    parser.add_argument('--expected-head')
    args = parser.parse_args()
    ledger = transition(json.loads(args.ledger.read_text()), json.loads(args.event.read_text()))
    args.output.write_text(json.dumps(ledger, indent=2, sort_keys=True) + '\n')
    if args.publish_repo:
        print(publish(args.publish_repo, args.remote, args.expected_head, ledger))


if __name__ == '__main__':
    main()
