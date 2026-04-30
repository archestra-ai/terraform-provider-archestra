# Archestra Terraform Provider

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

## Contributing

If you're modifying the provider itself, see
[CONTRIBUTING.md](CONTRIBUTING.md). It covers prerequisites, the
merge-patch + AttrSpec architecture, the drift-check tests, and the
new-resource checklist.
