# Archestra Provider


Provider 3.0.0 has **31 Terraform resources and four Terraform data sources**. Every resource is generated as a Crossplane managed resource in both the cluster and namespaced scopes. Relationships, team bindings, sharing and organization defaults are configured on their parent resource. Retired resource names have no aliases; the [migration guide](MIGRATION-v3.md) maps each one to its replacement.

[![Crossplane mappings](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Source for two artifacts, both regenerated from a single Terraform
schema and shipped together on every `vX.Y.Z` tag:

- `archestra-ai/archestra` on the [Terraform Registry](https://registry.terraform.io/providers/archestra-ai/archestra/latest)
- `xpkg.upbound.io/archestra/provider-archestra` on [Upbound Marketplace](https://marketplace.upbound.io/providers/archestra/provider-archestra)

End users install from those. This README walks a new contributor
through extending the provider.

API versions are independent of provider versions. See the [compatibility matrix](COMPATIBILITY.md) for the required API capability floor and exact tested releases.

Release evidence (native acceptance, both-scope Crossplane reconciliation and package checks) is recorded in [the audit](audit/README.md).

## Resource coverage

The implemented surface follows product objects and Settings. Configuration and
sharing live with their owner. See the generated [resource reference](docs/resources)
and [data sources](docs/data-sources) for exact fields, examples and import formats.

| Terraform | Crossplane group / kind |
|---|---|
| `archestra_agent` | `agent` / `Agent` |
| `archestra_api_key` | `credential` / `APIKey` |
| `archestra_credential` | `credential` / `Credential` |
| `archestra_environment` | `environment` / `Environment` |
| `archestra_github_app` | `credential` / `GitHubApp` |
| `archestra_github_pat` | `credential` / `GitHubPAT` |
| `archestra_identity_provider` | `identity` / `Provider` |
| `archestra_knowledge_base` | `knowledge` / `KnowledgeBase` |
| `archestra_knowledge_directory` | `knowledge` / `Directory` |
| `archestra_knowledge_file` | `knowledge` / `File` |
| `archestra_limit` | `limit` / `Limit` |
| `archestra_mcp_gateway` | `gateway` / `Gateway` |
| `archestra_mcp_registry_entry` | `mcp` / `RegistryEntry` |
| `archestra_mcp_server` | `mcp` / `Server` |
| `archestra_model` | `llm` / `Model` |
| `archestra_model_provider` | `llm` / `ModelProvider` |
| `archestra_oauth_client` | `credential` / `OAuthClient` |
| `archestra_organization_settings` | `organization` / `Settings` |
| `archestra_plugin` | `plugin` / `Plugin` |
| `archestra_project` | `project` / `Project` |
| `archestra_project_file` | `project` / `File` |
| `archestra_remote_agent` | `a2a` / `RemoteAgent` |
| `archestra_role` | `identity` / `Role` |
| `archestra_schedule` | `schedule` / `Schedule` |
| `archestra_service_account` | `identity` / `ServiceAccount` |
| `archestra_service_account_api_key` | `credential` / `ServiceAccountAPIKey` |
| `archestra_share_link` | `marketplace` / `ShareLink` |
| `archestra_skill` | `skill` / `Skill` |
| `archestra_team` | `team` / `Team` |
| `archestra_virtual_key` | `llm` / `VirtualKey` |

The four data sources are `archestra_agent`, `archestra_team`, `archestra_tool`
and `archestra_mcp_tool_calls`. Tool lookup accepts an optional MCP server filter;
agent lookup includes its relationships. Data sources read existing state.

Crossplane resources use `<group>.archestra.crossplane.io` in cluster scope and
`<group>.archestra.m.crossplane.io` in namespaced scope, serving `v1alpha1`.
See [the audit](audit/README.md) for executed evidence and each frozen binary's
boundary. Generated types are schema coverage, not proof of reconciliation.

## 1. Build it

```bash
git clone https://github.com/archestra-ai/terraform-provider-archestra
cd terraform-provider-archestra
make install   # compiles + installs to $GOPATH/bin
```

## 2. Point Terraform at your local build

Create `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "/path/to/your/$GOPATH/bin"
  }
  direct {}
}
```

Now any `terraform apply` resolves `archestra-ai/archestra` to your
local binary instead of the published Registry version.

## 3. Smoke-test against a local Archestra

```bash
export ARCHESTRA_BASE_URL=http://localhost:9000
export ARCHESTRA_API_KEY=arch_...      # mint via Settings → API Keys
cd examples/basic && terraform apply
```

If this works, your dev_overrides wiring is right. If it doesn't,
fix that before touching any code.

## 4. Add or modify a Terraform resource

Each resource lives in `internal/provider/resource_<name>.go` and is
registered in `provider.go`'s `Resources()` function. The loop:

1. Implement the resource (or change the existing one) + register it.
2. Add an example under `examples/resources/archestra_<name>/`.
3. Add an acceptance test (`resource_<name>_test.go`) — every bug fix needs a regression test too.
4. `make generate` — refreshes `docs/` from your example via tfplugindocs.
5. `make test && make testacc` — unit + drift checks, then acceptance against `$ARCHESTRA_BASE_URL`.

Architecture (merge-patch + AttrSpec, the two drift-check tests)
and the full new-resource checklist are in
[`ARCHITECTURE.md`](ARCHITECTURE.md) and
[`CONTRIBUTING.md`](CONTRIBUTING.md). Read them before touching the
schema — skipping leads to phantom plan diffs and silent backend drift.

## 5. Expose it as a Crossplane Managed Resource (optional)

A Managed Resource (**MR**) is the Kubernetes object a Crossplane
provider reconciles to backend state — `kubectl apply` a YAML, the
controller calls the Archestra API. To pick up a `—` row in the
coverage table above:

### a. Wire the config

1. Whitelist the TF resource name in `crossplane/config/external_name.go`.
2. Add a configurator block in **both** `crossplane/config/cluster/<group>/config.go` and `.../namespaced/<group>/config.go` setting `r.ShortGroup` (becomes `<group>.archestra.crossplane.io`) and `r.Kind` (CamelCase singular).
3. `make -C crossplane generate` — runs upjet over the TF schema (using the binary built in step 1) and produces the Go types, controllers, and CRDs for your new MR.

### b. Try it locally

Prereq: a local k8s cluster with Crossplane installed (OrbStack /
kind / minikube — the controller runs **out of cluster** against
whichever `kubectl` context you're pointing at, so you don't need
to package and install an xpkg).

```bash
# in repo root, rebuild the TF binary if you've changed the schema
go build -o terraform-provider-archestra .

cd crossplane
make generate
make build                                   # _output/bin/<host>/provider

# 1. Apply the CRDs so your cluster knows about the new Kind
kubectl apply -f package/crds/

# 2. ProviderConfig + creds pointing at your local Archestra
kubectl create secret generic archestra-creds -n crossplane-system \
  --from-literal=credentials='{"api_key":"arch_...","base_url":"http://localhost:9000"}'

cat <<EOF | kubectl apply -f -
apiVersion: archestra.crossplane.io/v1beta1
kind: ProviderConfig
metadata: { name: default }
spec:
  credentials:
    source: Secret
    secretRef: { namespace: crossplane-system, name: archestra-creds, key: credentials }
EOF

# 3. Run the controller (foreground, against $KUBECONFIG's current context)
./_output/bin/$(go env GOOS)_$(go env GOARCH)/provider --debug

# 4. In another shell: apply your MR YAML and watch it reconcile
kubectl apply -f examples/cluster/<group>/<kind>.yaml
kubectl get <kind>.<group>.archestra.crossplane.io -w
```

`make e2e` (kind + uptest), the namespaced (v2) scope wiring, and
the upjet config reference live in
[`crossplane/README.md`](crossplane/README.md).

## 6. Ship it

Open a PR with a conventional-commit title (`feat:`, `fix:`, etc.).
After it merges:

```
commit on main
        │
        ▼
release-please opens "chore(main): release vX.Y.Z"
        │  merge
        ▼
tag vX.Y.Z  ──▶  goreleaser       ──▶  GitHub release zips ──▶  Terraform Registry
                     │  then
                     ▼
                 crank xpkg push  ──▶  xpkg.upbound.io/archestra
                     │  then
                     ▼
                 up xpkg append   ──▶  Marketplace assets on the same package
```

The publish jobs run one after the other: `publish-crossplane` needs
`publish-terraform-provider`, because the runtime image downloads the
release zips. The same `publish-crossplane` job then adds the logo to that package
and verifies that both runtime manifests are unchanged, then enables the public
Marketplace listing for the existing repository through the Upbound CLI.
A publication failure does not roll back earlier publication steps.

See [the audit](audit/README.md) for the release evidence.
