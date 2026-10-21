#!/usr/bin/env python3
"""Generate role validation from upstream data without executing upstream code."""
import argparse
import json
from pathlib import Path
import re


def extract_actions(source):
    marker = "export const allAvailableActions: Record<Resource, Action[]> = {"
    if source.count(marker) != 1:
        raise ValueError("expected exactly one allAvailableActions registry")
    block = source.split(marker, 1)[1].split("\n};", 1)[0]
    rows = re.findall(r'^\s*(\w+): \[([^\]]+)\]', block, re.MULTILINE)
    result = {}
    for name, raw in rows:
        values = json.loads("[" + raw + "]")
        if name in result or not values or not all(isinstance(v, str) for v in values):
            raise ValueError("invalid or duplicate action registry entry")
        result[name] = values
    if not result:
        raise ValueError("empty action registry")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--upstream-sha", required=True)
    parser.add_argument("--output", type=Path, default=Path("internal/provider/role_capabilities_gen.go"))
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9a-f]{40}", args.upstream_sha):
        parser.error("upstream SHA must be a full lowercase commit hash")
    actions = extract_actions(args.source.read_text())
    text = f"// Code generated from Archestra allAvailableActions ({args.upstream_sha}). DO NOT EDIT.\npackage provider\n\nvar currentRoleActions = map[string][]string{{\n"
    for name, values in sorted(actions.items()):
        text += json.dumps(name) + ": {" + ", ".join(json.dumps(v) for v in values) + "},\n"
    args.output.write_text(text + "}\n")


if __name__ == "__main__":
    main()
