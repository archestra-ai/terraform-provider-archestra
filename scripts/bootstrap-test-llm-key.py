#!/usr/bin/env python3
"""Seed one stable model fixture on a loopback-only acceptance backend."""
import json
import os
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError('Local API redirects are refused')


def ensure_fixture(base_url, token, model_url, byos):
    parsed = urllib.parse.urlparse(base_url)
    if parsed.scheme != 'http' or parsed.hostname not in ('localhost', '127.0.0.1', '::1'):
        raise ValueError('Acceptance fixture creation requires a loopback HTTP backend')
    if not token:
        raise ValueError('ARCHESTRA_API_KEY must be set')
    opener = urllib.request.build_opener(NoRedirect())
    endpoint = base_url.rstrip('/') + '/api/llm-provider-api-keys'
    headers = {'Authorization': token, 'Content-Type': 'application/json'}
    with opener.open(urllib.request.Request(endpoint, headers=headers), timeout=15) as response:
        keys = json.load(response)
    name = 'terraform-acceptance-model-fixture'
    matches = [key for key in keys if key['name'] == name]
    if matches:
        if len(matches) != 1 or matches[0]['provider'] != 'ollama':
            raise ValueError('Existing test fixture is not a single Ollama model provider')
        fixture_id = matches[0]['id']
        if matches[0]['baseUrl'] != model_url:
            # Re-runs converge an older fixture on the requested endpoint.
            update = urllib.request.Request(endpoint + '/' + urllib.parse.quote(fixture_id, safe=''),
                                            data=json.dumps({'baseUrl': model_url}).encode(),
                                            headers=headers, method='PATCH')
            with opener.open(update, timeout=45):
                pass
        return fixture_id
    payload = {'name': name, 'provider': 'ollama', 'baseUrl': model_url, 'isPrimary': False}
    if byos:
        payload.update(vaultSecretPath='secret/data/test/ollama', vaultSecretKey='api_key')
    else:
        payload['apiKey'] = 'synthetic-acceptance-model-key'
    request = urllib.request.Request(endpoint, data=json.dumps(payload).encode(), headers=headers)
    with opener.open(request, timeout=45) as response:
        return json.load(response)['id']


def main():
    print(ensure_fixture(os.getenv('ARCHESTRA_BASE_URL', 'http://localhost:9000'),
          os.getenv('ARCHESTRA_API_KEY', ''),
          os.getenv('ARCHESTRA_TEST_LLM_BASE_URL', 'http://ollama-mock.default.svc.cluster.local:11434/v1'),
          os.getenv('ARCHESTRA_READONLY_VAULT_ENABLED') == 'true'))


if __name__ == '__main__':
    main()
