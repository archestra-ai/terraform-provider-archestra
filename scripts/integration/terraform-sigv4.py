#!/usr/bin/env python3
"""Exercise synthetic Bedrock credentials against an isolated DB-mode API."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import urllib.parse
import urllib.error
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--api-url', required=True)
    parser.add_argument('--model-url', required=True)
    parser.add_argument('--key-file', type=Path, required=True)
    parser.add_argument('--terraform', type=Path, required=True)
    parser.add_argument('--provider-directory', type=Path, required=True)
    parser.add_argument('--evidence', type=Path, required=True)
    args = parser.parse_args()
    for value in (args.api_url, args.model_url):
        parsed = urllib.parse.urlparse(value)
        if parsed.scheme != 'http' or parsed.hostname not in ('127.0.0.1', 'localhost', '::1'):
            parser.error('Only loopback test endpoints are supported')
    workspace = Path(tempfile.mkdtemp(prefix='provider-sigv4-'))
    os.chmod(workspace, 0o700)
    config = workspace / 'terraform.rc'
    config.write_text('provider_installation {\n dev_overrides {\n "archestra-ai/archestra" = '
                      + json.dumps(str(args.provider_directory.resolve())) + '\n }\n direct {}\n }\n')
    env = dict(os.environ, TF_CLI_CONFIG_FILE=str(config), ARCHESTRA_BASE_URL=args.api_url,
               ARCHESTRA_API_KEY=args.key_file.read_text().strip())
    evidence = {'outcome': 'pending', 'cleanup': 'pending', 'steps': [],
                'provider_sha256': hashlib.sha256((args.provider_directory / 'terraform-provider-archestra').read_bytes()).hexdigest(),
                'environment': 'isolated DB-mode API; synthetic AWS values; loopback model fixture'}

    def run(stage, *arguments, expected=0):
        result = subprocess.run([str(args.terraform), *arguments], env=env, cwd=workspace,
                                text=True, capture_output=True, timeout=90)
        log = workspace / (stage + '.log')
        log.write_text(result.stdout + result.stderr)
        os.chmod(log, 0o600)
        if result.returncode != expected:
            raise RuntimeError(stage + ' failed; private diagnostic workspace: ' + str(workspace))
        return result.stdout

    name = 'tf-sigv4-' + uuid.uuid4().hex[:8]

    def write(secret, token, credentials=True):
        material = ('aws_access_key_id = "synthetic-access-key"\n'
                    'aws_secret_access_key = ' + json.dumps(secret) + '\n'
                    + ('aws_session_token = ' + json.dumps(token) + '\n' if token else '')) if credentials else ''
        (workspace / 'main.tf').write_text('terraform {\n required_providers {\n archestra = { source = "archestra-ai/archestra" }\n }\n }\n'
            'provider "archestra" {}\nresource "archestra_llm_provider_api_key" "test" {\n'
            'name = ' + json.dumps(name) + '\nllm_provider = "bedrock"\nbase_url = ' + json.dumps(args.model_url)
            + '\n' + material + '}\n')

    identifier = None
    try:
        write('synthetic-secret-initial', 'synthetic-session-initial')
        run('create', 'apply', '-auto-approve', '-input=false', '-no-color')
        state = json.loads(run('state', 'show', '-json'))
        identifier = state['values']['root_module']['resources'][0]['values']['id']
        run('initial-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        evidence['steps'].append('create and no-op')
        write('synthetic-secret-initial', 'synthetic-session-rotated')
        run('rotate-session', 'apply', '-auto-approve', '-input=false', '-no-color')
        write('synthetic-secret-rotated', '')
        run('rotate-pair-clear-session', 'apply', '-auto-approve', '-input=false', '-no-color')
        run('rotation-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        evidence['steps'].append('session rotation, paired-secret rotation, session removal and no-op')
        run('forget-binding', 'state', 'rm', 'archestra_llm_provider_api_key.test')
        run('import', 'import', 'archestra_llm_provider_api_key.test', identifier)
        run('adopt-secret-inputs', 'apply', '-auto-approve', '-input=false', '-no-color')
        state = json.loads(run('state-after-import', 'show', '-json'))
        if state['values']['root_module']['resources'][0]['values']['id'] != identifier:
            raise RuntimeError('Import replaced the key')
        run('import-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        evidence['steps'].append('import, explicit secret re-adoption, stable ID and no-op')
        write('', '', credentials=False)
        run('stop-managing-secret', 'apply', '-auto-approve', '-input=false', '-no-color')
        run('unmanaged-noop', 'plan', '-detailed-exitcode', '-input=false', '-no-color')
        evidence['steps'].append('omission stops managing write-only inputs; does not revoke remote credentials')
        evidence['outcome'] = 'passed'
    except Exception as error:
        evidence['outcome'] = 'failed'
        evidence['error'] = str(error)
        raise
    finally:
        try:
            run('destroy', 'destroy', '-auto-approve', '-input=false', '-no-color')
            if identifier:
                class NoRedirect(urllib.request.HTTPRedirectHandler):
                    def redirect_request(self, *unused):
                        raise RuntimeError('Redirect refused')
                request = urllib.request.Request(args.api_url.rstrip('/') + '/api/llm-provider-api-keys/' + identifier,
                            headers={'Authorization': env['ARCHESTRA_API_KEY']})
                try:
                    urllib.request.build_opener(NoRedirect()).open(request, timeout=15)
                    raise RuntimeError('Destroyed key remains readable')
                except urllib.error.HTTPError as error:
                    if error.code != 404:
                        raise
            evidence['cleanup'] = 'passed'
        except Exception:
            evidence['cleanup'] = 'failed; retain private workspace ' + str(workspace)
        args.evidence.write_text(json.dumps(evidence, indent=2) + '\n')


if __name__ == '__main__':
    main()
