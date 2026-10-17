# crossplane-provider-archestra


The v3 review candidate has **29 Terraform resources and four Terraform data sources**. All 29 resources have generated Crossplane types in both scopes. Agent/gateway relationships, team bindings, object sharing and organization defaults belong to their parents. Manual and Git-backed plugins share one resource. The [migration guide](../MIGRATION-v3.md) describes the clean-major names; there are no aliases for retired types. Additional durable API areas, final runtime/package validation, protected review and publication remain in progress. The [full native acceptance proof](../audit/consolidated-native-acceptance.json) records 147 passing cases with zero failures or skips against API 1.4.0-rc.49.

Crossplane provider for [Archestra](https://archestra.ai), generated from
[`terraform-provider-archestra`](https://registry.terraform.io/providers/archestra-ai/archestra/latest/docs)
using the [`upjet`](https://github.com/crossplane/upjet) code-generation
toolchain.

The [organization-defaults runtime proof](../audit/crossplane-organization-defaults.json) covers current settings behavior in both scopes.

The v3 working schema includes 29 Terraform resources in both scopes. The shared
configurator in [config/resources.go](config/resources.go) defines each group/kind
and reference. Gateways use the `gateway` API group so their agent references do
not form an import cycle with agent references to MCP servers. See the current
[resource matrix](../README.md#resource-coverage) and [runtime evidence](../audit/README.md).
Historical lifecycle and packaging evidence applies only to its recorded binaries;
final combined-candidate repetition remains required.

The [current consolidated runtime proof](../audit/consolidated-crossplane-runtime.json)
verifies all 29 families in both scopes with normal cleanup.

The v3 package remains unreleased pending full validation. Upgrade existing state
and manifests using [MIGRATION-v3.md](../MIGRATION-v3.md).

## Installation

Install a supported Crossplane release first. The v3 runtime probes use Crossplane 2.2.1.
The package tag below is the v2 maintenance release; select v3 only after its validation and publication complete.

```bash
kubectl apply -f - <<'EOF'
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-archestra
spec:
  package: xpkg.upbound.io/archestra/provider-archestra:v2.4.1
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
kind: RegistryEntry
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
kind: Server
metadata:
  name: everything
spec:
  forProvider:
    name: everything
    catalogIdRef:
      name: everything       # resolves to the RegistryEntry above
  providerConfigRef:
    name: default
```

`tool_id_by_name` (the per-tool UUID map) lands on
`status.atProvider.toolIdByName` once the installation reconciles —
use those IDs in `Agent.spec.forProvider.tools` or `Gateway.spec.forProvider.tools`.

## Development

`apis/**/zz_*.go`, `internal/controller/**/zz_*.go`, and
`package/crds/*.yaml` are gitignored — `make generate` invokes
upjet's codegen pipeline (`cmd/generator`) to (re)produce them
from the TF provider's JSON schema. No `make generate` ⇒ those
packages don't exist on disk ⇒ `go build` fails with "package not
found".

First time on a clean clone:

```bash
# in repo root: produce the TF binary that upjet shells out to
go build -o terraform-provider-archestra .

# in crossplane/: dump schema, run upjet codegen, build the controller
cd crossplane
make schema     # writes config/schema.json from the local TF binary
make generate   # runs upjet -> apis/, internal/controller/, package/crds/
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

2. **Configure both scopes.** Add the group, kind and references to the shared
   [config/resources.go](config/resources.go). `ConfigureResources` applies the
   same declarations to both cluster and namespaced providers.

   ```go
   p.AddResourceConfigurator("archestra_<thing>", func(r *ujconfig.Resource) {
       r.ShortGroup = "<group>"           // -> <group>.archestra.crossplane.io
       r.Kind       = "<Thing>"           // CamelCase singular
       r.References["<fk_field>"] = ujconfig.Reference{
           TerraformName: "archestra_<other>",  // for cross-MR refs
       }
   })
   ```

3. **Run upjet codegen.** `make generate` is what turns the
   config changes from steps 1–2 into real Go types
   (`apis/<scope>/<group>/v1alpha1/zz_<kind>_types.go`),
   reconcilers (`internal/controller/<scope>/<group>/<kind>/zz_controller.go`),
   and a CRD (`package/crds/<group>.archestra....yaml`). Without
   this step the new resource exists only in the upjet config and
   nothing else compiles. Commit nothing from these trees —
   they're gitignored and CI regenerates them every build.

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

## Required collection serialization

The generator preserves required collection tags for both Terraform parameters
and Kubernetes JSON. Explicit empty sets and maps must survive reconciliation:
omitting them can fail Terraform validation or Kubernetes object updates. Optional
collections retain their normal omission behavior. Regression tests exercise the
actual generated parameter and Kubernetes JSON serializers in both scopes.
