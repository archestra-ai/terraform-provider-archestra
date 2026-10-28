# Basic getting-started demo

The smallest end-to-end Archestra Terraform configuration that produces
a working agent. ~6 resources, applies in under 30 seconds against a
local backend.

Use this when you want to:

- Sanity-check that your backend is reachable + your API key works.
- See a realistic minimum chain you can copy into your own module.
- Smoke-test the provider after a version bump.

For a configuration that exercises every resource and data source,
see [../complete/](../complete/) instead.

## What this creates

| Resource | Purpose |
| --- | --- |
| `archestra_team.main` | One team for scoping |
| `archestra_model_provider.ollama` | Ollama model provider |
| `archestra_mcp_registry_entry.memory` | Registers the in-memory MCP server in the private catalog |
| `archestra_mcp_server.memory` | Installs the catalog item — backend discovers ~9 tools |
| `archestra_agent.main` | Hello-world chat agent backed by Ollama |
| `archestra_agent.main.tools` | Wires the `create_entities` MCP tool to the agent |

## Prerequisites

- A running Archestra backend you can reach.
- An API key minted from that backend (UI → Settings → API Keys, or
  `scripts/bootstrap-api-key.sh` against a local stack).
- A reachable Ollama endpoint for model discovery.

## Running it

```bash
export ARCHESTRA_BASE_URL="http://localhost:9000"
export ARCHESTRA_API_KEY="arch_..."
terraform init
terraform apply
```

No `terraform.tfvars` needed — every value is hard-coded so the apply
runs out of the box.

## Cleanup

```bash
terraform destroy
```
