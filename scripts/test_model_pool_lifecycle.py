import argparse
import contextlib
import copy
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch, Mock, call

spec = importlib.util.spec_from_file_location('pool_lifecycle', Path(__file__).parent / 'integration/model-pool-lifecycle.py')
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)
POOL_ID = '11111111-1111-4111-8111-111111111111'
KEY_ID = '22222222-2222-4222-8222-222222222222'


class LifecycleTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.provider = self.root / 'provider'
        self.provider.mkdir()
        binary = self.provider / 'terraform-provider-archestra'
        binary.write_text('synthetic executable fixture')
        binary.chmod(0o700)
        self.workspace = self.root / 'run'
        self.pools = [{'id': 'unrelated', 'name': 'existing-pool'}]
        self.requests = []
        self.commands = []
        self.bound = False
        self.fail_creation_response = False
        self.auth_status = 200
        self.api = patch.object(probe, 'request', side_effect=self.request)
        self.api.start()
        self.addCleanup(self.api.stop)

    def request(self, base, path, key=None, method='GET', body=None):
        self.requests.append((path, key, method, body))
        if path == '/health':
            return 200, {'status': 'ok', 'version': '1.5.0-rc.11'}
        if path == '/openapi.json':
            return 200, {'paths': {'/api/model-pools': {'get': {}, 'post': {'requestBody': {'content': {'application/json': {'schema': {'properties': {'name': {}, 'members': {}}}}}}}}, '/api/model-pools/{id}': {'put': {}, 'delete': {}}}}
        if path == '/api/model-pools' and method == 'GET':
            return self.auth_status, copy.deepcopy(self.pools)
        if path != '/api/model-pools/' + POOL_ID:
            self.fail('mutation escaped the one synthetic pool')
        if method == 'PUT':
            self.pools[-1].update(copy.deepcopy(body))
            return 200, self.pools[-1]
        if method == 'DELETE':
            self.pools = [p for p in self.pools if p['id'] != POOL_ID]
            return 200, {'success': True}
        self.fail('unexpected request')

    def prepare(self):
        with contextlib.redirect_stdout(io.StringIO()):
            probe.prepare(argparse.Namespace(api_url='http://127.0.0.1:9000', provider_directory=self.provider,
                terraform=Path('/synthetic/terraform'), workspace=self.workspace))
        self.key_file = self.root / 'api-key'
        self.key_file.write_text('arch_synthetic_test_credential')

    def execute(self, ui=False):
        with contextlib.redirect_stdout(io.StringIO()), patch.object(probe, 'terraform_process', side_effect=self.terraform):
            return probe.execute(argparse.Namespace(workspace=self.workspace, key_file=self.key_file, provider_key_id=KEY_ID, ui_checkpoints=ui))

    def terraform(self, command, **kwargs):
        self.commands.append(command[1:])
        config = json.loads((self.workspace / 'main.tf.json').read_text())['resource']['archestra_model_pool']['test']
        config = copy.deepcopy(config)
        for member in config['members']:
            member['providerApiKeyId'] = KEY_ID
            member['modelId'] = member.pop('model_id')
            member.pop('provider_api_key_id')
        verb = command[1]
        output, status = '', 0
        if verb == 'apply':
            pool = next((p for p in self.pools if p['id'] == POOL_ID), None)
            if pool is None:
                self.pools.append(dict(config, id=POOL_ID, virtualApiKeyIds=[]))
            else:
                pool.update(config)
            if self.fail_creation_response:
                return subprocess.CompletedProcess(command, 1, '', 'synthetic create follow-up failure')
            self.bound = True
        elif verb == 'output':
            output = POOL_ID
        elif verb == 'plan':
            pool = next(p for p in self.pools if p['id'] == POOL_ID)
            status = 0 if pool['name'] == config['name'] else 2
        elif verb == 'state':
            self.bound = False
        elif verb == 'import':
            self.bound = True
        elif verb == 'show':
            if command[-1] == 'drift.tfplan':
                output = json.dumps({'resource_changes': [{'address': probe.ADDRESS, 'change': {'actions': ['update']}}]})
            else:
                output = json.dumps({'values': {'root_module': {'resources': [{'address': probe.ADDRESS, 'values': {'id': POOL_ID}}] if self.bound else []}}})
        elif verb == 'destroy':
            if self.bound:
                self.pools = [p for p in self.pools if p['id'] != POOL_ID]
        elif verb != 'validate':
            self.fail('unexpected Terraform command')
        return subprocess.CompletedProcess(command, status, output, '')

    def test_prepare_never_reads_credentials_or_mutates(self):
        self.prepare()
        self.assertTrue(all(key is None and method == 'GET' for _, key, method, _ in self.requests))
        document = json.loads((self.workspace / 'main.tf.json').read_text())
        resource = document['resource']['archestra_model_pool']['test']
        self.assertTrue(all(m['enabled'] is False for m in resource['members']))
        self.assertNotIn('virtual_api_key_ids', resource)

    def test_lifecycle_import_drift_and_cleanup_preserve_other_pools(self):
        self.prepare()
        self.assertEqual(self.execute(), 0)
        self.assertEqual(self.pools, [{'id': 'unrelated', 'name': 'existing-pool'}])
        evidence = json.loads((self.workspace / 'evidence.json').read_text())
        self.assertEqual(evidence['cleanup'], 'passed')
        self.assertIn('import-noop', evidence['steps'])
        self.assertIn('repair-drift', evidence['steps'])
        self.assertTrue(any(c[:2] == ['state', 'rm'] for c in self.commands))
        self.assertNotIn('arch_synthetic_test_credential', (self.workspace / 'evidence.json').read_text())

    def test_ui_checkpoints_preserve_the_observable_pool_until_resume(self):
        self.prepare()
        with patch('builtins.input',return_value='') as pause:
            self.assertEqual(self.execute(ui=True),0)
        self.assertEqual(pause.call_count,2)
        record=json.loads((self.workspace/'evidence.json').read_text())
        self.assertNotIn('ui_checkpoint',record)
        self.assertEqual([s['stage'] for s in record['api_snapshots']],['create','update','drift-repaired'])
        self.assertEqual(record['api_snapshots'][1]['members'][0]['weight'],7)

    def test_failed_create_recovers_exact_owned_pool_without_state(self):
        self.prepare()
        self.fail_creation_response = True
        self.assertEqual(self.execute(), 1)
        self.assertEqual(self.pools, [{'id': 'unrelated', 'name': 'existing-pool'}])
        self.assertTrue(any(method == 'DELETE' for _, _, method, _ in self.requests))
        self.assertEqual(json.loads((self.workspace / 'evidence.json').read_text())['cleanup'], 'passed')

    def test_wrong_state_binding_cannot_destroy_another_pool(self):
        self.prepare()
        def wrong_state(command, **kwargs):
            if command[1:] == ['show', '-json']:
                return subprocess.CompletedProcess(command, 0, json.dumps({'values': {'root_module': {'resources': [
                    {'address': probe.ADDRESS, 'values': {'id': 'unrelated'}}]}}}), '')
            return self.terraform(command, **kwargs)
        with contextlib.redirect_stdout(io.StringIO()), patch.object(probe, 'terraform_process', side_effect=wrong_state):
            result = probe.execute(argparse.Namespace(workspace=self.workspace, key_file=self.key_file, provider_key_id=KEY_ID))
        self.assertEqual(result, 1)
        self.assertFalse(any(c[0] == 'destroy' for c in self.commands))
        self.assertEqual(self.pools, [{'id': 'unrelated', 'name': 'existing-pool'}])
        record = json.loads((self.workspace / 'evidence.json').read_text())
        self.assertEqual(record['cleanup'], 'passed')
        self.assertIn('State', record['terraform_cleanup_skipped'])

    def test_api_fallback_does_not_prove_terraform_deletion(self):
        self.prepare()
        def no_delete(command, **kwargs):
            if command[1] == 'destroy':
                return subprocess.CompletedProcess(command, 0, '', '')
            return self.terraform(command, **kwargs)
        with contextlib.redirect_stdout(io.StringIO()), patch.object(probe, 'terraform_process', side_effect=no_delete):
            result = probe.execute(argparse.Namespace(workspace=self.workspace, key_file=self.key_file, provider_key_id=KEY_ID))
        self.assertEqual(result, 1)
        self.assertEqual(self.pools, [{'id': 'unrelated', 'name': 'existing-pool'}])
        record = json.loads((self.workspace / 'evidence.json').read_text())
        self.assertEqual(record['cleanup'], 'passed')
        self.assertTrue(record['api_cleanup_fallback'])
        self.assertEqual(record['outcome'], 'failed')

    def test_disabled_or_denied_api_never_applies(self):
        self.prepare()
        self.auth_status = 404
        self.assertEqual(self.execute(), 1)
        self.assertEqual(self.commands, [])
        self.assertTrue(all(method == 'GET' for _, _, method, _ in self.requests))

    def test_existing_state_and_extra_configuration_are_rejected(self):
        self.prepare()
        (self.workspace / 'extra.tf').write_text('resource "other" "test" {}')
        with self.assertRaisesRegex(ValueError, 'configuration changed'):
            self.execute()
        self.assertEqual(self.commands, [])

    def test_inflight_create_timeout_never_claims_cleanup(self):
        self.prepare()
        def timed_out(command, **kwargs):
            if command[1] == 'apply':
                raise RuntimeError('Terraform timed out; server mutation outcome may be uncertain')
            return self.terraform(command, **kwargs)
        with contextlib.redirect_stdout(io.StringIO()), patch.object(probe, 'terraform_process', side_effect=timed_out):
            result = probe.execute(argparse.Namespace(workspace=self.workspace, key_file=self.key_file, provider_key_id=KEY_ID))
        self.assertEqual(result, 1)
        record = json.loads((self.workspace / 'evidence.json').read_text())
        self.assertTrue(record['cleanup'].startswith('blocked'))
        self.assertIn('uncertain',record['cleanup_error'])

    def test_prepare_rejects_old_grant_replacement_contract(self):
        def older(base,path,*args,**kwargs):
            status,body=self.request(base,path,*args,**kwargs)
            if path=='/openapi.json':
                body['paths']['/api/model-pools']['post']['requestBody']['content']['application/json']['schema']['properties']['virtualApiKeyIds']={}
            return status,body
        with patch.object(probe,'request',side_effect=older),self.assertRaisesRegex(RuntimeError,'Old pool-owned grant contract'):
            self.prepare()
        self.assertFalse(self.workspace.exists())

    def test_non_loopback_urls_are_rejected(self):
        for value in ('https://staging.example', 'http://127.0.0.1:9000/api', 'http://key@localhost:9000'):
            with self.assertRaises(ValueError):
                probe.local_url(value)


class ProcessLifetimeTests(unittest.TestCase):
    def test_interruption_reaps_the_owned_group_before_propagating(self):
        child = Mock(pid=12345)
        child.communicate.side_effect = [KeyboardInterrupt(), ('', ''), ('', '')]
        with patch.object(probe.subprocess, 'Popen', return_value=child), patch.object(probe.os, 'killpg') as kill:
            with self.assertRaises(KeyboardInterrupt):
                probe.terraform_process(['fixture'], env={}, cwd=Path('/tmp'))
        self.assertEqual(kill.call_args_list, [call(12345, probe.signal.SIGTERM), call(12345, probe.signal.SIGKILL)])
        self.assertEqual(child.communicate.call_count, 3)


class RequestBoundaryTests(unittest.TestCase):
    def test_loopback_request_explicitly_disables_environment_proxies(self):
        response = io.BytesIO(b'{}')
        response.status = 200
        with patch.object(probe.urllib.request, 'build_opener') as factory:
            factory.return_value.open.return_value = response
            status, _ = probe.request('http://127.0.0.1:9000', '/api/model-pools', 'arch_synthetic')
            self.assertEqual(status,200)
            handlers = factory.call_args.args
            self.assertTrue(any(isinstance(h,probe.urllib.request.ProxyHandler) and h.proxies == {} for h in handlers))


if __name__ == '__main__':
    unittest.main()
