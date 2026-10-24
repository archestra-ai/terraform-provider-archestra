import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("scope", Path(__file__).with_name("ci_integration_scope.py"))
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)


class IntegrationScopeTests(unittest.TestCase):
    def test_publication_and_documentation_do_not_change_api_behavior(self):
        paths = ["README.md", "crossplane/README.md", "crossplane/marketplace/icons/icon.png",
                 "crossplane/marketplace/README.md", ".github/workflows/release-please.yml"]
        for suite in ("native", "crossplane"):
            self.assertFalse(scope.should_run(suite, "pull_request", "publication", paths))

    def test_runtime_dependencies_and_harness_changes_run_both_suites(self):
        for path in ("internal/provider/resource_agent.go", "main.go", "go.mod", "go.sum",
                     "Makefile", "platform-version.txt", "scripts/bootstrap-local-stack.sh",
                     "scripts/integration/crossplane-lifecycle.py", "scripts/ci_integration_scope.py",
                     ".github/actions/setup-go/action.yml", ".github/workflows/on-pull-request.yml",
                     ".github/values-ci.yaml"):
            for suite in ("native", "crossplane"):
                with self.subTest(path=path, suite=suite):
                    self.assertTrue(scope.should_run(suite, "pull_request", "feature", [path]))

    def test_unknown_and_crossplane_inputs_do_not_skip_native_tests(self):
        for path in ("new-runtime/config.json", "crossplane/config/resources.go"):
            self.assertTrue(scope.should_run("native", "pull_request", "feature", [path]))
        self.assertTrue(scope.should_run("crossplane", "pull_request", "feature", ["crossplane/config/resources.go"]))

    def test_releases_and_manual_runs_always_exercise_every_suite(self):
        for suite in ("native", "crossplane"):
            self.assertTrue(scope.should_run(suite, "pull_request", "release-please--branches--main", ["CHANGELOG.md"]))
            self.assertTrue(scope.should_run(suite, "workflow_dispatch", "", []))

    def test_runtime_rename_is_not_hidden_by_documentation_destination(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(
                    ["git", "-c", "user.name=Scope test", "-c", "user.email=scope@example.invalid",
                     "-c", "commit.gpgsign=false", *args], cwd=root, stderr=subprocess.DEVNULL,
                ).decode().strip()
            git("init", "--quiet")
            (root / "main.go").write_text("package main\n")
            git("add", ".")
            git("commit", "--quiet", "-m", "runtime")
            base = git("rev-parse", "HEAD")
            git("mv", "main.go", "README.md")
            git("commit", "--quiet", "-m", "rename")
            paths = scope.changed_paths(base, git("rev-parse", "HEAD"), cwd=root)
            for suite in ("native", "crossplane"):
                self.assertTrue(scope.should_run(suite, "pull_request", "feature", paths))

    def test_missing_commit_range_fails_instead_of_skipping_tests(self):
        with self.assertRaises(ValueError):
            scope.changed_paths("", "")


if __name__ == "__main__":
    unittest.main()
