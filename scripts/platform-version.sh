#!/usr/bin/env bash
# Print ARCHESTRA_VERSION=<release> from platform-version.txt, the tested
# Archestra release. CI appends the output to $GITHUB_ENV.
set -euo pipefail

file="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/platform-version.txt"
version="$(cat "$file")"
if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  printf 'platform-version.txt must hold one release version, got: %s\n' "$version" >&2
  exit 1
fi
printf 'ARCHESTRA_VERSION=%s\n' "$version"
