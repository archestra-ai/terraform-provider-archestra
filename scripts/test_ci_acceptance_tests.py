import importlib.util
from contextlib import redirect_stderr, redirect_stdout
import io
from pathlib import Path
import re
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("acceptance", Path(__file__).with_name("ci_acceptance_tests.py"))
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)


class AcceptanceShardTests(unittest.TestCase):
    listing = "\n".join(["TestAccAgent", "TestAccAgent_Update", "TestAccRole", "TestAccTeam",
                         "TestSchema", "ExampleProvider", "FuzzParser", "BenchmarkSchema",
                         "TestSchema", "ok example.invalid/provider 0.1s", "? other [no test files]"])

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


if __name__ == "__main__":
    unittest.main()
