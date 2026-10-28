# Developing the Archestra Crossplane provider

## Documentation generation

The Terraform schema and `../docs/resources/` are the sources of truth.
After changing a Terraform description or example, run `make generate` at the
repository root, then `make -C crossplane generate` with a rebuilt Terraform binary.
Upjet's pinned scraper reads resource summaries and HCL examples from those local
docs; field descriptions continue to come directly from the schema. The metadata,
Go types, CRDs and `examples-generated/` are build artifacts and are not committed.

The package build includes `examples-generated/` through `--examples-root`, which
Marketplace uses for resource examples. Its field reference and resource summaries
come from the packaged CRDs. The [README](README.md) is embedded in
`package/crossplane.yaml` during generation;
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

## Required collection serialization

The generator preserves required collection tags for both Terraform parameters
and Kubernetes JSON. Explicit empty sets and maps must survive reconciliation:
omitting them can fail Terraform validation or Kubernetes object updates. Optional
collections retain their normal omission behavior. Regression tests exercise the
actual generated parameter and Kubernetes JSON serializers in both scopes.
