#!/usr/bin/env python3
"""Produce a deterministic read-only catch-up plan for the coding agent.

The ledger is fetched from a dedicated state branch by the trusted runner.
This command never writes a checkpoint, executes upstream code, or calls a model.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess

SHA = re.compile(r'^[0-9a-f]{40}$')


def git(repo, *args):
    return subprocess.run(['git', '-C', str(repo), *args], check=True,
                          capture_output=True, text=True).stdout.strip()


def plan(repo, target, ledger, inventory):
    target_sha = git(repo, 'rev-parse', '--verify', '--end-of-options', target + '^{commit}')
    if not SHA.fullmatch(target_sha):
        raise ValueError('Upstream commit must resolve to a SHA-1 commit')
    previous = ledger.get('last_successful_commit')
    if previous is not None and not SHA.fullmatch(previous):
        raise ValueError('Invalid ledger checkpoint; refusing to skip upstream history')
    fingerprint = hashlib.sha256(json.dumps(inventory, sort_keys=True).encode()).hexdigest()
    missing = [r['terraform_resource'] for r in inventory['resources']
               if not r['crossplane_configured'] or
               any(e['present_in_upstream'] is not True or e.get('retired_request_fields') for e in r['endpoints'])]
    # Runtime evidence is intentionally distinct from configuration inclusion.
    unverified = [r['terraform_resource'] for r in inventory['resources']
                  if r.get('acceptance_execution') != 'passed' or
                  r.get('crossplane_reconciliation') != 'passed']
    full_audit = previous is None
    changed = []
    if previous:
        result = subprocess.run(['git', '-C', str(repo), 'merge-base', '--is-ancestor', previous, target_sha], capture_output=True)
        full_audit = result.returncode != 0
        if not full_audit:
            changed = git(repo, 'diff', '--name-only', previous, target_sha, '--').splitlines()
    pending = ledger.get('pending_pr')
    if pending:
        if not re.fullmatch(r'https://github.com/archestra-ai/terraform-provider-archestra/pull/[1-9][0-9]*', pending):
            raise ValueError('Unexpected pending PR URL')
        action = 'inspect_pending_pr'
    elif full_audit or changed or missing or unverified or ledger.get('inventory_fingerprint') != fingerprint:
        action = 'reconcile'
    else:
        action = 'no_change'
    return {
        'action': action,
        'upstream_commit': target_sha,
        'last_successful_commit': previous,
        'full_audit_required': full_audit,
        'changed_paths': changed,
        'configuration_or_api_gaps': sorted(missing),
        'runtime_evidence_missing': sorted(unverified),
        'inventory_fingerprint': fingerprint,
        'pending_pr': pending,
        'branch': 'automation/provider-sync',
        'manual_review_required': True,
        'checkpoint_advanced': False,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--upstream', type=Path, required=True)
    parser.add_argument('--target', default='origin/main')
    parser.add_argument('--ledger', type=Path, required=True)
    parser.add_argument('--inventory', type=Path, required=True)
    args = parser.parse_args()
    result = plan(args.upstream, args.target, json.loads(args.ledger.read_text()), json.loads(args.inventory.read_text()))
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
