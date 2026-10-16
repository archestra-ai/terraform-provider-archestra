#!/usr/bin/env python3
import json, os, pathlib, shutil, subprocess, urllib.request
import argparse, tempfile, urllib.parse
parser = argparse.ArgumentParser(description='Verify real v2 import and v3 schema upgrade against a disposable local backend.')
parser.add_argument('--v2-provider', type=pathlib.Path, required=True)
parser.add_argument('--v3-provider', type=pathlib.Path, required=True)
parser.add_argument('--terraform', type=pathlib.Path, required=True)
parser.add_argument('--api-key-file', type=pathlib.Path, required=True)
parser.add_argument('--base-url', default='http://127.0.0.1:19000')
parser.add_argument('--evidence', type=pathlib.Path, required=True)
options = parser.parse_args()
if urllib.parse.urlsplit(options.base_url).hostname not in ('127.0.0.1', 'localhost', '::1'):
    raise SystemExit('Only a disposable loopback backend is permitted')
probe = pathlib.Path(tempfile.mkdtemp(prefix='archestra-state-upgrade-'))
probe.chmod(448)
tf = str(options.terraform.resolve())
key = options.api_key_file.read_text().strip()
identifier = None
try:
    request = urllib.request.Request(options.base_url + '/api/llm-provider-api-keys', data=json.dumps({'name': 'v2-v3-state-probe', 'provider': 'ollama', 'vaultSecretPath': 'secret/data/test/ollama', 'vaultSecretKey': 'api_key', 'isPrimary': False}).encode(), headers={'Authorization': key, 'Content-Type': 'application/json'})
    if (probe / 'fixture-id').exists():
        identifier = (probe / 'fixture-id').read_text()
    else:
        with urllib.request.urlopen(request) as response:
            identifier = json.load(response)['id']
    (probe / 'fixture-id').write_text(identifier)
    for version in ['v2', 'v3']:
        d = probe / version
        d.mkdir(exist_ok=True)
        shutil.copy(options.v2_provider if version == 'v2' else options.v3_provider, d / 'terraform-provider-archestra')
        (probe / (version + '.terraformrc')).write_text('provider_installation {\n dev_overrides { "archestra-ai/archestra" = "' + str(d) + '" }\n direct {}\n}\n')
    base = 'terraform {\n required_providers {\n archestra = { source = "archestra-ai/archestra" }\n }\n}\nprovider "archestra" {}\n'

    def config(version):
        flag = 'is_organization_default' if version == 'v2' else 'is_primary'
        (probe / 'main.tf').write_text(base + 'resource "archestra_llm_provider_api_key" "test" {\nname="v2-v3-state-probe"\nllm_provider="ollama"\n' + flag + '=false\n}\n')

    def run(version, *args):
        env = dict(os.environ, ARCHESTRA_API_KEY=key, ARCHESTRA_BASE_URL=options.base_url, TF_CLI_CONFIG_FILE=str(probe / (version + '.terraformrc')))
        p = subprocess.run([tf, *args], cwd=probe, env=env, capture_output=True, text=True)
        (probe / ('-'.join([version, *[a for a in args[:1]]]) + '.log')).write_text(p.stdout + p.stderr)
        if p.returncode not in (0, 2):
            raise RuntimeError(version + ' ' + args[0] + ' failed; inspect local log')
        return p.returncode
    config('v2')
    run('v2', 'import', 'archestra_llm_provider_api_key.test', identifier)
    old = json.loads((probe / 'terraform.tfstate').read_text())
    assert old['resources'][0]['instances'][0]['schema_version'] == 0
    assert old['resources'][0]['instances'][0]['attributes']['is_organization_default'] is False
    config('v3')
    assert run('v3', 'plan', '-detailed-exitcode', '-out=upgrade.tfplan') == 0
    run('v3', 'apply', '-auto-approve', 'upgrade.tfplan')
    new = json.loads((probe / 'terraform.tfstate').read_text())
    attrs = new['resources'][0]['instances'][0]['attributes']
    assert new['resources'][0]['instances'][0]['schema_version'] == 1
    assert attrs['id'] == identifier and attrs['is_primary'] is False and ('is_organization_default' not in attrs)
    assert run('v3', 'plan', '-detailed-exitcode') == 0
    run('v3', 'destroy', '-auto-approve')
    check = urllib.request.Request(options.base_url + '/api/llm-provider-api-keys/' + identifier, headers={'Authorization': key})
    try:
        urllib.request.urlopen(check)
        raise AssertionError('Fixture not deleted')
    except urllib.error.HTTPError as e:
        assert e.code == 404
    options.evidence.write_text(json.dumps({'old_schema': 0, 'new_schema': 1, 'id_preserved': True, 'renamed_flag_preserved': True, 'upgrade_plan_exit_code': 0, 'second_plan_exit_code': 0, 'destroy_read_status': 404}, indent=2) + '\n')
finally:
    if identifier:
        cleanup = urllib.request.Request(options.base_url + '/api/llm-provider-api-keys/' + identifier, headers={'Authorization': key}, method='DELETE')
        try:
            urllib.request.urlopen(cleanup).close()
        except urllib.error.HTTPError as e:
            if e.code != 404:
                raise
shutil.rmtree(probe)
print('PASS: real v2 import -> v3 schema upgrade, no-change plans, same ID, destroy 404')
