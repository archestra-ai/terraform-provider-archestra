import http.server
import json
import os
from pathlib import Path
import subprocess
import threading
import unittest


class ApiKeyOriginTests(unittest.TestCase):
    def test_custom_frontend_origin_is_used_for_login_and_key_creation(self):
        requests = []
        origin = 'http://localhost:13002'

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def respond(self, value, status=200):
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps(value).encode())

            def do_GET(self):
                self.respond({})

            def do_POST(self):
                requests.append((self.path, self.headers.get('Origin')))
                self.rfile.read(int(self.headers.get('Content-Length', 0)))
                if self.headers.get('Origin') != origin:
                    self.respond({'error': 'Invalid origin'}, 403)
                elif self.path.endswith('/sign-in/email'):
                    self.respond({'user': {'id': 'test-user'}})
                else:
                    self.respond({'key': 'disposable-fixture-key'})

        with http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler) as server:
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                result = subprocess.run(['bash', str(Path(__file__).with_name('bootstrap-api-key.sh'))],
                                        env={**os.environ, 'ARCHESTRA_BASE_URL': f'http://127.0.0.1:{server.server_port}',
                                             'ARCHESTRA_FRONTEND_URL': origin}, capture_output=True, text=True, check=True)
                self.assertEqual(result.stdout.strip(), 'disposable-fixture-key')
                self.assertEqual(requests, [('/api/auth/sign-in/email', origin), ('/api/auth/api-key/create', origin)])
            finally:
                server.shutdown()
                thread.join()


if __name__ == '__main__':
    unittest.main()
