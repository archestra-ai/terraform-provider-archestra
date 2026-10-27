# Archestra Crossplane provider

Manage Archestra agents, MCP servers, teams, projects, credentials, and other
resources as Kubernetes objects. The provider is generated from the
[Archestra Terraform provider](https://registry.terraform.io/providers/archestra-ai/archestra/latest/docs)
with [Upjet](https://github.com/crossplane/upjet).

## Install

Install [Crossplane](https://docs.crossplane.io/latest/get-started/install/) first.
The provider is tested with Crossplane 2.2.1. Choose a released version from
[Upbound Marketplace](https://marketplace.upbound.io/providers/archestra/provider-archestra)
and pin it in your manifest; this example uses v3.0.3:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-archestra
spec:
  package: xpkg.upbound.io/archestra/provider-archestra:v3.0.3
```

Save this as `provider.yaml`, then wait for the provider to become healthy:

```bash
kubectl apply -f provider.yaml
kubectl wait provider.pkg.crossplane.io/provider-archestra --for=condition=Healthy --timeout=5m
```

## Configure credentials

Create an Archestra API key under **Settings → API Keys**. Set `base_url` to your
Archestra API endpoint reachable from the provider pod. The default,
`http://localhost:9000`, points inside the pod and usually needs overriding.

```bash
kubectl create secret generic archestra-creds -n crossplane-system \
  --from-literal=credentials='{"api_key":"arch_REPLACE_ME","base_url":"https://archestra.example.com"}'
```

The following namespaced `ProviderConfig` uses that Secret. Save it as
`providerconfig.yaml` and apply it with `kubectl apply -f providerconfig.yaml`:

```yaml
apiVersion: archestra.m.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
  namespace: crossplane-system
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: archestra-creds
      key: credentials
```

## Create a team

Save this as `team.yaml` and run `kubectl apply -f team.yaml`:

```yaml
apiVersion: team.archestra.m.crossplane.io/v1alpha1
kind: Team
metadata:
  name: engineering
  namespace: crossplane-system
spec:
  forProvider:
    name: Engineering
    description: Engineering team
  providerConfigRef:
    kind: ProviderConfig
    name: default
```

```bash
kubectl wait teams.team.archestra.m.crossplane.io/engineering -n crossplane-system --for=condition=Ready --timeout=5m
kubectl get teams.team.archestra.m.crossplane.io/engineering -n crossplane-system -o yaml
```

Inputs belong in `spec.forProvider`; observed values appear in
`status.atProvider`. Use `*Ref` or `*Selector` fields where available to refer to
other managed resources. Sensitive inputs use Kubernetes Secret references.

Deleting this managed resource deletes its Archestra object by default. To retain
the object, set `spec.deletionPolicy: Orphan` before deleting the manifest.

## API reference and examples

The **Managed Resources** tab on
[Upbound Marketplace](https://marketplace.upbound.io/providers/archestra/provider-archestra)
contains the field reference for each package version. Both API scopes are included:

| Scope | API group suffix | Credentials configuration |
| --- | --- | --- |
| Namespaced | `archestra.m.crossplane.io` | `ProviderConfig` in the resource namespace, or `ClusterProviderConfig` |
| Cluster | `archestra.crossplane.io` | Cluster-scoped `ProviderConfig` |

Managed resources use `v1alpha1`; credential configuration uses `v1beta1`.
For cluster-scoped resources, omit `metadata.namespace` and the
`providerConfigRef.kind` field. See the
[credential examples](https://github.com/archestra-ai/terraform-provider-archestra/tree/main/crossplane/examples).

Resource examples are generated from the existing Terraform examples for both
scopes. Treat them as templates: replace placeholders and Terraform expressions
such as `${...}`, supply referenced resources and Secrets, and select your
`providerConfigRef` before applying them. Terraform data sources are not generated
as managed resources.

For upgrades from v2, read the
[v3 migration guide](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/MIGRATION-v3.md).
Report bugs or request features in
[GitHub issues](https://github.com/archestra-ai/terraform-provider-archestra/issues).

## Data sources

Crossplane has no data sources. To read an existing agent or team, the
equivalent of the `archestra_agent` and `archestra_team` Terraform data
sources, apply an observe-only managed resource. Set the external-name
annotation to the object's API ID; the controller never writes to it, the
current state appears in `status.atProvider`, and deleting the managed
resource leaves the object in place:

```yaml
apiVersion: team.archestra.m.crossplane.io/v1alpha1
kind: Team
metadata:
  name: platform-team
  namespace: crossplane-system
  annotations:
    crossplane.io/external-name: 00000000-0000-4000-8000-000000000000   # the team's API ID
spec:
  managementPolicies: ["Observe"]
  forProvider: {}
  providerConfigRef:
    kind: ProviderConfig
    name: default
```

Use `kind: Agent` with `apiVersion: agent.archestra.m.crossplane.io/v1alpha1`
for an agent. `archestra_tool` and `archestra_mcp_tool_calls` are read-only
lookups with no managed object behind them, so they have no Crossplane
equivalent; read them from the API.

## Development

### Documentation generation

The Terraform schema and `../docs/resources/` are the sources of truth.
After changing a Terraform description or example, run `make generate` at the
repository root, then `make -C crossplane generate` with a rebuilt Terraform binary.
Upjet's pinned scraper reads resource summaries and HCL examples from those local
docs; field descriptions continue to come directly from the schema. The metadata,
Go types, CRDs and `examples-generated/` are build artifacts and are not committed.

The package build includes `examples-generated/` through `--examples-root`, which
Marketplace uses for resource examples. Its field reference and resource summaries
come from the packaged CRDs. The public sections of this README (before
**Development**) are embedded in `package/crossplane.yaml` during generation;
commit that updated file when changing this quick start. Marketplace renders its
`meta.crossplane.io/readme` annotation. No hand-maintained per-resource pages or
additional publishing credentials are needed.

README and CRD changes reach Marketplace with the next provider package release.
The existing asset repair workflow only changes appended assets such as the logo;
it cannot replace the CRDs or README annotation in an existing runtime package.

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
make generate   # runs upjet -> apis/, internal/controller/, package/crds/
make build      # binary at _output/bin/<host-platform>/provider
```

After a Terraform schema change, rebuild the binary at the repository root and
run `make generate` here. Generation always uses that local binary; override
`TF_PROVIDER_BIN` to select another build. See `hack/generate-schema.sh`.

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

4. **Add a Terraform example.** Update `../examples/resources/archestra_<thing>/`
   and run `make generate` at the repository root to refresh `../docs/resources/`.
   Crossplane generation scrapes those docs and converts their first example to
   YAML for each scope under `examples-generated/`. These are starting points;
   replace placeholders and supply referenced resources and Secrets before use.

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
xpkg is published to `xpkg.upbound.io/archestra/provider-archestra` alongside
each Terraform provider release. The release workflow then adds the
[Marketplace assets](marketplace/README.md) to that same package.

The existing [Crossplane release job](../.github/workflows/release-please.yml)
uses the existing `UPBOUND_ACCESS_ID` and `UPBOUND_TOKEN` GitHub Actions secrets
for the `archestra/provider-archestra` repository. No second namespace or
publisher credentials are needed.

After the Crossplane package push succeeds, that same job appends the logo and
verifies that the Linux amd64 and arm64 runtime manifests are unchanged.
Retry a failed publication by re-running **Publish Crossplane Provider** for
that release in GitHub Actions.

Registry publication and Marketplace listing are separate. After adding the
assets, the workflow uses `up repository update --private=false --publish=true`
to enable the public listing for the existing `archestra/provider-archestra`
repository. If its current robot credential cannot update repository settings,
an administrator must enable **Publish to Marketplace** once. Valid release
versions and their assets are then listed at
[Upbound Marketplace](https://marketplace.upbound.io/providers/archestra/provider-archestra).

## License

Apache 2.0 — see [`LICENSE`](../LICENSE).

## Required collection serialization

The generator preserves required collection tags for both Terraform parameters
and Kubernetes JSON. Explicit empty sets and maps must survive reconciliation:
omitting them can fail Terraform validation or Kubernetes object updates. Optional
collections retain their normal omission behavior. Regression tests exercise the
actual generated parameter and Kubernetes JSON serializers in both scopes.
