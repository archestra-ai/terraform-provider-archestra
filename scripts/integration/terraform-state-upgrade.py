#!/usr/bin/env python3
"""Import current synthetic resources with v2, then verify native v3 upgrades."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import urllib.parse
import urllib.request
import urllib.error
import uuid


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--api-url', required=True)
    p.add_argument('--model-url', required=True)
    p.add_argument('--key-file', type=Path, required=True)
    p.add_argument('--terraform', type=Path, required=True)
    p.add_argument('--old-provider-directory', type=Path, required=True)
    p.add_argument('--new-provider-directory', type=Path, required=True)
    p.add_argument('--evidence', type=Path, required=True)
    p.add_argument('--case', choices=('base', 'identity', 'model'), default='base')
    args = p.parse_args()
    for endpoint in (args.api_url, args.model_url):
        url = urllib.parse.urlparse(endpoint)
        if url.scheme != 'http' or url.hostname not in ('127.0.0.1', 'localhost', '::1'):
            p.error('Only isolated loopback HTTP endpoints are supported')
    work = Path(tempfile.mkdtemp(prefix='provider-state-upgrade-'))
    os.chmod(work, 0o700)
    env = dict(os.environ, ARCHESTRA_BASE_URL=args.api_url,
               ARCHESTRA_API_KEY=args.key_file.read_text().strip(),
               TF_CLI_CONFIG_FILE=str(work / 'terraform.rc'))
    evidence = {'outcome': 'pending', 'cleanup': 'pending', 'resources': [],
                'old_binary_sha256': hashlib.sha256((args.old_provider_directory / 'terraform-provider-archestra').read_bytes()).hexdigest(),
                'new_binary_sha256': hashlib.sha256((args.new_provider_directory / 'terraform-provider-archestra').read_bytes()).hexdigest()}
    name = 'tf-upgrade-' + uuid.uuid4().hex[:8]
    (work / 'main.tf').write_text('''terraform {
 required_providers {
  archestra = { source = "archestra-ai/archestra" }
 }
}
provider "archestra" {}
resource "archestra_agent" "test" {
 name = "NAME-agent"
 system_prompt = "Synthetic migration fixture."
}
resource "archestra_mcp_gateway" "test" {
 name = "NAME-gateway"
}
resource "archestra_team" "test" {
 name = "NAME-team"
}
resource "archestra_mcp_registry_catalog_item" "test" {
 name = "NAME-catalog"
 remote_config = { url = "http://127.0.0.1:19999/mcp" }
}
resource "archestra_llm_provider_api_key" "test" {
 name = "NAME-key"
 llm_provider = "ollama"
 base_url = MODEL
}
resource "archestra_organization_settings" "test" {
 font = "lato"
 color_theme = "cosmic-night"
}
'''.replace('NAME', name).replace('MODEL', json.dumps(args.model_url)))
    if args.case == 'identity':
        (work / 'main.tf').write_text('''terraform {
 required_providers {
  archestra = { source = "archestra-ai/archestra" }
 }
}
provider "archestra" {}
resource "archestra_identity_provider" "test" {
 provider_id = "NAME"
 domain = "NAME.example.invalid"
 issuer = "https://identity.example.invalid"
 oidc_config {
  issuer = "https://identity.example.invalid"
  discovery_endpoint = "https://identity.example.invalid/.well-known/openid-configuration"
  client_id = "synthetic-client"
  client_secret = "synthetic-local-only"
  skip_discovery = true
 }
}
'''.replace('NAME', name))
    elif args.case == 'model':
        with (work / 'main.tf').open('a') as stream:
            stream.write('''
resource "archestra_llm_model" "test" {
 model_id = "llama3"
 depends_on = [archestra_llm_provider_api_key.test]
}
''')

    def select(directory):
        (work / 'terraform.rc').write_text('provider_installation {\n dev_overrides {\n "archestra-ai/archestra" = '
                         + json.dumps(str(directory.resolve())) + '\n }\n direct {}\n }\n')

    def run(stage, *parts, expected=0):
        result = subprocess.run([str(args.terraform), *parts], cwd=work, env=env,
                                text=True, capture_output=True, timeout=90)
        log = work / (stage + '.log')
        log.write_text(result.stdout + result.stderr)
        os.chmod(log, 0o600)
        if result.returncode != expected:
            raise RuntimeError(stage + ' failed; private diagnostics: ' + str(work))
        return result.stdout

    identifiers = {}
    try:
        select(args.new_provider_directory)
        run('create', 'apply', '-auto-approve', '-input=false', '-no-color')
        current = json.loads(run('state-v3', 'state', 'pull'))
        identifiers = {r['type'] + '.' + r['name']: r['instances'][0]['attributes']['id'] for r in current['resources']}
        run('forget-synthetic-bindings', 'state', 'rm', *identifiers)
        select(args.old_provider_directory)
        for address, identifier in identifiers.items():
            import_id = 'llama3' if address == 'archestra_llm_model.test' else identifier
            run('v2-import-' + address, 'import', address, import_id)
        old = json.loads(run('state-v2', 'state', 'pull'))
        if args.case == 'identity':
            old['resources'][0]['instances'][0]['attributes']['oidc_config']['client_secret'] = 'synthetic-local-only'
            old['serial'] += 1
            (work / 'saved-secret.tfstate').write_text(json.dumps(old))
            os.chmod(work / 'saved-secret.tfstate', 0o600)
            run('restore-synthetic-saved-secret', 'state', 'push', 'saved-secret.tfstate')
            evidence['identity_fixture_source'] = 'Actual v2 import; synthetic saved secret restored because v2 cannot read current redacted secrets. This models saved pre-redaction state, not a successful v2 create against the current API.'
        for resource in old['resources']:
            if resource['instances'][0]['schema_version'] != 0:
                raise RuntimeError('Old fixture did not use a v2 schema-zero state')
        select(args.new_provider_directory)
        schemas = json.loads(run('new-schemas', 'providers', 'schema', '-json'))
        versions = schemas['provider_schemas']['registry.terraform.io/archestra-ai/archestra']['resource_schemas']
        run('upgrade-plan', 'plan', '-input=false', '-no-color', '-detailed-exitcode', '-out=upgrade.tfplan')
        run('upgrade-apply', 'apply', '-input=false', '-no-color', 'upgrade.tfplan')
        upgraded = json.loads(run('state-upgraded', 'state', 'pull'))
        for resource in upgraded['resources']:
            address = resource['type'] + '.' + resource['name']
            instance = resource['instances'][0]
            version = versions[resource['type']]['version']
            if instance['attributes']['id'] != identifiers[address] or instance['schema_version'] != version:
                raise RuntimeError('Upgrade failed to preserve identity and upgrade schema: ' + address)
            evidence['resources'].append({'resource': resource['type'], 'old_schema': 0, 'new_schema': version,
                                         'id_preserved': True, 'upgrade_plan': 'no changes'})
            if resource['type'] == 'archestra_identity_provider':
                if instance['attributes']['oidc_config']['client_secret'] != 'synthetic-local-only':
                    raise RuntimeError('Identity upgrade lost the saved secret')
                evidence['identity_secret_preserved'] = True
        run('second-plan', 'plan', '-input=false', '-no-color', '-detailed-exitcode')
        evidence['second_plan'] = 'no changes'
        evidence['outcome'] = 'passed'
    except Exception as error:
        evidence['outcome'] = 'failed'
        evidence['error'] = str(error)
        raise
    finally:
        select(args.new_provider_directory)
        try:
            current = json.loads(run('cleanup-bindings', 'state', 'pull'))
            bound = {r['type'] + '.' + r['name'] for r in current.get('resources', [])}
            for address, identifier in identifiers.items():
                if address not in bound:
                    run('restore-cleanup-' + address, 'import', address, identifier)
            run('cleanup-v3', 'destroy', '-auto-approve', '-input=false', '-no-color')
            class NoRedirect(urllib.request.HTTPRedirectHandler):
                def redirect_request(self, *unused):
                    raise RuntimeError('Redirect refused')
            opener = urllib.request.build_opener(NoRedirect())
            paths = {'archestra_agent': '/api/agents/', 'archestra_mcp_gateway': '/api/agents/',
                     'archestra_team': '/api/teams/', 'archestra_llm_provider_api_key': '/api/llm-provider-api-keys/',
                     'archestra_identity_provider': '/api/identity-providers/',
                     'archestra_mcp_registry_catalog_item': '/api/internal_mcp_catalog/'}
            for address, identifier in identifiers.items():
                kind = address.split('.')[0]
                endpoint = paths[kind] + identifier if kind in paths else '/api/organization'
                request = urllib.request.Request(args.api_url.rstrip('/') + endpoint,
                            headers={'Authorization': env['ARCHESTRA_API_KEY']})
                try:
                    response = opener.open(request, timeout=15)
                    status = response.status
                    response.close()
                except urllib.error.HTTPError as error:
                    status = error.code
                if status != (404 if kind in paths else 200):
                    raise RuntimeError('Backend cleanup verification failed: ' + kind)
            evidence['cleanup'] = 'passed'
        except Exception:
            evidence['cleanup'] = 'failed; retain private diagnostics: ' + str(work)
        args.evidence.write_text(json.dumps(evidence, indent=2) + '\n')


if __name__ == '__main__':
    main()
