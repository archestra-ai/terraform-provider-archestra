#!/usr/bin/env bash
# Writes the provider image's build context for the organization's Image
# Benchmark check (.github/images.json) to crossplane/_output/benchmark-context,
# with its build arguments in build-args.env there.
#
# The check reads how the image is built (its user, its layers, file names and
# modes), not what the binaries do, so the context holds placeholder binaries
# at the paths the release puts the real ones: building the provider needs
# the generated upjet code. The release checks the real image again before it
# is published. The Terraform version and OpenTofu checksums come from
# crossplane/Makefile, as for a release.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
crossplane=$(cd "$here/../../.." && pwd)
out="$crossplane/_output/benchmark-context"
version=0.0.0
name=terraform-provider-archestra
make_var() { sed -n "s/^export $1 ?= //p" "$crossplane/Makefile"; }

rm -rf "$out"
mkdir -p "$out/bin/linux_amd64" "$out/provider-zips" "$out/native"
cp "$here/Dockerfile" "$here/terraformrc.hcl" "$out/"
printf '#!/bin/sh\necho "placeholder for the image benchmark" >&2\nexit 1\n' > "$out/bin/linux_amd64/provider"
cp "$out/bin/linux_amd64/provider" "$out/native/${name}_v${version}"
chmod 0755 "$out/bin/linux_amd64/provider" "$out/native/${name}_v${version}"
(cd "$out/native" && python3 -m zipfile -c "$out/provider-zips/${name}_${version}_linux_amd64.zip" "${name}_v${version}")
rm -r "$out/native"

cat > "$out/build-args.env" <<ARGS
RUNTIME_BASE=public
TERRAFORM_VERSION=$(make_var TERRAFORM_VERSION)
OPENTOFU_SHA256_AMD64=$(make_var OPENTOFU_SHA256_AMD64)
OPENTOFU_SHA256_ARM64=$(make_var OPENTOFU_SHA256_ARM64)
TERRAFORM_PROVIDER_SOURCE=$(make_var TERRAFORM_PROVIDER_SOURCE)
TERRAFORM_PROVIDER_VERSION=$version
TERRAFORM_PROVIDER_DOWNLOAD_NAME=$name
TERRAFORM_NATIVE_PROVIDER_BINARY=${name}_v$version
ARGS
grep -q '^TERRAFORM_VERSION=.' "$out/build-args.env" || { echo "TERRAFORM_VERSION not found in crossplane/Makefile" >&2; exit 1; }
