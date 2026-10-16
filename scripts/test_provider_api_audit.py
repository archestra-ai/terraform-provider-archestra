import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('audit', Path(__file__).with_name('provider-api-audit.py'))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class AuditTests(unittest.TestCase):
    def test_matches_paths_despite_parameter_renames(self):
        self.assertEqual(audit.canonical_path('/api/teams/%s/members/%s'), audit.canonical_path('/api/teams/{teamId}/members/{userId}'))

    def test_parses_generated_request_builder(self):
        builders = audit.request_builders('''func NewDeleteThingRequest(server string) {\n operationPath := fmt.Sprintf("/api/things/%s", id)\n req, err := http.NewRequest("DELETE", queryURL.String(), nil)\n}\nfunc unrelated() {}''')
        self.assertEqual(builders, {'DeleteThing': ('DELETE', '/api/things/{}')})

    def test_reports_removed_endpoint_and_does_not_claim_runtime_coverage(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for directory in ['internal/client', 'internal/provider', 'crossplane/config']:
                (root / directory).mkdir(parents=True)
            (root / 'internal/client/client.go').write_text('''func NewDeleteThingRequest(server string) {\n operationPath := fmt.Sprintf("/api/old/%s", id)\n req, err := http.NewRequest("DELETE", queryURL.String(), nil)\n}''')
            (root / 'internal/provider/resource_thing.go').write_text('''resp.TypeName = req.ProviderTypeName + "_thing"\nr.client.DeleteThingWithResponse(ctx, id)''')
            (root / 'crossplane/config/external_name.go').write_text('"archestra_thing": identifier')
            result = audit.audit(root, {'paths': {'/api/new': {'post': {'operationId': 'createNew'}}}})
            self.assertEqual(result['terraform_count'], 1)
            self.assertEqual(result['crossplane_configured_count'], 1)
            resource = result['resources'][0]
            self.assertFalse(resource['endpoints'][0]['present_in_upstream'])
            self.assertEqual(resource['crossplane_reconciliation'], 'unverified')
            self.assertEqual(len(result['unreferenced_mutating_operations']), 1)
            self.assertIn('not successful CRUD', audit.markdown(result))


    def test_finds_forbidden_request_fields_even_when_route_still_exists(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for directory in ['internal/client', 'internal/provider', 'crossplane/config']:
                (root / directory).mkdir(parents=True)
            (root / 'internal/client/client.go').write_text('func NewCreateThingRequestWithBody(server string) {\n operationPath := fmt.Sprintf("/api/things")\n req, err := http.NewRequest("POST", queryURL.String(), body)\n}')
            (root / 'internal/provider/resource_thing.go').write_text('resp.TypeName = req.ProviderTypeName + "_thing"\nr.client.CreateThingWithBodyWithResponse(ctx, body)')
            (root / 'crossplane/config/external_name.go').write_text('')
            result = audit.audit(root, {'paths': {'/api/things': {'post': {'requestBody': {'content': {'application/json': {'schema': {'properties': {'scope': {'not': {}}, 'initialGrants': {'type': 'array'}}}}}}}}}})
            endpoint = result['resources'][0]['endpoints'][0]
            self.assertTrue(endpoint['present_in_upstream'])
            self.assertEqual(endpoint['retired_request_fields'], ['scope'])
            self.assertIn('retired scope', audit.markdown(result))
            current = {'provider_schemas': {'registry.terraform.io/archestra-ai/archestra': {'resource_schemas': {'archestra_thing': {'version': 1, 'block': {'attributes': {'scope': {'computed': True}}}}}}}}
            upstream = {'paths': {'/api/things': {'post': {'requestBody': {'content': {'application/json': {'schema': {'properties': {'scope': {'not': {}}}}}}}}}}}
            endpoint = audit.audit(root, upstream, current)['resources'][0]['endpoints'][0]
            self.assertEqual(endpoint['api_forbidden_request_fields'], ['scope'])
            self.assertEqual(endpoint['retired_request_fields'], [])
            current['provider_schemas']['registry.terraform.io/archestra-ai/archestra']['resource_schemas']['archestra_thing']['block']['attributes']['scope']['optional'] = True
            self.assertEqual(audit.audit(root, upstream, current)['resources'][0]['endpoints'][0]['retired_request_fields'], ['scope'])



    def test_wire_contracts_include_helper_calls_and_flag_unmapped_writes(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for directory in ['internal/client', 'internal/provider', 'crossplane/config']:
                (root / directory).mkdir(parents=True)
            (root / 'internal/client/client.go').write_text('func NewUpdateThingRequestWithBody(server string) {\n operationPath := fmt.Sprintf("/api/things/%s", id)\n req, err := http.NewRequest("PATCH", queryURL.String(), body)\n}')
            (root / 'internal/provider/resource_thing.go').write_text('resp.TypeName = req.ProviderTypeName + "_thing"')
            (root / 'crossplane/config/external_name.go').write_text('')
            upstream = {'paths': {'/api/things/{id}': {'patch': {'requestBody': {'content': {'application/json': {'schema': {'properties': {'name': {'type': 'string'}, 'scope': {'not': {}}, 'newCapability': {'type': 'boolean'}}}}}}}}}}
            contracts = {'archestra_thing': {'api_methods': ['UpdateThing'], 'wire_fields': {'name': 'display_name'}, 'synthetic_fields': []}}
            report = audit.audit(root, upstream, wire_contracts=contracts)
            endpoint = report['resources'][0]['endpoints'][0]
            self.assertTrue(endpoint['present_in_upstream'])
            self.assertEqual(endpoint['declared_request_field_mappings'], {'name': 'display_name'})
            self.assertEqual(endpoint['request_fields_without_declared_mapping'], ['newCapability'])
            self.assertIn('newCapability', audit.markdown(report))


if __name__ == '__main__':
    unittest.main()
