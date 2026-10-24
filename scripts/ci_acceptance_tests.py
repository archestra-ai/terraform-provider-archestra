#!/usr/bin/env python3
"""Run a disjoint share of the native Go suite against this job's test stack."""

import argparse
import os
import re
import subprocess
import sys


def select_tests(output, shard, total):
    if not 1 <= shard <= total:
        raise ValueError("Shard must be between 1 and the shard count")
    names = sorted({line for line in output.splitlines()
                    if re.fullmatch(r"(?:Test|Example|Fuzz)\w*", line)})
    selected = names[shard - 1::total]
    if not any(name.startswith("TestAcc") for name in selected):
        raise ValueError("Every shard must contain acceptance tests")
    return selected


def run(shard, total):
    env = {**os.environ, "TF_ACC": "1"}
    listed = subprocess.run(["go", "test", "-list", ".", "./..."],
                            env=env, capture_output=True, text=True)
    if listed.returncode:
        print(listed.stdout, end="")
        print(listed.stderr, end="", file=sys.stderr)
        return listed.returncode
    names = select_tests(listed.stdout, shard, total)
    acceptance = sum(name.startswith("TestAcc") for name in names)
    print(f"Native shard {shard}/{total}: {acceptance} acceptance tests; "
          f"{len(names)} total top-level tests", flush=True)
    # Keep whole top-level tests together, including all of their subtests.
    pattern = "^(" + "|".join(re.escape(name) for name in names) + ")$"
    return subprocess.run(["go", "test", "-v", "-cover", "-count=1", "-timeout=120m",
                           "-run", pattern, "./..."], env=env).returncode


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--shard", type=int, default=1)
    parser.add_argument("--total", type=int, default=1)
    args = parser.parse_args()
    try:
        return run(args.shard, args.total)
    except ValueError as error:
        parser.error(str(error))


if __name__ == "__main__":
    raise SystemExit(main())
