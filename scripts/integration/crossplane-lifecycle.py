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



CASE_RESOURCES = {
    'apps': {'App', 'AppInit'},
    'project-file-init': {'ProjectFileInit'},
    'knowledge-files': {'KnowledgeFile', 'KnowledgeFileInit'},
    'team-bindings': {'TeamInlineBindings'},
    'service-account-team': {'ServiceAccountTeamOwnership'},
    'inline-owners': {'AgentInlineOwnership', 'GatewayInlineOwnership'},
    'project-files': {'ProjectFileText', 'ProjectFileBinary'},
    'marketplace-share-links': {'ShareLinkSkills', 'ShareLinkPlugins', 'ShareLinkExpiry'},
    'repository-plugins': {'PluginGitSource'}, 'manual-plugins': {'PluginFiles'},
    'github-registrations': {'GitHubPAT', 'GitHubApp'},
    'api-compatibility': {'APICompatibilityConstraint'},
    'model-pools': {'AgentModelPool', 'OAuthClientModelPools'},
    'routing-credentials': {'VirtualKey', 'OAuthClient'}, 'remote-agents': {'RemoteAgent'},
    'extensions': {'Directory', 'ServiceAccount', 'ServiceAccountAPIKey', 'GatewaySubagentControl', 'InstallationControls'},
    'core': {'Agent', 'Environment', 'Team', 'Gateway', 'Project', 'KnowledgeBase', 'Role'},
    'catalog': {'RegistryEntry'}, 'limit': {'Limit'}, 'llm': {'Model', 'ModelProvider', 'ModelSharing'},
    'organization': {'Settings'}, 'organization-defaults': {'OrganizationInlineDefaults'},
    'identity': {'Provider'}, 'tools': {'Server'}, 'credential': {'Credential'},
    'schedule': {'Schedule'}, 'api-key': {'APIKey'}, 'skill': {'Skill'},
    'git-skills': {'SkillGitSource', 'SkillGitSourceInit'},
    'project-instructions': {'ProjectInstructions'},
    'observe': {'AgentObserve', 'TeamObserve'},
    'parent-sharing': {kind + 'Sharing' for kind in ('Environment', 'Agent', 'Gateway', 'ServiceAccount', 'RemoteAgent', 'VirtualKey', 'OAuthClient', 'Plugin', 'Project', 'KnowledgeBase', 'RegistryEntry', 'Skill', 'Credential', 'ModelProvider')},
}



def resume_results(path, evidence):
    previous = json.loads(path.read_text())
    for key in ('terraform_sha256', 'controller_sha256', 'kubernetes_context', 'api_url', 'api_spec_sha256'):
        if previous.get(key) != evidence.get(key):
            raise ValueError('Resume refused: different ' + key)
    if previous.get('cleanup') != 'passed' or previous.get('api_cleanup') not in (None, 'passed'):
        raise ValueError('Resume refused: previous owned fixtures need normal cleanup')
    return previous['results']


def completed_cases(results, scope):
    done = {item['resource'] for item in results if item['scope'] == scope}
    completed = {case for case, expected in CASE_RESOURCES.items() if expected.issubset(done)}
    defaults = {item.get('parameter_source') for item in results if item['scope'] == scope and item['resource'] == 'CredentialSharingDefaults'}
    if {'default', 'empty', 'init-empty'}.issubset(defaults):
        completed.add('sharing-defaults')
    return completed


def a2a_source(name):
    return {'type': 'inline_card', 'agentCard': {
        'name': name, 'description': 'Synthetic external registration.', 'version': '1.0.0',
        'supportedInterfaces': [{'url': 'http://127.0.0.1:19998/a2a/' + name, 'protocolBinding': 'JSONRPC', 'protocolVersion': '1.0'}],
        'capabilities': {'streaming': False}, 'defaultInputModes': ['text/plain'], 'defaultOutputModes': ['text/plain'], 'skills': [],
        'securitySchemes': {'apiKeyAuth': {'type': 'apiKey', 'in': 'header', 'name': 'X-API-Key'}},
        'securityRequirements': [{}, {'apiKeyAuth': []}]}}


GIT_PLUGIN_FIXTURE_SHA = '9e6a43f5347f6f3730ffeb75946f6e162791851d'


def git_plugin_source_fields():
    return {'gitSource': {'repo': 'archestra-ai/terraform-provider-archestra', 'commitSha': GIT_PLUGIN_FIXTURE_SHA,
            'subdir': 'scripts/integration/fixtures/plugins/synthetic', 'exclude': ['excluded.txt']}}


def model_provider_fixture(namespace, scope, run):
    name = 'cp-model-key-' + scope + '-' + run
    secret = {'apiVersion': 'v1', 'kind': 'Secret',
              'metadata': {'name': name, 'namespace': namespace,
                           'labels': {'provider-test-run': run}},
              'stringData': {'api-key': 'synthetic-local-only'}}
    reference = {'name': name, 'key': 'api-key'}
    if scope == 'cluster':
        reference['namespace'] = namespace
    return secret, {'apiKeySecretRef': reference}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--api-url', required=True)
    parser.add_argument('--key-file', type=Path, required=True)
    parser.add_argument('--llm-base-url', required=True)
    parser.add_argument('--mcp-url', default='http://127.0.0.1:19006/mcp')
    parser.add_argument('--cases', nargs='+', choices=tuple(CASE_RESOURCES) + ('sharing-defaults',), default=['catalog', 'limit', 'llm'])
    parser.add_argument('--resume-from', type=Path, help='Resume normally cleaned evidence only when both binary hashes and the private context match')
    parser.add_argument('--scopes', nargs='+', choices=('cluster', 'namespaced'), default=['cluster', 'namespaced'])
    parser.add_argument('--grant-kinds', nargs='+', choices=tuple(kind.removesuffix('Sharing') for kind in sorted(CASE_RESOURCES['parent-sharing'])))
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

    resource_names = {}

    def resource_name(api_group, kind):
        key = (api_group, kind)
        if key not in resource_names:
            discovery = json.loads(kube('get', '--raw', '/apis/' + api_group + '/v1alpha1'))
            matches = [row['name'] for row in discovery['resources'] if row['kind'] == kind and '/' not in row['name']]
            if len(matches) != 1:
                raise RuntimeError('Expected one API resource for ' + api_group + '/' + kind)
            resource_names[key] = matches[0]
        return resource_names[key]

    def create(scope, group, kind, plural, fields, suffix_name="", init_fields=None, external_name=None):
        if kind == 'Gateway':
            group = 'gateway'
        suffix = '.m' if scope == 'namespaced' else ''
        api_group = group + '.archestra' + suffix + '.crossplane.io'
        plural = resource_name(api_group, kind)
        name = 'cp-proof-' + kind.lower() + suffix_name + '-' + scope + '-' + run
        namespace = ['-n', args.namespace] if scope == 'namespaced' else []
        metadata = {'name': name, 'labels': {'provider-test-run': run}}
        if external_name is not None:
            metadata['annotations'] = {'crossplane.io/external-name': external_name}
        config = {'name': args.provider_config}
        if scope == 'namespaced':
            metadata['namespace'] = args.namespace
            config['kind'] = 'ClusterProviderConfig'
        manifest = {'apiVersion': api_group + '/v1alpha1', 'kind': kind,
                    'metadata': metadata, 'spec': {'forProvider': fields, 'providerConfigRef': config}}
        if init_fields is not None:
            manifest['spec']['initProvider'] = init_fields
        if kind in ('ServiceAccountAPIKey', 'VirtualKey', 'OAuthClient', 'ShareLink'):
            ref = {'name': name + '-connection'}
            if scope == 'cluster':
                ref['namespace'] = args.namespace
            manifest['spec']['writeConnectionSecretToRef'] = ref
            leases.append((['-n', args.namespace], 'secrets', ref['name']))
        kube('apply', '-f', '-', payload=manifest)
        lease = (namespace, plural + '.' + api_group, name)
        leases.append(lease)
        try:
            kube(*namespace, 'wait', '--for=condition=Ready', lease[1] + '/' + name, '--timeout=75s')
        except RuntimeError:
            obj = json.loads(kube(*namespace, 'get', lease[1], name, '-o', 'json'))
            raise RuntimeError(kind + ' did not become Ready: ' + json.dumps(obj.get('status', {}).get('conditions', []))) from None
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
        kube(*lease[0], 'delete', lease[1], lease[2], '--ignore-not-found', '--wait=true', '--timeout=75s')
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
        status, snapshot = api('/openapi.json')
        if status != 200:
            raise RuntimeError('Unable to verify the isolated API snapshot')
        evidence['api_url'] = args.api_url.rstrip('/')
        evidence['api_spec_sha256'] = hashlib.sha256(json.dumps(snapshot, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        evidence['api_version'] = snapshot['info']['version']
        if args.resume_from:
            results.extend(resume_results(args.resume_from, evidence))
            evidence['resumed_from'] = {'file': args.resume_from.name, 'sha256': hashlib.sha256(args.resume_from.read_bytes()).hexdigest()}
            save()
        status, org = api('/api/organization')
        if status != 200:
            raise RuntimeError('Unable to read the isolated test organization')
        for scope in args.scopes:
            scope_cases = set(args.cases) - completed_cases(results, scope)
            if scope_cases & {'llm', 'routing-credentials', 'model-pools'}:
                secret, model_fixture = model_provider_fixture(args.namespace, scope, run)
                kube('apply', '-f', '-', payload=secret)
                leases.append((['-n', args.namespace], 'secrets', secret['metadata']['name']))
            if 'inline-owners' in scope_cases:
                child, child_id, _ = create(scope, 'agent', 'Agent', 'agents', {'name': 'cp-inline-child-' + run + '-' + scope}, '-child')
                remote, remote_id, _ = create(scope, 'a2a', 'RemoteAgent', 'remoteagents', {'source': json.dumps(a2a_source('cp-inline-remote-' + run + '-' + scope))})
                skill, skill_id, _ = create(scope, 'skill', 'Skill', 'skills', {'name': 'cp-inline-skill-' + run + '-' + scope, 'description': 'Synthetic ownership fixture', 'content': 'Inspect only local synthetic fixtures.', 'files': {}, 'environmentIds': [], 'labels': []})
                catalog, catalog_id, _ = create(scope, 'mcp', 'RegistryEntry', 'registrycatalogitems', {'name': 'cp-inline-tools-' + run + '-' + scope, 'multitenant': False, 'remoteConfig': {'url': args.mcp_url}})
                server, server_id, _ = create(scope, 'mcp', 'Server', 'serverinstallations', {'name': 'cp-inline-server-' + run + '-' + scope, 'catalogId': catalog_id, 'scope': 'org'})
                code, discovered = api('/api/mcp_server/' + server_id + '/tools')
                if code != 200 or len(discovered) < 2:
                    raise RuntimeError('Inline tool fixture did not advertise two tools')
                tool_ids = sorted(item['id'] for item in discovered)[:2]
                code, connector = api('/api/connectors', {'name': 'cp-inline-connector-' + run + '-' + scope, 'connectorType': 'web_crawler', 'enabled': False, 'config': {'type': 'web_crawler', 'startUrl': 'http://127.0.0.1:19999/', 'allowPrivateNetwork': True, 'maxPages': 1}}, 'POST')
                if code != 200 or connector.get('enabled') is not False:
                    raise RuntimeError('Disabled local connector fixture was not created')
                connector_lease = ([], 'api-fixture', '/api/connectors/' + connector['id'])
                leases.append(connector_lease)
                required_names = {'get_remedy_plans', 'execute_remedy_plan', 'list_peer_messages', 'read_peer_message', 'yell'}
                def checked(path):
                    code, value = api(path)
                    if code != 200:
                        raise RuntimeError('Inline state read failed: ' + path + ': HTTP ' + str(code))
                    return value
                for owner_kind, owner_group, owner_plural in [('Agent', 'agent', 'agents'), ('Gateway', 'gateway', 'gateways')]:
                    owner_name = 'cp-inline-' + owner_kind.lower() + '-' + run + '-' + scope
                    fields = {'name': owner_name, 'subagentIdsRefs': [{'name': child[2]}], 'remoteAgentIdsRefs': [{'name': remote[2]}], 'excludedSubagentIds': [child_id], 'excludedConnectorIds': [connector['id']], 'tools': [{'toolId': tool_id, 'mcpServerIdRef': {'name': server[2]}, 'credentialResolutionMode': 'static'} for tool_id in tool_ids], 'excludedToolIds': []}
                    fields['skills'] = {'mode': 'manual', 'allowed': [{'source': 'native', 'skillIdRef': {'name': skill[2]}}], 'excluded': []} if owner_kind == 'Agent' else {'mode': 'manual', 'idsRefs': [{'name': skill[2]}], 'excludedIds': []}
                    if owner_kind == 'Agent':
                        fields['hooks'] = [{'event': 'session_start', 'fileName': 'verify.sh', 'content': 'echo initial', 'requirements': [], 'enabled': False}]
                    owner, owner_id, _ = create(scope, owner_group, owner_kind, owner_plural, fields, '-inline')
                    path = '/api/agents/' + owner_id
                    def state():
                        agent = checked(path)
                        ordinary, required = set(), set()
                        for tool in agent['tools']:
                            mandatory = tool.get('catalogId') == '00000000-0000-4000-8000-000000000001' and tool['name'].split('__')[-1] in required_names
                            if mandatory:
                                required.add(tool['id'])
                            elif not tool.get('delegateToAgentId') and not tool.get('delegateToA2aConnectionId'):
                                ordinary.add(tool['id'])
                        assignments = checked('/api/agent-tools?agentId=' + owner_id + '&limit=100&offset=0')['data']
                        selected = {row['tool']['id']: (row.get('mcpServerId'), row['credentialResolutionMode']) for row in assignments if row['tool']['id'] in ordinary}
                        skill_state = checked(path + ('/activation-skill-policy' if owner_kind == 'Agent' else '/skills'))
                        if owner_kind == 'Agent':
                            mode = skill_state['mode']
                            allowed = {item['skillId'] for item in skill_state['allowedReferences']}
                            excluded = {item['skillId'] for item in skill_state['excludedReferences']}
                        else:
                            mode = 'all' if skill_state['accessAllSkills'] else 'manual'
                            allowed = set(skill_state['skillIds'])
                            excluded = set(checked(path + '/skill-exclusions')['excludedSkillIds'])
                        value = {'local': {row['id'] for row in checked(path + '/delegations')}, 'remote': {row['remoteAgentId'] for row in checked(path + '/a2a-delegations')}, 'excludedLocal': set(checked(path + '/subagent-exclusions')['excludedSubagentIds']), 'excludedConnectors': set(checked(path + '/knowledge-source-exclusions')['excludedConnectorIds']), 'tools': selected, 'excludedTools': set(checked(path + '/tool-exclusions')['excludedToolIds']), 'required': required, 'mode': mode, 'allowed': allowed, 'excluded': excluded}
                        if owner_kind == 'Agent':
                            value['hooks'] = [(hook['id'], hook['content'], hook['requirements'], hook['enabled']) for hook in checked('/api/hooks?agentId=' + owner_id)]
                        return value
                    initial = state()
                    expected = {'local': {child_id}, 'remote': {remote_id}, 'excludedLocal': {child_id}, 'excludedConnectors': {connector['id']}, 'tools': {tool_id: (server_id, 'static') for tool_id in tool_ids}, 'excludedTools': set(), 'mode': 'manual', 'allowed': {skill_id}, 'excluded': set()}
                    if any(initial[key] != value for key, value in expected.items()):
                        raise RuntimeError('Inline parent create or reference resolution did not persist')
                    if owner_kind == 'Agent' and not initial['required']:
                        raise RuntimeError('Agent fixture has no required recovery tools')
                    patch(owner, {'tools': [{'toolId': tool_ids[0], 'credentialResolutionMode': 'dynamic'}], 'excludedToolIds': [tool_ids[1]]})
                    wait('Inline unpin and selection update', lambda: state()['tools'] == {tool_ids[0]: (None, 'dynamic')} and state()['excludedTools'] == {tool_ids[1]})
                    patch(owner, {'tools': []})
                    wait('Ordinary tool clear preserves subagents', lambda: not state()['tools'] and state()['local'] == {child_id} and state()['remote'] == {remote_id} and state()['required'] == initial['required'])
                    new_skills = {'mode': 'all', 'allowed': [], 'excluded': [{'source': 'native', 'skillId': skill_id}]} if owner_kind == 'Agent' else {'mode': 'all', 'ids': [], 'excludedIds': [skill_id]}
                    replace_fields(owner, {'skills': new_skills})
                    if owner_kind == 'Agent':
                        patch(owner, {'hooks': [{'event': 'session_start', 'fileName': 'verify.sh', 'content': 'echo updated', 'requirements': [], 'enabled': False}]})
                    wait('Inline skill and hook update', lambda: state()['mode'] == 'all' and state()['excluded'] == {skill_id} and (owner_kind != 'Agent' or state()['hooks'][0][1] == 'echo updated'))
                    if owner_kind == 'Agent' and state()['hooks'][0][0] != initial['hooks'][0][0]:
                        raise RuntimeError('Inline hook content update replaced the hook')
                    for endpoint, payload in [('/delegations', {'targetAgentIds': []}), ('/a2a-delegations', {'connectionIds': []})]:
                        if api(path + endpoint, payload, 'POST')[0] != 200:
                            raise RuntimeError('Unable to write inline assignment drift')
                    if api(path + '/tool-exclusions', {'excludedToolIds': []}, 'PUT')[0] != 200:
                        raise RuntimeError('Unable to write inline exclusion drift')
                    wait('Inline relationship drift repair', lambda: state()['local'] == {child_id} and state()['remote'] == {remote_id} and state()['excludedTools'] == {tool_ids[1]})
                    managed_names = ['subagentIds', 'subagentIdsRefs', 'remoteAgentIds', 'remoteAgentIdsRefs', 'excludedSubagentIds', 'excludedConnectorIds', 'tools', 'excludedToolIds', 'skills'] + (['hooks'] if owner_kind == 'Agent' else [])
                    before_omit = state()
                    patch(owner, {**dict.fromkeys(managed_names), 'name': owner_name + '-omitted'})
                    wait('Inline omitted-field update', lambda: checked(path)['name'] == owner_name + '-omitted' and condition(owner, 'True'))
                    time.sleep(7)
                    after_omit = state()
                    if after_omit != before_omit:
                        evidence['inline_omission_difference'] = {key: {'before': repr(before_omit[key]), 'after': repr(after_omit[key])} for key in before_omit if after_omit[key] != before_omit[key]}
                        raise RuntimeError('Omitting inline fields changed existing configuration')
                    if any(name in observation(owner)['spec']['forProvider'] for name in managed_names):
                        raise RuntimeError('Inline omitted relationships were late initialized into managed inputs')
                    empty = {'name': owner_name + '-cleared', 'subagentIds': [], 'remoteAgentIds': [], 'excludedSubagentIds': [], 'excludedConnectorIds': [], 'tools': [], 'excludedToolIds': [], 'skills': {'mode': 'manual', 'allowed': [], 'excluded': []} if owner_kind == 'Agent' else {'mode': 'manual', 'ids': [], 'excludedIds': []}}
                    if owner_kind == 'Agent':
                        empty['hooks'] = []
                    patch(owner, empty)
                    def cleared():
                        value = state()
                        return not any(value[key] for key in ['local', 'remote', 'excludedLocal', 'excludedConnectors', 'tools', 'excludedTools', 'allowed', 'excluded'] + (['hooks'] if owner_kind == 'Agent' else [])) and value['mode'] == 'manual' and value['required'] == initial['required'] and condition(owner, 'True')
                    wait('All inline collections clear', cleared)
                    if observation(owner)['metadata']['annotations']['crossplane.io/external-name'] != owner_id:
                        raise RuntimeError('Inline configuration update replaced the parent')
                    kube(*owner[0], 'patch', owner[1], owner[2], '--type=merge', '-p', json.dumps({'spec': {'managementPolicies': ['Create', 'Observe', 'Update', 'LateInitialize']}}))
                    if 'Delete' in observation(owner)['spec']['managementPolicies']:
                        raise RuntimeError('Orphan policy was not retained by the CRD')
                    fallback = ([], 'api-fixture', path)
                    leases.append(fallback)
                    delete(owner)
                    if api(path)[0] != 200:
                        raise RuntimeError('Orphan deleted its existing parent')
                    adopted, adopted_id, _ = create(scope, owner_group, owner_kind, owner_plural, empty, '-adopt', external_name=owner_id)
                    leases.remove(fallback)
                    owner = adopted
                    if adopted_id != owner_id or not cleared():
                        raise RuntimeError('Inline adoption changed identity or persisted configuration')
                    delete(adopted)
                    if api(path)[0] != 404:
                        raise RuntimeError('Normal inline parent deletion retained the API object')
                    record(owner_kind + 'InlineOwnership', scope, create='passed', native_references='passed', update='passed', tool_unpin='passed', ordinary_tool_clear_preserves_subagents='passed', recovery_tools_preserved='passed', drift_repair='passed', omission_preserves_unmanaged='passed', empty_collections_clear='passed', orphan_adoption='passed', identity_preserved=True, delete='passed', execution='not_invoked')
                for fixture in [connector_lease, server, catalog, skill, remote, child]:
                    delete(fixture)


            if 'team-bindings' in scope_cases:
                team_name = 'cp-team-bindings-' + run + '-' + scope
                team, team_id, _ = create(scope, 'team', 'Team', 'teams', {'name': team_name, 'externalGroupIds': ['engineering', 'support']})
                path = '/api/teams/' + team_id
                def mappings():
                    code, value = api(path + '/external-groups')
                    if code != 200:
                        raise RuntimeError('Team group state read failed: HTTP ' + str(code))
                    return {row['groupIdentifier']: row['id'] for row in value}
                if set(mappings()) != {'engineering', 'support'}:
                    raise RuntimeError('Inline team bindings did not persist')
                patch(team, {'externalGroupIds': ['operations']})
                wait('Team binding update', lambda: set(mappings()) == {'operations'})
                mapping_id = mappings()['operations']
                if api(path + '/external-groups/' + mapping_id, method='DELETE')[0] != 200:
                    raise RuntimeError('Unable to modify owned team group drift')
                wait('Team binding drift repair', lambda: set(mappings()) == {'operations'} and mappings()['operations'] != mapping_id)
                patch(team, {'externalGroupIds': None, 'name': team_name + '-omitted'})
                wait('Team omitted-field update', lambda: api(path)[1]['name'] == team_name + '-omitted' and condition(team, 'True'))
                time.sleep(7)
                if set(mappings()) != {'operations'}:
                    raise RuntimeError('Omitted team fields changed existing bindings')
                if any(key in observation(team)['spec']['forProvider'] for key in ('externalGroupIds',)):
                    raise RuntimeError('Team bindings were late initialized into managed configuration')
                patch(team, {'externalGroupIds': []})
                wait('Team explicit clear', lambda: not mappings() and condition(team, 'True'))
                if observation(team)['metadata']['annotations']['crossplane.io/external-name'] != team_id:
                    raise RuntimeError('Binding changes replaced the team')
                delete(team)
                if api(path)[0] != 404:
                    raise RuntimeError('Team delete retained the parent')
                record('TeamInlineBindings', scope, create='passed', update='passed', drift_repair='passed', omission_preserves_unmanaged='passed', empty_set_and_string_clear='passed', identity_preserved=True, delete='passed', external_secrets_created=False)

            if 'service-account-team' in scope_cases:
                first, first_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-account-first-' + run + '-' + scope}, '-first')
                second, second_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-account-second-' + run + '-' + scope}, '-second')
                account, account_id, _ = create(scope, 'identity', 'ServiceAccount', 'serviceaccounts', {'name': 'cp-team-account-' + run + '-' + scope, 'role': 'member', 'teamIdRef': {'name': first[2]}})
                path = '/api/service-accounts/' + account_id
                if api(path)[1].get('teamId') != first_id:
                    raise RuntimeError('Service-account team reference did not resolve')
                if api(path, {'teamId': second_id}, 'PATCH')[0] != 200:
                    raise RuntimeError('Unable to modify service-account team drift')
                wait('Service-account team drift repair', lambda: api(path)[1].get('teamId') == first_id)
                patch(account, {'teamId': None, 'teamIdRef': {'name': second[2]}})
                wait('Service-account team move', lambda: api(path)[1].get('teamId') == second_id and condition(account, 'True'))
                patch(account, {'teamId': None, 'teamIdRef': None})
                wait('Service-account team clear', lambda: api(path)[1].get('teamId') is None and condition(account, 'True'))
                if observation(account)['metadata']['annotations']['crossplane.io/external-name'] != account_id:
                    raise RuntimeError('Team ownership replaced the service account')
                delete(account)
                if api(path)[0] != 404:
                    raise RuntimeError('Service-account delete retained the parent')
                delete(second)
                delete(first)
                record('ServiceAccountTeamOwnership', scope, create='passed', reference='passed', drift_repair='passed', update='passed', clear='passed', identity_preserved=True, delete='passed')


            cases = [
                ('mcp', 'RegistryEntry', 'registrycatalogitems', '/api/internal_mcp_catalog/',
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
                if ('catalog' if kind == 'RegistryEntry' else 'limit' if kind == 'Limit' else 'core') not in scope_cases:
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

            if 'observe' in scope_cases:
                # Crossplane's equivalent of the archestra_agent and archestra_team data sources.
                for kind, group, endpoint, body, method in [
                        ('Agent', 'agent', '/api/agents', {'agentType': 'agent'}, 'PUT'),
                        ('Team', 'team', '/api/teams', {}, 'PUT')]:
                    api_name = 'cp-observe-' + kind.lower() + '-' + run + '-' + scope
                    code, created = api(endpoint, dict(name=api_name, **body), 'POST')
                    if code not in (200, 201) or not created.get('id'):
                        raise RuntimeError('Unable to create the observed ' + kind + ' through the API: HTTP ' + str(code))
                    path = endpoint + '/' + created['id']
                    fixture = ([], 'api-fixture', path)
                    leases.append(fixture)
                    suffix = '.m' if scope == 'namespaced' else ''
                    api_group = group + '.archestra' + suffix + '.crossplane.io'
                    name = 'cp-proof-' + kind.lower() + '-observe-' + scope + '-' + run
                    namespace = ['-n', args.namespace] if scope == 'namespaced' else []
                    metadata = {'name': name, 'labels': {'provider-test-run': run},
                                'annotations': {'crossplane.io/external-name': created['id']}}
                    config = {'name': args.provider_config}
                    if scope == 'namespaced':
                        metadata['namespace'] = args.namespace
                        config['kind'] = 'ClusterProviderConfig'
                    kube('apply', '-f', '-', payload={'apiVersion': api_group + '/v1alpha1', 'kind': kind, 'metadata': metadata,
                         'spec': {'managementPolicies': ['Observe'], 'forProvider': {}, 'providerConfigRef': config}})
                    lease = (namespace, resource_name(api_group, kind) + '.' + api_group, name)
                    leases.append(lease)
                    def observed(field='name'):
                        obj = observation(lease)
                        conditions = {item.get('type'): item.get('status') for item in obj.get('status', {}).get('conditions', [])}
                        if conditions.get('Ready') != 'True' or conditions.get('Synced') != 'True':
                            return None
                        return obj.get('status', {}).get('atProvider', {}).get(field)
                    wait(kind + ' observe-only Ready and Synced', lambda: observed() == api_name)
                    if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != created['id']:
                        raise RuntimeError('Observe-only ' + kind + ' changed its external name')
                    renamed = 'cp-observe-renamed-' + kind.lower() + '-' + run + '-' + scope
                    if api(path, {'name': renamed}, method)[0] != 200:
                        raise RuntimeError('Unable to rename the observed ' + kind + ' through the API')
                    wait(kind + ' observation follows the API', lambda: observed() == renamed)
                    if api(path)[1].get('name') != renamed:
                        raise RuntimeError('Observe-only ' + kind + ' wrote to the API')
                    delete(lease)
                    status, _ = api(path)
                    if status != 200:
                        raise RuntimeError('Deleting the observe-only ' + kind + ' removed the API object: HTTP ' + str(status))
                    delete(fixture)
                    if api(path)[0] != 404:
                        raise RuntimeError('Observed ' + kind + ' fixture was not removed')
                    record(kind + 'Observe', scope, api_create='passed', observe_only_ready_synced='passed',
                           at_provider_matches_api='passed', external_rename_observed='passed', no_api_writes=True,
                           managed_resource_delete='passed', backend_after_managed_delete=200,
                           api_delete='passed', backend_after_api_delete=404)

            if 'apps' in scope_cases:
                environment, environment_id, _ = create(scope, 'environment', 'Environment', 'environments',
                    {'name': 'cp-app-env-' + run + '-' + scope, 'namespace': 'default'}, '-app')
                catalog, catalog_id, _ = create(scope, 'mcp', 'RegistryEntry', 'registrycatalogitems',
                    {'name': 'cp-app-tools-' + run + '-' + scope, 'multitenant': False,
                     'remoteConfig': {'url': args.mcp_url}}, '-app')
                server, server_id, _ = create(scope, 'mcp', 'Server', 'serverinstallations',
                    {'name': 'cp-app-install-' + run + '-' + scope, 'catalogId': catalog_id, 'scope': 'org'}, '-app')
                code, discovered = api('/api/mcp_server/' + server_id + '/tools')
                if code != 200 or len(discovered) < 2:
                    raise RuntimeError('App fixture did not advertise two upstream tools')
                tool_ids = sorted(item['id'] for item in discovered)[:2]
                initial_html = '<html><head></head><body>Initial App fixture.</body></html>'
                desired_html = '<html><head></head><body>Updated App fixture.</body></html>'
                secret_name = 'cp-app-html-' + run + '-' + scope
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                    'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                    'stringData': {'initial': initial_html, 'updated': desired_html}})
                secret = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret)
                ref = {'name': secret_name, 'key': 'initial'}
                if scope == 'cluster':
                    ref['namespace'] = args.namespace

                def app_metadata(identifier):
                    code, row = api('/api/apps/' + identifier)
                    if code != 200:
                        raise RuntimeError('App metadata read failed: HTTP ' + str(code))
                    return row

                def app_content(identifier):
                    row = app_metadata(identifier)
                    code, version = api('/api/apps/' + identifier + '/versions/' + str(row['latestVersion']))
                    if code != 200 or version['appId'] != identifier or version['version'] != row['latestVersion']:
                        raise RuntimeError('App immutable content version was not readable')
                    return version

                def app_tools(identifier):
                    code, tools = api('/api/apps/' + identifier + '/tools')
                    if code != 200:
                        raise RuntimeError('App assignment read failed')
                    return {tool['id']: (tool.get('mcpServerId'), tool.get('credentialResolutionMode')) for tool in tools}

                name = 'cp-app-' + run + '-' + scope
                fields = {'name': name, 'htmlSecretRef': ref, 'browserPermissions': ['camera'],
                    'environmentIdRef': {'name': environment[2]}, 'enabled': True, 'locked': False,
                    'openInFullscreen': False, 'labels': [{'key': 'stage', 'value': 'initial'}],
                    'permissions': [{'subjectType': 'organization', 'subjectId': '*', 'actions': ['read']}],
                    'tools': [{'toolId': tool_id, 'mcpServerIdRef': {'name': server[2]}, 'credentialResolutionMode': 'static'} for tool_id in tool_ids]}
                app, identifier, obj = create(scope, 'app', 'App', 'apps', fields)
                path = '/api/apps/' + identifier
                if app_metadata(identifier)['environmentId'] != environment_id or app_content(identifier)['html'] != initial_html:
                    raise RuntimeError('App environment reference or Secret HTML was not persisted')
                if app_tools(identifier) != {tool_id: (server_id, 'static') for tool_id in tool_ids}:
                    raise RuntimeError('App tool server references or credential modes did not persist')
                if initial_html in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Sensitive App HTML leaked into managed resource status')
                policy_path = '/api/resource-permissions/app/' + identifier
                code, policy = api(policy_path)
                if code != 200 or not any(g['subject'] == {'type': 'organization', 'id': '*'} for g in policy['grants']):
                    raise RuntimeError('App creation sharing was not persisted')
                renamed = name + '-updated'
                next_ref = dict(ref, key='updated')
                dynamic_tools = [{'toolId': tool_ids[0], 'credentialResolutionMode': 'dynamic'}]
                patch(app, {'name': renamed, 'htmlSecretRef': next_ref, 'browserPermissions': [],
                    'environmentId': None, 'environmentIdRef': None, 'openInFullscreen': True,
                    'labels': [], 'permissions': [], 'tools': dynamic_tools})
                wait('App versioned update, assignments and clears', lambda: condition(app, 'True')
                    and app_metadata(identifier)['name'] == renamed and app_metadata(identifier)['environmentId'] is None
                    and app_metadata(identifier)['openInFullscreen'] is True and not app_metadata(identifier)['labels']
                    and app_content(identifier)['html'] == desired_html and not app_content(identifier)['uiPermissions']
                    and app_tools(identifier) == {tool_ids[0]: (None, 'dynamic')})
                code, policy = api(policy_path)
                if code != 200 or any(g['subject'] == {'type': 'organization', 'id': '*'} for g in policy['grants']):
                    raise RuntimeError('App sharing clear was not persisted')
                base_version = app_metadata(identifier)['latestVersion']
                if api(path, {'name': 'Out of band ' + run, 'html': '<html><head></head><body>External edit</body></html>',
                             'baseVersion': base_version})[0] != 200:
                    raise RuntimeError('Unable to inject owned App content drift')
                if api(path + '/tools/' + tool_ids[1], {'mcpServerId': server_id, 'credentialResolutionMode': 'static'}, 'POST')[0] != 200:
                    raise RuntimeError('Unable to inject owned App assignment drift')
                wait('App content and assignment drift repair', lambda: app_metadata(identifier)['name'] == renamed
                    and app_content(identifier)['html'] == desired_html and app_tools(identifier) == {tool_ids[0]: (None, 'dynamic')})
                patch(app, {'enabled': False, 'locked': True})
                wait('App explicit disable and lock', lambda: condition(app, 'True') and app_metadata(identifier)['enabled'] is False and app_metadata(identifier)['locked'] is True)
                patch(app, {'enabled': True, 'locked': False})
                wait('App explicit enable and unlock', lambda: condition(app, 'True') and app_metadata(identifier)['enabled'] is True and app_metadata(identifier)['locked'] is False)
                kube(*app[0], 'patch', app[1], app[2], '--type=merge', '-p',
                     json.dumps({'spec': {'managementPolicies': ['Observe', 'Create', 'Update', 'LateInitialize']}}))
                delete(app)
                if api(path)[0] != 200:
                    raise RuntimeError('Orphaned App was deleted')
                adopted_fields = {'name': renamed, 'htmlSecretRef': next_ref, 'browserPermissions': [],
                    'enabled': True, 'locked': False, 'openInFullscreen': True, 'labels': [], 'permissions': [], 'tools': dynamic_tools}
                adopted, adopted_id, _ = create(scope, 'app', 'App', 'apps', adopted_fields, '-adopt', external_name=identifier)
                if adopted_id != identifier:
                    raise RuntimeError('App UUID adoption changed identity')
                delete(adopted)
                if api(path)[0] != 404:
                    raise RuntimeError('Deleted App remains readable')
                record('App', scope, create='passed', secret_html='passed', environment_reference='passed',
                    tool_server_references='passed', browser_permissions_clear='passed', environment_clear='passed',
                    sharing_clear='passed', content_update='passed', content_and_tool_drift_repair='passed',
                    credential_mode_and_pin_clear='passed', explicit_lifecycle='passed', orphan='passed', uuid_adoption='passed', delete='passed')
                initialized, initialized_id, _ = create(scope, 'app', 'App', 'apps', {}, '-init',
                    init_fields={'name': name + '-init', 'htmlSecretRef': ref, 'browserPermissions': [], 'enabled': True, 'locked': False})
                if app_content(initialized_id)['html'] != initial_html:
                    raise RuntimeError('App initProvider Secret HTML was not persisted')
                external_html = '<html><head></head><body>Externally authored init App.</body></html>'
                current_version = app_metadata(initialized_id)['latestVersion']
                if api('/api/apps/' + initialized_id, {'html': external_html, 'baseVersion': current_version})[0] != 200:
                    raise RuntimeError('Unable to edit initialized App content')
                time.sleep(8)
                if app_content(initialized_id)['html'] != external_html or not condition(initialized, 'True'):
                    raise RuntimeError('App initProvider content was managed again after creation')
                delete(initialized)
                if api('/api/apps/' + initialized_id)[0] != 404:
                    raise RuntimeError('Deleted initProvider App remains readable')
                record('AppInit', scope, create='passed', init_provider_secret='passed', creation_only_content='passed', delete='passed')
                delete(secret)
                delete(server)
                delete(catalog)
                delete(environment)

            if 'project-file-init' in scope_cases:
                import base64
                raw = b'Synthetic initProvider project file.'
                project, project_id, _ = create(scope, 'project', 'Project', 'projects',
                    {'name': 'cp-project-init-' + run + '-' + scope}, '-file-init')
                secret_name = 'cp-project-init-' + run + '-' + scope
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                    'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                    'stringData': {'content': base64.b64encode(raw).decode()}})
                secret = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret)
                ref = {'name': secret_name, 'key': 'content'}
                if scope == 'cluster':
                    ref['namespace'] = args.namespace
                document, identifier, _ = create(scope, 'project', 'File', 'files',
                    {'projectIdRef': {'name': project[2]}, 'filename': 'init-notes.md'}, '-init',
                    init_fields={'contentBase64SecretRef': ref})
                if identifier.split(':')[0] != project_id:
                    raise RuntimeError('initProvider project file lost its project reference')
                request = urllib.request.Request(args.api_url.rstrip('/') + '/api/skill-sandbox/artifacts/' + identifier.split(':')[1],
                                                 headers={'Authorization': token})
                with opener.open(request, timeout=30) as response:
                    if response.read() != raw:
                        raise RuntimeError('initProvider project file did not persist Secret bytes')
                delete(document)
                if api('/api/skill-sandbox/artifacts/' + identifier.split(':')[1])[0] != 404:
                    raise RuntimeError('Deleted initProvider project file remains readable')
                record('ProjectFileInit', scope, create='passed', init_provider_secret='passed', project_reference='passed', delete='passed')
                delete(secret)
                delete(project)

            if 'knowledge-files' in scope_cases:
                import base64
                raw = b'# Synthetic document\nPrivate Knowledge lifecycle fixture.\n'
                directory, directory_id, _ = create(scope, 'knowledge', 'Directory', 'directories',
                    {'name': 'cp-knowledge-files-' + run + '-' + scope}, '-files')
                secret_name = 'cp-knowledge-bytes-' + run + '-' + scope
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                    'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                    'stringData': {'content': base64.b64encode(raw).decode(), 'replacement': base64.b64encode(b'Replacement bytes.').decode()}})
                secret = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret)
                ref = {'name': secret_name, 'key': 'content'}
                if scope == 'cluster':
                    ref['namespace'] = args.namespace

                def knowledge_bytes(identifier):
                    request = urllib.request.Request(args.api_url.rstrip('/') + '/api/knowledge-files/' + identifier + '/content', headers={'Authorization': token})
                    try:
                        with opener.open(request, timeout=30) as response:
                            return response.status, response.read()
                    except urllib.error.HTTPError as error:
                        return error.code, None

                def knowledge_row(identifier):
                    offset = 0
                    while True:
                        code, page = api('/api/knowledge-files?limit=100&offset=' + str(offset))
                        if code != 200:
                            raise RuntimeError('Knowledge repository read failed: HTTP ' + str(code))
                        row = next((item for item in page['data'] if item['id'] == identifier), None)
                        if row or not page['pagination']['hasNext']:
                            return row
                        if not page['data']:
                            raise RuntimeError('Knowledge repository returned invalid pagination')
                        offset += len(page['data'])

                filename = 'cp-document-' + run + '-' + scope + '.md'
                fields = {'filename': filename, 'mimeType': 'text/markdown', 'contentBase64SecretRef': ref,
                          'directoryIdRef': {'name': directory[2]}, 'labels': [{'key': 'stage', 'value': 'initial'}],
                          'permissions': [{'subjectType': 'organization', 'subjectId': '*', 'actions': ['read']}]}
                document, identifier, obj = create(scope, 'knowledge', 'File', 'files', fields)
                row = knowledge_row(identifier)
                if row['directoryId'] != directory_id or knowledge_bytes(identifier) != (200, raw):
                    raise RuntimeError('Knowledge file bytes or directory reference were not persisted')
                if base64.b64encode(raw).decode() in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Knowledge file bytes leaked into managed resource status')
                policy_path = '/api/resource-permissions/knowledgeFile/' + identifier
                code, policy = api(policy_path)
                if code != 200 or not any(g['subject'] == {'type': 'organization', 'id': '*'} and g['actions'] == ['read'] for g in policy['grants']):
                    raise RuntimeError('Knowledge file creation did not persist sharing')
                renamed = 'renamed-' + filename
                patch(document, {'filename': renamed, 'labels': [], 'permissions': [], 'directoryIdRef': None, 'directoryId': None})
                wait('Knowledge file rename, label and directory clear', lambda: condition(document, 'True')
                     and knowledge_row(identifier)['filename'] == renamed and not knowledge_row(identifier)['labels']
                     and knowledge_row(identifier)['directoryId'] is None)
                code, policy = api(policy_path)
                if code != 200 or any(g['subject'] == {'type': 'organization', 'id': '*'} for g in policy['grants']):
                    raise RuntimeError('Knowledge file sharing was not cleared')
                if api('/api/knowledge-files/' + identifier, {'filename': 'out-of-band-' + filename, 'labels': [{'key': 'stage', 'value': 'drift'}]})[0] != 200:
                    raise RuntimeError('Unable to inject owned Knowledge metadata drift')
                wait('Knowledge file metadata drift repair', lambda: knowledge_row(identifier)['filename'] == renamed and not knowledge_row(identifier)['labels'])
                immutable_update(document, {'contentBase64SecretRef': dict(ref, key='replacement')}, {'contentBase64SecretRef': ref})
                if knowledge_bytes(identifier) != (200, raw):
                    raise RuntimeError('Refused content replacement changed the original document')
                kube(*document[0], 'patch', document[1], document[2], '--type=merge', '-p',
                     json.dumps({'spec': {'managementPolicies': ['Observe', 'Create', 'Update', 'LateInitialize']}}))
                delete(document)
                if knowledge_bytes(identifier) != (200, raw):
                    raise RuntimeError('Orphaned Knowledge file was deleted')
                adopted_fields = {'filename': renamed, 'mimeType': 'text/markdown', 'contentBase64SecretRef': ref, 'labels': [], 'permissions': []}
                adopted, adopted_id, _ = create(scope, 'knowledge', 'File', 'files', adopted_fields, '-adopt', external_name=identifier)
                if adopted_id != identifier:
                    raise RuntimeError('Knowledge file adoption changed identity')
                delete(adopted)
                if knowledge_bytes(identifier)[0] != 404:
                    raise RuntimeError('Deleted Knowledge file remains readable')
                record('KnowledgeFile', scope, create='passed', directory_reference='passed', secret_input='passed',
                       metadata_update='passed', metadata_drift_repair='passed', sharing_clear='passed', directory_clear='passed',
                       immutable_content_refusal='passed', orphan='passed', uuid_adoption='passed', delete='passed')
                initial = {'filename': 'init-' + filename, 'mimeType': 'text/markdown', 'contentBase64SecretRef': ref,
                           'labels': [], 'permissions': []}
                initialized, initialized_id, _ = create(scope, 'knowledge', 'File', 'files', {}, '-init', init_fields=initial)
                if knowledge_bytes(initialized_id) != (200, raw):
                    raise RuntimeError('Knowledge file initProvider did not persist bytes')
                delete(initialized)
                if knowledge_bytes(initialized_id)[0] != 404:
                    raise RuntimeError('initProvider Knowledge file remains readable after deletion')
                record('KnowledgeFileInit', scope, create='passed', init_provider='passed', delete='passed')
                delete(secret)
                delete(directory)

            if 'project-files' in scope_cases:
                import base64
                project, project_id, _ = create(scope, 'project', 'Project', 'projects', {'name': 'cp-files-' + run + '-' + scope})
                secret_name = 'cp-file-input-' + run + '-' + scope
                initial = b'Synthetic project text.'
                desired = b'Updated synthetic text.'
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                    'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                    'stringData': {'content': base64.b64encode(initial).decode()}})
                secret_lease = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret_lease)
                secret_ref = {'name': secret_name, 'key': 'content'}
                if scope == 'cluster':
                    secret_ref['namespace'] = args.namespace
                def file_bytes(identifier):
                    request = urllib.request.Request(args.api_url.rstrip('/') + '/api/skill-sandbox/artifacts/' + identifier,
                                                     headers={'Authorization': token})
                    try:
                        with opener.open(request, timeout=30) as response:
                            return response.status, response.read()
                    except urllib.error.HTTPError as error:
                        return error.code, None
                text, text_id, obj = create(scope, 'project', 'File', 'files', {
                    'projectIdRef': {'name': project[2]}, 'filename': 'notes.txt', 'contentBase64SecretRef': secret_ref})
                file_id = text_id.split(':')[1]
                if text_id.split(':')[0] != project_id or file_bytes(file_id) != (200, initial):
                    raise RuntimeError('Scoped project file creation did not persist the declared bytes')
                if base64.b64encode(initial).decode() in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Project file content leaked into resource status')
                kube('-n', args.namespace, 'patch', 'secret', secret_name, '--type=merge', '-p',
                     json.dumps({'stringData': {'content': base64.b64encode(desired).decode()}}))
                wait('Project file text update', lambda: file_bytes(file_id) == (200, desired))
                if api('/api/skill-sandbox/artifacts/' + file_id + '/content', {'content': 'Out of band'}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to modify owned file bytes')
                wait('Project file content drift repair', lambda: file_bytes(file_id) == (200, desired))
                wait('Project file digest observation', lambda: observation(text).get('status', {}).get('atProvider', {}).get('sha256') == hashlib.sha256(desired).hexdigest())
                kube('-n', args.namespace, 'patch', 'secret', secret_name, '--type=merge', '-p', json.dumps({'stringData': {'content': ''}}))
                wait('Project text file clear', lambda: file_bytes(file_id) == (200, b''))
                wait('Project empty file size', lambda: observation(text).get('status', {}).get('atProvider', {}).get('sizeBytes') == 0)
                kube('-n', args.namespace, 'patch', 'secret', secret_name, '--type=merge', '-p', json.dumps({'stringData': {'content': base64.b64encode(desired).decode()}}))
                wait('Project text restored before replacement probe', lambda: file_bytes(file_id) == (200, desired))
                immutable_update(text, {'filename': 'renamed.txt'}, {'filename': 'notes.txt'})
                if observation(text)['metadata']['annotations']['crossplane.io/external-name'] != text_id:
                    raise RuntimeError('Text editing changed the scoped file identity')
                delete(text)
                if file_bytes(file_id)[0] != 404:
                    raise RuntimeError('Project file remained downloadable after deletion')
                record('ProjectFileText', scope, create='passed', project_reference='passed', sensitive_input_reference='passed',
                       update_same_identity='passed', drift_repair='passed', byte_hash_observed='passed', empty_content_clear='passed',
                       filename_replacement_refused='passed', sensitive_content_absent_from_status=True, delete='passed', backend_after_delete=404)
                initial_binary = base64.b64encode(b'\x00\xff\x01').decode()
                changed_binary = base64.b64encode(b'\x00\xfe\x02').decode()
                kube('-n', args.namespace, 'patch', 'secret', secret_name, '--type=merge', '-p', json.dumps({'stringData': {'content': initial_binary}}))
                binary, binary_id, _ = create(scope, 'project', 'File', 'files', {
                    'projectIdRef': {'name': project[2]}, 'filename': 'blob.bin', 'contentBase64SecretRef': secret_ref}, '-binary')
                binary_file = binary_id.split(':')[1]
                if file_bytes(binary_file) != (200, b'\x00\xff\x01'):
                    raise RuntimeError('Binary file bytes changed on upload')
                kube('-n', args.namespace, 'patch', 'secret', secret_name, '--type=merge', '-p', json.dumps({'stringData': {'content': changed_binary}}))
                wait('Binary content replacement refusal', lambda: any(item.get('type') == 'Synced' and item.get('status') == 'False'
                     and any(marker in item.get('message', '').lower() for marker in ('replac', 'prevent_destroy'))
                     for item in observation(binary).get('status', {}).get('conditions', [])))
                if observation(binary)['metadata']['annotations']['crossplane.io/external-name'] != binary_id or file_bytes(binary_file) != (200, b'\x00\xff\x01'):
                    raise RuntimeError('Refused binary replacement changed existing bytes or identity')
                kube('-n', args.namespace, 'patch', 'secret', secret_name, '--type=merge', '-p', json.dumps({'stringData': {'content': initial_binary}}))
                wait('Binary replacement refusal recovery', lambda: condition(binary, 'True'))
                delete(binary)
                if file_bytes(binary_file)[0] != 404:
                    raise RuntimeError('Binary file remained downloadable after deletion')
                record('ProjectFileBinary', scope, create='passed', exact_bytes='passed', replacement_refused='passed',
                       identity_and_bytes_preserved=True, recovery='passed', delete='passed', backend_after_delete=404)
                delete(secret_lease)
                delete(project)

            if 'marketplace-share-links' in scope_cases:
                import base64
                def share_connection(lease, key):
                    secret = json.loads(kube('-n', args.namespace, 'get', 'secret', lease[2] + '-connection', '-o', 'json'))
                    return base64.b64decode(secret['data']['attribute.' + key]).decode()
                def share_metadata(identifier):
                    code, page = api('/api/skill-share-links')
                    if code != 200:
                        raise RuntimeError('Unable to read owned share-link metadata')
                    return next((item for item in page['links'] if item['id'] == identifier), None)
                def share_access(clone_url):
                    path = urllib.parse.urlparse(clone_url).path + '/info/refs?service=git-upload-pack'
                    request = urllib.request.Request(args.api_url.rstrip('/') + path)
                    try:
                        with opener.open(request, timeout=30) as response:
                            response.read()
                            return response.status
                    except urllib.error.HTTPError as error:
                        return error.code
                    except urllib.error.URLError:
                        raise RuntimeError('Local clone access transport failed') from None
                parent, parent_id, _ = create(scope, 'skill', 'Skill', 'skills', {
                    'name': 'cp-share-skill-' + run + '-' + scope, 'description': 'Synthetic link fixture',
                    'content': 'Synthetic shared content.', 'files': {}, 'environmentIds': [], 'labels': []}, '-share')
                package, package_id, _ = create(scope, 'plugin', 'Plugin', 'plugins', {
                    'displayName': 'cp-share-plugin-' + run + '-' + scope, 'clientType': 'claude-code', 'supportedPlatforms': ['posix'],
                    'enabled': True, 'files': {'.claude-plugin/plugin.json': {'content': '{"name":"synthetic-share","version":"1.0.0"}', 'encoding': 'utf8', 'mode': '100644'}}}, '-share')
                for label, fields in [
                    ('Skills', {'skillIdsRefs': [{'name': parent[2]}], 'pluginIds': []}),
                    ('Plugins', {'skillIds': [], 'pluginIdsRefs': [{'name': package[2]}], 'pluginPlatform': 'posix', 'expiresAt': '2040-01-01T00:00:00Z'}),
                ]:
                    lease, identifier, obj = create(scope, 'marketplace', 'ShareLink', 'sharelinks', {
                        'name': 'cp-share-' + label.lower() + '-' + run + '-' + scope, 'rotationNonce': '1', **fields}, '-' + label.lower())
                    raw = wait('Share-link token publication', lambda: share_connection(lease, 'raw_token'))
                    clone = wait('Share-link URL publication', lambda: share_connection(lease, 'clone_url'))
                    if not raw.startswith('archestra_skl_') or raw not in clone or raw in json.dumps(obj.get('status', {})):
                        raise RuntimeError('Share-link secret publication or status redaction failed')
                    expected = parent_id if label == 'Skills' else package_id
                    member_key = 'skills' if label == 'Skills' else 'plugins'
                    if [item['id'] for item in share_metadata(identifier)[member_key]] != [expected] or share_access(clone) != 200:
                        raise RuntimeError('Share-link references or anonymous read-only access failed')
                    time.sleep(4)
                    if share_connection(lease, 'raw_token') != raw:
                        raise RuntimeError('Share-link refresh erased the creation-only secret')
                    immutable_update(lease, {'rotationNonce': '2'}, {'rotationNonce': '1'})
                    if api('/api/skill-share-links/' + identifier, method='DELETE')[0] != 200:
                        raise RuntimeError('Unable to inject owned share-link revocation drift')
                    replacement_id = wait('Share-link revoked identity recreated', lambda: (value if (value := observation(lease)['metadata']['annotations']['crossplane.io/external-name']) != identifier else None))
                    replacement_raw = wait('Share-link replacement token publication', lambda: (value if (value := share_connection(lease, 'raw_token')) != raw else None))
                    replacement_clone = wait('Share-link replacement URL publication', lambda: (value if (value := share_connection(lease, 'clone_url')) != clone else None))
                    if share_access(clone) != 404 or share_access(replacement_clone) != 200 or not replacement_raw:
                        raise RuntimeError('Revocation drift did not replace and revoke the token safely')
                    delete(lease)
                    if share_metadata(replacement_id)['status'] != 'revoked' or share_access(replacement_clone) != 404:
                        raise RuntimeError('Managed share-link deletion failed to revoke public access')
                    record('ShareLink' + label, scope, create='passed', membership_references='passed', secret_once_publication='passed',
                           refresh_preserves_secret='passed', anonymous_read_only_access='passed', explicit_rotation_replacement_refused='passed',
                           revoked_identity_drift_repair='passed', old_token_rejected='passed', delete='passed', backend_after_delete='revoked')
                expired, expired_id, _ = create(scope, 'marketplace', 'ShareLink', 'sharelinks', {
                    'name': 'cp-expired-share-' + run + '-' + scope, 'skillIds': [], 'pluginIdsRefs': [{'name': package[2]}],
                    'pluginPlatform': 'posix', 'expiresAt': '2001-01-01T00:00:00Z'}, '-expired')
                expired_clone = wait('Expired-link secret publication', lambda: share_connection(expired, 'clone_url'))
                time.sleep(7)
                if share_metadata(expired_id)['status'] != 'expired' or share_access(expired_clone) != 404 or observation(expired)['metadata']['annotations']['crossplane.io/external-name'] != expired_id:
                    raise RuntimeError('Expired share link was recreated or remained publicly readable')
                delete(expired)
                if share_metadata(expired_id)['status'] != 'revoked':
                    raise RuntimeError('Expired link did not revoke normally on deletion')
                delete(package)
                delete(parent)
                record('ShareLinkExpiry', scope, create='passed', expired_observation='passed', anonymous_access_rejected='passed',
                       identity_retained_across_reconciliation='passed', delete='passed', backend_after_delete='revoked')

            if 'repository-plugins' in scope_cases:
                package, package_id, _ = create(scope, 'plugin', 'Plugin', 'plugins', {
                    'displayName': 'cp-source-plugin-' + run + '-' + scope, 'clientType': 'claude-code', 'supportedPlatforms': ['posix'],
                    'enabled': False, 'labels': [{'key': 'purpose', 'value': 'source'}], **git_plugin_source_fields()})
                path = '/api/plugins/' + package_id
                initial = api(path)[1]
                content_hash = initial['contentHash']
                if initial['sourceSha'] != GIT_PLUGIN_FIXTURE_SHA or initial['sourceKind'] != 'github' or initial['enabled'] or len(initial['files']) != 2:
                    raise RuntimeError('Pinned repository import did not preserve approved source bytes and controls')
                patch(package, {'displayName': 'Updated source plugin ' + run, 'description': 'Metadata only', 'enabled': True})
                wait('Repository plugin metadata update', lambda: api(path)[1]['displayName'] == 'Updated source plugin ' + run and api(path)[1]['enabled'])
                if api(path)[1]['contentHash'] != content_hash:
                    raise RuntimeError('Repository metadata update changed approved bytes')
                if api(path, {'description': 'Out of band', 'enabled': False, 'githubSource': {
                        'repoUrl': initial['sourceRepo'], 'ref': GIT_PLUGIN_FIXTURE_SHA, 'syncInterval': '1h',
                        'authentication': {'githubAppConfigId': None, 'githubPatId': None, 'gitlabTokenId': None}}}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to inject owned source configuration drift')
                wait('Repository source and metadata drift repair', lambda: api(path)[1]['description'] == 'Metadata only'
                     and api(path)[1]['enabled'] and api(path)[1]['githubSyncInterval'] is None and api(path)[1]['githubSyncRef'] is None)
                patch(package, {'gitSource': {'trackingRef': GIT_PLUGIN_FIXTURE_SHA, 'syncInterval': '1d'}})
                wait('Repository candidate-check configuration', lambda: api(path)[1]['githubSyncRef'] == GIT_PLUGIN_FIXTURE_SHA and api(path)[1]['githubSyncInterval'] == '1d')
                secret_name = 'cp-source-pat-' + run + '-' + scope
                kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                    'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                    'stringData': {'input': 'synthetic-local-only'}})
                leases.append((['-n', args.namespace], 'secrets', secret_name))
                ref = {'name': secret_name, 'key': 'input'}
                if scope == 'cluster':
                    ref['namespace'] = args.namespace
                pat, pat_id, _ = create(scope, 'credential', 'GitHubPAT', 'githubpats', {
                    'name': 'cp-source-auth-' + run + '-' + scope, 'tokenSecretRef': ref})
                patch(package, {'gitSource': {'syncInterval': None, 'githubPatIdRef': {'name': pat[2]}}})
                wait('Repository existing credential reference', lambda: api(path)[1]['githubPatId'] == pat_id and api(path)[1]['githubSyncInterval'] is None)
                patch(package, {'gitSource': {'githubPatIdRef': None, 'githubPatId': None, 'trackingRef': None}, 'labels': []})
                wait('Repository authentication/tracking clear', lambda: api(path)[1]['githubPatId'] is None
                     and api(path)[1]['githubSyncRef'] is None and api(path)[1]['githubSyncInterval'] is None and not api(path)[1]['labels'])
                final = api(path)[1]
                if final['sourceSha'] != GIT_PLUGIN_FIXTURE_SHA or final['contentHash'] != content_hash:
                    raise RuntimeError('Source control changes altered approved package bytes')
                immutable_update(package, {'gitSource': {'commitSha': '0' * 40}}, {'gitSource': {'commitSha': GIT_PLUGIN_FIXTURE_SHA}})
                immutable_update(package, {'gitSource': {'exclude': []}}, {'gitSource': {'exclude': ['excluded.txt']}})
                delete(package)
                if api(path)[0] != 404:
                    raise RuntimeError('Repository package survived normal deletion')
                delete(pat)
                record('PluginGitSource', scope, create='passed', approved_commit=GIT_PLUGIN_FIXTURE_SHA, derived_files='passed', initially_disabled='passed',
                       metadata_update_preserves_hash=True, update='passed', drift_repair='passed', scheduled_candidate_configuration='passed',
                       stored_credential_reference_and_clear='passed', empty_labels_clear='passed', source_selection_replacement_refused='passed',
                       delete='passed', hooks_executed=False)

            if 'manual-plugins' in scope_cases:
                plugin_files = {
                    '.claude-plugin/plugin.json': {'content': '{"name":"synthetic-plugin","version":"1.0.0"}', 'encoding': 'utf8', 'mode': '100644'},
                    'README.md': {'content': 'Synthetic plugin fixture.', 'encoding': 'utf8', 'mode': '100644'},
                    'scripts/example0.sh': {'content': '#!/bin/sh\nprintf "synthetic\\n"\n', 'encoding': 'utf8', 'mode': '100755'},
                    'assets/icon.bin': {'content': 'AAECAwQ=', 'encoding': 'base64', 'mode': '100644'},
                }
                package, package_id, _ = create(scope, 'plugin', 'Plugin', 'plugins', {
                    'displayName': 'cp-plugin-' + run + '-' + scope, 'clientType': 'claude-code', 'supportedPlatforms': ['posix'],
                    'enabled': False, 'files': plugin_files, 'labels': [{'key': 'purpose', 'value': 'plugin'}]})
                path = '/api/plugins/' + package_id
                initial = api(path)[1]
                initial_hash = initial['contentHash']
                if initial['enabled'] or next(item for item in initial['files'] if item['path'] == 'scripts/example0.sh')['mode'] != '100755':
                    raise RuntimeError('Plugin initial disablement or executable mode was lost')
                patch(package, {'displayName': 'Updated plugin ' + run, 'description': 'Metadata only', 'enabled': True})
                wait('Plugin metadata update', lambda: api(path)[1]['displayName'] == 'Updated plugin ' + run and api(path)[1]['enabled'])
                if api(path)[1]['contentHash'] != initial_hash:
                    raise RuntimeError('Plugin metadata update changed file bytes')
                if api(path, {'displayName': 'Out of band plugin', 'enabled': False}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to inject owned plugin metadata drift')
                wait('Plugin metadata drift repair', lambda: api(path)[1]['displayName'] == 'Updated plugin ' + run and api(path)[1]['enabled'])
                desired_files = json.loads(json.dumps(plugin_files))
                desired_files.pop('assets/icon.bin')
                desired_files['README.md']['content'] = 'Desired updated bytes.'
                desired_files['scripts/example0.sh']['mode'] = '100644'
                replace_fields(package, {'files': desired_files})
                patch(package, {'labels': []})
                def desired_package():
                    code, value = api(path)
                    if code != 200:
                        return False
                    observed = {item['path']: {key: item[key] for key in ('content', 'encoding', 'mode')} for item in value['files']}
                    return observed == desired_files and value['contentHash'] != initial_hash and not value['labels']
                wait('Authoritative plugin file and label update', desired_package)
                drift_files = [{'path': name, **item} for name, item in desired_files.items()]
                next(item for item in drift_files if item['path'] == 'README.md')['content'] = 'Out of band bytes.'
                if api(path, {'files': drift_files}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to inject owned plugin file drift')
                wait('Plugin byte drift repair', desired_package)
                immutable_update(package, {'clientType': 'codex'}, {'clientType': 'claude-code'})
                delete(package)
                if api(path)[0] != 404:
                    raise RuntimeError('Plugin package survived deletion')
                record('PluginFiles', scope, create='passed', initially_disabled='passed', utf8_base64_and_executable_mode='passed',
                       metadata_update_preserves_hash=True, update='passed', drift_repair='passed', authoritative_file_removal='passed',
                       empty_labels_clear='passed', client_format_replacement_refused='passed', delete='passed', hooks_executed=False)

            if 'github-registrations' in scope_cases:
                for kind, plural, endpoint, secret_field, wire_secret, extra in [
                    ('GitHubPAT', 'githubpats', '/api/github-pats', 'tokenSecretRef', 'token', {}),
                    ('GitHubApp', 'githubappconfigs', '/api/github-app-configs', 'privateKeySecretRef', 'privateKey',
                     {'githubUrl': 'http://127.0.0.1:19997', 'appId': '100', 'installationId': '200'}),
                ]:
                    secret_name = 'cp-github-input-' + kind.lower() + '-' + run + '-' + scope
                    def write_github_input(value):
                        kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                            'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                            'stringData': {'input': value}})
                    input_value = 'synthetic-local-only'
                    write_github_input(input_value)
                    secret_lease = (['-n', args.namespace], 'secrets', secret_name)
                    leases.append(secret_lease)
                    ref = {'name': secret_name, 'key': 'input'}
                    if scope == 'cluster':
                        ref['namespace'] = args.namespace
                    lease, identifier, obj = create(scope, 'credential', kind, plural,
                        {'name': 'cp-github-' + kind.lower() + '-' + run + '-' + scope, secret_field: ref, **extra})
                    def github_value():
                        if kind == 'GitHubApp':
                            code, value = api(endpoint + '/' + identifier)
                            return value if code == 200 else None
                        code, rows = api(endpoint)
                        if code != 200:
                            raise RuntimeError('Unable to list stored GitHub registrations')
                        return next((item for item in rows if item['id'] == identifier), None)
                    value = github_value()
                    if value is None or any(key in value for key in ('token', 'privateKey', 'secretId')) or input_value in json.dumps(obj.get('status', {})):
                        raise RuntimeError('Stored GitHub registration leaked a credential or failed creation')
                    updated_name = 'Updated GitHub ' + kind + ' ' + run
                    patch(lease, {'name': updated_name})
                    wait('GitHub registration metadata update', lambda: github_value()['name'] == updated_name)
                    if api(endpoint + '/' + identifier, {'name': 'Out of band GitHub registration'}, 'PUT')[0] != 200:
                        raise RuntimeError('Unable to inject owned GitHub registration drift')
                    wait('GitHub registration drift repair', lambda: github_value()['name'] == updated_name)
                    write_github_input('synthetic-local-replacement')
                    patch(lease, {'name': updated_name + ' credential changed'})
                    wait('GitHub credential input update', lambda: condition(lease, 'True') and github_value()['name'] == updated_name + ' credential changed')
                    if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier:
                        raise RuntimeError('Stored credential update replaced its registration identity')
                    if kind == 'GitHubApp':
                        patch(lease, {'installationId': '201'})
                        wait('Existing GitHub installation binding update', lambda: github_value()['installationId'] == '201')
                    delete(lease)
                    if github_value() is not None:
                        raise RuntimeError('Stored GitHub registration survived deletion')
                    delete(secret_lease)
                    record(kind, scope, create='passed', update='passed', drift_repair='passed', sensitive_input_update='passed',
                           identity_preserved=True, credential_absent_from_api_and_status=True, delete='passed', external_github_requests='not_invoked')

            if 'api-compatibility' in scope_cases:
                config_name = 'cp-api-constraint-' + run + '-' + scope
                secret_name = config_name + '-credentials'
                def write_constraint(constraint):
                    credentials = json.dumps({'api_key': token, 'base_url': args.api_url, 'api_version_constraint': constraint})
                    kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                        'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                        'stringData': {'credentials': credentials}})
                write_constraint('= 9.9.9')
                secret_lease = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret_lease)
                config_group = 'archestra.crossplane.io' if scope == 'cluster' else 'archestra.m.crossplane.io'
                config_ns = [] if scope == 'cluster' else ['-n', args.namespace]
                config_metadata = {'name': config_name, 'labels': {'provider-test-run': run}}
                if scope == 'namespaced':
                    config_metadata['namespace'] = args.namespace
                kube('apply', '-f', '-', payload={'apiVersion': config_group + '/v1beta1', 'kind': 'ProviderConfig',
                    'metadata': config_metadata, 'spec': {'credentials': {'source': 'Secret',
                    'secretRef': {'name': secret_name, 'namespace': args.namespace, 'key': 'credentials'}}}})
                config_lease = (config_ns, 'providerconfigs.' + config_group, config_name)
                leases.append(config_lease)
                env_name = 'cp-constraint-environment-' + run + '-' + scope
                env_group = 'environment.archestra' + ('.m' if scope == 'namespaced' else '') + '.crossplane.io'
                env_metadata = {'name': env_name, 'labels': {'provider-test-run': run}}
                config_ref = {'name': config_name}
                if scope == 'namespaced':
                    env_metadata['namespace'] = args.namespace
                    config_ref['kind'] = 'ProviderConfig'
                kube('apply', '-f', '-', payload={'apiVersion': env_group + '/v1alpha1', 'kind': 'Environment',
                    'metadata': env_metadata, 'spec': {'forProvider': {'name': env_name}, 'providerConfigRef': config_ref}})
                lease = (config_ns, 'environments.' + env_group, env_name)
                leases.append(lease)
                try:
                    wait('Deployment API constraint rejects configuration', lambda: any(
                        item.get('type') == 'Synced' and item.get('status') == 'False'
                        and 'version' in item.get('message', '').lower() and 'constraint' in item.get('message', '').lower()
                        for item in observation(lease).get('status', {}).get('conditions', [])))
                    if observation(lease)['metadata'].get('annotations', {}).get('crossplane.io/external-name'):
                        raise RuntimeError('Mismatched API constraint allowed resource creation')
                    code, page = api('/api/environments')
                    if code != 200 or any(item['name'] == env_name for item in page['environments']):
                        raise RuntimeError('Mismatched API constraint allowed a backend mutation')
                finally:
                    # Restore a valid provider configuration before any normal finalizer cleanup.
                    write_constraint('= ' + evidence['api_version'])
                wait('API constraint repair reconciles normally', lambda: condition(lease, 'True') and bool(observation(lease)['metadata'].get('annotations', {}).get('crossplane.io/external-name')))
                identifier = observation(lease)['metadata']['annotations']['crossplane.io/external-name']
                if api('/api/environments/' + identifier)[0] != 200:
                    raise RuntimeError('Matching API constraint did not create the desired resource')
                delete(lease)
                if api('/api/environments/' + identifier)[0] != 404:
                    raise RuntimeError('Recovered compatibility probe survived deletion')
                delete(config_lease)
                delete(secret_lease)
                record('APICompatibilityConstraint', scope, mismatch_rejected='passed', mutation_before_match=False,
                       repair_and_reconciliation='passed', delete='passed', provider_config_kind='ProviderConfig')

            if 'routing-credentials' in scope_cases:
                import base64
                team, team_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-routing-billing-' + run + '-' + scope}, '-routing')
                provider, provider_id, _ = create(scope, 'llm', 'ModelProvider', 'providerkeys',
                    {'name': 'cp-routing-provider-' + run + '-' + scope, 'llmProvider': 'ollama', 'baseUrl': args.llm_base_url, **model_fixture}, '-routing')
                provider_fields = [{'provider': 'ollama', 'providerApiKeyIdRef': {'name': provider[2]}}]
                cap = {'limitValue': 100, 'cleanupInterval': 'calendar_month'}
                def connection_value(lease, key):
                    value = json.loads(kube('-n', args.namespace, 'get', 'secret', lease[2] + '-connection', '-o', 'json'))
                    return base64.b64decode(value['data']['attribute.' + key]).decode()
                def oauth_value(identifier):
                    code, page = api('/api/oauth-clients?limit=100')
                    if code != 200:
                        raise RuntimeError('Unable to read managed OAuth client metadata')
                    return next((item for item in page['data'] if item['id'] == identifier), None)
                def oauth_auth(identifier, secret):
                    request = urllib.request.Request(args.api_url.rstrip('/') + '/api/auth/oauth2/token',
                        data=urllib.parse.urlencode({'grant_type': 'client_credentials', 'client_id': identifier,
                            'client_secret': secret, 'scope': 'llm:proxy'}).encode(),
                        headers={'Content-Type': 'application/x-www-form-urlencoded'}, method='POST')
                    try:
                        with opener.open(request, timeout=20) as response:
                            response.read()
                            return response.status
                    except urllib.error.HTTPError as error:
                        return error.code

                virtual, virtual_id, obj = create(scope, 'llm', 'VirtualKey', 'virtualkeys',
                    {'name': 'cp-virtual-' + run + '-' + scope, 'providerApiKeys': provider_fields,
                     'expiresAt': '2040-01-01T00:00:00Z', 'billingTeamIdRef': {'name': team[2]},
                     'spendCap': cap, 'labels': [{'key': 'purpose', 'value': 'routing'}], 'rotationNonce': '1'})
                virtual_path = '/api/llm-virtual-keys/' + virtual_id
                secret = wait('Virtual key connection publication', lambda: connection_value(virtual, 'value'))
                if secret in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Virtual key leaked into Kubernetes status')
                patch(virtual, {'name': 'Updated virtual ' + run})
                wait('Virtual key metadata update', lambda: api(virtual_path)[1]['name'] == 'Updated virtual ' + run)
                current = api(virtual_path)[1]
                if current['billingTeamId'] != team_id or current['spendCap']['limitValue'] != 100 or not current['expiresAt'].startswith('2040-01-01T00:00:00') or current['providerApiKeys'][0]['providerApiKeyId'] != provider_id or connection_value(virtual, 'value') != secret:
                    raise RuntimeError('Metadata update changed routing, expiry, billing or the virtual credential')
                wire = {'name': 'Out of band virtual', 'keyType': 'standard', 'providerApiKeys': [{'provider': 'ollama', 'providerApiKeyId': provider_id}], 'includeFutureProviders': False, 'expiresAt': '2041-01-01T00:00:00Z'}
                if api(virtual_path, wire, 'PATCH')[0] != 200:
                    raise RuntimeError('Unable to inject owned virtual-key drift')
                wait('Virtual key name and expiry drift repair', lambda: api(virtual_path)[1]['name'] == 'Updated virtual ' + run and api(virtual_path)[1]['expiresAt'].startswith('2040-01-01T00:00:00'))
                patch(virtual, {'expiresAt': None, 'billingTeamId': None, 'billingTeamIdRef': None, 'spendCap': None, 'labels': []})
                wait('Virtual nullable fields and labels clear', lambda: api(virtual_path)[1]['expiresAt'] is None and api(virtual_path)[1]['billingTeamId'] is None and api(virtual_path)[1]['spendCap'] is None and not api(virtual_path)[1]['labels'])
                patch(virtual, {'includeFutureProviders': True, 'providerApiKeys': []})
                wait('Virtual explicit mappings clear', lambda: api(virtual_path)[1]['includeFutureProviders'] and not api(virtual_path)[1]['providerApiKeys'])
                immutable_update(virtual, {'rotationNonce': '2'}, {'rotationNonce': '1'})
                if connection_value(virtual, 'value') != secret:
                    raise RuntimeError('Refused virtual-key replacement changed its credential')
                delete(virtual)
                if api(virtual_path)[0] != 404:
                    raise RuntimeError('Virtual key survived deletion')
                record('VirtualKey', scope, create='passed', nested_provider_reference='passed', update='passed', drift_repair='passed', metadata_preserves_routing_expiry_billing_secret=True, nullable_and_empty_clears='passed', credential_absent_from_status=True, replacement_refused='passed', delete='passed')

                passthrough, passthrough_id, _ = create(scope, 'llm', 'VirtualKey', 'virtualkeys',
                    {'name': 'cp-passthrough-' + run + '-' + scope, 'keyType': 'passthrough'}, '-passthrough')
                patch(passthrough, {'name': 'Updated passthrough ' + run})
                passthrough_path = '/api/llm-virtual-keys/' + passthrough_id
                wait('Passthrough metadata update preserves type', lambda: api(passthrough_path)[1]['name'] == 'Updated passthrough ' + run and api(passthrough_path)[1]['keyType'] == 'passthrough')
                delete(passthrough)
                if api(passthrough_path)[0] != 404:
                    raise RuntimeError('Passthrough virtual key survived deletion')
                record('VirtualKeyPassthrough', scope, create='passed', metadata_preserves_type='passed', delete='passed')

                account, account_id, _ = create(scope, 'identity', 'ServiceAccount', 'serviceaccounts',
                    {'name': 'cp-oauth-identity-' + run + '-' + scope, 'role': 'admin'}, '-routing')
                oauth, oauth_id, obj = create(scope, 'credential', 'OAuthClient', 'oauthclients',
                    {'name': 'cp-oauth-' + run + '-' + scope, 'scopes': ['llm:proxy'], 'providerApiKeys': provider_fields,
                     'serviceAccountIdRef': {'name': account[2]}, 'billingTeamIdRef': {'name': team[2]},
                     'spendCap': cap, 'labels': [{'key': 'purpose', 'value': 'oauth'}], 'rotationNonce': '1'})
                secret = wait('OAuth connection publication', lambda: connection_value(oauth, 'client_secret'))
                public_id = oauth_value(oauth_id)['clientId']
                if secret in json.dumps(obj.get('status', {})) or oauth_auth(public_id, secret) != 200 or oauth_auth(public_id, 'deliberately-invalid-secret') not in (400, 401):
                    raise RuntimeError('OAuth credential failed authentication isolation or leaked into status')
                patch(oauth, {'name': 'Updated OAuth ' + run})
                wait('OAuth metadata update', lambda: oauth_value(oauth_id)['name'] == 'Updated OAuth ' + run)
                current = oauth_value(oauth_id)
                if current['serviceAccountId'] != account_id or current['billingTeam']['id'] != team_id or current['spendCap']['limitValue'] != 100 or connection_value(oauth, 'client_secret') != secret:
                    raise RuntimeError('Metadata update changed OAuth bindings or secret')
                if api('/api/oauth-clients/' + oauth_id, {'name': 'Out of band OAuth', 'grantType': 'client_credentials', 'scopes': ['llm:proxy']}, 'PUT')[0] != 200:
                    raise RuntimeError('Unable to inject owned OAuth metadata drift')
                wait('OAuth drift repair', lambda: oauth_value(oauth_id)['name'] == 'Updated OAuth ' + run)
                patch(oauth, {'rotationNonce': '2'})
                replacement = wait('OAuth in-place rotation publication', lambda: (value if (value := connection_value(oauth, 'client_secret')) != secret else None))
                if observation(oauth)['metadata']['annotations']['crossplane.io/external-name'] != oauth_id or oauth_value(oauth_id)['clientId'] != public_id or oauth_auth(public_id, secret) not in (400, 401) or oauth_auth(public_id, replacement) != 200:
                    raise RuntimeError('OAuth rotation did not preserve identity and revoke its old secret')
                patch(oauth, {'serviceAccountId': None, 'serviceAccountIdRef': None, 'billingTeamId': None, 'billingTeamIdRef': None, 'spendCap': None, 'labels': []})
                wait('OAuth bindings clear', lambda: oauth_value(oauth_id)['serviceAccountId'] is None and oauth_value(oauth_id)['billingTeam'] is None and oauth_value(oauth_id)['spendCap'] is None and not oauth_value(oauth_id)['labels'])
                patch(oauth, {'includeFutureProviders': True, 'providerApiKeys': []})
                wait('OAuth explicit mappings clear', lambda: oauth_value(oauth_id)['includeFutureProviders'] and not oauth_value(oauth_id)['providerApiKeys'])
                delete(oauth)
                if oauth_value(oauth_id) is not None or oauth_auth(public_id, replacement) not in (400, 401):
                    raise RuntimeError('Deleted OAuth client still exists or issues tokens')
                delete(account)
                record('OAuthClient', scope, create='passed', nested_provider_and_identity_references='passed', authentication='passed', update='passed', drift_repair='passed', metadata_preserves_bindings_and_secret=True, in_place_rotation='passed', old_secret_rejected=True, nullable_and_empty_clears='passed', credential_absent_from_status=True, delete='passed')

                code_client, code_id, _ = create(scope, 'credential', 'OAuthClient', 'oauthclients',
                    {'name': 'cp-user-oauth-' + run + '-' + scope, 'scopes': ['llm:proxy'], 'grantType': 'authorization_code', 'redirectUris': ['https://example.test/callback']}, '-code')
                patch(code_client, {'name': 'Updated user OAuth ' + run, 'redirectUris': ['https://example.test/updated']})
                wait('Authorization-code client update', lambda: oauth_value(code_id)['name'] == 'Updated user OAuth ' + run and oauth_value(code_id)['redirectUris'] == ['https://example.test/updated'] and oauth_value(code_id)['grantType'] == 'authorization_code')
                delete(code_client)
                if oauth_value(code_id) is not None:
                    raise RuntimeError('Authorization-code client survived deletion')
                record('OAuthClientAuthorizationCode', scope, create='passed', metadata_preserves_grant_type='passed', redirect_update='passed', delete='passed', interactive_consent='not_invoked')
                delete(provider)
                delete(team)

            if 'remote-agents' in scope_cases:
                secret_name = 'cp-remote-auth-' + run + '-' + scope
                credential = 'synthetic-local-only'
                auth = {'type': 'api_key', 'headerName': 'X-API-Key', 'credential': credential}
                def write_auth(value):
                    kube('apply', '-f', '-', payload={'apiVersion': 'v1', 'kind': 'Secret',
                        'metadata': {'name': secret_name, 'namespace': args.namespace, 'labels': {'provider-test-run': run}},
                        'stringData': {'auth': json.dumps(value)}})
                write_auth(auth)
                secret = (['-n', args.namespace], 'secrets', secret_name)
                leases.append(secret)
                auth_ref = {'name': secret_name, 'key': 'auth'}
                if scope == 'cluster':
                    auth_ref['namespace'] = args.namespace
                desired_source = a2a_source('cp-remote-first-' + run + '-' + scope)
                first, first_id, obj = create(scope, 'a2a', 'RemoteAgent', 'remoteagents',
                    {'source': json.dumps(desired_source), 'name': 'cp-remote-' + run + '-' + scope,
                     'authSecretRef': auth_ref, 'enabled': False}, '-first')
                path = '/api/a2a/remote-agents/' + first_id
                initial = api(path)[1]
                first_connection = initial['connection']['id']
                if initial['connection']['enabled'] or not initial['connection']['hasCredential']:
                    raise RuntimeError('Initial remote connection controls were not persisted')
                if credential in json.dumps(obj.get('status', {})) or credential in json.dumps(initial):
                    raise RuntimeError('Remote credential material leaked into observations')
                patch(first, {'name': 'Updated remote ' + run, 'enabled': True})
                wait('Remote metadata and enablement update', lambda: api(path)[1]['name'] == 'Updated remote ' + run and api(path)[1]['connection']['enabled'])
                api(path, {'name': 'Out of band remote'}, 'PUT')
                wait('Remote metadata drift repair', lambda: api(path)[1]['name'] == 'Updated remote ' + run)
                write_auth({'type': 'none'})
                wait('Remote authentication clear', lambda: api(path)[1]['connection']['authType'] == 'none' and not api(path)[1]['connection']['hasCredential'])
                desired_source['agentCard']['description'] = 'Updated inline card.'
                patch(first, {'source': json.dumps(desired_source)})
                wait('Remote card update', lambda: api(path)[1]['agentCard']['description'] == 'Updated inline card.')
                drift_source = json.loads(json.dumps(desired_source))
                drift_source['agentCard']['description'] = 'Out of band card.'
                api(path, {'source': drift_source, 'auth': {'type': 'none'}}, 'PUT')
                wait('Remote card drift repair', lambda: api(path)[1]['agentCard']['description'] == 'Updated inline card.')
                if api(path)[1]['connection']['id'] != first_connection:
                    raise RuntimeError('Remote metadata or source update replaced its connection identity')

                delete(first)
                if api(path)[0] != 404:
                    raise RuntimeError('Remote registration survived deletion')
                delete(secret)
                record('RemoteAgent', scope, create='passed', initially_disabled='passed', update='passed', source_update='passed', drift_repair='passed', source_drift_repair='passed', identity_preserved=True, authentication_clear='passed', credential_absent_from_status=True, delete='passed', outbound_agent_runs='not_invoked')


            if 'extensions' in scope_cases:
                # Each selector reads the authoritative API projection, not Kubernetes status.
                def collection(path, identifier):
                    code, values = api(path)
                    if code != 200:
                        raise RuntimeError('Collection read failed: HTTP ' + str(code))
                    return next((item for item in values if item['id'] == identifier), None)

                directory, directory_id, _ = create(scope, 'knowledge', 'Directory', 'directories', {'name': 'cp-directory-' + run + '-' + scope})
                path = '/api/knowledge-directories/' + directory_id
                patch(directory, {'name': 'Updated directory ' + run})
                wait('Directory update', lambda: collection('/api/knowledge-directories', directory_id)['name'] == 'Updated directory ' + run)
                api(path, {'name': 'Out of band directory'}, 'PATCH')
                wait('Directory drift repair', lambda: collection('/api/knowledge-directories', directory_id)['name'] == 'Updated directory ' + run)
                delete(directory)
                if collection('/api/knowledge-directories', directory_id) is not None:
                    raise RuntimeError('Directory survived deletion')
                record('Directory', scope, create='passed', update='passed', drift_repair='passed', delete='passed')

                account, account_id, _ = create(scope, 'identity', 'ServiceAccount', 'serviceaccounts',
                    {'name': 'cp-account-' + run + '-' + scope, 'role': 'member', 'disabled': False})
                account_path = '/api/service-accounts/' + account_id
                patch(account, {'name': 'Updated account ' + run})
                wait('Account update', lambda: api(account_path)[1]['name'] == 'Updated account ' + run)
                api(account_path, {'disabled': True}, 'PATCH')
                wait('Account disabled drift repair', lambda: not api(account_path)[1]['disabled'])
                token_lease, token_id, obj = create(scope, 'credential', 'ServiceAccountAPIKey', 'serviceaccounttokens',
                    {'serviceAccountId': account_id, 'name': 'cp-token-' + run, 'expiresIn': 3600, 'disabled': False})
                token_uuid = token_id.split(':')[1]
                token_path = account_path + '/tokens/' + token_uuid
                connection_name = token_lease[2] + '-connection'
                def issued_token():
                    value = json.loads(kube('-n', args.namespace, 'get', 'secret', connection_name, '-o', 'json'))
                    return __import__('base64').b64decode(value['data']['attribute.token']).decode()
                secret_value = wait('Token connection secret publication', issued_token)
                def token_auth(expected):
                    req = urllib.request.Request(args.api_url.rstrip('/') + '/api/organization', headers={'Authorization': secret_value})
                    try:
                        with opener.open(req, timeout=20) as response:
                            return response.status == expected
                    except urllib.error.HTTPError as error:
                        return error.code == expected
                if not token_auth(200) or secret_value in json.dumps(obj.get('status', {})):
                    raise RuntimeError('Issued token failed authentication or leaked into status')
                patch(token_lease, {'name': 'Updated token ' + run})
                wait('Token metadata update', lambda: next(item for item in api(account_path)[1]['tokens'] if item['id'] == token_uuid)['name'] == 'Updated token ' + run)
                if issued_token() != secret_value:
                    raise RuntimeError('Metadata update changed the issued token secret')
                api(token_path, {'name': 'Out of band token'}, 'PATCH')
                wait('Token drift repair', lambda: next(item for item in api(account_path)[1]['tokens'] if item['id'] == token_uuid)['name'] == 'Updated token ' + run)
                patch(token_lease, {'disabled': True})
                wait('Token disablement', lambda: token_auth(401))
                delete(token_lease)
                if any(item['id'] == token_uuid for item in api(account_path)[1]['tokens']) or not token_auth(401):
                    raise RuntimeError('Deleted token still exists or authenticates')
                record('ServiceAccountAPIKey', scope, create='passed', authentication='passed', update='passed', drift_repair='passed', secret_preserved='passed', secret_absent_from_status=True, disable='passed', delete='passed', revoked_authentication=401)
                delete(account)
                if api(account_path)[0] != 404:
                    raise RuntimeError('Service account survived deletion')
                record('ServiceAccount', scope, create='passed', update='passed', drift_repair='passed', delete='passed')

                gateway, gateway_id, _ = create(scope, 'mcp', 'Gateway', 'gateways',
                    {'name': 'cp-delegation-gateway-' + run + '-' + scope, 'accessAllSubagents': False})
                path = '/api/agents/' + gateway_id
                patch(gateway, {'accessAllSubagents': True})
                wait('Gateway subagent control', lambda: api(path)[1]['accessAllSubagents'])
                api(path, {'accessAllSubagents': False}, 'PUT')
                wait('Gateway subagent control drift repair', lambda: api(path)[1]['accessAllSubagents'])
                delete(gateway)
                record('GatewaySubagentControl', scope, create='passed', update='passed', drift_repair='passed', delete='passed')

                catalog, catalog_id, _ = create(scope, 'mcp', 'RegistryEntry', 'registrycatalogitems',
                    {'name': 'cp-hibernation-' + run + '-' + scope, 'multitenant': False,
                     'localConfig': {'command': 'npx', 'arguments': ['-y', '@modelcontextprotocol/server-filesystem', '/tmp']}})
                keys_status, api_keys = api('/api/api-keys')
                owners = {item['userId'] for item in api_keys} if keys_status == 200 else set()
                if len(owners) != 1:
                    raise RuntimeError('Unable to resolve one owner from the authenticated user API-key list')
                caller = owners.pop()
                installation, installation_id, _ = create(scope, 'mcp', 'Server', 'serverinstallations',
                    {'name': 'cp-hibernation-' + run + '-' + scope, 'catalogId': catalog_id,
                     'scope': 'personal', 'userId': caller, 'hibernationMode': 'disabled'})
                path = '/api/mcp_server/' + installation_id
                if api(path)[1]['ownerId'] != caller or api(path)[1]['hibernationMode'] != 'disabled':
                    raise RuntimeError('Personal owner or initial hibernation override was not persisted')
                patch(installation, {'hibernationMode': 'enabled'})
                wait('Installation hibernation update', lambda: api(path)[1]['hibernationMode'] == 'enabled')
                api(path + '/reinstall', {'hibernationMode': 'inherit'}, 'POST')
                wait('Installation hibernation drift repair', lambda: api(path)[1]['hibernationMode'] == 'enabled')
                if observation(installation)['metadata']['annotations']['crossplane.io/external-name'] != installation_id:
                    raise RuntimeError('Hibernation update replaced the installation')
                delete(installation)
                if api(path)[0] != 404:
                    raise RuntimeError('Installation survived deletion')
                delete(catalog)
                record('InstallationControls', scope, create='passed', personal_user='passed', update='passed', drift_repair='passed', identity_preserved=True, delete='passed')

            if 'model-pools' in scope_cases:
                keys, pools = [], []
                for index in range(2):
                    key, key_id, _ = create(scope, 'llm', 'ModelProvider', 'providerkeys',
                        {'name': 'cp-pool-key-' + str(index) + '-' + run + '-' + scope,
                         'llmProvider': 'ollama', 'shared': True, 'baseUrl': args.llm_base_url,
                         'permissions': [{'subjectType': 'role', 'subjectId': 'admin', 'actions': ['read', 'use']}],
                         **model_fixture}, '-pool-' + str(index))
                    keys.append((key, key_id))
                    status, pool = api('/api/model-pools',
                        {'name': 'cp-pool-' + str(index) + '-' + run + '-' + scope,
                         'members': [{'providerApiKeyId': key_id, 'modelId': 'llama3'}]}, 'POST')
                    if status != 200 or not pool.get('id'):
                        raise RuntimeError('Unable to create owned model pool: HTTP ' + str(status))
                    fixture = ([], 'api-fixture', '/api/model-pools/' + pool['id'])
                    leases.append(fixture)
                    pools.append((fixture, pool['id']))
                agent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                    {'name': 'cp-pool-agent-' + run + '-' + scope, 'modelPoolId': pools[0][1]}, '-pool')
                oauth, oauth_id, _ = create(scope, 'credential', 'OAuthClient', 'oauthclients',
                    {'name': 'cp-pool-client-' + run + '-' + scope, 'scopes': ['llm:proxy'],
                     'modelPoolIds': [pools[0][1]],
                     'providerApiKeys': [{'provider': 'ollama', 'providerApiKeyId': keys[0][1]}]}, '-pool')
                agent_path = '/api/agents/' + agent_id
                def pool_oauth_value():
                    code, page = api('/api/oauth-clients?limit=100')
                    if code != 200:
                        raise RuntimeError('Unable to read managed pool OAuth client metadata')
                    return next((item for item in page['data'] if item['id'] == oauth_id), None)
                def pooled_agent_matches(pool_id, key_id):
                    code, value = api(agent_path)
                    return code == 200 and value['modelPoolId'] == pool_id and value['llmApiKeyId'] == key_id
                wait('Agent model-pool anchors', lambda: pooled_agent_matches(pools[0][1], keys[0][1]))
                patch(agent, {'modelPoolId': pools[1][1]})
                patch(oauth, {'modelPoolIds': [pools[1][1]]})
                wait('Agent model-pool switch', lambda: condition(agent, 'True') and pooled_agent_matches(pools[1][1], keys[1][1]))
                wait('OAuth model-pool grant switch', lambda: condition(oauth, 'True') and pool_oauth_value()['modelPoolIds'] == [pools[1][1]])
                desired = observation(agent)['spec']['forProvider']
                if desired.get('modelId') or desired.get('llmApiKeyId'):
                    raise RuntimeError('Pool-derived anchors were late-initialized into conflicting desired configuration')
                patch(agent, {'name': 'cp-pool-renamed-' + run + '-' + scope})
                wait('Pool metadata preserves anchors', lambda: condition(agent, 'True') and pooled_agent_matches(pools[1][1], keys[1][1])
                     and api(agent_path)[1]['name'] == 'cp-pool-renamed-' + run + '-' + scope)
                patch(agent, {'modelPoolId': None})
                patch(oauth, {'modelPoolIds': []})
                wait('Agent model-pool clear', lambda: condition(agent, 'True') and pooled_agent_matches(None, None)
                     and api(agent_path)[1]['modelId'] is None)
                wait('OAuth model-pool grant clear', lambda: condition(oauth, 'True') and pool_oauth_value()['modelPoolIds'] == [])
                for lease, identifier in ((agent, agent_id), (oauth, oauth_id)):
                    if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier:
                        raise RuntimeError('Model-pool update replaced a managed identity')
                    delete(lease)
                if api(agent_path)[0] != 404 or pool_oauth_value() is not None:
                    raise RuntimeError('Model-pool managed resources survived deletion')
                for fixture, _ in pools:
                    delete(fixture)
                for lease, key_id in keys:
                    delete(lease)
                    if api('/api/llm-provider-api-keys/' + key_id)[0] != 404:
                        raise RuntimeError('Owned model-pool provider key survived deletion')
                record('AgentModelPool', scope, create='passed', source_switch='passed', metadata_preserves_anchors='passed',
                       derived_anchors_not_initialized=True, clear='passed', identity_preserved=True, delete='passed')
                record('OAuthClientModelPools', scope, create='passed', grants_update='passed', grants_clear='passed',
                       identity_preserved=True, delete='passed')

            if 'llm' in scope_cases:
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
                key_lease, key_id, key_obj = create(scope, 'llm', 'ModelProvider', 'providerkeys',
                        {'name': key_name, 'llmProvider': 'ollama-native',
                         'baseUrl': args.llm_base_url, 'extraHeadersSecretRef': secret_ref,
                         'labels': [{'key': 'provider-test', 'value': run}], **model_fixture})
                key_path = '/api/llm-provider-api-keys/' + key_id
                status, key_wire = api(key_path)
                if status != 200 or key_wire.get('extraHeaders', {}).get('X-Provider-Test') != header_value:
                    raise RuntimeError('ModelProvider secret reference was not applied')
                if header_value in json.dumps(key_obj.get('status', {})):
                    raise RuntimeError('ModelProvider exposed a sensitive header in status')
                updated_name = key_name + '-updated'
                patch(key_lease, {'name': updated_name})
                wait('ModelProvider update', lambda: api(key_path)[1].get('name') == updated_name)
                if api(key_path, {'name': 'Out of band ' + run})[0] != 200:
                    raise RuntimeError('ModelProvider drift mutation failed')
                wait('ModelProvider drift repair', lambda: api(key_path)[1].get('name') == updated_name)
                header_value = 'synthetic-' + run + '-rotated'
                secret['stringData']['X-Provider-Test'] = header_value
                kube('apply', '-f', '-', payload=secret)
                wait('ModelProvider secret rotation', lambda: api(key_path)[1].get('extraHeaders', {}).get('X-Provider-Test') == header_value)
                key_obj = json.loads(kube(*key_lease[0], 'get', key_lease[1], key_lease[2], '-o', 'json'))
                if header_value in json.dumps(key_obj.get('status', {})):
                    raise RuntimeError('ModelProvider exposed a rotated sensitive header in status')

                model_lease, model_id, _ = create(scope, 'llm', 'Model', 'modeloverrides',
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
                    raise RuntimeError('Model did not adopt a readable model')
                patch(model_lease, {'customContextLength': 4096})
                wait('Model update', lambda: model_wire().get('customContextLength') == 4096)
                if api('/api/llm-models/' + model_id, {'customContextLength': 3072})[0] != 200:
                    raise RuntimeError('Model drift mutation failed')
                wait('Model drift repair', lambda: model_wire().get('customContextLength') == 4096)
                delete(model_lease)
                reset = model_wire()
                if reset is None or any(reset.get(field) is not None for field in
                            ('customContextLength', 'customOutputLength', 'configuredParameters',
                             'customPricePerMillionCacheRead', 'customPricePerMillionCacheWrite')) or reset.get('labels'):
                    raise RuntimeError('Model delete failed to retain and reset the model')
                results.append({'resource': 'Model', 'scope': scope, 'create': 'passed',
                                'update': 'passed', 'drift_repair': 'passed', 'delete_reset': 'passed',
                                'adopted_model_preserved': True})
                save()
                print(json.dumps(results[-1]), flush=True)
                # The discovered model has its own sharing policy even though delete only resets overrides.
                policy_path = '/api/resource-permissions/llmModel/' + model_id
                original_policy = api(policy_path)[1]
                if not original_policy:
                    raise RuntimeError('Unable to snapshot discovered-model sharing')
                owner_subject = {'type': 'user', 'id': original_policy.get('ownerId')}
                owner_grants = [g for g in original_policy['grants'] if g['subject'] == owner_subject]
                injected_grants = owner_grants + [{'subject': {'type': 'organization', 'id': '*'}, 'actions': ['read']}]
                def canonical_model_grants(grants):
                    return sorted((g['subject']['type'], g['subject']['id'], tuple(sorted(g['actions']))) for g in grants)
                def model_additional_grants():
                    return [g for g in api(policy_path)[1]['grants'] if g['subject'] != owner_subject]
                sharing_model = None
                evidence['api_cleanup'] = 'pending'
                try:
                    sharing_model, adopted_id, _ = create(scope, 'llm', 'Model', 'models', {
                        'modelId': 'llama3', 'llmProvider': 'ollama-native', 'permissions': []}, '-sharing', external_name=model_id)
                    wait('Adopted model sharing clear', lambda: not model_additional_grants())
                    current = api(policy_path)[1]
                    if api(policy_path, {'revision': current['revision'], 'grants': injected_grants}, 'PUT')[0] != 200:
                        raise RuntimeError('Unable to inject owned model sharing drift')
                    wait('Adopted model empty-sharing drift repair', lambda: not model_additional_grants())
                    if adopted_id != model_id or observation(sharing_model)['metadata']['annotations']['crossplane.io/external-name'] != model_id:
                        raise RuntimeError('Sharing update replaced the adopted model')
                    delete(sharing_model)
                finally:
                    if sharing_model in leases:
                        delete(sharing_model)
                    current = api(policy_path)[1]
                    actual = canonical_model_grants(current['grants'])
                    if actual not in [canonical_model_grants(value) for value in (original_policy['grants'], owner_grants, injected_grants)]:
                        raise RuntimeError('Concurrent model sharing edit; restore refused')
                    if actual != canonical_model_grants(original_policy['grants']):
                        if api(policy_path, {'revision': current['revision'], 'grants': original_policy['grants']}, 'PUT')[0] != 200:
                            raise RuntimeError('Unable to restore discovered-model sharing')
                    if canonical_model_grants(api(policy_path)[1]['grants']) != canonical_model_grants(original_policy['grants']):
                        raise RuntimeError('Model sharing restore was not confirmed')
                    evidence['api_cleanup'] = 'passed'
                record('ModelSharing', scope, adoption='passed', explicit_empty_clear='passed',
                       sharing_only_drift_repair='passed', identity_preserved=True, delete_retains_discovered_model=True)

                delete(key_lease)
                if api(key_path)[0] != 404:
                    raise RuntimeError('ModelProvider delete did not remove the key')
                delete(secret_lease)
                results.append({'resource': 'ModelProvider', 'scope': scope, 'create': 'passed',
                                'update': 'passed', 'drift_repair': 'passed', 'delete': 'passed',
                                'sensitive_header_secret_rotation': 'passed', 'sensitive_header_absent_from_status': True})
                save()
                print(json.dumps(results[-1]), flush=True)



            if 'organization-defaults' in scope_cases:
                # This complete-set probe refuses pre-existing spending rows and restores global policies.
                baseline_defaults = api('/api/environments')[1]['resourceDefaults']
                baseline_policy = api('/api/resource-permissions/agent/*')[1]
                if api('/api/default-user-limits')[1] or any(baseline_defaults.get(k) for k in ('agent', 'mcpGateway')):
                    raise RuntimeError('Inspect existing isolated spending/environment defaults before this probe')
                recipient, recipient_id, _ = create(scope, 'team', 'Team', 'teams', {'name': 'cp-settings-recipient-' + run + '-' + scope}, '-settings')
                first, first_id, _ = create(scope, 'environment', 'Environment', 'environments', {'name': 'cp-settings-first-' + run + '-' + scope}, '-first')
                second, second_id, _ = create(scope, 'environment', 'Environment', 'environments', {'name': 'cp-settings-second-' + run + '-' + scope}, '-second')
                def projected(grants):
                    return [{'subjectType': g['subject']['type'], 'subjectId': g['subject']['id'], 'actions': g['actions']} for g in grants]
                extra_grant = {'subject': {'type': 'team', 'id': recipient_id}, 'actions': ['read']}
                desired_grants = baseline_policy['grants'] + [extra_grant]
                settings = None
                def same_grants(left, right):
                    def canonical(values):
                        return sorted((g['subject']['type'], g['subject']['id'], tuple(sorted(g['actions']))) for g in values)
                    return canonical(left) == canonical(right)
                def policies():
                    return api('/api/resource-permissions/agent/*')[1]
                def defaults():
                    return api('/api/environments')[1]['resourceDefaults']
                def rows():
                    return api('/api/default-user-limits')[1]
                def limit_for(environment):
                    return next((r for r in rows() if r.get('environmentId') == environment), None)
                def cleanup_defaults():
                    current = policies()
                    if not (same_grants(current['grants'], baseline_policy['grants']) or same_grants(current['grants'], desired_grants)):
                        raise RuntimeError('Concurrent organization-policy edit; restore refused')
                    if not same_grants(current['grants'], baseline_policy['grants']):
                        if api('/api/resource-permissions/agent/*', {'revision': current['revision'], 'grants': baseline_policy['grants']}, 'PUT')[0] != 200:
                            raise RuntimeError('Unable to restore isolated organization policy')
                    current_defaults = defaults()
                    if any(current_defaults.get(k) not in (None, first_id, second_id) for k in ('agent', 'mcpGateway')):
                        raise RuntimeError('Concurrent environment-default edit; restore refused')
                    if api('/api/environments/defaults', {k: baseline_defaults.get(k) for k in ('agent', 'mcpGateway')})[0] != 200:
                        raise RuntimeError('Unable to restore environment defaults')
                    for row in rows():
                        if row.get('environmentId') not in (None, first_id, second_id) or row['limitValue'] not in (5, 7, 10, 20, 37):
                            raise RuntimeError('Concurrent spending-default edit; cleanup refused')
                        if api('/api/default-user-limits/' + row['id'], method='DELETE')[0] not in (200, 204, 404):
                            raise RuntimeError('Unable to delete owned spending default')
                evidence['api_cleanup'] = 'pending'
                try:
                    fields = {'font': org['customFont'], 'colorTheme': org['theme'],
                              'environmentDefaults': {'agentsRef': {'name': first[2]}, 'mcpGatewaysRef': {'name': second[2]}},
                              'defaultUserLimits': {'default': {'limitValue': 10, 'models': [], 'cleanupInterval': 'calendar_month'},
                                                   first_id: {'limitValue': 5, 'models': ['synthetic-model'], 'cleanupInterval': '1h'}},
                              'defaultPermissions': {'agents': projected(desired_grants)}}
                    settings, identifier, _ = create(scope, 'organization', 'Settings', 'settings', fields, '-defaults')
                    wait('Inline organization defaults persist after singleton adoption', lambda: defaults()['agent'] == first_id and defaults()['mcpGateway'] == second_id
                         and len(rows()) == 2 and same_grants(policies()['grants'], desired_grants))
                    global_id = limit_for(None)['id']
                    # Replace nested fields so reference selectors cannot repopulate explicitly cleared values.
                    replace_fields(settings, {'environmentDefaults': {'agentsRef': {'name': second[2]}, 'mcpGateways': ''},
                                              'defaultUserLimits': {'default': {'limitValue': 20, 'models': [], 'cleanupInterval': 'calendar_month'},
                                                                   second_id: {'limitValue': 7, 'models': [], 'cleanupInterval': '24h'}}})
                    wait('Inline settings update', lambda: defaults()['agent'] == second_id and defaults()['mcpGateway'] is None
                         and len(rows()) == 2 and limit_for(None)['limitValue'] == 20 and limit_for(second_id) is not None)
                    if limit_for(None)['id'] != global_id:
                        raise RuntimeError('Spending-default update replaced its identity')
                    current = policies()
                    if api('/api/resource-permissions/agent/*', {'revision': current['revision'], 'grants': baseline_policy['grants']}, 'PUT')[0] != 200:
                        raise RuntimeError('Unable to inject isolated global sharing drift')
                    api('/api/environments/defaults', {'agent': first_id})
                    api('/api/default-user-limits/' + global_id, {'limitValue': 37})
                    wait('Inline settings drift repair', lambda: defaults()['agent'] == second_id and limit_for(None)['limitValue'] == 20 and same_grants(policies()['grants'], desired_grants))
                    patch(settings, {'environmentDefaults': None, 'defaultUserLimits': None, 'defaultPermissions': None})
                    def settings_generation_synced():
                        obj = observation(settings)
                        return any(c.get('type') == 'Synced' and c.get('status') == 'True' and c.get('observedGeneration') == obj['metadata']['generation'] for c in obj.get('status', {}).get('conditions', []))
                    wait('Omitted defaults are unmanaged', settings_generation_synced)
                    if defaults()['agent'] != second_id or len(rows()) != 2 or not same_grants(policies()['grants'], desired_grants):
                        raise RuntimeError('Omitting organization defaults changed persisted settings')
                    patch(settings, {'environmentDefaults': {'agents': '', 'mcpGateways': ''}, 'defaultUserLimits': {}, 'defaultPermissions': {'agents': projected(baseline_policy['grants'])}})
                    wait('Inline settings clear', lambda: defaults()['agent'] is None and defaults()['mcpGateway'] is None and not rows() and same_grants(policies()['grants'], baseline_policy['grants']))
                    time.sleep(7)
                    obj = observation(settings)
                    if obj['spec']['forProvider']['defaultUserLimits'] != {} or obj['metadata']['annotations']['crossplane.io/external-name'] != identifier or not condition(settings, 'True'):
                        raise RuntimeError('Empty collections or singleton identity changed across reconciliation')
                    delete(settings)
                finally:
                    if settings in leases:
                        delete(settings)
                    cleanup_defaults()
                    if not same_grants(policies()['grants'], baseline_policy['grants']) or rows() or any(defaults().get(k) != baseline_defaults.get(k) for k in ('agent', 'mcpGateway')):
                        raise RuntimeError('Organization defaults restore was not confirmed')
                    evidence['api_cleanup'] = 'passed'
                delete(second)
                delete(first)
                delete(recipient)
                record('OrganizationInlineDefaults', scope, create='passed', environment_references='passed', update='passed', drift_repair='passed',
                       omission_preserves='passed', empty_collections_clear='passed', spending_row_identity_preserved=True,
                       delete_preserves_singleton=api('/api/organization')[0] == 200)

            if 'organization' in scope_cases:
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

            if 'project-instructions' in scope_cases:
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

            if 'sharing-defaults' in scope_cases:
                for label in ('default', 'empty', 'init-empty'):
                    if any(item['scope'] == scope and item['resource'] == 'CredentialSharingDefaults' and item.get('parameter_source') == label for item in results):
                        continue
                    explicit_empty = label != 'default'
                    fields = {'name': 'cp-grant-' + label + '-' + run + '-' + scope,
                              'key': 'cp-grant-' + label + '-' + run + '-' + scope}
                    if label == 'empty':
                        fields['permissions'] = []
                    lease, _, obj = create(scope, 'credential', 'Credential', 'runtimecredentials', fields, '-' + label,
                                           init_fields={'permissions': []} if label == 'init-empty' else None)
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
                        kube(*lease[0], 'patch', lease[1], lease[2], '--type=merge', '-p', json.dumps({'spec': {'initProvider': {'permissions': [{'subjectType': 'organization', 'subjectId': '*', 'actions': ['read']}]}}}))
                        def init_change_reconciled():
                            current = observation(lease)
                            return any(item.get('type') == 'Synced' and item.get('status') == 'True' and item.get('observedGeneration') == current['metadata']['generation'] for item in current.get('status', {}).get('conditions', []))
                        wait('initProvider creation grant update ignored', init_change_reconciled)
                        if api('/api/resource-permissions/credential/' + definition_id)[1]['grants'] != policy['grants']:
                            raise RuntimeError('initProvider update changed already-created grants')
                        init_update = 'passed'
                    delete(lease)
                    record('CredentialSharingDefaults', scope, parameter_source=label, explicit_empty=explicit_empty,
                           metadata_update_preserves_grants='passed', init_provider_update_ignored=init_update,
                           persisted_grants=len(policy['grants']), owner_grant_preserved=True, create='passed', delete='passed')

            if 'parent-sharing' in scope_cases:
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
                    ('environment', 'Environment', 'environments', 'environment', {}),
                    ('agent', 'Agent', 'agents', 'agent', {}),
                    ('mcp', 'Gateway', 'gateways', 'mcpGateway', {}),
                    ('identity', 'ServiceAccount', 'serviceaccounts', 'serviceAccount', {'role': 'member'}),
                    ('a2a', 'RemoteAgent', 'remoteagents', 'externalAgent', {'source': json.dumps(a2a_source('cp-grant-remote-' + run + '-' + scope))}),
                    ('llm', 'VirtualKey', 'virtualkeys', 'llmVirtualKey', {'includeFutureProviders': True, 'providerApiKeys': []}),
                    ('credential', 'OAuthClient', 'oauthclients', 'oauthClient', {'scopes': ['llm:proxy'], 'includeFutureProviders': True, 'providerApiKeys': []}),
                    ('plugin', 'Plugin', 'plugins', 'plugin', {'clientType': 'claude-code', 'supportedPlatforms': ['posix'], 'files': {'.claude-plugin/plugin.json': {'content': '{"name":"synthetic-plugin","version":"1.0.0"}', 'encoding': 'utf8', 'mode': '100644'}}}),
                    ('project', 'Project', 'projects', 'project', {}),
                    ('knowledge', 'KnowledgeBase', 'knowledgebases', 'knowledgeBase', {}),
                    ('mcp', 'RegistryEntry', 'registrycatalogitems', 'mcpRegistry', {'remoteConfig': {'url': 'http://127.0.0.1:19999/mcp'}}),
                    ('skill', 'Skill', 'skills', 'skill', {'description': 'Synthetic initial grants', 'content': 'Synthetic body.', 'files': {}, 'environmentIds': [], 'labels': []}),
                    ('credential', 'Credential', 'runtimecredentials', 'credential', {'key': 'cp-grant-' + run + '-' + scope}),
                    ('llm', 'ModelProvider', 'providerkeys', 'llmProviderApiKey', {'llmProvider': 'ollama', 'shared': True,
                        'apiKeySecretRef': secret_ref, 'baseUrl': args.llm_base_url}),
                ]
                for group, kind, plural, resource_type, extra in cases:
                    if any(item['scope'] == scope and item['resource'] == kind + 'Sharing' for item in results):
                        continue
                    if args.grant_kinds and kind not in args.grant_kinds:
                        continue
                    fields = {'name': 'cp-grant-' + kind.lower() + '-' + run + '-' + scope,
                              'permissions': [grant], **extra}
                    if kind == 'Plugin':
                        fields['displayName'] = fields.pop('name')
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
                    updated_name = 'updated-' + (fields['displayName'] if kind == 'Plugin' else fields['name'])
                    metadata_field = 'displayName' if kind == 'Plugin' else 'description' if kind in ('RegistryEntry', 'Credential') else 'name'
                    patch(lease, {metadata_field: updated_name})

                    def metadata_updated():
                        if resource_type == 'credential':
                            code, definitions = api('/api/credentials')
                            return code == 200 and any(item['key'] == identifier and item[metadata_field] == updated_name for item in definitions)
                        if resource_type == 'oauthClient':
                            code, page = api('/api/oauth-clients?limit=100')
                            return code == 200 and any(item['id'] == identifier and item[metadata_field] == updated_name for item in page['data'])
                        prefix = {'plugin': '/api/plugins/', 'llmVirtualKey': '/api/llm-virtual-keys/', 'agent': '/api/agents/', 'mcpGateway': '/api/agents/', 'project': '/api/projects/',
                                  'knowledgeBase': '/api/knowledge-bases/', 'mcpRegistry': '/api/internal_mcp_catalog/', 'serviceAccount': '/api/service-accounts/', 'externalAgent': '/api/a2a/remote-agents/',
                                  'environment': '/api/environments/', 'skill': '/api/skills/', 'llmProviderApiKey': '/api/llm-provider-api-keys/'}[resource_type]
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
                    policy_before = api(path)[1]
                    owner_grants = [g for g in policy_before['grants'] if g['subject'] == {'type': 'user', 'id': policy_before.get('ownerId')}]
                    def additional_grants():
                        code, current = api(path)
                        if code != 200:
                            raise RuntimeError('Sharing read failed')
                        return [g for g in current['grants'] if g['subject'] != {'type': 'user', 'id': current.get('ownerId')}]
                    patch(lease, {'permissions': []})
                    wait('Sharing cleared in place ' + kind, lambda: condition(lease, 'True') and not additional_grants())
                    current = api(path)[1]
                    if api(path, {'revision': current['revision'], 'grants': owner_grants + [{'subject': {'type': 'team', 'id': team_id}, 'actions': ['read']}]}, 'PUT')[0] != 200:
                        raise RuntimeError('Unable to inject owned sharing drift')
                    wait('Empty sharing drift repair ' + kind, lambda: not additional_grants())
                    if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier:
                        raise RuntimeError('Sharing update replaced the parent')
                    delete(lease)
                    record(kind + 'Sharing', scope, create_with_policy='passed', metadata_update_preserves_grants='passed',
                           sharing_clear_in_place='passed', empty_sharing_drift_repair='passed', identity_preserved=True, delete='passed')
                delete(secret)
                delete(team)


            if 'git-skills' in scope_cases:
                fixture_commit = '411935fb1648ca812dff1f3503c0be77db65bcf3'
                alternate_commit = '453ee6fb4cb0ad3564f588114dbdaf891a10a488'
                source = {'repoUrl': 'https://github.com/archestra-ai/terraform-provider-archestra/tree/' + fixture_commit,
                          'skillPath': 'scripts/integration/fixtures/skills/synthetic', 'syncInterval': '1d'}
                team, team_id, _ = create(scope, 'team', 'Team', 'teams',
                    {'name': 'cp-git-skill-team-' + run + '-' + scope, 'members': []}, '-git-skill')
                environment, environment_id, _ = create(scope, 'environment', 'Environment', 'environments',
                    {'name': 'cp-git-skill-env-' + run + '-' + scope}, '-git-skill')
                configured = {'gitSource': source, 'icon': 'book', 'environmentIds': [environment_id],
                              'labels': [{'key': 'purpose', 'value': 'git-source'}],
                              'permissions': [{'subjectType': 'team', 'subjectId': team_id, 'actions': ['read']}]}
                lease, identifier, _ = create(scope, 'skill', 'Skill', 'skills', configured, '-git-source')
                path = '/api/skills/' + identifier
                permission_path = '/api/resource-permissions/skill/' + identifier

                def git_skill_value():
                    code, value = api(path)
                    if code != 200:
                        raise RuntimeError('Git Skill read failed: HTTP ' + str(code))
                    return value

                def git_skill_additional_grants():
                    code, value = api(permission_path)
                    if code != 200:
                        raise RuntimeError('Git Skill permissions read failed')
                    return [g for g in value['grants'] if g['subject'] != {'type': 'user', 'id': value.get('ownerId')}]

                initial = git_skill_value()
                if (initial['name'] != 'provider-synthetic-import' or initial['sourceCommit'] != fixture_commit
                        or initial['latestVersion'] != 1 or initial['icon'] != 'book'
                        or {e['id'] for e in initial['environments']} != {environment_id}
                        or not any(g['subject'] == {'type': 'team', 'id': team_id} for g in git_skill_additional_grants())):
                    raise RuntimeError('Git Skill import did not apply source, environment, presentation and sharing')
                patch(lease, {'icon': 'books', 'environmentIds': [], 'labels': [], 'permissions': [],
                              'gitSource': {'syncInterval': '1h'}})
                wait('Git Skill settings and sharing update', lambda: condition(lease, 'True')
                     and git_skill_value()['icon'] == 'books' and not git_skill_value()['environments']
                     and not git_skill_value()['labels'] and not git_skill_additional_grants()
                     and git_skill_value()['githubSyncInterval'] == '1h')
                if git_skill_value()['latestVersion'] != 1:
                    raise RuntimeError('Git Skill settings forked repository-owned content')
                alternate = 'https://github.com/archestra-ai/terraform-provider-archestra/tree/' + alternate_commit
                patch(lease, {'gitSource': {'repoUrl': alternate}})
                wait('Git Skill source change in place', lambda: condition(lease, 'True')
                     and git_skill_value()['sourceCommit'] == alternate_commit)
                if api(path + '/github-sync', {'interval': '1d'}, 'PATCH')[0] != 200:
                    raise RuntimeError('Unable to inject owned Git Skill interval drift')
                wait('Git Skill interval drift repair', lambda: git_skill_value()['githubSyncInterval'] == '1h')
                if api(path + '/github-sync', {'disconnect': True}, 'PATCH')[0] != 200:
                    raise RuntimeError('Unable to inject owned Git Skill disconnect drift')
                wait('Git Skill reconnect after external disconnect', lambda: condition(lease, 'True')
                     and git_skill_value()['githubSyncInterval'] == '1h')
                if observation(lease)['metadata']['annotations']['crossplane.io/external-name'] != identifier:
                    raise RuntimeError('Git Skill settings, source, or reconnect replaced identity')
                immutable_update(lease,
                    {'gitSource': None, 'name': 'cp-authored-mode-' + run, 'description': 'Authored mode',
                     'content': 'Synthetic authored body.', 'files': {}},
                    {'gitSource': {'repoUrl': alternate, 'skillPath': source['skillPath'], 'syncInterval': '1h'},
                     'name': None, 'description': None, 'content': None, 'files': None})
                kube(*lease[0], 'patch', lease[1], lease[2], '--type=merge',
                     '-p', json.dumps({'spec': {'managementPolicies': ['Create', 'Observe', 'Update', 'LateInitialize']}}))
                orphan = ([], 'api-fixture', path)
                leases.append(orphan)
                delete(lease)
                if api(path)[0] != 200:
                    raise RuntimeError('Orphaning deleted the owned Git Skill')
                adopted_fields = {'gitSource': {'repoUrl': alternate, 'skillPath': source['skillPath'], 'syncInterval': '1h'},
                                  'icon': 'books', 'environmentIds': [], 'labels': [], 'permissions': []}
                adopted, adopted_id, _ = create(scope, 'skill', 'Skill', 'skills', adopted_fields,
                                                '-git-adopt', external_name=identifier)
                if adopted_id != identifier or git_skill_value()['sourceCommit'] != alternate_commit:
                    raise RuntimeError('Git Skill UUID adoption changed source or identity')
                leases.remove(orphan)
                delete(adopted)
                if api(path)[0] != 404:
                    raise RuntimeError('Adopted Git Skill survived normal deletion')
                delete(environment)
                delete(team)
                record('SkillGitSource', scope, create='passed', settings_and_sharing='passed',
                       settings_preserve_content_version=True, environment_and_labels_clear='passed',
                       source_change_in_place='passed', interval_drift_repair='passed',
                       disconnected_source_reconnect='passed', authored_mode_replacement_refused='passed',
                       orphan='passed', uuid_adoption='passed', identity_preserved=True, delete='soft-delete passed')

                init, init_id, _ = create(scope, 'skill', 'Skill', 'skills',
                    {'icon': 'book', 'environmentIds': [], 'labels': [], 'permissions': []}, '-git-init',
                    init_fields={'gitSource': source})
                init_path = '/api/skills/' + init_id
                if api(init_path)[1]['sourceCommit'] != fixture_commit:
                    raise RuntimeError('Git Skill initProvider source did not create its content')
                patch(init, {'icon': 'books'})
                wait('Git Skill initProvider metadata update', lambda: api(init_path)[1]['icon'] == 'books')
                if observation(init)['metadata']['annotations']['crossplane.io/external-name'] != init_id:
                    raise RuntimeError('Git Skill initProvider metadata changed identity')
                delete(init)
                if api(init_path)[0] != 404:
                    raise RuntimeError('Git Skill initProvider fixture survived normal deletion')
                record('SkillGitSourceInit', scope, parameter_source='initProvider', create='passed',
                       settings_update='passed', identity_preserved=True, delete='soft-delete passed')


            if 'skill' in scope_cases:
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







            if 'credential' in scope_cases:
                credential_key = 'cp-credential-' + run + '-' + scope
                lease, identifier, _ = create(scope, 'credential', 'Credential', 'runtimecredentials',
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
                record('Credential', scope, create='passed', update='passed', drift_repair='passed', delete='passed',
                       external_authorization='not_requested; synthetic secret slot only')

            if 'schedule' in scope_cases:
                parent, agent_id, _ = create(scope, 'agent', 'Agent', 'agents',
                    {'name': 'cp-scheduled-agent-' + run + '-' + scope}, '-schedule')
                project, project_id, _ = create(scope, 'project', 'Project', 'projects',
                    {'name': 'cp-scheduled-project-' + run + '-' + scope, 'defaultAgentId': agent_id}, '-schedule')
                lease, identifier, _ = create(scope, 'schedule', 'Schedule', 'triggers',
                    {'name': 'cp-schedule-' + run + '-' + scope, 'agentId': agent_id, 'projectId': project_id,
                     'cronExpression': '0 8 * * *', 'timezone': 'Etc/UTC', 'enabled': False,
                     'messageTemplate': 'Synthetic disabled schedule; do not execute.'})
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
                delete(project)
                delete(parent)
                record('Schedule', scope, create_disabled='passed', update='passed', drift_repair='passed', delete='passed',
                       scheduled_runs='not_executed')

            if 'api-key' in scope_cases:
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


            if 'tools' in scope_cases:
                catalog, catalog_id, _ = create(scope, 'mcp', 'RegistryEntry', 'registrycatalogitems',
                    {'name': 'cp-tools-' + run + '-' + scope, 'multitenant': False,
                     'remoteConfig': {'url': args.mcp_url}})
                server, server_id, _ = create(scope, 'mcp', 'Server', 'serverinstallations',
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
                delete(server)
                if api(server_path)[0] != 404:
                    raise RuntimeError('Installation delete did not remove the object')
                record('Server', scope, create='passed', immutable_update_refused='passed',
                       tools_discovered=2, delete='passed', backend_after_delete=404)
                delete(catalog)


            if 'identity' in scope_cases:
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
        errors = ['Global API restore is unconfirmed; owned fixtures retained'] if evidence.get('api_cleanup') == 'pending' else []
        for lease in ([] if errors else reversed(leases.copy())):
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
