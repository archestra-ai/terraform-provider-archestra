#!/usr/bin/env bash
# Dump the Terraform provider schema as JSON for upjet codegen.
#
# Usage: hack/generate-schema.sh [path-to-provider-binary]
#
# Requires `terraform` on PATH. Writes to stdout; redirect to config/schema.json.
set -euo pipefail

PROVIDER_BIN="${1:-../terraform-provider-archestra}"
if [[ ! -x "$PROVIDER_BIN" ]]; then
    echo "provider binary not found: $PROVIDER_BIN" >&2
    echo "build it first: (cd .. && make build)" >&2
    exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Plugin cache layout: <cache>/<host>/<namespace>/<name>/<version>/<os_arch>/binary
HOST="registry.terraform.io"
NAMESPACE="archestra-ai"
NAME="archestra"
VERSION="${TERRAFORM_PROVIDER_VERSION:-1.0.0}"
OS_ARCH="$(go env GOOS)_$(go env GOARCH)"

PLUGIN_DIR="$WORK/plugins/$HOST/$NAMESPACE/$NAME/$VERSION/$OS_ARCH"
mkdir -p "$PLUGIN_DIR"
cp "$PROVIDER_BIN" "$PLUGIN_DIR/terraform-provider-${NAME}_v${VERSION}"

cat > "$WORK/main.tf" <<EOF
terraform {
  required_providers {
    ${NAME} = {
      source  = "${NAMESPACE}/${NAME}"
      version = "${VERSION}"
    }
  }
}
EOF

(
    cd "$WORK"
    terraform init -plugin-dir="$WORK/plugins" -input=false >&2
    terraform providers schema -json
)
