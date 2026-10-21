#!/usr/bin/env python3
"""Stateless MCP fixture with two tools and no external calls; binds loopback by default."""
import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port', type=int, default=19006)
    # The in-cluster fixture (scripts/k8s/provider-mcp-mock.yaml) listens on the pod network.
    parser.add_argument('--host', default='127.0.0.1')
    args = parser.parse_args()

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            identifier = request.get('id')
            if identifier is None:
                self.send_response(204)
                self.end_headers()
                return
            method = request.get('method')
            if method == 'initialize':
                result = {'protocolVersion': request['params']['protocolVersion'],
                          'capabilities': {'tools': {}},
                          'serverInfo': {'name': 'provider-test-mock', 'version': '1.0.0'}}
            elif method == 'tools/list':
                result = {'tools': [{'name': name, 'description': 'Synthetic provider lifecycle fixture',
                           'inputSchema': {'type': 'object', 'properties': {'text': {'type': 'string'}}}}
                          for name in ('echo', 'inspect')]}
            elif method == 'tools/call':
                result = {'content': [{'type': 'text', 'text': 'synthetic fixture response'}]}
            else:
                result = {}
            body = json.dumps({'jsonrpc': '2.0', 'id': identifier, 'result': result}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    print('MCP fixture listening on', args.host, 'port', args.port, flush=True)
    ThreadingHTTPServer((args.host, args.port), Handler).serve_forever()


if __name__ == '__main__':
    main()
