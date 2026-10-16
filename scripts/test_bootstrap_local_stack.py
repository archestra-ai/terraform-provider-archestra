import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


class StackIsolationTests(unittest.TestCase):
    def test_private_kubeconfig_and_custom_ports_reach_child_bootstrappers(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            scripts = root / 'scripts'
            scripts.mkdir()
            shutil.copy(Path(__file__).with_name('bootstrap-local-stack.sh'), scripts)
            (root / '.github').mkdir()
            (root / '.github/kind.yaml').write_text('hostPort: 9000\nhostPort: 3000\nhostPort: 9050\n')
            (root / '.github/values-ci.yaml').write_text('')
            binary_dir = root / 'bin'
            binary_dir.mkdir()
            tool = '''#!/bin/bash
set -euo pipefail
printf '%s %s %s\\n' "$(basename "$0")" "${KUBECONFIG:-unset}" "$*" >> "$CAPTURE/commands"
case "$(basename "$0") $*" in
  'kind get clusters') exit 0 ;;
  'kind get nodes'*) echo isolated-node ;;
  'kind load docker-image'*) [ "${FAIL_IMAGE_IMPORT:-0}" != 1 ] || exit 1 ;;
  'kind create cluster'*)
    while [ "$#" -gt 0 ]; do
      if [ "$1" = --config ]; then cp "$2" "$CAPTURE/rendered-kind"; fi
      shift
    done
    ;;
  'kind get kubeconfig'*) echo private-test-config ;;
  'curl '*) echo 200 ;;
  'jq '*) echo existing-key ;;
esac
'''
            for name in ['docker', 'kind', 'kubectl', 'helm', 'curl', 'jq']:
                file = binary_dir / name
                file.write_text(tool)
                file.chmod(0o755)
            for name in ['bootstrap-ollama-mock.sh', 'bootstrap-vault.sh', 'bootstrap-test-idp.sh']:
                file = scripts / name
                file.write_text('#!/bin/bash\nprintf "child %s %s\\n" "$KUBECONFIG" "$ARCHESTRA_BASE_URL" >> "$CAPTURE/commands"\n')
                file.chmod(0o755)
            file = scripts / 'bootstrap-api-key.sh'
            file.write_text('#!/bin/bash\necho test-key\n')
            file.chmod(0o755)
            original = root / 'user-kubeconfig'
            original.write_text('preserve-current-context')
            env = {**os.environ, 'PATH': f'{binary_dir}:{os.environ["PATH"]}',
                   'CAPTURE': str(root), 'KUBECONFIG': str(original),
                   'ARCHESTRA_VERSION': 'test', 'KIND_CLUSTER': 'isolated-provider',
                   'ARCHESTRA_TEST_BACKEND_PORT': '19000',
                   'ARCHESTRA_TEST_FRONTEND_PORT': '13000',
                   'ARCHESTRA_TEST_METRICS_PORT': '19050'}
            subprocess.run(['bash', str(scripts / 'bootstrap-local-stack.sh')], env=env, check=True, capture_output=True)
            commands = (root / 'commands').read_text()
            self.assertNotIn('use-context', commands)
            self.assertNotIn(str(original), commands)
            self.assertIn('http://localhost:19000', commands)
            self.assertIn('child ', commands)
            self.assertEqual(original.read_text(), 'preserve-current-context')
            env['FAIL_IMAGE_IMPORT'] = '1'
            subprocess.run(['bash', str(scripts / 'bootstrap-local-stack.sh')], env=env, check=True, capture_output=True)
            self.assertIn('docker.io/archestra/platform:test', (root / 'commands').read_text())
            config = (root / 'rendered-kind').read_text()
            for port in ['19000', '13000', '19050']:
                self.assertIn(f'hostPort: {port}', config)

    def test_rejects_invalid_ports_before_tool_execution(self):
        for port in ['0', '65536', 'localhost:9000', '9000; echo bad']:
            result = subprocess.run(['bash', str(Path(__file__).with_name('bootstrap-local-stack.sh')), '--print-env'],
                                    env={**os.environ, 'ARCHESTRA_TEST_BACKEND_PORT': port}, capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'Invalid test port', result.stderr)


if __name__ == '__main__':
    unittest.main()
