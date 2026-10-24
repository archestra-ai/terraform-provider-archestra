#!/usr/bin/env python3
"""Run audited independent lifecycle probes concurrently on one isolated stack."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import time


HARNESS = Path(__file__).parent / "integration/crossplane-lifecycle.py"
spec = importlib.util.spec_from_file_location("lifecycle", HARNESS)
lifecycle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lifecycle)
SCOPES = ("cluster", "namespaced")
# Each probe owns UUID-labelled fixtures. Common model adoption, organization
# settings and GitHub installation identities stay serial; new cases do too.
PARALLEL_CASES = {
    "apps", "project-file-init", "knowledge-files", "team-bindings",
    "service-account-team", "inline-owners", "project-files", "marketplace-share-links",
    "repository-plugins", "manual-plugins", "api-compatibility", "routing-credentials",
    "remote-agents", "extensions", "core", "catalog", "limit", "identity", "tools",
    "credential", "schedule", "api-key", "skill", "git-skills", "project-instructions",
    "observe", "parent-sharing", "sharing-defaults",
}
SCOPED_COMPANIONS = {
    "llm": {"llm", "catalog", "tools", "limit", "credential", "sharing-defaults"},
    "github-registrations": {"github-registrations", "routing-credentials", "remote-agents"},
}
IDENTITY = ("terraform_sha256", "controller_sha256", "kubernetes_context", "api_url",
            "api_spec_sha256", "api_version")
# Long probes start first so short fixture checks do not hold up the last worker.
CASE_SECONDS = {"inline-owners": 335, "extensions": 275, "marketplace-share-links": 210,
                "git-skills": 400, "core": 250, "apps": 185, "project-files": 180,
                "repository-plugins": 125, "sharing-defaults": 120,
                "routing-credentials": 170, "service-account-team": 90,
                "llm": 200, "github-registrations": 180}


def plan(cases, jobs, scopes=SCOPES):
    if not 1 <= jobs <= 4:
        raise ValueError("Use between one and four concurrent probes")
    if not scopes or len(set(scopes)) != len(scopes) or set(scopes) - set(SCOPES):
        raise ValueError("Use distinct cluster and/or namespaced scopes")
    cases = list(dict.fromkeys(cases))
    if not cases:
        raise ValueError("At least one lifecycle case is required")
    independent = PARALLEL_CASES | {case for case, companions in SCOPED_COMPANIONS.items()
                                    if set(cases).issubset(companions)}
    serial, parallel = [], []
    global_cases = [case for case in cases if case not in independent]
    if global_cases:
        serial.append({"name": "serial", "cases": global_cases, "scopes": list(scopes)})
    for case in cases:
        if case not in independent:
            continue
        # Both imports use the same immutable Git repository fixture.
        if case in {"git-skills", *SCOPED_COMPANIONS}:
            parallel.append({"name": case + "-both", "cases": [case], "scopes": list(scopes)})
            continue
        kinds = sorted(kind.removesuffix("Sharing") for kind in lifecycle.CASE_RESOURCES["parent-sharing"])
        partitions = [kinds[i::jobs] for i in range(jobs)] if case == "parent-sharing" else [None]
        for scope in scopes:
            for part, selected in enumerate(partitions):
                task = {"name": f"{case}-{scope}-{part + 1}", "cases": [case], "scopes": [scope]}
                if selected is not None:
                    task["grant_kinds"] = selected
                parallel.append(task)
    parallel.sort(key=lambda task: (len(task["grant_kinds"]) * 45 if "grant_kinds" in task
                                   else CASE_SECONDS.get(task["cases"][0], 60)), reverse=True)
    return serial, parallel


def command(task, forwarded, evidence):
    owned = {"--cases", "--scopes", "--grant-kinds", "--evidence", "--resume-from"}
    if any(argument.split("=", 1)[0] in owned for argument in forwarded):
        raise ValueError("The CI planner owns case, scope, grant and evidence selection")
    args = [sys.executable, str(HARNESS), *forwarded, "--evidence", str(evidence),
            "--cases", *task["cases"], "--scopes", *task["scopes"]]
    if "grant_kinds" in task:
        args += ["--grant-kinds", *task["grant_kinds"]]
    return args


def merge_evidence(cases, tasks, outcomes, scopes=SCOPES):
    merged = {"results": [], "tasks": [], "outcome": "passed", "cleanup": "passed"}
    errors = []
    for task in tasks:
        code, data = outcomes.get(task["name"], (1, {}))
        merged["tasks"].append({"name": task["name"], "returncode": code, "run_id": data.get("run_id")})
        for field in IDENTITY:
            if field not in data or (field in merged and merged[field] != data[field]):
                errors.append(task["name"] + ": missing or mismatched " + field)
            elif field not in merged:
                merged[field] = data[field]
        merged["results"].extend(data.get("results", []))
        if code or data.get("outcome") != "passed" or data.get("cleanup") != "passed" or data.get("api_cleanup") not in (None, "passed"):
            errors.append(task["name"] + ": " + data.get("error", "probe or cleanup failed"))
    for scope in scopes:
        missing = set(cases) - lifecycle.completed_cases(merged["results"], scope)
        if missing:
            errors.append(scope + ": missing complete evidence for " + ", ".join(sorted(missing)))
    if errors:
        merged.update(outcome="failed", cleanup="unconfirmed", error="; ".join(errors))
    return merged


def run(cases, jobs, evidence, forwarded, scopes=SCOPES):
    serial, parallel = plan(cases, jobs, scopes)
    directory = evidence.with_suffix(".d")
    directory.mkdir(parents=True, exist_ok=False)

    def probe(task):
        path = directory / (task["name"] + ".json")
        started = time.monotonic()
        print("Starting " + task["name"], flush=True)
        result = subprocess.run(command(task, forwarded, path), capture_output=True, text=True)
        (directory / (task["name"] + ".log")).write_text(result.stdout + result.stderr)
        print(f"Finished {task['name']}: exit {result.returncode}, {time.monotonic() - started:.1f}s", flush=True)
        print(result.stdout + (result.stderr if result.returncode else ""), end="", flush=True)
        try:
            data = json.loads(path.read_text())
        except (OSError, ValueError):
            data = {}
        return result.returncode, data

    outcomes = {}
    for task in serial:
        outcomes[task["name"]] = probe(task)
        code, data = outcomes[task["name"]]
        if code or data.get("outcome") != "passed" or data.get("cleanup") != "passed" or data.get("api_cleanup") not in (None, "passed"):
            break
    else:
        # Wait for every started probe to finish its own normal cleanup, even after a failure.
        with ThreadPoolExecutor(max_workers=jobs) as pool:
            futures = {task["name"]: pool.submit(probe, task) for task in parallel}
            for name, future in futures.items():
                outcomes[name] = future.result()
    merged = merge_evidence(cases, serial + parallel, outcomes, scopes)
    evidence.write_text(json.dumps(merged, indent=2) + "\n")
    print(f"Lifecycle: {merged['outcome']}; {len(merged['results'])} evidence records", flush=True)
    return 0 if merged["outcome"] == "passed" else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", nargs="+", required=True, choices=tuple(lifecycle.CASE_RESOURCES) + ("sharing-defaults",))
    parser.add_argument("--scopes", nargs="+", choices=SCOPES, default=SCOPES)
    parser.add_argument("--jobs", type=int, default=4)
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("harness_args", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    forwarded = args.harness_args[1:] if args.harness_args[:1] == ["--"] else args.harness_args
    return run(args.cases, args.jobs, args.evidence, forwarded, args.scopes)


if __name__ == "__main__":
    raise SystemExit(main())
