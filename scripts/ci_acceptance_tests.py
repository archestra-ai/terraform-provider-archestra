#!/usr/bin/env python3
"""Run a disjoint share of the native Go suite against this job's test stack."""

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path


PACKAGE_DIR = Path(__file__).resolve().parents[1] / "internal/provider"


def cache_supported():
    inventory = subprocess.run(["go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.Dir}}{{end}}", "./..."],
                               capture_output=True, text=True)
    packages = {Path(line).resolve() for line in inventory.stdout.splitlines() if line.strip()}
    # A future test package must use the complete source runner, never disappear from native CI.
    return inventory.returncode == 0 and packages == {PACKAGE_DIR.resolve()}


def select_tests(output, shard, total):
    if not 1 <= shard <= total:
        raise ValueError("Shard must be between 1 and the shard count")
    names = sorted({line for line in output.splitlines()
                    if re.fullmatch(r"(?:Test|Example|Fuzz)\w*", line)})
    selected = names[shard - 1::total]
    if not any(name.startswith("TestAcc") for name in selected):
        raise ValueError("Every shard must contain acceptance tests")
    return selected


def run(shard, total, binary=None):
    env = {**os.environ, "TF_ACC": "1"}
    listing = [str(binary), "-test.list", "."] if binary else ["go", "test", "-cover", "-list", ".", "./..."]
    options = {"cwd": PACKAGE_DIR} if binary else {}
    listed = subprocess.run(listing, env=env, capture_output=True, text=True, **options)
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
    if binary:
        return subprocess.run([str(binary), "-test.v", "-test.count=1", "-test.timeout=120m",
                               "-test.coverprofile=" + str(binary.parent / f"coverage-{shard}.out"),
                               "-test.run", pattern], env=env, **options).returncode
    return subprocess.run(["go", "test", "-v", "-cover", "-count=1", "-timeout=120m",
                           "-run", pattern, "./..."], env=env).returncode


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--shard", type=int, default=1)
    parser.add_argument("--total", type=int, default=1)
    parser.add_argument("--binary", type=Path, help="Verified compiled provider test suite; results always execute live")
    parser.add_argument("--cache-supported", action="store_true", help="Check that the compiled package contains the complete test suite")
    args = parser.parse_args()
    if args.cache_supported:
        return 0 if cache_supported() else 1
    try:
        return run(args.shard, args.total, args.binary.resolve() if args.binary else None)
    except ValueError as error:
        parser.error(str(error))


if __name__ == "__main__":
    raise SystemExit(main())
