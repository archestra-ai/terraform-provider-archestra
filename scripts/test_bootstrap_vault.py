import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class VaultStartupTests(unittest.TestCase):
    def test_default_restarts_existing_backend_but_initial_seed_does_not(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            kubectl = root / "kubectl"
            kubectl.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$CAPTURE"\n')
            kubectl.chmod(0o755)
            capture = root / "commands"
            env = {**os.environ, "PATH": str(root) + os.pathsep + os.environ["PATH"], "CAPTURE": str(capture)}
            for value in (None, "false", "true"):
                capture.write_text("")
                selected = {key: item for key, item in env.items() if key != "ARCHESTRA_SKIP_BACKEND_RESTART"}
                if value is not None:
                    selected["ARCHESTRA_SKIP_BACKEND_RESTART"] = value
                result = subprocess.run(["bash", str(Path(__file__).with_name("bootstrap-vault.sh"))],
                        env=selected, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                commands = capture.read_text()
                self.assertIn("apply -f", commands)
                self.assertIn("run vault-seed", commands)
                self.assertEqual("rollout restart deployment" in commands, value != "true")
            capture.write_text("")
            result = subprocess.run(["bash", str(Path(__file__).with_name("bootstrap-vault.sh"))],
                        env={**env, "ARCHESTRA_SKIP_BACKEND_RESTART": "invalid"}, capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(capture.read_text(), "")


if __name__ == "__main__":
    unittest.main()
