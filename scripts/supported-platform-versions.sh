#!/usr/bin/env bash
# Prints the older Archestra releases that pull request CI tests besides the
# tested release in platform-version.txt, as a GitHub Actions matrix: the
# oldest supported release (minimumAPIVersion) and the newest stable release.
# Jobs are named by role, not version, so their checks can be required.
# Needs gh and jq.
set -euo pipefail

cd "$(dirname "$0")/.."
pattern='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'

minimum="$(sed -n 's/^const minimumAPIVersion = "\(.*\)"$/\1/p' internal/provider/api_compatibility.go)"
stable="$(gh release list --repo archestra-ai/archestra --exclude-drafts --exclude-pre-releases \
  --limit 100 --json tagName --jq '.[].tagName | select(startswith("platform-v")) | ltrimstr("platform-v")' |
  sort -V | tail -n 1)"

for name in minimum stable; do
  if ! [[ "${!name}" =~ $pattern ]]; then
    echo "The ${name} release is not a release version: '${!name}'" >&2
    exit 1
  fi
done

matrix="$(jq -cn --arg minimum "$minimum" --arg stable "$stable" '
  {include: [{role: "oldest supported", version: $minimum},
             {role: "newest stable", version: $stable}]}')"
echo "matrix=${matrix}"
