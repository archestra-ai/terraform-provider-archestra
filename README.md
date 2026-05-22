# Archestra Terraform Provider

[![Crossplane coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/archestra-ai/terraform-provider-archestra/main/.github/badges/crossplane-coverage.json)](.github/workflows/crossplane-coverage.yml)

Manage Archestra agents, MCP servers, identity providers, teams,
LLM keys, and security policies as Terraform code.

- **Terraform Registry:** <https://registry.terraform.io/providers/archestra-ai/archestra/latest>
- **Crossplane xpkg:** `xpkg.upbound.io/archestra/provider-archestra` ([details](#crossplane))
- **Changelog:** [`CHANGELOG.md`](CHANGELOG.md)

## Quick start

```hcl
terraform {
  required_providers {
    archestra = {
      source  = "archestra-ai/archestra"
      version = "~> 1.0"
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

Full walk-through in the [Getting Started guide](docs/guides/getting-started.md).

## Docs & examples

- **Guides:** [Authentication](docs/guides/authentication.md) ·
  [Resource Bring-up Order](docs/guides/bring-up-order.md) ·
  [BYOS Vault](docs/guides/byos-vault.md) ·
  [Common Issues](docs/guides/common-issues.md)
- **Schema reference:** [`docs/`](docs/) (auto-generated per release)
- **HCL snippets:** [`examples/resources/`](examples/resources/) — one per resource
- **Runnable examples:** [`examples/basic/`](examples/basic/) (smallest end-to-end chain) ·
  [`examples/complete/`](examples/complete/) (full bring-up)

## Crossplane

Same resources, packaged as a Crossplane v1/v2 xpkg at
`xpkg.upbound.io/archestra/provider-archestra`. The xpkg is
[upjet](https://github.com/crossplane/upjet)-generated from this
provider's schema and shipped from the same `vX.Y.Z` tag. Setup,
contributor flow, and the new-resource checklist live in
[`crossplane/README.md`](crossplane/README.md). Coverage of TF
resources is partial — the badge above links to the live report.

## Contributing

Architecture, prerequisites, `dev_overrides` setup, drift-check
tests, the new-resource checklist, and acceptance-test env gates
(`ARCHESTRA_READONLY_VAULT_ENABLED`, `ARCHESTRA_TEST_IDP_ID`) live
in [`CONTRIBUTING.md`](CONTRIBUTING.md). Common targets:

```bash
make build       # provider binary
make test        # unit tests + drift checks
make testacc     # acceptance tests against $ARCHESTRA_BASE_URL
make generate    # regenerate docs/ from schema + examples
make lint        # golangci-lint v2
```

## Releases

One `vX.Y.Z` tag ships both artifacts in parallel — Terraform
binaries (signed) to the Registry and the Crossplane xpkg to
Upbound — driven by
[`release-please`](https://github.com/googleapis/release-please)
from conventional-commit messages on `main`. The commit **type**
drives the bump (`feat:` → minor, `fix:` → patch, `feat!:` /
`BREAKING CHANGE:` → major); the **scope is decorative**, so
`fix(ci): …` still cuts a patch — use bare `ci:` or `chore(ci):`
for plumbing-only changes.
