# Archestra Provider


The v3 review candidate has **29 resources and four data sources**, generated in both Crossplane scopes. Agent/gateway relationships, team bindings, object sharing and organization defaults belong to their parents. Manual and Git-backed plugins share one resource. The [migration guide](MIGRATION-v3.md) describes the clean-major names; there are no aliases for retired types. Additional durable API areas, final runtime/package validation, protected review and publication remain in progress. The [full native acceptance proof](audit/consolidated-native-acceptance.json) records 146 passing cases with zero failures or skips against API 1.4.0-rc.49.

[![Crossplane mappings](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Source for two artifacts, both regenerated from a single Terraform
schema and shipped together on every `vX.Y.Z` tag:

- `archestra-ai/archestra` on the [Terraform Registry](https://registry.terraform.io/providers/archestra-ai/archestra/latest)
- `xpkg.upbound.io/archestra/provider-archestra` on [Upbound](https://marketplace.upbound.io)

End users install from those. This README walks a new contributor
through extending the provider.

API versions are independent of provider versions. See the [compatibility matrix](COMPATIBILITY.md) for the required API capability floor and exact tested releases.

The [organization-defaults runtime proof](audit/crossplane-organization-defaults.json) verifies reference resolution, updates, drift, omission and explicit clears in both scopes on the current candidate.

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
tag vX.Y.Z   ──┬──▶  goreleaser    ──▶  Terraform Registry
               │
               └──▶  crank xpkg push ──▶  xpkg.upbound.io
```

The two publish jobs run in parallel; one failing won't roll back
the other.

Manual plugin packages add one family in both scopes, including creation grants, authoritative file replacement and executable-mode preservation (`audit/crossplane-manual-plugins.json` from the repository root). The full 44-family native suite passes all 142 acceptance cases with zero failures or skips (`audit/manual-plugins-full-native-acceptance.json`). Both records retain their exact frozen candidate hashes; current Linux packaging and the final combined sweep remain release gates.

Pinned repository packages add one family in both scopes, including creation grants, scheduled candidate configuration, credential-reference clearing and approved-byte preservation ([runtime proof](audit/crossplane-repository-plugins.json)). The full 45-family native suite passes all 145 acceptance cases ([native proof](audit/repository-plugins-full-native-acceptance.json)). Final combined-candidate and Linux package repetition remain release gates.

Revocable marketplace share links add one family and six both-scope records for skill/plugin membership, secret publication, anonymous read-only access, revocation drift and expiry ([runtime proof](audit/crossplane-marketplace-share-links.json)). The full 46-family native suite passes all 148 acceptance cases ([native proof](audit/marketplace-share-links-full-native-acceptance.json)). Final combined-candidate and Linux package repetition remain release gates.

Project-file content adds one family and four both-scope records for exact text/binary bytes, secret input and project references, drift repair, empty text updates, replacement refusal and normal deletion ([runtime proof](audit/crossplane-project-files.json)). All152 native acceptance cases pass ([native proof](audit/project-files-full-native-acceptance.json)). This is a pre-consolidation checkpoint; the major-version surface is being reviewed against product navigation.
