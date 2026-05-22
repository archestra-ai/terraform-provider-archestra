# Archestra Terraform Provider

[![Crossplane coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

The Archestra Terraform provider lets you manage Archestra resources —
agents, MCP servers, identity providers, teams, LLM keys, security
policies, organization settings — as code.

- **Registry:** <https://registry.terraform.io/providers/archestra-ai/archestra/latest>
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

## Releases

Single `vX.Y.Z` track. Each release publishes:

- **Terraform provider** binaries (signed) to the
  [Terraform Registry](https://registry.terraform.io/providers/archestra-ai/archestra)
  and GitHub Release assets.
- **Crossplane provider** xpkg (regenerated from the same TF
  schema) to `xpkg.upbound.io/archestra/provider-archestra`.

[`release-please`](https://github.com/googleapis/release-please)
reads conventional-commit messages on `main`:

| Commit prefix | Bumps |
|---|---|
| `feat:` | minor |
| `fix:` | patch |
| `feat!:` / `BREAKING CHANGE:` | major |
| `chore:`, `docs:`, `ci:`, `refactor:`, `test:`, `style:` | nothing |

The **type** (before the colon) drives the bump; the scope in
parentheses is decorative. Use `chore(ci):` or `ci:` for CI-only
changes you don't want to ship as a release.
