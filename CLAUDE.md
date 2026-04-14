# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

The `codegen-api-client` command requires the Archestra backend running locally at `http://localhost:9000`. It fetches the OpenAPI spec, patches it for compatibility (via `scripts/patch_openapi.py`), then uses `oapi-codegen` to generate `internal/client/archestra_client.go`.

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

**OpenAPI Spec Patching**: `scripts/patch_openapi.py`

- Fixes numeric `exclusiveMinimum`/`exclusiveMaximum` values (OpenAPI 3.1 feature) in the backend's 3.0 spec
- Run automatically by `make codegen-api-client`

### Resources

All resources follow Terraform Plugin Framework patterns (`internal/provider/resource_*.go`):

| Resource                              | File                                    | Description                                           |
| ------------------------------------- | --------------------------------------- | ----------------------------------------------------- |
| `archestra_profile`                   | `resource_profile.go`                   | Manage agents/profiles (config, LLM, email, security) |
| `archestra_profile_tool`              | `resource_profile_tool.go`              | Assign tools to profiles with MCP server binding      |
| `archestra_mcp_registry_catalog_item` | `resource_mcp_registry_catalog_item.go` | Register MCP servers with full catalog metadata       |
| `archestra_mcp_server_installation`   | `resource_mcp_server_installation.go`   | Install MCP servers with auth + team scoping          |
| `archestra_team`                      | `resource_team.go`                      | Manage teams with TOON compression settings           |
| `archestra_team_external_group`       | `resource_team_external_group.go`       | Map external IdP groups to teams                      |
| `archestra_tool_invocation_policy`    | `resource_tool_invocation_policy.go`    | Security policies for tool invocations (conditions)   |
| `archestra_trusted_data_policy`       | `resource_trusted_data_policy.go`       | Define trusted data sources (conditions)              |
| `archestra_limit`                     | `resource_limit.go`                     | Set usage limits (token cost, tool calls, MCP calls)  |
| `archestra_optimization_rule`         | `resource_optimization_rule.go`         | Cost optimization rules for model routing             |
| `archestra_organization_settings`     | `resource_organization_settings.go`     | Full org settings (appearance, security, LLM, MCP, knowledge) |
| `archestra_chat_llm_provider_api_key` | `resource_chat_llm_provider_api_key.go` | Manage LLM provider API keys (17 providers, BYOS vault) |
| `archestra_sso_provider`              | `resource_sso_provider.go`              | SSO/Identity provider (OIDC + SAML + enterprise creds) |
| `archestra_llm_model`                 | `resource_llm_model.go`                 | Manage LLM model pricing and settings (replaces token_price) |

**Disabled Resources** (files have `//go:build ignore`):

- `resource_prompt.go` - Prompts are now inline on agents (`system_prompt` field). No standalone API.
- `resource_token_price.go` - Replaced by `archestra_llm_model` resource.
- `resource_dual_llm_config.go` - Replaced by `built_in_agent_config` block on `archestra_profile`.
- `resource_user.go` - User management (API not exposed in OpenAPI spec)

### Data Sources

Data sources for reading existing resources (`internal/provider/datasource_*.go`):

| Data Source                      | File                                 | Description                       |
| -------------------------------- | ------------------------------------ | --------------------------------- |
| `archestra_tool`                 | `datasource_tool.go`                 | Look up any tool by name          |
| `archestra_profile_tool`         | `datasource_profile_tool.go`         | Look up profile tools by name     |
| `archestra_mcp_server_tool`      | `datasource_mcp_server_tool.go`      | Look up MCP server tools          |
| `archestra_team`                 | `datasource_team.go`                 | Look up team information          |
| `archestra_team_external_groups` | `datasource_team_external_groups.go` | List external groups for a team   |

**Disabled Data Sources** (files have `//go:build ignore`):

- `datasource_prompt.go` - Prompts are now inline on agents. No standalone API.
- `datasource_prompt_versions.go` - Prompt versioning removed.
- `datasource_token_prices.go` - Token pricing moved to LLM Models API.
- `datasource_user.go` - User lookups (API not exposed in OpenAPI spec)

### Helper Utilities

- `retry.go` - Retry logic for async operations (used by profile tool and MCP server tool data sources)
- `prompt_shared.go` - Shared prompt helpers (disabled, `//go:build ignore`)

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

## Important Notes

- Disabled resources have `//go:build ignore` build tags - do not remove these unless re-enabling
- The API client is generated code - always regenerate rather than editing manually
- Default base URL is `http://localhost:9000` for local development
- Provider uses API key authentication via `Authorization` header (format: `arch_...`)
- Documentation is auto-generated from examples using tfplugindocs tool - run `make generate` after making changes to ensure docs are updated
- The API client uses `AgentId` internally (from OpenAPI spec) but Terraform schema exposes this as `profile_id` - this is intentional mapping
- The `profile_tool_id` attribute in policy resources maps to the tool ID (not the agent-tool assignment ID)
- The backend's OpenAPI spec may use numeric `exclusiveMinimum` (3.1 feature) - the `scripts/patch_openapi.py` handles this automatically
