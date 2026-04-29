# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> **Adding or modifying a resource? Read [CONTRIBUTING.md](CONTRIBUTING.md) first.** It covers the merge-patch + AttrSpec architecture, the two drift-check tests (TestSpecDrift, TestApiCoverage) and the per-resource opt-in interfaces (`resourceWithAttrSpec`, `resourceWithAPIShape`), the schema-attr-vs-skip-list convention, and the new-resource checklist. Skipping it leads to phantom plan diffs and silent backend-drift bugs.

## Overview

This is a Terraform provider for Archestra, built using the Terraform Plugin Framework. It enables infrastructure-as-code management of Archestra resources including profiles, MCP servers, teams, security policies, cost optimization, and organization settings.

## Development Commands

### Building and Installation

```bash
make build                    # Build the provider
make install                  # Build and install locally (creates binary in $GOPATH/bin)
go build -v ./...            # Direct build command
```

### Testing

#### Unit Tests

```bash
make test                     # Run unit tests (timeout=120s, parallel=10)
go test -v -cover -timeout=120s -parallel=10 ./...  # Direct test command
```

#### Acceptance Tests

Acceptance tests require environment configuration:

```bash
export ARCHESTRA_BASE_URL="http://localhost:9000"
export ARCHESTRA_API_KEY="your-api-key"
make testacc                  # Run acceptance tests against hosted environment
```

**One-shot local setup for the *full* suite (EE + BYOS + EMC):**

```bash
scripts/bootstrap-local-stack.sh                          # ~90 s, idempotent
eval "$(scripts/bootstrap-local-stack.sh --print-env)"
make testacc                                              # 75/75 PASS
scripts/bootstrap-local-stack.sh --down                   # tear down
```

The script runs the platform image + `hashicorp/vault:1.18` + an Ollama
mock (hashicorp/http-echo) on a shared docker network, with
`ARCHESTRA_ENTERPRISE_LICENSE_ACTIVATED=true` and
`ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT`. It also seeds the BYOS test
secret and provisions the EMC IdP. The platform image tag tracks
`ARCHESTRA_VERSION` (default matches `.github/workflows/on-pull-request.yml`;
override the env var to test against a different release). Without the
script, the BYOS / EMC / LLM-model subsets `t.Fatal` with actionable
messages instead of running against an under-configured backend.

### Code Quality

```bash
make fmt                      # Format code with gofmt and terraform fmt
make lint                     # Run golangci-lint v2 (requires golangci-lint installed)
gofmt -s -w -e .             # Direct format command
```

### Code Generation

```bash
make generate                 # Generate Terraform documentation (uses tfplugindocs)
make codegen-api-client       # Regenerate API client from OpenAPI spec
```

The `codegen-api-client` command requires the Archestra backend running locally at `http://localhost:9000` and uses `oapi-codegen` to generate `internal/client/archestra_client.go` from the OpenAPI spec.

### Local Development with Terraform

To use a locally-built provider, configure development overrides in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "archestra-ai/archestra" = "/path/to/your/terraform-provider-archestra"
  }

  direct {}
}
```

Then run `make install` and use Terraform commands in the `examples/` directory.

## Architecture

### Provider Structure

**Main Entry Point**: `main.go` - Standard Terraform plugin server setup using Plugin Framework

**Provider Core**: `internal/provider/provider.go`

- Defines `ArchestraProvider` with configuration for `base_url` and `api_key`
- Defaults to `http://localhost:9000` if base_url not provided
- API key passed in Authorization header (not Bearer token format)
- Registers all resources and data sources

**API Client**: `internal/client/archestra_client.go`

- Auto-generated from OpenAPI spec using `oapi-codegen`
- Large file (~1.5MB) containing all API models and client methods
- Configuration in `oapi-config.yaml` excludes non-Terraform routes (LLM Proxy, Auth, MCP Gateway, Chat, etc.)
- DO NOT manually edit this file - regenerate using `make codegen-api-client`

### Resources

All resources follow Terraform Plugin Framework patterns (`internal/provider/resource_*.go`):

| Resource                              | File                                    | Description                                           |
| ------------------------------------- | --------------------------------------- | ----------------------------------------------------- |
| `archestra_agent`                     | `resource_agent.go`                     | Manage internal chat agents (system prompt, LLM, knowledge sources, email triggers) |
| `archestra_llm_proxy`                 | `resource_llm_proxy.go`                 | Manage LLM proxies (upstream LLM, passthrough headers, identity provider)            |
| `archestra_mcp_gateway`               | `resource_mcp_gateway.go`               | Manage MCP gateways (knowledge sources, passthrough headers, identity provider)      |
| `archestra_agent_tool`                | `resource_agent_tool.go`                | Assign tools to agents (any type) with MCP server binding |
| `archestra_mcp_registry_catalog_item` | `resource_mcp_registry_catalog_item.go` | Register MCP servers with full catalog metadata       |
| `archestra_mcp_server_installation`   | `resource_mcp_server_installation.go`   | Install MCP servers with auth + team scoping          |
| `archestra_team`                      | `resource_team.go`                      | Manage teams with TOON compression settings           |
| `archestra_team_external_group`       | `resource_team_external_group.go`       | Map external IdP groups to teams                      |
| `archestra_tool_invocation_policy`    | `resource_tool_invocation_policy.go`    | Security policies for tool invocations (conditions)   |
| `archestra_trusted_data_policy`       | `resource_trusted_data_policy.go`       | Define trusted data sources (conditions)              |
| `archestra_limit`                     | `resource_limit.go`                     | Set usage limits (token cost, tool calls, MCP calls)  |
| `archestra_optimization_rule`         | `resource_optimization_rule.go`         | Cost optimization rules for model routing             |
| `archestra_organization_settings`     | `resource_organization_settings.go`     | Full org settings (appearance, security, LLM, MCP, knowledge) |
| `archestra_llm_provider_api_key`      | `resource_llm_provider_api_key.go`      | Manage LLM provider API keys (17 providers, BYOS vault) |
| `archestra_identity_provider`         | `resource_identity_provider.go`         | Identity provider for SSO (OIDC + SAML + enterprise creds) |
| `archestra_llm_model`                 | `resource_llm_model.go`                 | Manage LLM model pricing and settings (replaces token_price) |

### Data Sources

Data sources for reading existing resources (`internal/provider/datasource_*.go`):

| Data Source                      | File                                 | Description                       |
| -------------------------------- | ------------------------------------ | --------------------------------- |
| `archestra_tool`                 | `datasource_tool.go`                 | Look up any tool by name          |
| `archestra_agent_tool`           | `datasource_agent_tool.go`           | Look up agent-tool assignments    |
| `archestra_mcp_server_tool`      | `datasource_mcp_server_tool.go`      | Look up MCP server tools          |
| `archestra_team`                 | `datasource_team.go`                 | Look up team information          |
| `archestra_team_external_groups` | `datasource_team_external_groups.go` | List external groups for a team   |

### Helper Utilities

- `retry.go` - Retry logic for async operations (used by agent-tool and MCP server tool data sources)
- `agent_shared.go` - Shared helpers for the three agent-type resources (`archestra_agent`, `archestra_llm_proxy`, `archestra_mcp_gateway`)

### Each Resource/Data Source Contains

- Resource/DataSource struct with `client *client.ClientWithResponses`
- Model struct with `tfsdk` tags mapping to Terraform schema
- Standard CRUD methods: Create, Read, Update, Delete (resources only)
- Configure method to receive API client from provider

### Policy Resources

The provider supports two types of security policies, both using a conditions array:

1. **Tool Invocation Policies**: Control when tools can be invoked based on argument values
   - Actions: `allow_when_context_is_untrusted`, `block_always`
   - Conditions: `key` (argument name), `operator`, `value`
   - Operators: `equal`, `notEqual`, `contains`, `notContains`, `startsWith`, `endsWith`, `regex`

2. **Trusted Data Policies**: Define which data sources are considered trusted
   - Actions: `mark_as_trusted`, `block_always`, `sanitize_with_dual_llm`
   - Conditions: `key` (attribute path), `operator`, `value`

### Organization Settings

Organization settings are managed through multiple backend endpoints:
- `UpdateAppearanceSettings` - Font, theme, logo
- `UpdateLlmSettings` - Compression scope, tool result conversion, limit cleanup interval
- `CompleteOnboarding` - One-way onboarding completion

### Cost Management Resources

1. **Limits** (`archestra_limit`): Set usage limits at organization, team, or profile level
   - Types: `token_cost`, `tool_calls`, `mcp_server_calls`
2. **Optimization Rules** (`archestra_optimization_rule`): Route requests to cheaper models based on conditions
   - Conditions: `max_length`, `has_tools`

### Testing

Tests follow the pattern `*_test.go` alongside each resource/data source. Acceptance tests use the Terraform Plugin Testing framework and require:

- `TF_ACC=1` environment variable
- Running Archestra backend (default: localhost:9000)
- Valid API key in environment or configuration

Tests use MCP server tools (via `@modelcontextprotocol/server-filesystem`) for tool-dependent test scenarios rather than built-in tools.

Some tests opt in via extra environment variables:

- `ARCHESTRA_READONLY_VAULT_ENABLED=true` — required by BYOS-vault-dependent tests (`TestAccMcpRegistryCatalogItemResourceWithVaultRefs` and all `TestAccLLMProviderApiKeyResource*`). The helper `testAccRequireByosEnabled` calls `t.Fatal` loudly instead of silently skipping, so a DB-mode backend fails fast with an actionable message. The backend must also run with `ARCHESTRA_SECRETS_MANAGER=READONLY_VAULT` plus an enterprise license.
- `ARCHESTRA_TEST_IDP_ID=<uuid>` — run `TestAccMcpRegistryCatalogItemResourceWithEnterpriseManagedConfig` against an existing identity provider; skipped otherwise because the EE IdP API is license-gated.

CI runs BYOS-mode: the workflow deploys a dev Vault pod and an Ollama `/v1/models` HTTP stub, and overrides `ARCHESTRA_OLLAMA_BASE_URL` at `.github/values-ci.yaml` so backend key-validation passes without real Ollama.

## Important Notes

- The API client is generated code - always regenerate rather than editing manually
- Default base URL is `http://localhost:9000` for local development
- Provider uses API key authentication via `Authorization` header (format: `arch_...`)
- Documentation is auto-generated from examples using tfplugindocs tool - run `make generate` after making changes to ensure docs are updated
- Backend models all four agent variants (`agent`, `llm_proxy`, `mcp_gateway`, plus the legacy `profile` type) on a single `agents` table with an `agentType` discriminator. The provider exposes them as three separate resources (`archestra_agent`, `archestra_llm_proxy`, `archestra_mcp_gateway`) for 1:1 parity with the UI; the legacy `profile` type is not surfaced.
- The `tool_id` attribute on `archestra_tool_invocation_policy` and `archestra_trusted_data_policy` is the **bare tool UUID** (matching the `tools` table — the backend stores `toolId` directly). It is NOT the agent-tool assignment composite ID. Preferred lookup: `archestra_mcp_server_installation.<n>.tool_id_by_name["<server>__<short>"]` — one line, no extra data source.
- Conditional vs bulk-default policy resources hit different backend endpoints — `archestra_tool_invocation_policy` (with non-empty `conditions`) maps to the UI's "Add Policy" button; `archestra_tool_invocation_policy_default` (with `tool_ids` + `action`) maps to the UI's `DEFAULT` row. Same split for `archestra_trusted_data_policy` ↔ `_default` and for `archestra_agent_tool` ↔ `archestra_agent_tool_batch`. Don't fake a default with a wildcard regex — use the `_default` resource.
