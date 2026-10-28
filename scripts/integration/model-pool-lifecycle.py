#!/usr/bin/env python3
"""Prepare or exercise one disabled synthetic pool using a local Terraform binary.

Preparation reads only public loopback endpoints. Execution needs separately
approved existing credentials, an existing shared fixture key, and pool CRUD.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import urllib.error
import urllib.parse
import urllib.request
import uuid

ADDRESS = 'archestra_model_pool.test'
SOURCE = 'archestra-ai/archestra'
MODELS = ('tf-acc-primary', 'tf-acc-fallback')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError('API redirects are refused')


def local_url(raw):
    parsed = urllib.parse.urlsplit(raw)
    if (parsed.scheme != 'http' or parsed.hostname not in ('127.0.0.1', 'localhost', '::1')
            or parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.path not in ('', '/')):
        raise ValueError('Use a plain loopback HTTP API origin, without credentials or a path')
    return raw.rstrip('/')


def request(base, path, key=None, method='GET', body=None):
    headers = {'Content-Type': 'application/json'}
    if key is not None:
        headers['Authorization'] = key
    req = urllib.request.Request(base + path, method=method, headers=headers,
                                 data=None if body is None else json.dumps(body).encode())
    try:
        with urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect()).open(req, timeout=20) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        status = error.code
        error.close()
        return status, None


def public_preflight(base):
    status, health = request(base, '/health')
    if status != 200 or not isinstance(health, dict) or health.get('status') != 'ok':
        raise RuntimeError('Public health check failed')
    status, document = request(base, '/openapi.json')
    if status != 200:
        raise RuntimeError('Public OpenAPI check failed')
    paths = document.get('paths', {})
    for path, methods in {'/api/model-pools': ('get', 'post'), '/api/model-pools/{id}': ('put', 'delete')}.items():
        if any(method not in paths.get(path, {}) for method in methods):
            raise RuntimeError('API does not advertise all pool CRUD operations')
    schema = paths['/api/model-pools']['post']['requestBody']['content']['application/json']['schema']
    fields = schema.get('properties', {})
    if not {'name', 'members'}.issubset(fields):
        raise RuntimeError('Pool input contract is incompatible')
    if 'virtualApiKeyIds' in fields:
        raise RuntimeError('Old pool-owned grant contract is unsupported; use the credential-owned grant API')
    return {'api_version': health.get('version'), 'pool_write_fields': sorted(fields),
            'credential_grants_writable_on_pool': 'virtualApiKeyIds' in fields,
            'feature_flag_and_authorization': 'not established by public schema'}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')
    path.chmod(0o600)


def configuration(name, weight=3):
    members = [{'provider_api_key_id': '${var.provider_api_key_id}', 'model_id': model,
                'priority': index + 1, 'weight': weight if index == 0 else 1, 'enabled': False}
               for index, model in enumerate(MODELS)]
    # JSON Terraform configuration keeps literal strings and interpolation separate from shell syntax.
    return {'terraform': {'required_providers': {'archestra': {'source': SOURCE}}},
            'provider': {'archestra': {}}, 'variable': {'provider_api_key_id': {'type': 'string'}},
            'resource': {'archestra_model_pool': {'test': {'name': name, 'members': members}}},
            'output': {'pool_id': {'value': '${archestra_model_pool.test.id}'}}}


def prepare(args):
    base = local_url(args.api_url)
    binary = args.provider_directory.resolve() / 'terraform-provider-archestra'
    if not binary.is_file() or not os.access(binary, os.X_OK):
        raise ValueError('Build the local provider first')
    preflight = public_preflight(base)
    workspace = args.workspace.resolve()
    workspace.mkdir(mode=0o700, parents=False, exist_ok=False)
    name = 'tf-acc-t2089-' + uuid.uuid4().hex[:12]
    manifest = {'api_url': base, 'provider_directory': str(binary.parent),
                'provider_sha256': digest(binary), 'terraform': str(args.terraform.resolve()),
                'name': name, 'preflight': preflight}
    write_json(workspace / 'manifest.json', manifest)
    write_json(workspace / 'main.tf.json', configuration(name))
    (workspace / 'terraform.rc').write_text('provider_installation {\n dev_overrides {\n "' + SOURCE
        + '" = ' + json.dumps(str(binary.parent)) + '\n }\n direct {}\n}\n')
    (workspace / 'terraform.rc').chmod(0o600)
    write_json(workspace / 'evidence.json', {'outcome': 'prepared-only', 'cleanup': 'no mutations',
               'name': name, 'preflight': preflight, 'provider_sha256': manifest['provider_sha256']})
    print(json.dumps({'workspace': str(workspace), 'name': name, 'preflight': preflight,
                      'next': 'Review files, obtain scoped execution approval, then use the run subcommand.'}, indent=2))


def terraform_process(command, *, env, cwd):
    process = subprocess.Popen(command, env=env, cwd=cwd, text=True,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    try:
        output, error = process.communicate(timeout=180)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGTERM)
            process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.communicate()
        except ProcessLookupError:
            process.communicate()
        raise RuntimeError('Terraform timed out; server mutation outcome may be uncertain') from None
    return subprocess.CompletedProcess(command, process.returncode, output, error)


def execute(args):
    workspace = args.workspace.resolve()
    manifest = json.loads((workspace / 'manifest.json').read_text())
    base = local_url(manifest['api_url'])
    previous = json.loads((workspace / 'evidence.json').read_text())
    if previous.get('outcome') != 'prepared-only' or any(workspace.glob('*.tfstate*')):
        raise ValueError('Use a fresh prepared workspace with no existing Terraform state')
    if (list(workspace.glob('*.tf')) or set(workspace.glob('*.tf.json')) != {workspace / 'main.tf.json'}
            or json.loads((workspace / 'main.tf.json').read_text()) != configuration(manifest['name'])):
        raise ValueError('Prepared configuration changed; prepare a new workspace')
    provider = Path(manifest['provider_directory']) / 'terraform-provider-archestra'
    if digest(provider) != manifest['provider_sha256']:
        raise ValueError('Provider binary changed; prepare a new workspace')
    provider_id = str(uuid.UUID(args.provider_key_id))
    if provider_id != args.provider_key_id:
        raise ValueError('Use a lowercase canonical shared fixture key UUID')
    preflight = public_preflight(base)
    key = args.key_file.read_text().strip()
    if not key.startswith('arch_') or '\n' in key or '\r' in key:
        raise ValueError('Expected an existing Archestra management API credential')
    (workspace / 'terraform.rc').write_text('provider_installation {\n dev_overrides {\n "' + SOURCE
        + '" = ' + json.dumps(str(provider.parent)) + '\n }\n direct {}\n}\n')
    env = {k: v for k, v in os.environ.items() if not k.startswith(('TF_LOG', 'TF_CLI_ARGS'))
           and k.lower() not in ('tf_workspace', 'tf_data_dir', 'http_proxy', 'https_proxy', 'all_proxy', 'no_proxy')}
    env.update(TF_CLI_CONFIG_FILE=str(workspace / 'terraform.rc'), TF_DATA_DIR=str(workspace / '.terraform'),
               ARCHESTRA_BASE_URL=base, ARCHESTRA_API_KEY=key, TF_IN_AUTOMATION='1', TF_INPUT='0')
    record = {'outcome': 'pending', 'cleanup': 'pending', 'name': manifest['name'],
              'provider_sha256': manifest['provider_sha256'], 'preflight': preflight, 'steps': []}
    evidence = workspace / 'evidence.json'
    write_json(evidence, record)
    identifier = None
    attempted = False
    creation_uncertain = False

    def pools():
        status, values = request(base, '/api/model-pools', key)
        if status != 200 or not isinstance(values, list):
            raise RuntimeError('Authenticated pool listing failed; check feature flag and permissions')
        return values

    def owned(pool):
        return (pool.get('name', '').startswith(manifest['name']) and not pool.get('virtualApiKeyIds')
                and len(pool.get('members', [])) == len(MODELS)
                and {m.get('modelId') for m in pool['members']} == set(MODELS)
                and all(m.get('providerApiKeyId') == provider_id and m.get('enabled') is False for m in pool['members']))

    def run(stage, *arguments, expected=0):
        result = terraform_process([manifest['terraform'], *arguments], env=env, cwd=workspace)
        (workspace / (stage + '.log')).write_text((result.stdout + result.stderr).replace(key, '[redacted]'))
        if result.returncode != expected:
            raise RuntimeError(stage + ' failed; inspect private workspace logs')
        record['steps'].append(stage)
        write_json(evidence, record)
        return result.stdout

    def remote():
        matches = [p for p in pools() if p.get('id') == identifier]
        if len(matches) != 1 or not owned(matches[0]):
            raise RuntimeError('Synthetic pool ownership changed; refusing further mutation')
        return matches[0]

    try:
        if any(p.get('name', '').startswith(manifest['name']) for p in pools()):
            raise RuntimeError('Synthetic name already exists; prepare a new workspace')
        write_json(workspace / 'terraform.tfvars.json', {'provider_api_key_id': provider_id})
        run('validate', 'validate', '-no-color')
        attempted = True
        creation_uncertain = True
        run('create', 'apply', '-auto-approve', '-input=false', '-no-color')
        identifier = str(uuid.UUID(run('id', 'output', '-raw', 'pool_id').strip()))
        record['pool_id'] = identifier
        remote()
        creation_uncertain = False
        run('read-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        updated_name = manifest['name'] + '-updated'
        write_json(workspace / 'main.tf.json', configuration(updated_name, 7))
        run('update', 'apply', '-auto-approve', '-input=false', '-no-color')
        if remote()['name'] != updated_name:
            raise RuntimeError('Update did not persist the requested name')
        run('forget-local-binding', 'state', 'rm', ADDRESS)
        run('import', 'import', '-input=false', '-no-color', ADDRESS, identifier)
        run('import-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        current = remote()
        body = {'name': manifest['name'] + '-drift', 'members': [
            {field: m[field] for field in ('providerApiKeyId', 'modelId', 'priority', 'weight', 'enabled')}
            for m in current['members']]}
        status, _ = request(base, '/api/model-pools/' + identifier, key, 'PUT', body)
        if status != 200:
            raise RuntimeError('Owned-pool drift injection failed')
        run('drift-plan', 'plan', '-detailed-exitcode', '-input=false', '-no-color', '-out=drift.tfplan', expected=2)
        plan = json.loads(run('drift-plan-json', 'show', '-json', 'drift.tfplan'))
        changes = [c for c in plan.get('resource_changes', []) if c['change']['actions'] != ['no-op']]
        if len(changes) != 1 or changes[0]['address'] != ADDRESS or changes[0]['change']['actions'] != ['update']:
            raise RuntimeError('Drift plan would mutate something other than the one owned pool in place')
        run('repair-drift', 'apply', '-input=false', '-no-color', 'drift.tfplan')
        run('final-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        record['outcome'] = 'passed'
    except Exception as error:
        record['outcome'] = 'failed'
        record['error'] = str(error)
    finally:
        if attempted:
            try:
                matches = [p for p in pools() if p.get('id') == identifier or
                           (identifier is None and p.get('name', '').startswith(manifest['name']))]
                record['cleanup_candidate_ids'] = [p.get('id') for p in matches]
                if not matches and creation_uncertain:
                    raise RuntimeError('Create outcome is uncertain; an immediate empty list cannot establish cleanup')
                if len(matches) > 1 or any(not owned(p) for p in matches):
                    raise RuntimeError('Ambiguous or changed ownership; pool retained for manual review')
                if matches:
                    identifier = matches[0]['id']
                    record['pool_id'] = identifier
                    run('destroy', 'destroy', '-auto-approve', '-input=false', '-no-color')
                    if any(p.get('id') == identifier for p in pools()):
                        status, _ = request(base, '/api/model-pools/' + identifier, key, 'DELETE')
                        if status not in (200, 404):
                            raise RuntimeError('Owned-pool cleanup failed')
                if any(p.get('id') == identifier for p in pools()):
                    raise RuntimeError('Owned pool remains after cleanup')
                record['cleanup'] = 'passed'
            except Exception as error:
                record['cleanup'] = 'blocked; retain workspace and any recorded pool identity'
                record['cleanup_error'] = str(error)
        else:
            record['cleanup'] = 'no mutations attempted'
        write_json(evidence, record)
    print(json.dumps({'outcome': record['outcome'], 'cleanup': record['cleanup'], 'evidence': str(evidence)}, indent=2))
    return 0 if record['outcome'] == 'passed' and record['cleanup'] == 'passed' else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='mode', required=True)
    prep = sub.add_parser('prepare', help='Read public API capabilities and write private reviewable files; no credentials')
    prep.add_argument('--api-url', required=True)
    prep.add_argument('--provider-directory', type=Path, required=True)
    prep.add_argument('--terraform', type=Path, default=Path(shutil.which('terraform') or 'terraform'))
    prep.add_argument('--workspace', type=Path, required=True)
    run = sub.add_parser('run', help='Execute only after approval for existing credential use and one synthetic pool lifecycle')
    run.add_argument('--workspace', type=Path, required=True)
    run.add_argument('--key-file', type=Path, required=True)
    run.add_argument('--provider-key-id', required=True)
    args = parser.parse_args()
    os.umask(0o077)
    if args.mode == 'prepare':
        prepare(args)
        return 0
    return execute(args)


if __name__ == '__main__':
    raise SystemExit(main())
