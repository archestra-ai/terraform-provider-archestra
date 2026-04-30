---
page_title: "Getting Started - archestra Provider"
subcategory: ""
description: |-
  Declare the provider, mint an API key, configure your shell, and apply your first resource.
---

# Getting Started

## 1. Declare the provider

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
  # Don't commit keys to source.
}
```

`~> 0.6.0` pins to the `0.6.x` patch line (Terraform reads it as `>= 0.6.0, < 0.7.0`).
The provider is pre-1.0; minor bumps may include breaking changes, so widen
the constraint deliberately after reviewing the [CHANGELOG](https://github.com/archestra-ai/terraform-provider-archestra/blob/main/CHANGELOG.md).

## 2. Mint an API key

In the Archestra UI: **Settings → API Keys → New Key**. The token starts
with `arch_`. Treat it like a password.

## 3. Configure your shell

```bash
export ARCHESTRA_BASE_URL="https://archestra.your-company.example"
export ARCHESTRA_API_KEY="arch_..."
```

## 4. Add a resource and apply

Pick a resource from the [resource reference](../resources) — every
resource ships a runnable example — and add it to your `.tf` file.
The examples are intentionally illustrative: most reference *other*
resources (e.g., `archestra_team.engineering`) or input variables
(e.g., `var.openai_api_key`). When copy-pasting, either declare those
referenced resources/variables yourself or strip the cross-references.

```bash
terraform init
terraform plan
terraform apply
```

**Skipping ahead?** Two runnable example modules ship with the repo:

- [`examples/basic/`](https://github.com/archestra-ai/terraform-provider-archestra/tree/main/examples/basic) — the minimum chain that produces
  a working agent (provider, team, vault-backed LLM key, one MCP
  install, one agent, one tool wire). Apply in <30 seconds. Use this
  to sanity-check your backend setup or as a copy-paste starting point.
- [`examples/complete/`](https://github.com/archestra-ai/terraform-provider-archestra/tree/main/examples/complete) — wires the full bring-up
  chain together (org settings, identity provider, teams + external
  groups, LLM keys, MCP servers, agents, gateways, policies, limits,
  optimisation rules, data sources). Use this as a realistic
  configuration reference.

> **Secrets in state.** Inline `api_key = var.openai_api_key` lands in
> Terraform state in plaintext. For production, prefer the BYOS Vault
> flow ([guide](./byos-vault)) or a remote state backend with
> encryption at rest.
