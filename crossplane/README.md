# crossplane-provider-archestra

Crossplane provider for [Archestra](https://archestra.ai), generated from
[`terraform-provider-archestra`](https://registry.terraform.io/providers/archestra-ai/archestra/latest/docs)
using the [`upjet`](https://github.com/crossplane/upjet) code-generation
toolchain.

Resources currently exposed (per group, both Cluster and Namespaced
scopes — Crossplane v1 / v2):

| Group | Kinds |
|---|---|
| `mcp.archestra.crossplane.io` / `mcp.archestra.m.crossplane.io` | `RegistryCatalogItem`, `ServerInstallation` |
| `agent.archestra.crossplane.io` / `agent.archestra.m.crossplane.io` | `Agent`, `ToolBatch` |
| `policy.archestra.crossplane.io` / `policy.archestra.m.crossplane.io` | `ToolInvocationPolicyDefault` |

## Installation

Crossplane v1 or v2 must already be installed in the cluster.

```bash
kubectl apply -f - <<'EOF'
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-archestra
spec:
  package: xpkg.upbound.io/archestra/crossplane-provider-archestra:v1.0.0
EOF
```

Replace the tag with the latest release from
[GitHub Releases](https://github.com/archestra-ai/terraform-provider-archestra/releases?q=crossplane-).

## Configure credentials

The provider authenticates to the Archestra API with an API key. Mint
one in the Archestra UI under Settings → API Keys.

```bash
kubectl create secret generic archestra-creds \
  -n crossplane-system \
  --from-literal=credentials='{"api_key":"arch_...","base_url":"https://app.archestra.ai"}'

kubectl apply -f - <<'EOF'
apiVersion: archestra.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: archestra-creds
      key: credentials
EOF
```

`base_url` is optional; falls back to the Archestra TF provider's default
when absent. For namespaced (Crossplane v2) usage see
`examples/namespaced/providerconfig/`.

## Create a managed resource

```yaml
apiVersion: mcp.archestra.crossplane.io/v1alpha1
kind: RegistryCatalogItem
metadata:
  name: everything
spec:
  forProvider:
    name: everything
    localConfig:
      command: npx
      arguments:
        - "-y"
        - "@modelcontextprotocol/server-everything"
  providerConfigRef:
    name: default
---
apiVersion: mcp.archestra.crossplane.io/v1alpha1
kind: ServerInstallation
metadata:
  name: everything
spec:
  forProvider:
    name: everything
    catalogIdRef:
      name: everything       # resolves to the RegistryCatalogItem above
  providerConfigRef:
    name: default
```

`tool_id_by_name` (the per-tool UUID map) lands on
`status.atProvider.toolIdByName` once the installation reconciles —
extract from there to populate `ToolBatch.spec.forProvider.toolIds` and
`ToolInvocationPolicyDefault.spec.forProvider.toolIds`.

## Development

The Crossplane provider is regenerated from the Terraform provider's
schema. Whenever the upstream `terraform-provider-archestra` schema
changes, refresh:

```bash
make schema     # dump the TF provider's schema as JSON
make generate   # regenerate apis/, internal/controller/, package/crds/
make build      # build the controller binary
```

`make schema` fetches the Terraform provider from the public registry
(`archestra-ai/archestra`) by default. Override
`TERRAFORM_PROVIDER_VERSION` to test against a specific release.

For development against an unreleased TF binary, see
`hack/generate-schema.sh` — point `TF_PROVIDER_BIN` at a locally-built
`terraform-provider-archestra` and the script will dump its schema.

### Running the controller out-of-cluster

```bash
make build
./bin/provider --debug \
  --terraform-version=1.5.7 \
  --terraform-provider-source=archestra-ai/archestra \
  --terraform-provider-version=$(TERRAFORM_PROVIDER_VERSION)
```

## Releasing

1. Cut a tag: `git tag -a v0.1.0 -m "v0.1.0"; git push origin v0.1.0`.
2. Run the **Tag** GitHub Actions workflow if you prefer to drive
   releases through the UI.
3. The **Publish Provider Package** workflow builds the controller image
   + xpkg and pushes both to the configured registry.

## License

Apache 2.0 — see [`LICENSE`](./LICENSE).
