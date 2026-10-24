import argparse
from contextlib import redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("ci_crossplane", Path(__file__).with_name("ci_crossplane_tests.py"))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


def proof(task):
    data = {key: key for key in ci.IDENTITY}
    data.update(run_id=task["name"], results=[], outcome="passed", cleanup="passed")
    for scope in task["scopes"]:
        for case in task["cases"]:
            if case == "sharing-defaults":
                data["results"] += [{"resource": "CredentialSharingDefaults", "scope": scope,
                                     "parameter_source": value} for value in ("default", "empty", "init-empty")]
            else:
                resources = ({kind + "Sharing" for kind in task["grant_kinds"]}
                             if "grant_kinds" in task else ci.lifecycle.CASE_RESOURCES[case])
                data["results"] += [{"resource": kind, "scope": scope} for kind in resources]
    return data


class LifecyclePlannerTests(unittest.TestCase):
    def test_every_case_scope_and_sharing_kind_is_assigned_once(self):
        cases = list(ci.lifecycle.CASE_RESOURCES) + ["sharing-defaults"]
        serial, parallel = ci.plan(cases, 4)
        for case in cases:
            for scope in ci.SCOPES:
                selected = [task for task in serial + parallel if case in task["cases"] and scope in task["scopes"]]
                if case == "parent-sharing":
                    kinds = [kind for task in selected for kind in task["grant_kinds"]]
                    self.assertEqual(len(kinds), len(set(kinds)))
                    self.assertEqual({kind + "Sharing" for kind in kinds}, ci.lifecycle.CASE_RESOURCES[case])
                else:
                    self.assertEqual(len(selected), 1, (case, scope))
        self.assertEqual(set(serial[0]["cases"]), {"organization", "organization-defaults", "llm", "github-registrations"})
        tasks = serial + parallel
        merged = ci.merge_evidence(cases, tasks, {task["name"]: (0, proof(task)) for task in tasks})
        self.assertEqual(merged["outcome"], "passed")

    def test_new_cases_stay_serial_and_empty_plans_fail(self):
        serial, parallel = ci.plan(["future-global-case"], 4)
        self.assertEqual(serial[0]["cases"], ["future-global-case"])
        self.assertEqual(parallel, [])
        for cases, jobs in (([], 4), (["core"], 0), (["core"], 5)):
            with self.assertRaises(ValueError):
                ci.plan(cases, jobs)

    def test_missing_evidence_cleanup_failures_or_mixed_binaries_fail(self):
        serial, tasks = ci.plan(["core"], 2)
        outcomes = {task["name"]: (0, proof(task)) for task in tasks}
        self.assertEqual(ci.merge_evidence(["core"], tasks, outcomes)["outcome"], "passed")
        name = tasks[0]["name"]
        for broken in ({"results": []}, {"cleanup": "blocked"}, {"api_cleanup": "pending"},
                       {"controller_sha256": "different"}, {"outcome": "failed"}):
            changed = {**outcomes, name: (0, {**proof(tasks[0]), **broken})}
            self.assertEqual(ci.merge_evidence(["core"], tasks, changed)["outcome"], "failed", broken)
        self.assertEqual(ci.merge_evidence(["core"], tasks, {})["outcome"], "failed")
        self.assertEqual(ci.merge_evidence(["core"], tasks, {**outcomes, name: (1, proof(tasks[0]))})["outcome"], "failed")

    def test_forwarded_arguments_cannot_override_coverage_or_reuse_old_results(self):
        task = ci.plan(["core"], 2)[1][0]
        for flag in ("--cases", "--scopes=cluster", "--grant-kinds", "--resume-from", "--evidence"):
            with self.assertRaises(ValueError):
                ci.command(task, [flag], Path("evidence.json"))

    def test_global_phase_is_serial_parallelism_is_bounded_and_failure_waits_for_cleanup(self):
        cases = ["organization", "core", "observe", "parent-sharing"]
        planned = sum(ci.plan(cases, 4), [])
        active = maximum = 0
        completed = []
        lock = threading.Lock()
        def fake(command, **kwargs):
            nonlocal active, maximum
            parser = argparse.ArgumentParser()
            parser.add_argument("--evidence", type=Path)
            parser.add_argument("--cases", nargs="+")
            parser.add_argument("--scopes", nargs="+")
            parser.add_argument("--grant-kinds", nargs="+")
            args = parser.parse_args(command[2:])
            task = {"name": args.evidence.stem, "cases": args.cases, "scopes": args.scopes}
            if args.grant_kinds:
                task["grant_kinds"] = args.grant_kinds
            with lock:
                active += 1
                maximum = max(maximum, active)
                if task["name"] == "serial":
                    self.assertEqual(active, 1)
                else:
                    self.assertIn("serial", completed)
            time.sleep(0.01)
            failed = task["name"] == "observe-cluster-1"
            data = proof(task)
            if failed:
                data["outcome"] = "failed"
            args.evidence.write_text(json.dumps(data))
            with lock:
                active -= 1
                completed.append(task["name"])
            return subprocess.CompletedProcess(command, int(failed), "", "")
        with tempfile.TemporaryDirectory() as directory, patch.object(ci.subprocess, "run", side_effect=fake), redirect_stdout(io.StringIO()):
            evidence = Path(directory) / "evidence.json"
            self.assertEqual(ci.run(cases, 4, evidence, []), 1)
            self.assertEqual(json.loads(evidence.read_text())["outcome"], "failed")
        self.assertEqual(len(completed), len(planned))
        self.assertGreater(maximum, 1)
        self.assertLessEqual(maximum, 4)


if __name__ == "__main__":
    unittest.main()
