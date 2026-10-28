#!/usr/bin/env python3
"""Print the companion image repository used by a published platform version."""
import re
import sys


def image(version):
    match = re.fullmatch(r'(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?', version)
    if match is None:
        raise ValueError('Expected a platform release version')
    release = tuple(map(int, match.group(1, 2, 3)))
    candidate = re.fullmatch(r'rc\.(\d+)', match[4] or '')
    legacy = release < (1, 4, 8) or (release == (1, 5, 0) and candidate is not None and int(candidate[1]) < 12)
    repository = ('europe-west1-docker.pkg.dev/friendly-path-465518-r6/archestra-public'
                  if legacy else 'ghcr.io/archestra-ai')
    return f'{repository}/mcp-server-base:{version}'


if __name__ == '__main__':
    print(image(sys.argv[1]))
