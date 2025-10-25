# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Terraform provider for Archestra, built using the Terraform Plugin Framework. It enables infrastructure-as-code management of Archestra resources including agents, MCP servers, teams, users, and security policies.

## Development Commands

### Building and Installation

```bash
make build                    # Build the provider
make install                  # Build and install locally (creates binary in $GOPATH/bin)
go build -v ./...            # Direct build command
```

### Testing

```bash
make test                     # Run unit tests (timeout=120s, parallel=10)
make testacc                  # Run acceptance tests (requires TF_ACC=1, timeout=120m)
go test -v -cover -timeout=120s -parallel=10 ./...  # Direct test command
```

Run a single test:

```bash
go test -v -run TestAccAgentResource ./internal/provider/
```

### Code Quality

```bash
make fmt                      # Format code with gofmt
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
- API key passed as Bearer token in Authorization header
- Registers all resources and data sources

**API Client**: `internal/client/archestra_client.go`

- Auto-generated from OpenAPI spec using `oapi-codegen`
- ~306KB file containing all API models and client methods
- Configuration in `oapi-config.yaml` excludes certain routes (llm-proxy, auth wildcards, mcp-gateway, interactions, health)
- DO NOT manually edit this file - regenerate using `make codegen-api-client`

### Resources and Data Sources

All resources and data sources follow Terraform Plugin Framework patterns:

**Resources** (`internal/provider/resource_*.go`):

- `resource_agent.go` - Manage Archestra agents (name, is_demo, is_default)
- `resource_mcp_server.go` - Manage MCP server installations
- `resource_team.go` - Manage teams with members
- `resource_tool_invocation_policy.go` - Security policies for tool invocations
- `resource_trusted_data_policy.go` - Security policies for trusted data
- `resource_user.go` - User management (currently commented out in provider.go)

**Data Sources** (`internal/provider/datasource_*.go`):

- `datasource_agent_tool.go` - Look up agent tools
- `datasource_mcp_server_tool.go` - Look up MCP server tools
- `datasource_team.go` - Look up team information
- `datasource_user.go` - User lookups (currently commented out in provider.go)

Each resource/data source file contains:

- Resource/DataSource struct with `client *client.ClientWithResponses`
- Model struct with `tfsdk` tags mapping to Terraform schema
- Standard CRUD methods: Create, Read, Update, Delete (resources only)
- Configure method to receive API client from provider

### Policy Resources

The provider supports two types of security policies:

1. **Tool Invocation Policies**: Control when tools can be invoked based on context trust level

   - Actions: `allow_when_context_is_untrusted`, `block_always`
   - Operators: `equal`, `notEqual`, `contains`, `notContains`, `startsWith`, `endsWith`, `regex`

2. **Trusted Data Policies**: Define which data sources are considered trusted
   - Critical for security model in multi-agent systems

### Testing

Tests follow the pattern `*_test.go` alongside each resource/data source. Acceptance tests use the Terraform Plugin Testing framework and require:

- `TF_ACC=1` environment variable
- Running Archestra backend (default: localhost:9000)
- Valid API key in configuration

## Important Notes

- User resources are currently disabled (commented out in `internal/provider/provider.go:148,155`) pending backend API implementation
- The API client is generated code - always regenerate rather than editing manually
- Default base URL is `http://localhost:9000` for local development
- Provider uses Bearer token authentication via `Authorization` header
- Documentation is auto-generated from examples using tfplugindocs tool
