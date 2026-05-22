# Archestra Terraform Provider

[![Crossplane coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

The Archestra Terraform provider lets you manage Archestra resources —
agents, MCP servers, identity providers, teams, LLM keys, security
policies, organization settings — as code.

- **Registry:** <https://registry.terraform.io/providers/archestra-ai/archestra/latest>
- **Crossplane:** `xpkg.upbound.io/archestra/provider-archestra` — same resources via upjet, see [`crossplane/`](crossplane/)
- **Guides:** [Getting Started](docs/guides/getting-started.md) · [Authentication](docs/guides/authentication.md) · [Resource Bring-up Order](docs/guides/bring-up-order.md) · [BYOS Vault](docs/guides/byos-vault.md) · [Common Issues](docs/guides/common-issues.md)
- **Schema reference:** [docs/](docs/) (auto-generated)
- **Per-resource snippets:** [examples/resources/](examples/resources/) — illustrative HCL for every resource
- **Runnable examples:** [examples/basic/](examples/basic/) (smallest end-to-end chain) · [examples/complete/](examples/complete/) (full bring-up wired together)
- **Changelog:** [CHANGELOG.md](CHANGELOG.md) — read before widening a version constraint
- **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)

## Quick declaration

```hcl
terraform {
  required_providers {
    archestra = {
      source  = "archestra-ai/archestra"
      version = "~> 0.6.0"
    }
  }
}

provider "archestra" {
  # base_url + api_key are read from ARCHESTRA_BASE_URL / ARCHESTRA_API_KEY.
}
```

```bash
export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
export ARCHESTRA_API_KEY="arch_..."   # mint via Settings → API Keys
terraform init && terraform apply
```

Full walkthrough in the [Getting Started guide](docs/guides/getting-started.md).

## Development

```bash
make build       # build the provider binary
make install     # build + install into $GOPATH/bin (for dev_overrides)
make test        # unit tests + drift checks
make testacc     # acceptance tests against $ARCHESTRA_BASE_URL
make generate    # regenerate docs/ from schema + examples
make lint        # golangci-lint v2
```

Prerequisites, `dev_overrides` setup, the merge-patch + AttrSpec
architecture, drift-check tests, the new-resource checklist, and the
acceptance-test env gates (`ARCHESTRA_READONLY_VAULT_ENABLED`,
`ARCHESTRA_TEST_IDP_ID`) live in [CONTRIBUTING.md](CONTRIBUTING.md).

## Crossplane provider

A Crossplane v1/v2 xpkg published from the same `vX.Y.Z` tag,
[upjet](https://github.com/crossplane/upjet)-generated from this
provider's TF schema. Lives under [`crossplane/`](crossplane/);
the subtree's [README](crossplane/README.md) covers install, the
ProviderConfig wiring, and the local `make generate` flow.

Resource coverage is partial — the badge at the top links to the
report of TF resources without a Crossplane MR yet. To expose a
new resource, add it to `ExternalNameConfigs` in
[`crossplane/config/external_name.go`](crossplane/config/external_name.go);
codegen and CRDs follow on the next `make generate`.

## Releases

One `vX.Y.Z` tag ships both artifacts in parallel:

- Terraform binaries (signed) → [Registry](https://registry.terraform.io/providers/archestra-ai/archestra) + GitHub Release assets
- Crossplane xpkg → `xpkg.upbound.io/archestra/provider-archestra`

Flow: a conventional-commit lands on `main` →
[`release-please`](https://github.com/googleapis/release-please)
opens a PR with the version bump + changelog entry → merging it
creates the `vX.Y.Z` tag and fires both publish jobs (goreleaser
for TF, multi-arch `crank xpkg push` for Crossplane). The two are
independent — one failing won't roll back the other.

The commit **type** drives the bump (`feat:` minor, `fix:` patch,
`feat!:` / `BREAKING CHANGE:` major); the **scope is decorative**.
That means `fix(ci): tweak workflow` still cuts a patch release —
use `ci:` or `chore(ci):` for plumbing-only changes you don't
want to ship. Likewise `docs:` and `refactor:` don't bump.
