#!/usr/bin/env bash
#
# Validates every complete example configuration (a directory with a
# required_providers block) against the provider built from this tree. The
# snippets under examples/resources and examples/data-sources are documentation
# fragments that reference objects declared elsewhere, so they are not
# configurations on their own. A development override points Terraform at the
# fresh binary, so no registry release is involved; the other providers the
# examples use are downloaded.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TERRAFORM="${TERRAFORM:-terraform}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

(cd "$REPO_ROOT" && go build -o "$work/terraform-provider-archestra" .)
cat > "$work/cli.tfrc" <<CFG
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "$work"
  }
  direct {}
}
CFG

status=0
while IFS= read -r dir; do
  rel="${dir#"$REPO_ROOT"/}"
  copy="$work/$rel"
  mkdir -p "$copy"
  cp -R "$dir"/. "$copy"/
  if ! (cd "$copy" && TF_CLI_CONFIG_FILE="$work/cli.tfrc" "$TERRAFORM" init -backend=false -input=false >"$copy/init.log" 2>&1 \
        && TF_CLI_CONFIG_FILE="$work/cli.tfrc" "$TERRAFORM" validate -no-color >"$copy/validate.log" 2>&1); then
    echo "FAIL $rel"
    # The override notice is noise here; an all-noise log leaves grep with exit 1.
    { cat "$copy/init.log" "$copy/validate.log" 2>/dev/null | grep -vE '^\s*$|development overrides|dev_overrides|may therefore not match|applying changes may cause|archestra-ai/archestra in' || true; } | tail -n 30
    status=1
  else
    echo "ok   $rel"
  fi
done < <(grep -rl 'required_providers' "$REPO_ROOT/examples" --include='*.tf' | xargs -n1 dirname | sort -u)
exit "$status"
