#!/usr/bin/env python3
"""Print a Markdown summary of one crossplane-lifecycle.py evidence file.

The Crossplane Lifecycle workflow appends it to the job summary. It never
fails the job itself: the lifecycle step's exit status does that.
"""
import json
from pathlib import Path
import sys


def summary(path):
    if not path.exists():
        return f'### Crossplane lifecycle\n\nNo evidence was written to `{path.name}`; the run stopped before the harness saved it.\n'
    evidence = json.loads(path.read_text())
    results = evidence.get('results', [])
    lines = ['### Crossplane lifecycle', '',
             f"- Outcome: {evidence.get('outcome', 'unknown')}",
             f"- Cleanup: {evidence.get('cleanup', 'unknown')}",
             f"- API version: {evidence.get('api_version', 'unknown')}",
             f'- Records: {len(results)}']
    if evidence.get('error'):
        lines.append(f"- Error: {evidence['error']}")
    # The harness records a case only after all of its checks pass and stops at
    # the first failure, which the error line above names.
    lines += ['', '| Resource | Scope | Checks recorded |', '|---|---|---|']
    for record in results:
        lines.append(f"| {record.get('resource', '?')} | {record.get('scope', '?')} | {len(record) - 2} |")
    return '\n'.join(lines) + '\n'


if __name__ == '__main__':
    print(summary(Path(sys.argv[1])), end='')
