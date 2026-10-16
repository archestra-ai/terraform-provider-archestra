# Archestra Provider

[![Crossplane mappings](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Source for two artifacts, both regenerated from a single Terraform
schema and shipped together on every `vX.Y.Z` tag:

- `archestra-ai/archestra` on the [Terraform Registry](https://registry.terraform.io/providers/archestra-ai/archestra/latest)
- `xpkg.upbound.io/archestra/provider-archestra` on [Upbound](https://marketplace.upbound.io)

End users install from those. This README walks a new contributor
through extending the provider.

## Resource coverage

This v3 branch is under development. The current Terraform schema registers 23
resources and all 23 are configured in both Crossplane scopes. Real lifecycle
proof currently covers 10 resources in both scopes. Remaining field, migration
and packaging gates are listed in [the evidence report](audit/v3-runtime-evidence.json).
This count does not establish complete coverage of the wider Archestra API.

| Terraform | Crossplane group / kind |
|---|---|
| `archestra_agent` | `agent` / `Agent` |
| `archestra_agent_delegation` | `agent` / `Delegation` |
| `archestra_agent_tool` | `agent` / `Tool` |
| `archestra_agent_tool_batch` | `agent` / `ToolBatch` |
| `archestra_agent_tool_exclusions` | `agent` / `ToolExclusions` |
| `archestra_api_key` | `credential` / `APIKey` |
| `archestra_environment` | `environment` / `Environment` |
| `archestra_identity_provider` | `identity` / `Provider` |
| `archestra_knowledge_base` | `knowledge` / `KnowledgeBase` |
| `archestra_limit` | `limit` / `Limit` |
| `archestra_llm_model` | `llm` / `ModelOverride` |
| `archestra_llm_provider_api_key` | `llm` / `ProviderKey` |
| `archestra_mcp_gateway` | `mcp` / `Gateway` |
| `archestra_mcp_registry_catalog_item` | `mcp` / `RegistryCatalogItem` |
| `archestra_mcp_server_installation` | `mcp` / `ServerInstallation` |
| `archestra_organization_settings` | `organization` / `Settings` |
| `archestra_project` | `project` / `Project` |
| `archestra_resource_permission_policy` | `permission` / `Policy` |
| `archestra_role` | `identity` / `Role` |
| `archestra_runtime_credential` | `credential` / `RuntimeCredential` |
| `archestra_schedule_trigger` | `schedule` / `Trigger` |
| `archestra_team` | `team` / `Team` |
| `archestra_team_external_group` | `team` / `ExternalGroup` |

Seven Terraform data sources remain available for lookups and audit data.
Crossplane resources use `<group>.archestra.crossplane.io` in cluster scope and
`<group>.archestra.m.crossplane.io` in namespaced scope. Both currently serve
`v1alpha1`. See [MIGRATION-v3.md](MIGRATION-v3.md) before upgrading existing state.

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
tag vX.Y.Z   ──┬──▶  goreleaser    ──▶  Terraform Registry
               │
               └──▶  crank xpkg push ──▶  xpkg.upbound.io
```

The two publish jobs run in parallel; one failing won't roll back
the other.
