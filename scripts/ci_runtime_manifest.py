#!/usr/bin/env python3
"""Bind cached build outputs to the checkout, host platform and exact file bytes."""

import argparse
import hashlib
import json
from pathlib import Path
import platform
import subprocess


def source_id():
    return subprocess.check_output(["git", "rev-parse", "HEAD"],
                                   cwd=Path(__file__).resolve().parents[1], text=True).strip()


def snapshot(paths):
    files = {}
    for index, argument in enumerate(paths):
        path = Path(argument)
        if path.is_symlink():
            raise ValueError("Cached runtime must not contain symbolic links")
        selected = sorted(path.rglob("*")) if path.is_dir() else [path]
        count = 0
        for file in selected:
            if file.is_symlink():
                raise ValueError("Cached runtime must not contain symbolic links")
            if file.is_dir():
                continue
            if not file.is_file() or not file.stat().st_size:
                raise ValueError("Missing or empty runtime output: " + str(file))
            name = str(file.relative_to(path)) if path.is_dir() else file.name
            files[str(index) + "/" + name] = hashlib.sha256(file.read_bytes()).hexdigest()
            count += 1
        if not count:
            raise ValueError("Runtime output directory is empty: " + str(path))
    return {"source": source_id(), "platform": [platform.system(), platform.machine()], "files": files}


def check(manifest, paths):
    if json.loads(manifest.read_text()) != snapshot(paths):
        raise ValueError("Cached runtime does not match the checkout, platform or output checksums")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("write", "check"))
    parser.add_argument("manifest", type=Path)
    parser.add_argument("paths", nargs="+", type=Path)
    args = parser.parse_args()
    if args.mode == "write":
        args.manifest.parent.mkdir(parents=True, exist_ok=True)
        args.manifest.write_text(json.dumps(snapshot(args.paths), sort_keys=True) + "\n")
    else:
        check(args.manifest, args.paths)


if __name__ == "__main__":
    main()
