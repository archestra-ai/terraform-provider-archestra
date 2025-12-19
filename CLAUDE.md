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
make lint                     # Run golangci-lint (requires golangci-lint installed)
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
- Large file (~1.4MB) containing all API models and client methods
- Configuration in `oapi-config.yaml` excludes certain routes (llm-proxy, auth wildcards, mcp-gateway, interactions, health)
- DO NOT manually edit this file - regenerate using `make codegen-api-client`

### Resources

All resources follow Terraform Plugin Framework patterns (`internal/provider/resource_*.go`):

| Resource                              | File                                    | Description                                                 |
| ------------------------------------- | --------------------------------------- | ----------------------------------------------------------- |
| `archestra_profile`                   | `resource_profile.go`                   | Manage Archestra profiles (name, labels)                    |
| `archestra_profile_tool`              | `resource_profile_tool.go`              | Assign tools to profiles with execution/security config     |
| `archestra_mcp_server`                | `resource_mcp_server_registry.go`       | Register MCP server definitions                             |
| `archestra_mcp_server_installation`   | `resource_mcp_server_installation.go`   | Install MCP servers                                         |
| `archestra_team`                      | `resource_team.go`                      | Manage teams                                                |
| `archestra_team_external_group`       | `resource_team_external_group.go`       | Map external IdP groups to teams                            |
| `archestra_tool_invocation_policy`    | `resource_tool_invocation_policy.go`    | Security policies for tool invocations                      |
| `archestra_trusted_data_policy`       | `resource_trusted_data_policy.go`       | Define trusted data sources                                 |
| `archestra_token_price`               | `resource_token_price.go`               | Configure custom token pricing                              |
| `archestra_limit`                     | `resource_limit.go`                     | Set usage limits (token cost, tool calls, MCP server calls) |
| `archestra_optimization_rule`         | `resource_optimization_rule.go`         | Cost optimization rules for model routing                   |
| `archestra_organization_settings`     | `resource_organization_settings.go`     | Organization-wide settings                                  |
| `archestra_chat_llm_provider_api_key` | `resource_chat_llm_provider_api_key.go` | Manage LLM provider API keys                                |

**Disabled Resources**:

- `resource_user.go` - User management (commented out in provider.go, pending backend API)

### Data Sources

Data sources for reading existing resources (`internal/provider/datasource_*.go`):

| Data Source                      | File                                 | Description                     |
| -------------------------------- | ------------------------------------ | ------------------------------- |
| `archestra_profile_tool`         | `datasource_profile_tool.go`         | Look up profile tools by name   |
| `archestra_mcp_server_tool`      | `datasource_mcp_server_tool.go`      | Look up MCP server tools        |
| `archestra_team`                 | `datasource_team.go`                 | Look up team information        |
| `archestra_team_external_groups` | `datasource_team_external_groups.go` | List external groups for a team |
| `archestra_token_prices`         | `datasource_token_prices.go`         | List token prices               |

**Disabled Data Sources**:

- `datasource_user.go` - User lookups (commented out in provider.go, pending backend API)

### Helper Utilities

- `retry.go` - Retry logic for async operations (used by profile tool data source)

### Each Resource/Data Source Contains

- Resource/DataSource struct with `client *client.ClientWithResponses`
- Model struct with `tfsdk` tags mapping to Terraform schema
- Standard CRUD methods: Create, Read, Update, Delete (resources only)
- Configure method to receive API client from provider

### Policy Resources

The provider supports two types of security policies:

1. **Tool Invocation Policies**: Control when tools can be invoked based on argument values

   - Actions: `allow_when_context_is_untrusted`, `block_always`
   - Operators: `equal`, `notEqual`, `contains`, `notContains`, `startsWith`, `endsWith`, `regex`

2. **Trusted Data Policies**: Define which data sources are considered trusted
   - Actions: `mark_as_trusted`, `block_always`, `sanitize_with_dual_llm`
   - Critical for security model in multi-agent systems

### Cost Management Resources

The provider supports cost optimization features:

1. **Token Prices** (`archestra_token_price`): Define custom pricing for models
2. **Limits** (`archestra_limit`): Set usage limits at organization, team, or profile level
   - Types: `token_cost`, `tool_calls`, `mcp_server_calls`
3. **Optimization Rules** (`archestra_optimization_rule`): Route requests to cheaper models based on conditions
   - Conditions: `max_length`, `has_tools`

### Testing

Tests follow the pattern `*_test.go` alongside each resource/data source. Acceptance tests use the Terraform Plugin Testing framework and require:

- `TF_ACC=1` environment variable
- Running Archestra backend (default: localhost:9000)
- Valid API key in environment or configuration

## Important Notes

- User resources are currently disabled (commented out in `internal/provider/provider.go`) pending backend API implementation
- The API client is generated code - always regenerate rather than editing manually
- Default base URL is `http://localhost:9000` for local development
- Provider uses API key authentication via `Authorization` header
- Documentation is auto-generated from examples using tfplugindocs tool - run `make generate` after making changes to ensure docs are updated
- The API client uses `AgentId` internally (from OpenAPI spec) but Terraform schema exposes this as `profile_id` - this is intentional mapping
