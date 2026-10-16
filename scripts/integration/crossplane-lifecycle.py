#!/usr/bin/env python3
"""Run real both-scope lifecycle probes against an isolated local API and Kind.

Requires a running provider controller, generated CRDs and named ProviderConfigs.
Never changes kubeconfig context, deploys a stack, or removes finalizers.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--api-url', required=True)
    parser.add_argument('--key-file', type=Path, required=True)
    parser.add_argument('--llm-base-url', required=True)
    parser.add_argument('--readonly-vault', action='store_true', help='Use the existing isolated Vault model fixture')
    parser.add_argument('--mcp-url', default='http://127.0.0.1:19006/mcp')
    parser.add_argument('--cases', nargs='+', choices=('catalog', 'limit', 'llm', 'permissions', 'organization', 'external-groups', 'delegation', 'identity', 'tools', 'core', 'credential', 'schedule', 'api-key', 'skill-policy', 'subagent-exclusions', 'knowledge-exclusions', 'initial-grants', 'initial-grant-defaults', 'skill', 'gateway-skills', 'project-instructions'), default=['catalog', 'limit', 'llm'])
    parser.add_argument('--grant-kinds', nargs='+', choices=('Agent', 'Gateway', 'Project', 'KnowledgeBase', 'RegistryCatalogItem', 'Skill', 'RuntimeCredential', 'ProviderKey'))
    parser.add_argument('--kubeconfig', type=Path, required=True)
    parser.add_argument('--provider-config', required=True)
    parser.add_argument('--namespace', default='crossplane-system')
    parser.add_argument('--evidence', type=Path, required=True)
    parser.add_argument('--terraform-binary', type=Path, required=True)
    parser.add_argument('--controller-binary', type=Path, required=True)
    args = parser.parse_args()
    parsed = urllib.parse.urlparse(args.api_url)
    if parsed.scheme != 'http' or parsed.hostname not in ('127.0.0.1', 'localhost', '::1'):
        parser.error('Only an isolated loopback HTTP API is supported')
    for endpoint in (args.mcp_url, args.llm_base_url):
        parsed = urllib.parse.urlparse(endpoint)
        kind_fixture = (parsed.hostname, parsed.port) in (('ollama-mock.default.svc.cluster.local', 11434), ('provider-mcp-mock.default.svc.cluster.local', 19006))
        if parsed.scheme != 'http' or (parsed.hostname not in ('127.0.0.1', 'localhost', '::1') and not kind_fixture):
            parser.error('Fixture endpoints must use loopback or a fixed private Kind fixture')
    env = dict(os.environ, KUBECONFIG=str(args.kubeconfig.resolve()))
    token = args.key_file.read_text().strip()
    vault_fixture = {'vaultSecretPath': 'secret/data/test/ollama', 'vaultSecretKey': 'api_key'} if args.readonly_vault else {}
    run = uuid.uuid4().hex[:8]
    leases = []
    results = []
    evidence = {'run_id': run, 'terraform_sha256': hashlib.sha256(args.terraform_binary.read_bytes()).hexdigest(),
                'controller_sha256': hashlib.sha256(args.controller_binary.read_bytes()).hexdigest(),
                'results': results, 'cleanup': 'pending'}

    def save():
        args.evidence.write_text(json.dumps(evidence, indent=2) + '\n')

    def kube(*parts, payload=None):
        result = subprocess.run(['kubectl', *parts], env=env, text=True,
                                input=json.dumps(payload) if payload is not None else None,
                                capture_output=True, timeout=90)
        if result.returncode:
            raise RuntimeError('kubectl failed: ' + result.stderr[:700])
        return result.stdout

    context = kube('config', 'current-context').strip()
    if not context.startswith('kind-'):
        parser.error('Only an explicitly supplied private Kind kubeconfig is supported')
    kube_config = json.loads(kube('config', 'view', '--minify', '-o', 'json'))
    kube_server = urllib.parse.urlparse(kube_config['clusters'][0]['cluster']['server'])
    if kube_server.hostname not in ('127.0.0.1', 'localhost', '::1'):
        parser.error('The private Kind API server must use loopback')
    evidence['kubernetes_context'] = context

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            raise RuntimeError('Local API redirects are refused')

    opener = urllib.request.build_opener(NoRedirect())

    def api(path, payload=None, method=None):
        request = urllib.request.Request(args.api_url.rstrip('/') + path,
                    data=json.dumps(payload).encode() if payload is not None else None,
                    headers={'Authorization': token, **({'Content-Type': 'application/json'} if payload is not None else {})},
                    method=method or ('PATCH' if payload is not None else 'GET'))
        try:
            with opener.open(request, timeout=30) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            return error.code, None

    def wait(description, check):
        deadline = time.monotonic() + 75
        while time.monotonic() < deadline:
            result = check()
            if result:
                return result
            time.sleep(1)
        raise RuntimeError('Timed out: ' + description)

    def create(scope, group, kind, plural, fields, suffix_name="", init_fields=None):
        suffix = '.m' if scope == 'namespaced' else ''
        api_group = group + '.archestra' + suffix + '.crossplane.io'
        name = 'cp-proof-' + kind.lower() + suffix_name + '-' + scope + '-' + run
        namespace = ['-n', args.namespace] if scope == 'namespaced' else []
        metadata = {'name': name, 'labels': {'provider-test-run': run}}
        config = {'name': args.provider_config}
        if scope == 'namespaced':
            metadata['namespace'] = args.namespace
            config['kind'] = 'ClusterProviderConfig'
        manifest = {'apiVersion': api_group + '/v1alpha1', 'kind': kind,
                    'metadata': metadata, 'spec': {'forProvider': fields, 'providerConfigRef': config}}
        if init_fields is not None:
            manifest['spec']['initProvider'] = init_fields
        kube('apply', '-f', '-', payload=manifest)
        lease = (namespace, plural + '.' + api_group, name)
        leases.append(lease)
        try:
            kube(*namespace, 'wait', '--for=condition=Ready', lease[1] + '/' + name, '--timeout=75s')
        except RuntimeError:
            if kind in ('SkillPolicy', 'SubagentExclusions', 'Skill', 'SkillPublication', 'SkillExclusions'):
                obj = json.loads(kube(*namespace, 'get', lease[1], name, '-o', 'json'))
                raise RuntimeError(kind + ' did not become Ready: ' + json.dumps(obj.get('status', {}).get('conditions', []))) from None
            raise
        obj = json.loads(kube(*namespace, 'get', lease[1], name, '-o', 'json'))
        return lease, obj['metadata']['annotations']['crossplane.io/external-name'], obj

    def patch(lease, fields):
        kube(*lease[0], 'patch', lease[1], lease[2], '--type=merge', '-p', json.dumps({'spec': {'forProvider': fields}}))

    def replace_fields(lease, fields):
        operations = [{'op': 'replace', 'path': '/spec/forProvider/' + name, 'value': value}
                      for name, value in fields.items()]
        kube(*lease[0], 'patch', lease[1], lease[2], '--type=json', '-p', json.dumps(operations))

    def delete(lease):
        if lease[1] == 'api-fixture':
            status, _ = api(lease[2], method='DELETE')
            if status not in (200, 204, 404):
                raise RuntimeError('Unable to remove owned API fixture: HTTP ' + str(status))
            leases.remove(lease)
            return
        kube(*lease[0], 'delete', lease[1], lease[2], '--wait=true', '--timeout=75s')
        leases.remove(lease)

    def observation(lease):
        return json.loads(kube(*lease[0], 'get', lease[1], lease[2], '-o', 'json'))

    def condition(lease, status):
        return any(item.get('type') == 'Synced' and item.get('status') == status
                   for item in observation(lease).get('status', {}).get('conditions', []))

    def immutable_update(lease, changed, original):
        identifier = observation(lease)['metadata']['annotations']['crossplane.io/external-name']
        patch(lease, changed)
        wait('immutable update refusal', lambda: any(
            item.get('type') == 'Synced' and item.get('status') == 'False'
            and any(marker in item.get('message', '').lower() for marker in ('replac', 'prevent_destroy'))
            for item in observation(lease).get('status', {}).get('conditions', [])))
        if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier:
            raise RuntimeError('Immutable update replaced the managed resource')
        patch(lease, original)
        wait('recovery from immutable update', lambda: condition(lease, 'True'))

    def record(kind, scope, **fields):
        results.append(dict(resource=kind, scope=scope, **fields))
        save()
        print(json.dumps(results[-1]), flush=True)

    try:
        status, org = api('/api/organization')
        if status != 200:
            raise RuntimeError('Unable to read the isolated test organization')
        for scope in ('cluster', 'namespaced'):
            cases = [
                ('mcp', 'RegistryCatalogItem', 'registrycatalogitems', '/api/internal_mcp_catalog/',
                 {'name': 'cp-catalog-' + run + '-' + scope, 'multitenant': False,
                  'remoteConfig': {'url': 'http://127.0.0.1:19999/mcp'}}, 'description', 'description', 'PUT'),
                ('limit', 'Limit', 'limits', '/api/limits/',
                 {'entityId': org['id'], 'entityType': 'organization', 'limitType': 'token_cost',
                  'limitValue': 100000, 'model': ['cp-model-' + run + '-' + scope], 'cleanupInterval': 'calendar_day'},
                 'cleanupInterval', 'cleanupInterval', 'PATCH'),
            ]
            for group, kind, plural, endpoint, method, extra in [
                ('agent', 'Agent', 'agents', '/api/agents/', 'PUT', {'systemPrompt': 'Synthetic controller fixture.'}),
                ('environment', 'Environment', 'environments', '/api/environments/', 'PATCH', {'namespace': 'default'}),
                ('team', 'Team', 'teams', '/api/teams/', 'PUT', {}),
                ('mcp', 'Gateway', 'gateways', '/api/agents/', 'PUT', {}),
                ('project', 'Project', 'projects', '/api/projects/', 'PATCH', {}),
                ('knowledge', 'KnowledgeBase', 'knowledgebases', '/api/knowledge-bases/', 'PUT', {}),
                ('identity', 'Role', 'roles', '/api/roles/', 'PUT', {'permission': {'agent': ['read']}}),
            ]:
                cases.append((group, kind, plural, endpoint, dict(name='cp-' + kind.lower() + '-' + run + '-' + scope, **extra), 'name', 'name', method))
            for group, kind, plural, endpoint, fields, desired_field, wire_field, method in cases:
                if ('catalog' if kind == 'RegistryCatalogItem' else 'limit' if kind == 'Limit' else 'core') not in args.cases:
                    continue
                lease, identifier, obj = create(scope, group, kind, plural, fields)
                path = endpoint + identifier
                if api(path)[0] != 200:
                    raise RuntimeError(kind + ' create did not produce a readable object')
                if kind in ('Agent', 'Gateway') and api(path)[1].get('agentType') != ('agent' if kind == 'Agent' else 'mcp_gateway'):
                    raise RuntimeError('Agent subtype does not match the requested resource')
                desired = 'calendar_month' if kind == 'Limit' else 'Updated ' + run
                patch(lease, {desired_field: desired})
                wait(kind + ' update', lambda: api(path)[1].get(wire_field) == desired)
                drift = 'calendar_week_monday' if kind == 'Limit' else 'Out of band ' + run
                if api(path, {wire_field: drift}, method)[0] != 200:
                    raise RuntimeError(kind + ' drift mutation failed')
                wait(kind + ' drift repair', lambda: api(path)[1].get(wire_field) == desired)
                delete(lease)
                status, _ = api(path)
                if status != 404:
                    raise RuntimeError(kind + ' delete did not remove the object')
                result = {'resource': kind, 'scope': scope, 'create': 'passed', 'update': 'passed',
                          'drift_repair': 'passed', 'delete': 'passed', 'backend_after_delete': status}
                results.append(result)
                save()
                print(json.dumps(result), flush=True)

            if 'llm' in args.cases:
                secret_name = 'cp-proof-headers-' + scope + '-' + run
                header_value = 'synthetic-' + run + '-initial'
                secret = {'apiVersion': 'v1', 'kind': 'Secret',
                          'metadata': {'name': secret_name, 'namespace': args.namespace,
                                       'labels': {'provider-test-run': run}},
                          'stringData': {'X-Provider-Test': header_value}}
                kube('apply', '-f', '-', payload=secret)
                secret_lease = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret_lease)
                secret_ref = {'name': secret_name}
                if scope == 'cluster':
                    secret_ref['namespace'] = args.namespace
                key_name = 'cp-key-' + run + '-' + scope
                key_lease, key_id, key_obj = create(scope, 'llm', 'ProviderKey', 'providerkeys',
                        {'name': key_name, 'llmProvider': 'ollama-native',
                         'baseUrl': args.llm_base_url, 'extraHeadersSecretRef': secret_ref,
                         'labels': [{'key': 'provider-test', 'value': run}], **vault_fixture})
                key_path = '/api/llm-provider-api-keys/' + key_id
                status, key_wire = api(key_path)
                if status != 200 or key_wire.get('extraHeaders', {}).get('X-Provider-Test') != header_value:
                    raise RuntimeError('ProviderKey secret reference was not applied')
                if header_value in json.dumps(key_obj.get('status', {})):
                    raise RuntimeError('ProviderKey exposed a sensitive header in status')
                updated_name = key_name + '-updated'
                patch(key_lease, {'name': updated_name})
                wait('ProviderKey update', lambda: api(key_path)[1].get('name') == updated_name)
                if api(key_path, {'name': 'Out of band ' + run})[0] != 200:
                    raise RuntimeError('ProviderKey drift mutation failed')
                wait('ProviderKey drift repair', lambda: api(key_path)[1].get('name') == updated_name)
                header_value = 'synthetic-' + run + '-rotated'
                secret['stringData']['X-Provider-Test'] = header_value
                kube('apply', '-f', '-', payload=secret)
                wait('ProviderKey secret rotation', lambda: api(key_path)[1].get('extraHeaders', {}).get('X-Provider-Test') == header_value)
                key_obj = json.loads(kube(*key_lease[0], 'get', key_lease[1], key_lease[2], '-o', 'json'))
                if header_value in json.dumps(key_obj.get('status', {})):
                    raise RuntimeError('ProviderKey exposed a rotated sensitive header in status')

                model_lease, model_id, _ = create(scope, 'llm', 'ModelOverride', 'modeloverrides',
                        {'modelId': 'llama3', 'llmProvider': 'ollama-native',
                         'customContextLength': 2048, 'customOutputLength': 256,
                         'customPricePerMillionCacheRead': '0.10', 'customPricePerMillionCacheWrite': '0.20',
                         'configuredParameters': json.dumps({'temperature': 0.6, 'num_ctx': 1024}),
                         'labels': [{'key': 'provider-test', 'value': run}]})

                def model_wire():
                    status, models = api('/api/llm-models')
                    if status != 200:
                        raise RuntimeError('Unable to read the adopted model')
                    return next((model for model in models if model['id'] == model_id), None)

                if model_wire() is None:
                    raise RuntimeError('ModelOverride did not adopt a readable model')
                patch(model_lease, {'customContextLength': 4096})
                wait('ModelOverride update', lambda: model_wire().get('customContextLength') == 4096)
                if api('/api/llm-models/' + model_id, {'customContextLength': 3072})[0] != 200:
                    raise RuntimeError('ModelOverride drift mutation failed')
                wait('ModelOverride drift repair', lambda: model_wire().get('customContextLength') == 4096)
                delete(model_lease)
                reset = model_wire()
                if reset is None or any(reset.get(field) is not None for field in
                            ('customContextLength', 'customOutputLength', 'configuredParameters',
                             'customPricePerMillionCacheRead', 'customPricePerMillionCacheWrite')) or reset.get('labels'):
                    raise RuntimeError('ModelOverride delete failed to retain and reset the model')
                results.append({'resource': 'ModelOverride', 'scope': scope, 'create': 'passed',
                                'update': 'passed', 'drift_repair': 'passed', 'delete_reset': 'passed',
                                'adopted_model_preserved': True})
                save()
                print(json.dumps(results[-1]), flush=True)
                delete(key_lease)
                if api(key_path)[0] != 404:
                    raise RuntimeError('ProviderKey delete did not remove the key')
                delete(secret_lease)
                results.append({'resource': 'ProviderKey', 'scope': scope, 'create': 'passed',
                                'update': 'passed', 'drift_repair': 'passed', 'delete': 'passed',
                                'sensitive_header_secret_rotation': 'passed', 'sensitive_header_absent_from_status': True})
                save()
                print(json.dumps(results[-1]), flush=True)


            if 'permissions' in args.cases:
                parent, environment_id, _ = create(scope, 'environment', 'Environment', 'environments',
                         {'name': 'cp-policy-environment-' + run + '-' + scope, 'namespace': 'default'})
                grant = {'subjectType': 'organization', 'subjectId': '*', 'actions': ['read', 'use']}
                lease, _, _ = create(scope, 'permission', 'Policy', 'policies',
                         {'resourceType': 'environment', 'resourceId': environment_id, 'grants': [grant]})
                path = '/api/resource-permissions/environment/' + environment_id

                def granted_actions():
                    code, policy = api(path)
                    if code != 200:
                        raise RuntimeError('Permission policy read failed')
                    return sorted(next((g['actions'] for g in policy['grants']
                                 if g['subject'] == {'type': 'organization', 'id': '*'}), []))

                if granted_actions() != ['read', 'use']:
                    raise RuntimeError('Permission policy create did not apply direct grants')
                grant['actions'] = ['read']
                patch(lease, {'grants': [grant]})
                wait('Permission policy update', lambda: granted_actions() == ['read'])
                code, policy = api(path)
                if code != 200 or api(path, {'revision': policy['revision'],
                          'grants': [{'subject': {'type': 'organization', 'id': '*'}, 'actions': ['read', 'use']}]}, 'PUT')[0] != 200:
                    raise RuntimeError('Permission policy drift mutation failed')
                wait('Permission policy drift repair', lambda: granted_actions() == ['read'])
                delete(lease)
                if api('/api/environments/' + environment_id)[0] != 200 or api(path)[1]['grants']:
                    raise RuntimeError('Permission policy deletion did not retain target and revoke direct grants')
                delete(parent)
                record('Policy', scope, create='passed', update='passed', drift_repair='passed',
                       delete_revokes_direct_grants='passed', target_preserved=True)

            if 'organization' in args.cases:
                original = api('/api/organization')[1].get('appName')
                lease = None
                try:
                    name = 'cp-settings-' + run + '-' + scope
                    lease, _, _ = create(scope, 'organization', 'Settings', 'settings', {'appName': name})
                    wait('Settings adoption', lambda: api('/api/organization')[1].get('appName') == name)
                    desired = name + '-updated'
                    patch(lease, {'appName': desired})
                    wait('Settings update', lambda: api('/api/organization')[1].get('appName') == desired)
                    if api('/api/organization/appearance-settings', {'appName': 'Out of band ' + run})[0] != 200:
                        raise RuntimeError('Settings drift mutation failed')
                    wait('Settings drift repair', lambda: api('/api/organization')[1].get('appName') == desired)
                    delete(lease)
                    if api('/api/organization')[0] != 200:
                        raise RuntimeError('Settings deletion removed the organization')
                    record('Settings', scope, adopt='passed', update='passed', drift_repair='passed',
                           delete_preserves_singleton='passed')
                finally:
                    if lease in leases:
                        delete(lease)
                    if api('/api/organization/appearance-settings', {'appName': original})[0] != 200:
                        raise RuntimeError('Unable to restore original isolated test appearance')

            if 'project-instructions' in args.cases:
                initial_content = 'Synthetic project instructions.'
                lease, identifier, _ = create(scope, 'project', 'Project', 'projects',
                        {'name': 'cp-instructions-' + run + '-' + scope, 'instructions': initial_content})
                path = '/api/projects/' + identifier
                def project_instructions():
                    code, value = api(path + '/instructions')
                    if code != 200:
                        raise RuntimeError('Project instruction read failed: HTTP ' + str(code))
                    return value['content']
                if project_instructions() != initial_content:
                    raise RuntimeError('Project instructions were not persisted at creation')
                updated_content = 'Updated synthetic project instructions.'
                patch(lease, {'instructions': updated_content})
                wait('Project instruction update', lambda: project_instructions() == updated_content)
                if api(path + '/instructions', {'content': 'Out-of-band synthetic instructions.'}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to write isolated project instruction drift')
                wait('Project instruction drift repair', lambda: project_instructions() == updated_content)
                patch(lease, {'instructions': ''})
                wait('Project instruction clear', lambda: project_instructions() == '')
                delete(lease)
                if api(path)[0] != 404:
                    raise RuntimeError('Project deletion left the fixture readable')
                record('ProjectInstructions', scope, create='passed', update='passed', drift_repair='passed',
                       empty_content_clear='passed', parent_delete='passed', backend_after_delete=404)

            if 'initial-grant-defaults' in args.cases:
                for label in ('default', 'empty', 'init-empty'):
                    explicit_empty = label != 'default'
                    fields = {'name': 'cp-grant-' + label + '-' + run + '-' + scope,
                              'key': 'cp-grant-' + label + '-' + run + '-' + scope}
                    if label == 'empty':
                        fields['initialGrants'] = []
                    lease, _, obj = create(scope, 'credential', 'RuntimeCredential', 'runtimecredentials', fields, '-' + label,
                                           init_fields={'initialGrants': []} if label == 'init-empty' else None)
                    definition_id = obj.get('status', {}).get('atProvider', {}).get('definitionId')
                    if not definition_id:
                        raise RuntimeError('Credential definition UUID was not observed')
                    code, policy = api('/api/resource-permissions/credential/' + definition_id)
                    non_owner_grants = [item for item in policy['grants'] if item['subject']['id'] != policy.get('ownerId')]
                    if code != 200 or bool(non_owner_grants) == explicit_empty:
                        raise RuntimeError('Explicit empty and omitted credential grants lost their distinct creation semantics')
                    patch(lease, {'description': 'Updated grant defaults fixture.'})
                    def default_metadata_updated():
                        code, definitions = api('/api/credentials')
                        return code == 200 and any(item['key'] == fields['key'] and item['description'] == 'Updated grant defaults fixture.' for item in definitions)
                    wait('Credential default grant metadata update', lambda: condition(lease, 'True') and default_metadata_updated())
                    if api('/api/resource-permissions/credential/' + definition_id)[1]['grants'] != policy['grants']:
                        raise RuntimeError('Metadata update changed creation grants')
                    init_update = 'not_applicable'
                    if label == 'init-empty':
                        kube(*lease[0], 'patch', lease[1], lease[2], '--type=merge', '-p', json.dumps({'spec': {'initProvider': {'initialGrants': [{'subjectType': 'organization', 'subjectId': '*', 'actions': ['read']}]}}}))
                        def init_change_reconciled():
                            current = observation(lease)
                            return any(item.get('type') == 'Synced' and item.get('status') == 'True' and item.get('observedGeneration') == current['metadata']['generation'] for item in current.get('status', {}).get('conditions', []))
                        wait('initProvider creation grant update ignored', init_change_reconciled)
                        if api('/api/resource-permissions/credential/' + definition_id)[1]['grants'] != policy['grants']:
                            raise RuntimeError('initProvider update changed already-created grants')
                        init_update = 'passed'
                    delete(lease)
                    record('RuntimeCredentialInitialGrantDefaults', scope, parameter_source=label, explicit_empty=explicit_empty,
                           metadata_update_preserves_grants='passed', init_provider_update_ignored=init_update,
                           persisted_grants=len(policy['grants']), owner_grant_preserved=True, create='passed', delete='passed')

            if 'initial-grants' in args.cases:
                team, team_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-grant-team-' + run + '-' + scope}, '-grant')
                grant = {'subjectType': 'team', 'subjectId': team_id, 'actions': ['read']}
                secret_name = 'cp-grant-key-' + run + '-' + scope
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                     'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                     'stringData': {'api-key': 'synthetic-loopback-only'}})
                secret = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret)
                secret_ref = {'name': secret_name, 'key': 'api-key'}
                if scope == 'cluster':
                    secret_ref['namespace'] = args.namespace
                cases = [
                    ('agent', 'Agent', 'agents', 'agent', {}),
                    ('mcp', 'Gateway', 'gateways', 'mcpGateway', {}),
                    ('project', 'Project', 'projects', 'project', {}),
                    ('knowledge', 'KnowledgeBase', 'knowledgebases', 'knowledgeBase', {}),
                    ('mcp', 'RegistryCatalogItem', 'registrycatalogitems', 'mcpRegistry', {'remoteConfig': {'url': 'http://127.0.0.1:19999/mcp'}}),
                    ('skill', 'Skill', 'skills', 'skill', {'description': 'Synthetic initial grants', 'content': 'Synthetic body.', 'files': {}, 'environmentIds': [], 'labels': []}),
                    ('credential', 'RuntimeCredential', 'runtimecredentials', 'credential', {'key': 'cp-grant-' + run + '-' + scope}),
                    ('llm', 'ProviderKey', 'providerkeys', 'llmProviderApiKey', {'llmProvider': 'ollama', 'shared': True,
                        **(vault_fixture if args.readonly_vault else {'apiKeySecretRef': secret_ref}), 'baseUrl': args.llm_base_url}),
                ]
                for group, kind, plural, resource_type, extra in cases:
                    if args.grant_kinds and kind not in args.grant_kinds:
                        continue
                    fields = {'name': 'cp-grant-' + kind.lower() + '-' + run + '-' + scope,
                              'initialGrants': [grant], **extra}
                    lease, identifier, obj = create(scope, group, kind, plural, fields, '-grant')
                    policy_id = obj.get('status', {}).get('atProvider', {}).get('definitionId') if resource_type == 'credential' else identifier
                    if not policy_id:
                        raise RuntimeError('Credential definition UUID was not observed')
                    path = '/api/resource-permissions/' + resource_type + '/' + policy_id

                    def has_initial_grant():
                        code, policy = api(path)
                        if code != 200:
                            raise RuntimeError('Unable to read initial grant policy for ' + kind)
                        return any(item['subject'] == {'type': 'team', 'id': team_id} and item['actions'] == ['read'] for item in policy['grants'])

                    if not has_initial_grant():
                        raise RuntimeError('Initial grant was not persisted for ' + kind)
                    updated_name = 'updated-' + fields['name']
                    metadata_field = 'description' if kind in ('RegistryCatalogItem', 'RuntimeCredential') else 'name'
                    patch(lease, {metadata_field: updated_name})

                    def metadata_updated():
                        if resource_type == 'credential':
                            code, definitions = api('/api/credentials')
                            return code == 200 and any(item['key'] == identifier and item[metadata_field] == updated_name for item in definitions)
                        prefix = {'agent': '/api/agents/', 'mcpGateway': '/api/agents/', 'project': '/api/projects/',
                                  'knowledgeBase': '/api/knowledge-bases/', 'mcpRegistry': '/api/internal_mcp_catalog/',
                                  'skill': '/api/skills/', 'llmProviderApiKey': '/api/llm-provider-api-keys/'}[resource_type]
                        code, current = api(prefix + identifier)
                        return code == 200 and current[metadata_field] == updated_name

                    try:
                        wait('Initial grant survives metadata update ' + kind,
                             lambda: condition(lease, 'True') and metadata_updated() and has_initial_grant())
                    except RuntimeError:
                        print(json.dumps({'kind': kind, 'conditions': observation(lease).get('status', {}).get('conditions', []),
                                          'backend_metadata_updated': metadata_updated()}), flush=True)
                        raise
                    if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier:
                        raise RuntimeError('Metadata update replaced initial-grant resource ' + kind)
                    immutable_update(lease, {'initialGrants': []}, {'initialGrants': [grant]})
                    if not has_initial_grant():
                        raise RuntimeError('Refused grant replacement changed the policy')
                    delete(lease)
                    record(kind + 'InitialGrants', scope, create_with_policy='passed', metadata_update_preserves_grants='passed',
                           grant_change_requires_replacement='passed', delete='passed')
                delete(secret)
                delete(team)

            if 'gateway-skills' in args.cases:
                gateway, gateway_id, _ = create(scope, 'mcp', 'Gateway', 'gateways',
                        {'name': 'cp-skill-gateway-' + run + '-' + scope})
                skill_fields = {'description': 'Synthetic publication fixture', 'content': 'Inspect only synthetic local data.',
                                'files': {}, 'environmentIds': [], 'labels': []}
                first, first_id, _ = create(scope, 'skill', 'Skill', 'skills',
                        dict(skill_fields, name='cp-publish-first-' + run + '-' + scope), '-first')
                second, second_id, _ = create(scope, 'skill', 'Skill', 'skills',
                        dict(skill_fields, name='cp-publish-second-' + run + '-' + scope), '-second')
                published, identifier, _ = create(scope, 'mcp', 'SkillPublication', 'skillpublications',
                        {'gatewayId': gateway_id, 'accessAllSkills': False, 'skillIds': [first_id]})
                excluded, excluded_id, _ = create(scope, 'mcp', 'SkillExclusions', 'skillexclusions',
                        {'gatewayId': gateway_id, 'excludedSkillIds': [second_id]})
                path = '/api/agents/' + gateway_id
                def publication():
                    code, value = api(path + '/skills')
                    if code != 200:
                        raise RuntimeError('Gateway publication read failed: HTTP ' + str(code))
                    return value
                def skill_exclusions():
                    code, value = api(path + '/skill-exclusions')
                    if code != 200:
                        raise RuntimeError('Gateway exclusion read failed: HTTP ' + str(code))
                    return value['excludedSkillIds']
                if identifier != gateway_id or excluded_id != gateway_id or publication()['skillIds'] != [first_id] or skill_exclusions() != [second_id]:
                    raise RuntimeError('Gateway skill policies did not persist their requested sets')
                patch(published, {'accessAllSkills': True, 'skillIds': [second_id]})
                patch(excluded, {'excludedSkillIds': [first_id]})
                wait('Gateway skill publication update', lambda: publication()['accessAllSkills'] and publication()['skillIds'] == [second_id])
                wait('Gateway skill exclusion update', lambda: skill_exclusions() == [first_id])
                patch(published, {'accessAllSkills': False})
                wait('Gateway mode flip retains assignments', lambda: not publication()['accessAllSkills'] and publication()['skillIds'] == [second_id])
                if api(path + '/skills', {'accessAllSkills': True, 'skillIds': []}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to write isolated publication drift')
                if api(path + '/skill-exclusions', {'excludedSkillIds': []}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to write isolated exclusion drift')
                wait('Gateway publication drift repair', lambda: not publication()['accessAllSkills'] and publication()['skillIds'] == [second_id])
                wait('Gateway exclusion drift repair', lambda: skill_exclusions() == [first_id])
                patch(published, {'skillIds': []})
                patch(excluded, {'excludedSkillIds': []})
                wait('Gateway publication clear', lambda: not publication()['skillIds'])
                wait('Gateway exclusions clear', lambda: not skill_exclusions())
                patch(published, {'accessAllSkills': True, 'skillIds': [first_id]})
                patch(excluded, {'excludedSkillIds': [second_id]})
                wait('Gateway publication restore', lambda: publication()['accessAllSkills'] and publication()['skillIds'] == [first_id])
                wait('Gateway exclusions restore', lambda: skill_exclusions() == [second_id])
                delete(published)
                if publication()['accessAllSkills'] or publication()['skillIds'] or skill_exclusions() != [second_id]:
                    raise RuntimeError('Publication deletion did not preserve independent exclusions')
                record('SkillPublication', scope, create='passed', update='passed', drift_repair='passed',
                       mode_flip_retains_manual_assignments=True, empty_set_clear='passed',
                       delete_disables_and_clears='passed', exclusions_preserved=True)
                delete(excluded)
                if skill_exclusions() or api(path)[0] != 200:
                    raise RuntimeError('Exclusion deletion did not clear while preserving gateway')
                record('SkillExclusions', scope, create='passed', update='passed', drift_repair='passed',
                       empty_set_clear='passed', delete_clears_exclusions='passed', gateway_preserved=True)
                delete(second)
                delete(first)
                delete(gateway)

            if 'skill' in args.cases:
                skill_name = 'cp-content-' + run + '-' + scope
                original_content = 'Inspect only synthetic local fixtures.'
                files = {'references/README.md': {'content': 'Synthetic reference', 'encoding': 'utf8'},
                         'assets/sample.bin': {'content': 'AAECAw==', 'encoding': 'base64'}}
                lease, identifier, _ = create(scope, 'skill', 'Skill', 'skills',
                        {'name': skill_name, 'description': 'Synthetic versioned skill fixture',
                         'content': original_content, 'files': files, 'environmentIds': [],
                         'labels': [], 'allowedTools': ['Read']})
                path = '/api/skills/' + identifier
                def skill_value():
                    code, value = api(path)
                    if code != 200:
                        raise RuntimeError('Skill read failed: HTTP ' + str(code))
                    return value
                initial = skill_value()
                if initial['content'] != original_content or initial['latestVersion'] != 1 or len(initial['files']) != 2:
                    raise RuntimeError('Skill content/file creation was not persisted')
                updated_content = 'Updated synthetic procedure.'
                updated_files = {'references/README.md': {'content': 'Updated reference', 'encoding': 'utf8'}}
                replace_fields(lease, {'content': updated_content, 'files': updated_files})
                wait('Skill content and files update', lambda: skill_value()['content'] == updated_content and len(skill_value()['files']) == 1)
                code, historical = api(path + '/versions/1')
                if code != 200 or historical['content'] != original_content or len(historical['files']) != 2:
                    raise RuntimeError('Skill update changed the immutable first version')
                current = skill_value()
                stale_status, _ = api(path, {'baseVersion': 1,
                         'content': '---\nname: ' + skill_name + '\ndescription: Synthetic versioned skill fixture\n---\nStale edit.\n'}, 'PUT')
                if stale_status != 409:
                    raise RuntimeError('Skill API accepted a stale content version')
                code, _ = api(path, {'baseVersion': current['latestVersion'],
                         'content': '---\nname: ' + skill_name + '\ndescription: Synthetic versioned skill fixture\n---\nOut-of-band synthetic drift.\n'}, 'PUT')
                if code != 200:
                    raise RuntimeError('Unable to write isolated skill drift')
                wait('Skill drift repair', lambda: skill_value()['content'] == updated_content)
                replace_fields(lease, {'files': {}})
                wait('Skill files clear', lambda: not skill_value()['files'])
                delete(lease)
                if api(path)[0] != 404:
                    raise RuntimeError('Skill soft deletion left the fixture readable')
                record('Skill', scope, create='passed', update='passed', drift_repair='passed',
                       terraform_import='separate CLI test', empty_files_clear='passed',
                       immutable_first_version_preserved=True, stale_version_refused=409,
                       delete='soft-delete passed', backend_after_delete=404)

            if 'knowledge-exclusions' in args.cases:
                fixtures = []
                for label in ('first', 'second'):
                    code, connector = api('/api/connectors', {
                        'name': 'cp-knowledge-' + label + '-' + run + '-' + scope,
                        'connectorType': 'web_crawler', 'enabled': False,
                        'config': {'type': 'web_crawler', 'startUrl': 'http://127.0.0.1:19999/',
                                   'allowPrivateNetwork': True, 'maxPages': 1}}, 'POST')
                    if code != 200 or connector.get('enabled') is not False:
                        raise RuntimeError('Unable to create disabled loopback connector fixture')
                    fixture = ([], 'api-fixture', '/api/connectors/' + connector['id'])
                    leases.append(fixture)
                    fixtures.append((fixture, connector['id']))
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                    {'name': 'cp-knowledge-agent-' + run + '-' + scope, 'accessAllTools': True}, '-knowledge')
                path = '/api/agents/' + agent_id + '/knowledge-source-exclusions'

                def excluded_connectors():
                    code, current = api(path)
                    if code != 200:
                        raise RuntimeError('Unable to read knowledge exclusions')
                    return sorted(current['excludedConnectorIds'])

                lease, _, _ = create(scope, 'agent', 'KnowledgeSourceExclusions', 'knowledgesourceexclusions',
                    {'agentId': agent_id, 'excludedConnectorIds': [fixtures[0][1]]})
                if excluded_connectors() != [fixtures[0][1]]:
                    raise RuntimeError('Knowledge exclusion create did not persist')
                patch(lease, {'excludedConnectorIds': [fixtures[1][1]]})
                wait('Knowledge exclusion update', lambda: excluded_connectors() == [fixtures[1][1]])
                if api(path, {'excludedConnectorIds': []}, 'PUT')[0] != 200:
                    raise RuntimeError('Knowledge exclusion drift mutation failed')
                wait('Knowledge exclusion drift repair', lambda: excluded_connectors() == [fixtures[1][1]])
                patch(lease, {'excludedConnectorIds': []})
                wait('Knowledge exclusion empty-set clear', lambda: excluded_connectors() == [] and condition(lease, 'True'))
                patch(lease, {'excludedConnectorIds': [fixtures[0][1]]})
                wait('Knowledge exclusion re-add', lambda: excluded_connectors() == [fixtures[0][1]])
                delete(lease)
                if excluded_connectors() or api('/api/agents/' + agent_id)[0] != 200:
                    raise RuntimeError('Knowledge exclusion delete failed to clear while preserving agent')
                delete(parent)
                for fixture, _ in reversed(fixtures):
                    delete(fixture)
                record('KnowledgeSourceExclusions', scope, create='passed', update='passed',
                       drift_repair='passed', empty_set_clear='passed', delete_clears_exclusions='passed',
                       agent_preserved=True, external_credentials_created=False)

            if 'subagent-exclusions' in args.cases:
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-auto-subagents-' + run + '-' + scope, 'accessAllSubagents': True}, '-parent')
                first, first_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-subagent-first-' + run + '-' + scope}, '-first')
                second, second_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-subagent-second-' + run + '-' + scope}, '-second')
                lease, identifier, _ = create(scope, 'agent', 'SubagentExclusions', 'subagentexclusions',
                        {'agentId': agent_id, 'excludedSubagentIds': [first_id]})
                path = '/api/agents/' + agent_id + '/subagent-exclusions'
                def excluded_subagents():
                    code, value = api(path)
                    if code != 200:
                        raise RuntimeError('Subagent exclusions read failed: HTTP ' + str(code))
                    return value['excludedSubagentIds']
                if identifier != agent_id or excluded_subagents() != [first_id]:
                    raise RuntimeError('Subagent exclusion create did not persist')
                patch(lease, {'excludedSubagentIds': [second_id]})
                wait('Subagent exclusions update', lambda: excluded_subagents() == [second_id])
                if api(path, {'excludedSubagentIds': []}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to write isolated subagent drift')
                wait('Subagent exclusions drift repair', lambda: excluded_subagents() == [second_id])
                patch(lease, {'excludedSubagentIds': []})
                wait('Subagent exclusions clear', lambda: not excluded_subagents())
                patch(lease, {'excludedSubagentIds': [first_id]})
                wait('Subagent exclusions restore', lambda: excluded_subagents() == [first_id])
                delete(lease)
                if excluded_subagents() or api('/api/agents/' + agent_id)[0] != 200:
                    raise RuntimeError('Subagent exclusions deletion did not clear while preserving agent')
                delete(second)
                delete(first)
                delete(parent)
                record('SubagentExclusions', scope, create='passed', update='passed', drift_repair='passed',
                       empty_set_clear='passed', delete_clears_exclusions='passed', agent_preserved=True)

            if 'skill-policy' in args.cases:
                code, skill = api('/api/skills', {'content': '---\nname: cp-skill-' + run + '-' + scope + '\ndescription: Synthetic policy fixture\n---\nInspect only local synthetic fixtures.\n'}, 'POST')
                if code != 200:
                    raise RuntimeError('Unable to create local skill fixture: HTTP ' + str(code))
                fixture = ([], 'api-fixture', '/api/skills/' + skill['id'])
                leases.append(fixture)
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-skill-policy-' + run + '-' + scope})
                ref = {'source': 'native', 'skillId': skill['id']}
                lease, identifier, _ = create(scope, 'agent', 'SkillPolicy', 'skillpolicies',
                        {'agentId': agent_id, 'mode': 'manual',
                         'allowedReferences': [ref], 'excludedReferences': []})
                path = '/api/agents/' + agent_id + '/activation-skill-policy'
                def policy():
                    code, value = api(path)
                    if code != 200:
                        raise RuntimeError('Policy read failed: HTTP ' + str(code))
                    return value
                if identifier != agent_id or policy()['allowedReferences'] != [ref]:
                    raise RuntimeError('Policy create did not persist its stable reference')
                patch(lease, {'mode': 'all', 'allowedReferences': [], 'excludedReferences': [ref]})
                wait('Skill policy update', lambda: policy()['mode'] == 'all' and policy()['excludedReferences'] == [ref] and not policy()['allowedReferences'])
                current = policy()
                code, _ = api(path, {'expectedRevision': current['revision'], 'mode': 'manual', 'operations': []})
                if code != 200:
                    raise RuntimeError('Unable to write isolated policy drift')
                wait('Skill policy drift repair', lambda: policy()['mode'] == 'all')
                delete(lease)
                reset = policy()
                if reset['mode'] != 'manual' or reset['allowedReferences'] or reset['excludedReferences']:
                    raise RuntimeError('Policy deletion did not reset to manual with empty references')
                if api('/api/agents/' + agent_id)[0] != 200:
                    raise RuntimeError('Policy deletion removed its agent')
                delete(parent)
                delete(fixture)
                record('SkillPolicy', scope, create='passed', update='passed', drift_repair='passed',
                       delete_resets_policy='passed', agent_preserved=True)

            if 'external-groups' in args.cases:
                parent, team_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-group-team-' + run + '-' + scope})
                external = 'cp-group-' + run
                lease, identifier, _ = create(scope, 'team', 'ExternalGroup', 'externalgroups',
                          {'teamId': team_id, 'externalGroupId': external})
                path = '/api/teams/' + team_id + '/external-groups'

                def mapping():
                    code, mappings = api(path)
                    if code != 200:
                        raise RuntimeError('External group list failed')
                    return next((item for item in mappings if item['groupIdentifier'] == external), None)

                if mapping() is None:
                    raise RuntimeError('External group create did not persist')
                immutable_update(lease, {'externalGroupId': external + '-replacement'}, {'externalGroupId': external})
                if mapping() is None:
                    raise RuntimeError('Immutable update deleted the existing external group')
                old_id = mapping()['id']
                delete_status, _ = api(path + '/' + old_id, method='DELETE')
                if delete_status != 200:
                    raise RuntimeError('Unable to delete isolated mapping for drift test: HTTP ' + str(delete_status))
                wait('External group recreation', lambda: mapping() is not None and mapping()['id'] != old_id)
                delete(lease)
                if mapping() is not None:
                    raise RuntimeError('External group delete did not remove mapping')
                delete(parent)
                record('ExternalGroup', scope, create='passed', immutable_update_refused='passed',
                       deleted_mapping_recreated='passed', delete='passed')

            if 'delegation' in args.cases:
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-delegator-' + run + '-' + scope}, '-parent')
                target, target_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-target-' + run + '-' + scope}, '-target')
                alternate, alternate_id, _ = create(scope, 'agent', 'Agent', 'agents',
                        {'name': 'cp-alternate-' + run + '-' + scope}, '-alternate')
                lease, _, _ = create(scope, 'agent', 'Delegation', 'delegations',
                        {'agentId': agent_id, 'targetAgentId': target_id})
                path = '/api/agents/' + agent_id + '/delegations'

                def has_edge():
                    code, targets = api(path)
                    if code != 200:
                        raise RuntimeError('Delegation list failed')
                    return any(item['id'] == target_id for item in targets)

                if not has_edge():
                    raise RuntimeError('Delegation create did not persist')
                immutable_update(lease, {'targetAgentId': alternate_id}, {'targetAgentId': target_id})
                if not has_edge():
                    raise RuntimeError('Immutable update removed the original delegation')
                if api(path + '/' + target_id, method='DELETE')[0] != 200:
                    raise RuntimeError('Delegation drift deletion failed')
                wait('Delegation recreation', has_edge)
                delete(lease)
                if has_edge():
                    raise RuntimeError('Delegation delete left an edge')
                delete(alternate)
                delete(target)
                delete(parent)
                record('Delegation', scope, create='passed', immutable_update_refused='passed',
                       deleted_edge_recreated='passed', delete='passed')


            if 'credential' in args.cases:
                credential_key = 'cp-credential-' + run + '-' + scope
                lease, identifier, _ = create(scope, 'credential', 'RuntimeCredential', 'runtimecredentials',
                    {'key': credential_key, 'name': 'Synthetic credential slot', 'description': 'initial'})

                def credential():
                    code, entries = api('/api/credentials')
                    if code != 200:
                        raise RuntimeError('Credential definition list failed')
                    return next((entry for entry in entries if entry['key'] == credential_key), None)

                if identifier != credential_key or credential() is None:
                    raise RuntimeError('Credential key identity did not persist')
                patch(lease, {'description': 'updated'})
                wait('Credential metadata update', lambda: credential().get('description') == 'updated')
                if api('/api/credentials/' + credential_key, {'description': 'drift'})[0] != 200:
                    raise RuntimeError('Credential drift mutation failed')
                wait('Credential drift repair', lambda: credential().get('description') == 'updated')
                delete(lease)
                if credential() is not None:
                    raise RuntimeError('Credential definition survived deletion')
                record('RuntimeCredential', scope, create='passed', update='passed', drift_repair='passed', delete='passed',
                       external_authorization='not_requested; synthetic secret slot only')

            if 'schedule' in args.cases:
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                    {'name': 'cp-scheduled-agent-' + run + '-' + scope}, '-schedule')
                lease, identifier, _ = create(scope, 'schedule', 'Trigger', 'triggers',
                    {'name': 'cp-schedule-' + run + '-' + scope, 'agentId': agent_id,
                     'cronExpression': '0 8 * * *', 'timezone': 'Etc/UTC', 'enabled': False})
                path = '/api/schedule-triggers/' + identifier
                if api(path)[0] != 200 or api(path)[1]['enabled']:
                    raise RuntimeError('Schedule fixture was not created disabled')
                patch(lease, {'cronExpression': '0 9 * * *'})
                wait('Schedule update', lambda: api(path)[1]['cronExpression'] == '0 9 * * *')
                if api(path, {'cronExpression': '0 10 * * *'}, 'PUT')[0] != 200:
                    raise RuntimeError('Schedule drift mutation failed')
                wait('Schedule drift repair', lambda: api(path)[1]['cronExpression'] == '0 9 * * *')
                delete(lease)
                if api(path)[0] != 404:
                    raise RuntimeError('Schedule survived deletion')
                delete(parent)
                record('Trigger', scope, create_disabled='passed', update='passed', drift_repair='passed', delete='passed',
                       scheduled_runs='not_executed')

            if 'api-key' in args.cases:
                key_name = 'cp-key-' + run + '-' + scope[0]
                lease, identifier, obj = create(scope, 'credential', 'APIKey', 'apikeys',
                    {'name': key_name, 'expiresInSeconds': 86400})

                def path():
                    return '/api/api-keys/' + observation(lease)['metadata']['annotations']['crossplane.io/external-name']

                if api(path())[0] != 200 or 'key' in obj.get('status', {}).get('atProvider', {}):
                    raise RuntimeError('API key create or secret status isolation failed')
                immutable_update(lease, {'name': key_name + '-replacement'}, {'name': key_name})
                if api(path())[0] != 200:
                    raise RuntimeError('Immutable update deleted the existing test API key')
                if api(path(), method='DELETE')[0] != 200:
                    raise RuntimeError('API key drift deletion failed')
                wait('Deleted API key recreation', lambda: observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier and api(path())[0] == 200)
                final_path = path()
                delete(lease)
                if api(final_path)[0] != 404:
                    raise RuntimeError('Test API key survived deletion')
                record('APIKey', scope, create='passed', immutable_update_refused='passed',
                       deleted_key_recreated='passed', delete='passed', key_absent_from_status=True)


            if 'tools' in args.cases:
                catalog, catalog_id, _ = create(scope, 'mcp', 'RegistryCatalogItem', 'registrycatalogitems',
                    {'name': 'cp-tools-' + run + '-' + scope, 'multitenant': False,
                     'remoteConfig': {'url': args.mcp_url}})
                server, server_id, _ = create(scope, 'mcp', 'ServerInstallation', 'serverinstallations',
                    {'name': 'cp-install-' + run + '-' + scope, 'catalogId': catalog_id, 'scope': 'org'})
                server_path = '/api/mcp_server/' + server_id
                if api(server_path)[0] != 200:
                    raise RuntimeError('Installation is not readable')
                immutable_update(server, {'name': 'replacement-' + run}, {'name': 'cp-install-' + run + '-' + scope})
                if api(server_path)[0] != 200:
                    raise RuntimeError('Immutable update removed the installation')
                code, discovered = api(server_path + '/tools')
                if code != 200 or len(discovered) != 2:
                    raise RuntimeError('Expected both synthetic MCP tools, got ' + str(discovered))
                tool_ids = sorted(tool['id'] for tool in discovered)
                team_lease, team_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-tools-team-' + run + '-' + scope})
                alternate, alternate_id, _ = create(scope, 'mcp', 'ServerInstallation', 'serverinstallations',
                    {'name': 'cp-alternate-' + run + '-' + scope, 'catalogId': catalog_id, 'scope': 'team', 'teamId': team_id}, '-alternate')
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                    {'name': 'cp-tool-agent-' + run + '-' + scope}, '-tools')

                def assignments(agent):
                    code, response = api('/api/agent-tools?agentId=' + agent + '&limit=100&offset=0')
                    if code != 200:
                        raise RuntimeError('Agent tool list failed')
                    if response['pagination']['hasNext']:
                        raise RuntimeError('Unexpected unrelated assignments in isolated fixture')
                    return response['data']

                def assignment():
                    return next((item for item in assignments(agent_id) if item['tool']['id'] == tool_ids[0]), None)

                binding, _, _ = create(scope, 'agent', 'Tool', 'tools',
                    {'agentId': agent_id, 'toolId': tool_ids[0], 'mcpServerId': server_id})
                if assignment() is None:
                    raise RuntimeError('Tool assignment did not persist')
                patch(binding, {'mcpServerId': alternate_id})
                wait('Tool binding update', lambda: assignment() and assignment().get('mcpServerId') == alternate_id)
                if api('/api/agent-tools/' + assignment()['id'], {'mcpServerId': server_id})[0] != 200:
                    raise RuntimeError('Tool binding drift mutation failed')
                wait('Tool binding drift repair', lambda: assignment() and assignment().get('mcpServerId') == alternate_id)
                delete(binding)
                if assignment() is not None:
                    raise RuntimeError('Tool assignment survived deletion')
                record('Tool', scope, create='passed', update='passed', drift_repair='passed', delete='passed')

                unrelated_ids = {item['id'] for item in assignments(agent_id) if item.get('mcpServerId') != server_id}

                def batch_assignments():
                    return [item for item in assignments(agent_id) if item.get('mcpServerId') == server_id]

                batch, _, _ = create(scope, 'agent', 'ToolBatch', 'toolbatches',
                    {'agentId': agent_id, 'mcpServerId': server_id, 'toolIds': [tool_ids[0]]})
                patch(batch, {'toolIds': tool_ids})
                wait('Tool batch update', lambda: sorted(item['tool']['id'] for item in batch_assignments()) == tool_ids)
                if api('/api/agents/' + agent_id + '/tools/' + tool_ids[0], method='DELETE')[0] != 200:
                    raise RuntimeError('Tool batch drift mutation failed')
                wait('Tool batch drift repair', lambda: sorted(item['tool']['id'] for item in batch_assignments()) == tool_ids)
                delete(batch)
                if batch_assignments():
                    raise RuntimeError('Tool batch deletion retained owned assignments')
                if {item['id'] for item in assignments(agent_id)} != unrelated_ids:
                    raise RuntimeError('Tool batch deletion changed unrelated assignments')
                record('ToolBatch', scope, create='passed', update='passed', drift_repair='passed', delete='passed')
                delete(parent)

                auto, auto_id, _ = create(scope, 'agent', 'Agent', 'agents',
                    {'name': 'cp-auto-agent-' + run + '-' + scope, 'accessAllTools': True}, '-auto')
                exclusion_path = '/api/agents/' + auto_id + '/tool-exclusions'

                def exclusions():
                    code, response = api(exclusion_path)
                    if code != 200:
                        raise RuntimeError('Tool exclusions read failed')
                    return sorted(response['excludedToolIds'])

                exclusions_lease, _, _ = create(scope, 'agent', 'ToolExclusions', 'toolexclusions',
                    {'agentId': auto_id, 'excludedToolIds': [tool_ids[0]]})
                if exclusions() != [tool_ids[0]]:
                    raise RuntimeError('Tool exclusion create did not persist')
                patch(exclusions_lease, {'excludedToolIds': tool_ids})
                wait('Tool exclusions update', lambda: exclusions() == tool_ids)
                if api(exclusion_path, {'excludedToolIds': []}, 'PUT')[0] != 200:
                    raise RuntimeError('Tool exclusion drift mutation failed')
                wait('Tool exclusions drift repair', lambda: exclusions() == tool_ids)
                delete(exclusions_lease)
                if exclusions() or api('/api/agents/' + auto_id)[0] != 200:
                    raise RuntimeError('Exclusions deletion failed to clear while preserving agent')
                delete(auto)
                record('ToolExclusions', scope, create='passed', update='passed', drift_repair='passed',
                       delete_clears_exclusions='passed', agent_preserved=True)
                delete(alternate)
                delete(team_lease)
                delete(server)
                if api(server_path)[0] != 404:
                    raise RuntimeError('Installation delete did not remove the object')
                record('ServerInstallation', scope, create='passed', immutable_update_refused='passed',
                       tools_discovered=2, delete='passed', backend_after_delete=404)
                delete(catalog)


            if 'identity' in args.cases:
                secret_name = 'cp-identity-secret-' + run + '-' + scope
                synthetic_secret = 'synthetic-identity-' + run
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                     'metadata': {'name': secret_name, 'namespace': args.namespace,
                                  'labels': {'provider-test-run': run}},
                     'stringData': {'client-secret': synthetic_secret}})
                secret_lease = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret_lease)
                secret_ref = {'name': secret_name, 'key': 'client-secret'}
                if scope == 'cluster':
                    secret_ref['namespace'] = args.namespace
                domain = 'cp-' + run + '-' + scope + '.example.invalid'
                lease, identifier, obj = create(scope, 'identity', 'Provider', 'providers',
                    {'providerId': 'cp-identity-' + run + '-' + scope, 'domain': domain,
                     'issuer': 'https://identity.example.invalid', 'ssoLoginEnabled': False,
                     'oidcConfig': {'issuer': 'https://identity.example.invalid',
                         'discoveryEndpoint': 'https://identity.example.invalid/.well-known/openid-configuration',
                         'clientId': 'synthetic-client', 'clientSecretSecretRef': secret_ref,
                         'skipDiscovery': True, 'scopes': ['openid', 'email', 'profile'],
                         'authorizationEndpoint': 'https://identity.example.invalid/authorize',
                         'tokenEndpoint': 'https://identity.example.invalid/token',
                         'jwksEndpoint': 'https://identity.example.invalid/keys'}})
                path = '/api/identity-providers/' + identifier
                if api(path)[0] != 200 or synthetic_secret in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Identity provider read or secret isolation failed')
                updated_domain = 'updated-' + domain
                patch(lease, {'domain': updated_domain})
                wait('Identity provider update', lambda: api(path)[1].get('domain') == updated_domain)
                if api(path, {'domain': 'drift-' + domain}, 'PUT')[0] != 200:
                    raise RuntimeError('Identity provider drift mutation failed')
                wait('Identity provider drift repair', lambda: api(path)[1].get('domain') == updated_domain)
                obj = observation(lease)
                if synthetic_secret in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Identity provider exposed its configured client secret')
                delete(lease)
                if api(path)[0] != 404:
                    raise RuntimeError('Identity provider delete did not remove the fixture')
                delete(secret_lease)
                record('Provider', scope, create='passed', update='passed', drift_repair='passed',
                       delete='passed', secret_absent_from_status=True,
                       live_oauth_consent='not_exercised; synthetic configuration only')

        evidence['outcome'] = 'passed'
    except Exception as error:
        evidence['outcome'] = 'failed'
        evidence['error'] = str(error)
        raise
    finally:
        errors = []
        for lease in reversed(leases.copy()):
            try:
                delete(lease)
            except Exception as error:
                errors.append(str(error))
                break
        evidence['cleanup'] = 'passed' if not errors else 'blocked; parents retained'
        if errors:
            evidence['cleanup_errors'] = errors
        save()


if __name__ == '__main__':
    main()
