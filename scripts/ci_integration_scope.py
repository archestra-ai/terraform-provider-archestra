#!/usr/bin/env python3
"""Select integration suites from runtime inputs; releases always run every suite."""

import argparse
import os
import re
import subprocess


DOCUMENTATION = {
    "README.md", "CHANGELOG.md", "CONTRIBUTING.md", "ARCHITECTURE.md",
    "COMPATIBILITY.md", "MIGRATION-v3.md", "CLAUDE.md", "LICENSE",
    "crossplane/README.md", ".github/workflows/release-please.yml",
}
CROSSPLANE_INPUT = re.compile(
    r"^(internal/|crossplane/|cmd/|scripts/|[^/]+\.go$|Makefile$|go\.(mod|sum)$|"
    r"platform-version\.txt$|\.github/(actions/|kind\.yaml$|values-ci\.yaml$|"
    r"workflows/(crossplane|crossplane-lifecycle|on-pull-request)\.yml$))"
)


def documentation_only(path):
    return (path in DOCUMENTATION or path.startswith("crossplane/marketplace/")
            or (path.startswith(("docs/", "audit/"))
                and path.endswith((".md", ".json", ".png", ".svg"))))


def should_run(suite, event, head_ref, paths):
    if event != "pull_request" or head_ref.startswith("release-please--"):
        return True
    runtime = [path for path in paths if not documentation_only(path)]
    if suite == "native":
        # Unknown inputs run the native suite rather than silently losing coverage.
        return bool(runtime)
    if suite == "crossplane":
        return any(CROSSPLANE_INPUT.match(path) for path in runtime)
    raise ValueError(f"Unknown suite: {suite}")


def changed_paths(base, head, cwd=None):
    if not all(re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", sha) for sha in (base, head)):
        raise ValueError("Pull request base and head must be commit SHAs")
    # A runtime file renamed into documentation still represents a runtime deletion.
    result = subprocess.run(
        ["git", "diff", "--name-only", "--no-renames", "-z", base, head, "--"],
        cwd=cwd, check=True, capture_output=True,
    )
    return [os.fsdecode(path) for path in result.stdout.split(b"\0") if path]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--suite", choices=("native", "crossplane"), required=True)
    parser.add_argument("--event", required=True)
    parser.add_argument("--head-ref", default="")
    parser.add_argument("--base", default="")
    parser.add_argument("--head", default="")
    args = parser.parse_args()
    force = args.event != "pull_request" or args.head_ref.startswith("release-please--")
    paths = [] if force else changed_paths(args.base, args.head)
    run = should_run(args.suite, args.event, args.head_ref, paths)
    print(f"run={str(run).lower()}")


if __name__ == "__main__":
    main()
