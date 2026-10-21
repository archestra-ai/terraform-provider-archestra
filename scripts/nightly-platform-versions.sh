#!/usr/bin/env bash
# Prints the Archestra releases that the nightly acceptance run tests, as a
# GitHub Actions matrix: the oldest supported release (minimumAPIVersion), the
# newest stable release, and the tested release in platform-version.txt.
# Needs gh and jq. Releases that appear in more than one role run once.
set -euo pipefail

cd "$(dirname "$0")/.."
pattern='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'

minimum="$(sed -n 's/^const minimumAPIVersion = "\(.*\)"$/\1/p' internal/provider/api_compatibility.go)"
stable="$(gh release list --repo archestra-ai/archestra --exclude-drafts --exclude-pre-releases \
  --limit 100 --json tagName --jq '.[].tagName | select(startswith("platform-v")) | ltrimstr("platform-v")' |
  sort -V | tail -n 1)"
tested="$(./scripts/platform-version.sh)"
tested="${tested#ARCHESTRA_VERSION=}"

for name in minimum stable tested; do
  if ! [[ "${!name}" =~ $pattern ]]; then
    echo "The ${name} release is not a release version: '${!name}'" >&2
    exit 1
  fi
done

matrix="$(jq -cn --arg minimum "$minimum" --arg stable "$stable" --arg tested "$tested" '
  [{version: $minimum, role: "oldest supported"},
   {version: $stable, role: "newest stable"},
   {version: $tested, role: "tested"}]
  | reduce .[] as $entry ([];
      if any(.[]; .version == $entry.version)
      then map(if .version == $entry.version then .role += ", " + $entry.role else . end)
      else . + [$entry] end)
  | {include: .}')"
echo "matrix=${matrix}"
