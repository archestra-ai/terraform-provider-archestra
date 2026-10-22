import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


REPO = Path(__file__).resolve().parent.parent


class CoverageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'scripts').mkdir()
        shutil.copy2(REPO / 'scripts/crossplane-coverage.sh', self.root / 'scripts')
        (self.root / 'crossplane/config').mkdir(parents=True)
        self.mapping = self.root / 'crossplane/config/external_name.go'
        self.mapping.write_text('"archestra_alpha"\n"archestra_orphan"\n')
        (self.root / '.github/badges').mkdir(parents=True)
        self.badge = self.root / '.github/badges/crossplane-coverage.json'
        self.badge.write_text(
            '{"schemaVersion":1,"label":"crossplane coverage",'
            '"message":"1/2 (50%)","color":"yellow"}\n'
        )
        binary_dir = self.root / 'bin'
        binary_dir.mkdir()
        # Stub the Go process boundary; execute the real report/check script.
        go = binary_dir / 'go'
        go.write_text(
            '#!/bin/sh\n'
            '[ "$*" = "run ./cmd/provider-contracts" ] || exit 2\n'
            '[ "${FAIL_GO:-0}" = 0 ] || exit 1\n'
            'echo \'{"archestra_alpha":{},"archestra_beta":{}}\'\n'
        )
        go.chmod(0o755)
        self.env = {**os.environ, 'PATH': f'{binary_dir}:{os.environ["PATH"]}'}

    def run_script(self, *args):
        return subprocess.run(
            ['bash', 'scripts/crossplane-coverage.sh', *args],
            cwd=self.root, env=self.env, text=True, capture_output=True,
        )

    def test_current_badge_passes_without_writes(self):
        before = self.badge.read_bytes()
        result = self.run_script('--check-badge', str(self.badge))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.badge.read_bytes(), before)

    def test_changed_mappings_fail_until_badge_is_regenerated(self):
        before = self.badge.read_bytes()
        self.mapping.write_text('"archestra_alpha"\n"archestra_beta"\n')
        result = self.run_script('--check-badge', str(self.badge))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('--badge', result.stderr)
        self.assertEqual(self.badge.read_bytes(), before)
        result = self.run_script('--badge', str(self.badge))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(self.badge.read_text())['message'], '2/2 (100%)')
        self.assertEqual(self.run_script('--check-badge', str(self.badge)).returncode, 0)

    def test_missing_badge_fails_without_creating_it(self):
        self.badge.unlink()
        self.assertNotEqual(self.run_script('--check-badge', str(self.badge)).returncode, 0)
        self.assertFalse(self.badge.exists())

    def test_relative_invocation_from_outside_repository(self):
        result = subprocess.run(
            ['bash', f'{self.root.name}/scripts/crossplane-coverage.sh',
             '--check-badge', '.github/badges/crossplane-coverage.json'],
            cwd=self.root.parent, env=self.env, text=True, capture_output=True,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_export_failure_is_not_reported_as_current(self):
        self.env['FAIL_GO'] = '1'
        self.assertNotEqual(self.run_script('--check-badge', str(self.badge)).returncode, 0)

    def test_workflow_reports_and_checks_without_repository_writes(self):
        workflow = (REPO / '.github/workflows/crossplane-coverage.yml').read_text()
        self.assertNotRegex(workflow, r'contents:\s*write|git\s+(push|commit)')
        self.assertIn('persist-credentials: false', workflow)
        # Both steps must run for PRs, main pushes and manual runs.
        self.assertNotRegex(workflow, r'(?m)^\s+if:', msg='coverage must not be event-gated')
        commands = re.findall(r'^\s+run: (scripts/crossplane-coverage\.sh .+)$',
                              workflow, re.MULTILINE)
        self.assertEqual(len(commands), 2)
        self.assertTrue(any('--check-badge' in command for command in commands))
        summary = self.root / 'summary.md'
        env = {**self.env, 'GITHUB_STEP_SUMMARY': str(summary)}
        before = self.badge.read_bytes()
        for command in commands:
            result = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', command],
                                    cwd=self.root, env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
        report = summary.read_text()
        self.assertIn('1 / 2 Terraform resources', report)
        self.assertIn('`archestra_beta`', report)
        self.assertIn('`archestra_orphan`', report)
        self.assertEqual(self.badge.read_bytes(), before)
        self.mapping.write_text('"archestra_alpha"\n"archestra_beta"\n')
        result = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', commands[-1]],
                                cwd=self.root, env=env, text=True, capture_output=True)
        self.assertNotEqual(result.returncode, 0, 'workflow must reject a stale badge')


if __name__ == '__main__':
    unittest.main()
