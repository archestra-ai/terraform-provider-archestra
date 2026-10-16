import importlib.util
import json
from pathlib import Path
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

spec = importlib.util.spec_from_file_location('seed_llm', Path(__file__).with_name('bootstrap-test-llm-key.py'))
seed = importlib.util.module_from_spec(spec)
spec.loader.exec_module(seed)


class FixtureTests(unittest.TestCase):
    def setUp(self):
        self.existing = []
        self.created = []
        self.redirect = False
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                if owner.redirect:
                    self.send_response(302)
                    self.send_header('Location', 'https://example.invalid/collect')
                    self.end_headers()
                    return
                self.send_response(200)
                self.end_headers()
                self.wfile.write(json.dumps(owner.existing).encode())

            def do_POST(self):
                owner.created.append(json.loads(self.rfile.read(int(self.headers['Content-Length']))))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b'{"id":"fixture-id"}')

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.url = 'http://127.0.0.1:' + str(self.server.server_port)
        self.model_url = 'http://ollama-mock.default.svc.cluster.local:11434'

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def test_current_byos_payload_has_no_retired_scope_or_inline_secret(self):
        self.assertEqual(seed.ensure_fixture(self.url, 'synthetic-auth', self.model_url, True), 'fixture-id')
        payload = self.created[0]
        self.assertEqual(payload['vaultSecretPath'], 'secret/data/test/ollama')
        self.assertNotIn('scope', payload)
        self.assertNotIn('apiKey', payload)

    def test_reuses_exact_existing_fixture_without_writing(self):
        self.existing = [{'id': 'existing-id', 'name': 'terraform-acceptance-model-fixture', 'provider': 'ollama', 'baseUrl': self.model_url}]
        self.assertEqual(seed.ensure_fixture(self.url, 'synthetic-auth', self.model_url, False), 'existing-id')
        self.assertEqual(self.created, [])

    def test_mismatch_does_not_overwrite_an_existing_object(self):
        self.existing = [{'id': 'existing-id', 'name': 'terraform-acceptance-model-fixture', 'provider': 'other', 'baseUrl': self.model_url}]
        with self.assertRaises(ValueError):
            seed.ensure_fixture(self.url, 'synthetic-auth', self.model_url, False)
        self.assertEqual(self.created, [])

    def test_authentication_is_not_forwarded_to_redirects(self):
        self.redirect = True
        with self.assertRaisesRegex(RuntimeError, 'redirects are refused'):
            seed.ensure_fixture(self.url, 'synthetic-auth', self.model_url, False)
        self.assertEqual(self.created, [])


if __name__ == '__main__':
    unittest.main()
