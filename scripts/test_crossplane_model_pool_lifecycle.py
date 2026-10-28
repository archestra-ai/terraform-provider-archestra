"""Failure-path coverage for the actual ModelPool lifecycle probe."""
import ast
import copy
import json
from pathlib import Path
import unittest


POOL_ID = '11111111-1111-4111-8111-111111111111'
WRONG_ID = '33333333-3333-4333-8333-333333333333'
PROVIDER_ID = '22222222-2222-4222-8222-222222222222'


class Probe:
    def __init__(self, *, wrong_adoption=False, orphan_delete_failure=False, noop_drift=False):
        self.leases = []
        self.remote = {}
        self.identities = {}
        self.desired = {}
        self.orphans = set()
        self.created = 0
        self.wrong_adoption = wrong_adoption
        self.orphan_delete_failure = orphan_delete_failure
        self.noop_drift = noop_drift
        self.repair_waits = 0
        tree = ast.parse((Path(__file__).parent / 'integration/crossplane-lifecycle.py').read_text())
        definition = next(node for node in ast.walk(tree) if isinstance(node, ast.FunctionDef) and node.name == 'model_pool_resource')
        namespace = {'json': json, 'run': 'fixture', 'leases': self.leases,
                     **{name: getattr(self, name) for name in ('create', 'api', 'observation', 'wait', 'patch', 'kube', 'delete', 'record')}}
        exec(compile(ast.Module(body=[definition], type_ignores=[]), '<actual lifecycle probe>', 'exec'), namespace)
        self.probe = namespace['model_pool_resource']

    def row(self, fields, identifier):
        return {'id': identifier, 'name': fields['name'], 'virtualApiKeyIds': [], 'members': [
            {**{k: v for k, v in member.items() if k != 'providerApiKeyIdRef'}, 'providerApiKeyId': PROVIDER_ID}
            for member in fields['members']]}

    def create(self, scope, group, kind, plural, fields, suffix='', **kwargs):
        self.created += 1
        identifier = WRONG_ID if self.created > 1 and self.wrong_adoption else POOL_ID
        lease = ((), 'pools', 'pool' + suffix)
        self.leases.append(lease)
        self.identities[lease] = identifier
        self.desired[lease] = copy.deepcopy(fields)
        self.remote[identifier] = self.row(fields, identifier)
        return lease, identifier, {}

    def api(self, path, payload=None, method=None):
        if payload is None:
            return 200, copy.deepcopy(list(self.remote.values()))
        if not self.noop_drift:
            self.remote[POOL_ID] = dict(copy.deepcopy(payload), id=POOL_ID, virtualApiKeyIds=[])
        return 200, copy.deepcopy(self.remote[POOL_ID])

    def observation(self, lease):
        return {'metadata': {'annotations': {'crossplane.io/external-name': self.identities[lease]}},
                'status': {'conditions': [{'type': kind, 'status': 'True'} for kind in ('Ready', 'Synced')]}}

    def wait(self, description, check):
        if 'automatic' in description:
            self.repair_waits += 1
            lease = next(iter(self.desired))
            self.remote[POOL_ID] = self.row(self.desired[lease], POOL_ID)
        if not check():
            raise RuntimeError(description)

    def patch(self, lease, fields):
        self.desired[lease] = copy.deepcopy(fields)
        self.remote[self.identities[lease]] = self.row(fields, self.identities[lease])

    def kube(self, *args):
        self.orphans.add(next(lease for lease in self.leases if lease[1] == 'pools'))

    def delete(self, lease):
        if lease[1] != 'api-fixture' and lease in self.orphans and self.orphan_delete_failure:
            self.orphan_delete_failure = False
            raise RuntimeError('orphan CR deletion timed out')
        if lease[1] == 'api-fixture':
            # An API fallback must never run while an owning CR can recreate it.
            if any(item in self.leases for item in self.orphans):
                raise AssertionError('API fallback ran before the owning CR was removed')
            self.remote.pop(POOL_ID, None)
        elif lease not in self.orphans:
            self.remote.pop(self.identities[lease], None)
        self.leases.remove(lease)

    def record(self, *args, **kwargs):
        pass

    def run(self):
        self.probe('cluster', ((), 'provider', 'fixture'), PROVIDER_ID)

    def cleanup(self):
        for lease in reversed(self.leases.copy()):
            self.delete(lease)


class ModelPoolProbeTests(unittest.TestCase):
    def test_normal_lifecycle_leaves_no_api_fallback(self):
        fixture = Probe()
        fixture.run()
        self.assertEqual(fixture.remote, {})
        self.assertEqual(fixture.leases, [])

    def test_wrong_adoption_retains_original_and_new_pool_for_cleanup(self):
        fixture = Probe(wrong_adoption=True)
        with self.assertRaisesRegex(RuntimeError, 'adoption changed identity'):
            fixture.run()
        self.assertEqual(set(fixture.remote), {POOL_ID, WRONG_ID})
        self.assertTrue(any(lease[1] == 'api-fixture' for lease in fixture.leases))
        fixture.cleanup()
        self.assertEqual(fixture.remote, {})

    def test_orphan_delete_failure_removes_cr_before_api_fallback(self):
        fixture = Probe(orphan_delete_failure=True)
        with self.assertRaisesRegex(RuntimeError, 'orphan CR deletion timed out'):
            fixture.run()
        fixture.cleanup()
        self.assertEqual(fixture.remote, {})

    def test_noop_drift_write_cannot_claim_drift_repair(self):
        fixture = Probe(noop_drift=True)
        with self.assertRaisesRegex(RuntimeError, 'Unable to write owned ModelPool drift'):
            fixture.run()
        self.assertEqual(fixture.repair_waits, 0)
        fixture.cleanup()
        self.assertEqual(fixture.remote, {})


if __name__ == '__main__':
    unittest.main()
