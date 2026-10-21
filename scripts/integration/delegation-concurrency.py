#!/usr/bin/env python3
"""Probe two native Terraform processes against one owned delegation parent.

Only loopback APIs are allowed. A read barrier exposes stale whole-set writes;
atomic edge mutations do not need that barrier. No credentials are created.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--api-url', required=True)
    parser.add_argument('--key-file', type=Path, required=True)
    parser.add_argument('--terraform', type=Path, required=True)
    parser.add_argument('--provider-directory', type=Path, required=True)
    parser.add_argument('--evidence', type=Path, required=True)
    parser.add_argument('--expect', choices=['lost-update', 'preserved'], required=True)
    args = parser.parse_args()
    endpoint = urllib.parse.urlparse(args.api_url)
    if endpoint.scheme != 'http' or endpoint.hostname not in ('127.0.0.1', 'localhost', '::1'):
        parser.error('Only isolated loopback HTTP APIs are supported')
    workspace = Path(tempfile.mkdtemp(prefix='provider-delegation-concurrency-'))
    os.chmod(workspace, 0o700)
    config = workspace / 'terraform.rc'
    config.write_text('provider_installation {\n dev_overrides {\n "archestra-ai/archestra" = '
                      + json.dumps(str(args.provider_directory.resolve())) + '\n }\n direct {}\n }\n')
    env = dict(os.environ, TF_CLI_CONFIG_FILE=str(config), ARCHESTRA_API_KEY=args.key_file.read_text().strip())
    evidence = {'outcome': 'pending', 'cleanup': 'pending', 'expected': args.expect,
                'provider_sha256': hashlib.sha256((args.provider_directory / 'terraform-provider-archestra').read_bytes()).hexdigest(),
                'environment': 'isolated loopback API; two independent native Terraform processes'}

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            raise RuntimeError('Local API redirects are refused')
    opener = urllib.request.build_opener(NoRedirect())

    def request(path, method='GET', data=None, headers=None):
        request = urllib.request.Request(args.api_url.rstrip('/') + path, data=data, method=method,
                    headers=headers or {'Authorization': env['ARCHESTRA_API_KEY']})
        try:
            with opener.open(request, timeout=45) as response:
                return response.status, response.read()
        except urllib.error.HTTPError as error:
            return error.code, error.read()

    def run(directory, stage, *arguments, expected=0):
        result = subprocess.run([str(args.terraform), *arguments], env=env, cwd=directory,
                                text=True, capture_output=True, timeout=120)
        log = directory / (stage + '.log')
        log.write_text(result.stdout + result.stderr)
        os.chmod(log, 0o600)
        if expected is not None and result.returncode != expected:
            raise RuntimeError(stage + ' failed; private diagnostics: ' + str(directory))
        return result

    def configure(directory, body, base_url):
        directory.mkdir(mode=0o700)
        (directory / 'main.tf').write_text('terraform {\n required_providers {\n archestra = { source = "archestra-ai/archestra" }\n }\n }\n'
                    + 'provider "archestra" {\n base_url = ' + json.dumps(base_url) + '\n }\n' + body)

    base, edges = workspace / 'base', [workspace / 'first', workspace / 'second']
    name = 'tf-delegation-race-' + uuid.uuid4().hex[:8]
    configure(base, '\n'.join('resource "archestra_agent" "' + role + '" {\n name = ' + json.dumps(name + '-' + role) + '\n system_prompt = "Synthetic concurrency fixture."\n}\n'
                              for role in ('parent', 'first', 'second')), args.api_url)
    parent = None
    targets = []
    server = None
    gate = threading.Barrier(2, timeout=15)
    gate_lock = threading.Lock()
    gated_reads = 0
    mutation_counts = {}
    mutation_lock = threading.Lock()

    class Proxy(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def forward(self):
            nonlocal gated_reads
            body = self.rfile.read(int(self.headers.get('Content-Length', '0')))
            headers = {'Authorization': self.headers.get('Authorization', '')}
            if self.headers.get('Content-Type'):
                headers['Content-Type'] = self.headers['Content-Type']
            if self.command == 'POST' and self.path == '/api/agents/' + parent + '/delegations':
                with mutation_lock:
                    status, response = request(self.path, self.command, body if body else None, headers)
            else:
                status, response = request(self.path, self.command, body if body else None, headers)
            should_wait = False
            if self.command == 'GET' and self.path == '/api/agents/' + parent + '/delegations':
                with gate_lock:
                    gated_reads += 1
                    should_wait = gated_reads <= 2
                if should_wait:
                    try:
                        gate.wait()
                    except threading.BrokenBarrierError:
                        pass
            if self.command != 'GET':
                with gate_lock:
                    mutation_counts[self.command + ' ' + self.path] = mutation_counts.get(self.command + ' ' + self.path, 0) + 1
            self.send_response(status)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        do_GET = forward
        do_POST = forward
        do_PUT = forward
        do_DELETE = forward

    try:
        run(base, 'create-owned-agents', 'apply', '-auto-approve', '-input=false', '-no-color')
        state = json.loads(run(base, 'read-agent-identities', 'show', '-json').stdout)
        identities = {item['name']: item['values']['id'] for item in state['values']['root_module']['resources']}
        parent = identities['parent']
        targets = [identities['first'], identities['second']]
        server = ThreadingHTTPServer(('127.0.0.1', 0), Proxy)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        proxy_url = 'http://127.0.0.1:' + str(server.server_port)
        for directory, target in zip(edges, targets):
            configure(directory, 'resource "archestra_agent_delegation" "test" {\n agent_id = ' + json.dumps(parent)
                      + '\n target_agent_id = ' + json.dumps(target) + '\n}\n', proxy_url)
        with ThreadPoolExecutor(max_workers=2) as pool:
            pending = [pool.submit(run, directory, 'concurrent-create', 'apply', '-auto-approve', '-input=false', '-no-color', expected=None)
                       for directory in edges]
            outcomes = [future.result().returncode for future in pending]
        code, body = request('/api/agents/' + parent + '/delegations')
        if code != 200:
            raise RuntimeError('Unable to observe the owned delegation parent')
        value = json.loads(body)
        # The endpoint returns target-agent detail records, whose id is the edge target.
        actual = sorted(item['id'] for item in value)
        preserved = set(targets).issubset(actual)
        evidence.update(native_apply_exit_codes=outcomes, requested_edges=2, observed_edges=len(actual),
                        all_requested_edges_preserved=preserved, stale_reads_synchronized=min(gated_reads, 2),
                        mutation_methods=mutation_counts)
        if args.expect == 'preserved' and (not preserved or outcomes != [0, 0]):
            raise RuntimeError('Concurrent independent edge creation did not preserve both desired edges')
        if args.expect == 'lost-update' and preserved:
            raise RuntimeError('The old provider did not reproduce a lost edge')
        evidence['outcome'] = 'passed'
    except Exception as error:
        evidence['outcome'] = 'failed'
        evidence['error'] = str(error)
        raise
    finally:
        try:
            if parent:
                code, body = request('/api/agents/' + parent + '/delegations')
                if code == 200:
                    current = [item['id'] for item in json.loads(body)]
                    if not set(current).issubset(targets):
                        raise RuntimeError('Unknown edges found; owned parents retained')
                    for target in current:
                        code, _ = request('/api/agents/' + parent + '/delegations/' + target, method='DELETE')
                        if code not in (200, 204, 404):
                            raise RuntimeError('Unable to clean up the owned delegation edge')
            for directory in edges:
                if directory.exists():
                    run(directory, 'destroy-owned-edge-state', 'destroy', '-auto-approve', '-input=false', '-no-color')
            run(base, 'destroy-owned-agents', 'destroy', '-auto-approve', '-input=false', '-no-color')
            evidence['cleanup'] = 'passed'
        except Exception as error:
            evidence['cleanup'] = 'blocked; owned parents retained'
            evidence['cleanup_error'] = str(error)
        if server:
            server.shutdown()
            server.server_close()
        args.evidence.write_text(json.dumps(evidence, indent=2) + '\n')


if __name__ == '__main__':
    main()
