import importlib.util
from contextlib import redirect_stderr, redirect_stdout
import io
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("acceptance", Path(__file__).with_name("ci_acceptance_tests.py"))
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)


class AcceptanceShardTests(unittest.TestCase):
    listing = "\n".join(["TestAccAgent", "TestAccAgent_Update", "TestAccRole", "TestAccTeam",
                         "TestSchema", "ExampleProvider", "FuzzParser", "BenchmarkSchema",
                         "TestSchema", "ok example.invalid/provider 0.1s", "? other [no test files]"])

    def test_new_test_packages_disable_the_single_package_cache(self):
        provider = str(acceptance.PACKAGE_DIR)
        for output, status, expected in ((provider, 0, True), (provider + "\n/another/test/package", 0, False),
                                         ("", 0, False), (provider, 1, False)):
            with patch.object(acceptance.subprocess, "run", return_value=subprocess.CompletedProcess([], status, output, "")):
                self.assertEqual(acceptance.cache_supported(), expected)

    def test_shards_cover_every_test_once_and_preserve_whole_subtests(self):
        first = acceptance.select_tests(self.listing, 1, 2)
        second = acceptance.select_tests(self.listing, 2, 2)
        self.assertFalse(set(first) & set(second))
        self.assertEqual(set(first + second), {"TestAccAgent", "TestAccAgent_Update", "TestAccRole",
                         "TestAccTeam", "TestSchema", "ExampleProvider", "FuzzParser"})
        self.assertEqual(first, acceptance.select_tests("\n".join(reversed(self.listing.splitlines())), 1, 2))
        self.assertEqual(set(first + second), set(acceptance.select_tests(self.listing, 1, 1)))

    def test_invalid_or_empty_acceptance_shards_fail(self):
        for shard, total in [(0, 2), (3, 2), (1, 0), (8, 8)]:
            with self.subTest(shard=shard, total=total), self.assertRaises(ValueError):
                acceptance.select_tests(self.listing, shard, total)
        with self.assertRaises(ValueError):
            acceptance.select_tests("TestUnit\nok example.invalid", 1, 1)

    def test_four_and_five_shards_have_complete_disjoint_coverage(self):
        listing = "\n".join(f"TestAccResource{i}" for i in range(32)) + "\nTestUnit\nExampleProvider\nFuzzParser"
        for count in (4, 5):
            shards = [acceptance.select_tests(listing, shard, count) for shard in range(1, count + 1)]
            combined = [name for shard in shards for name in shard]
            self.assertEqual(len(combined), len(set(combined)))
            self.assertEqual(set(combined), set(acceptance.select_tests(listing, 1, 1)))

    def test_runner_preserves_acceptance_environment_timeout_and_failures(self):
        with patch.object(acceptance.subprocess, "run", side_effect=[
                subprocess.CompletedProcess([], 0, self.listing, ""),
                subprocess.CompletedProcess([], 1)]) as command, redirect_stdout(io.StringIO()):
            self.assertEqual(acceptance.run(1, 2), 1)
        args, kwargs = command.call_args
        self.assertEqual(kwargs["env"]["TF_ACC"], "1")
        self.assertIn("-timeout=120m", args[0])
        self.assertIn("-cover", args[0])
        self.assertIn("-count=1", args[0])
        pattern = args[0][args[0].index("-run") + 1]
        for name in acceptance.select_tests(self.listing, 1, 2):
            self.assertRegex(name, pattern)
        self.assertIsNone(re.search(pattern, "TestAccAgent_Other"))

    def test_listing_failure_never_becomes_an_empty_green_shard(self):
        with patch.object(acceptance.subprocess, "run", return_value=
                          subprocess.CompletedProcess([], 2, "", "compile failed")) as command, \
                redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
            self.assertEqual(acceptance.run(1, 2), 2)
            self.assertEqual(command.call_count, 1)

    def test_compiled_go_suite_preserves_package_working_directory_and_live_coverage(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "go.mod").write_text("module example.invalid/native-smoke\n\ngo 1.20\n")
            (root / "fixture.txt").write_text("relative package fixture")
            (root / "main.go").write_text("package smoke\nfunc value() int { return 1 }\n")
            (root / "main_test.go").write_text('''package smoke
import ("os"; "testing")
func TestAccLive(t *testing.T) { if os.Getenv("TF_ACC") != "1" { t.Fatal("not live") }; if value() != 1 { t.Fatal("value") } }
func TestRelativeFixture(t *testing.T) { if _, err := os.ReadFile("fixture.txt"); err != nil { t.Fatal(err) } }
''')
            binary = root / "native-acceptance.test"
            subprocess.run(["go", "test", "-c", "-cover", "-o", str(binary)], cwd=root, check=True, capture_output=True)
            original_run = subprocess.run
            def quiet(*args, **kwargs):
                kwargs.setdefault("capture_output", True)
                return original_run(*args, **kwargs)
            with patch.object(acceptance, "PACKAGE_DIR", root), patch.object(acceptance.subprocess, "run", side_effect=quiet), redirect_stdout(io.StringIO()):
                self.assertEqual(acceptance.run(1, 1, binary), 0)
            self.assertRegex((root / "coverage-1.out").read_text(), r"main\.go:.* 1 1\n")


if __name__ == "__main__":
    unittest.main()
