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
  name: provider-archestra
spec:
  package: xpkg.upbound.io/archestra/provider-archestra:v1.1.0
EOF
```

Replace the tag with the latest release from
[GitHub Releases](https://github.com/archestra-ai/terraform-provider-archestra/releases).

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

`apis/**/zz_*.go`, `internal/controller/**/zz_*.go`, and
`package/crds/*.yaml` are gitignored — upjet regenerates them
from the TF provider's schema. First time on a clean clone:

```bash
# in repo root: produce the TF binary that upjet shells out to
go build -o terraform-provider-archestra .

# in crossplane/: dump schema, regen, build the controller
cd crossplane
make schema     # writes config/schema.json from the local TF binary
make generate   # produces apis/, internal/controller/, package/crds/
make build      # binary at _output/bin/<host-platform>/provider
```

Iteration loop after a TF schema change: rebuild the TF binary at
the repo root, then re-run `make generate` here. `make schema`
defaults to fetching the published TF provider from the registry;
the local-binary path is the relevant one for unreleased changes
(see `hack/generate-schema.sh` and the `TF_PROVIDER_BIN` env var).

### Run the controller out-of-cluster

Useful for fast iteration without rebuilding the xpkg:

```bash
make build
./_output/bin/$(go env GOOS)_$(go env GOARCH)/provider --debug \
  --terraform-version=1.5.7 \
  --terraform-provider-source=archestra-ai/archestra
```

Point `kubectl` at a cluster with Crossplane installed and the CRDs
applied; the controller will reconcile MRs against your local
Archestra instance using the `ProviderConfig` named `default`.

## Adding a resource

1. **Whitelist the TF resource.** Add it to `ExternalNameConfigs`
   in [`config/external_name.go`](config/external_name.go). The
   map doubles as upjet's IncludeList — anything not listed is
   skipped during codegen.

   ```go
   "archestra_<thing>": ujconfig.IdentifierFromProvider,
   ```

2. **Configure both scopes.** Crossplane v1 (cluster) and v2
   (namespaced) get separate configurator files. Pick the group
   the resource belongs to (`agent`, `mcp`, `policy`, …) and add
   a block to **both**
   [`config/cluster/<group>/config.go`](config/cluster/) and
   [`config/namespaced/<group>/config.go`](config/namespaced/):

   ```go
   p.AddResourceConfigurator("archestra_<thing>", func(r *ujconfig.Resource) {
       r.ShortGroup = "<group>"           // -> <group>.archestra.crossplane.io
       r.Kind       = "<Thing>"           // CamelCase singular
       r.References["<fk_field>"] = ujconfig.Reference{
           TerraformName: "archestra_<other>",  // for cross-MR refs
       }
   })
   ```

3. **Regenerate.** `make generate` produces the `zz_*_types.go`,
   `zz_controller.go`, and CRD YAML. Commit nothing from those
   trees — they're gitignored.

4. **Add an example.** YAML manifests under
   `examples/cluster/<group>/<kind>.yaml` and the namespaced
   sibling. Uptest reads from this directory.

5. **Sanity-check** with `go build ./... && go test ./...` from
   the `crossplane/` directory.

## Testing

```bash
make generate          # required after pulling — generated code is gitignored
go test ./...          # unit tests
make e2e               # full end-to-end against a local kind cluster
```

`make e2e` chains `local-deploy` (spins up a kind cluster, builds
the xpkg locally, installs it as a `Provider`) and `uptest` (runs
the examples and waits for `Ready=True`). It needs:

- `UPTEST_EXAMPLE_LIST` — comma-separated paths under `examples/`
  to exercise (e.g. `examples/cluster/agent/agent.yaml`).
- `UPTEST_CLOUD_CREDENTIALS` — Archestra API credentials,
  surfaced as a Kubernetes secret to the provider:

  ```bash
  export UPTEST_CLOUD_CREDENTIALS='{"api_key":"arch_...","base_url":"https://app.archestra.ai"}'
  ```

Per-PR CI runs `make -C crossplane generate && go test ./...`
(see [`.github/workflows/crossplane.yml`](../.github/workflows/crossplane.yml));
`make e2e` is not in CI yet — run it locally before promoting a
non-trivial schema change.

## Releasing

Released on the same `vX.Y.Z` track as the Terraform provider — see
the [root README's Releases section](../README.md#releases). The
xpkg is published to `xpkg.upbound.io/archestra/provider-archestra`
alongside each Terraform provider release.

## License

Apache 2.0 — see [`LICENSE`](./LICENSE).
