#!/usr/bin/env bash
# Reports which Terraform resources do not yet have a Crossplane managed-resource
# equivalent.
#
# Sources of truth:
#   - Terraform: `resp.TypeName = req.ProviderTypeName + "_<n>"` lines in
#     internal/provider/resource_*.go
#   - Crossplane: the `ExternalNameConfigs` map in
#     crossplane/config/external_name.go (also functions as the upjet IncludeList)
#
# Usage:
#   scripts/crossplane-coverage.sh                 # markdown report to stdout
#   scripts/crossplane-coverage.sh path/to/out.md  # markdown report to file
#   scripts/crossplane-coverage.sh --badge out.json  # shields.io endpoint JSON

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tf="$(grep -h 'TypeName = req.ProviderTypeName + "_' internal/provider/resource_*.go \
  | sed -E 's/.*"_([a-z_]+)".*/archestra_\1/' \
  | sort -u)"
cp="$(grep -oE '"archestra_[a-z_]+"' crossplane/config/external_name.go \
  | tr -d '"' \
  | sort -u)"

tf_total=$(printf '%s\n' "$tf" | grep -c .)
covered="$(comm -12 <(echo "$tf") <(echo "$cp"))"
cp_total=$(printf '%s\n' "$covered" | grep -c . || true)
pct=$(( cp_total * 100 / tf_total ))

if [ "${1:-}" = "--badge" ]; then
  out="${2:?--badge requires an output path}"
  color="red"
  [ "$pct" -ge 25 ] && color="orange"
  [ "$pct" -ge 50 ] && color="yellow"
  [ "$pct" -ge 75 ] && color="green"
  [ "$pct" -ge 95 ] && color="brightgreen"
  cat > "$out" <<EOF
{"schemaVersion":1,"label":"crossplane coverage","message":"${cp_total}/${tf_total} (${pct}%)","color":"${color}"}
EOF
  exit 0
fi

out="${1:-/dev/stdout}"
missing="$(comm -23 <(echo "$tf") <(echo "$cp"))"
orphans="$(comm -13 <(echo "$tf") <(echo "$cp"))"

{
  echo "# Crossplane coverage report"
  echo
  echo "**${cp_total} / ${tf_total} Terraform resources have Crossplane managed-resource equivalents (${pct}%).**"
  echo
  echo "## Missing Crossplane MR (drift)"
  echo
  if [ -z "$missing" ]; then
    echo "_(none — full coverage)_"
  else
    while IFS= read -r r; do
      [ -n "$r" ] && echo "- \`$r\`"
    done <<< "$missing"
  fi
  echo
  echo "## Covered"
  echo
  while IFS= read -r r; do
    [ -n "$r" ] && echo "- \`$r\`"
  done <<< "$covered"
  if [ -n "$orphans" ]; then
    echo
    echo "## Crossplane mappings without a Terraform resource"
    echo
    echo "_(should be empty)_"
    echo
    while IFS= read -r r; do
      [ -n "$r" ] && echo "- \`$r\`"
    done <<< "$orphans"
  fi
} > "$out"
