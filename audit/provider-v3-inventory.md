# Provider API inventory

static inventory; no runtime coverage implied

Terraform: 23; Crossplane configured: 23.

Mapping counts are configuration coverage, not successful CRUD or reconciliation coverage.

| Terraform resource | Crossplane configured | Import | Missing direct API calls / retired request fields | Acceptance tests declared |
|---|---|---|---|---|
| `archestra_agent` | True | True | none detected | 5 |
| `archestra_agent_delegation` | True | True | none detected | 1 |
| `archestra_agent_tool` | True | True | none detected | 1 |
| `archestra_agent_tool_batch` | True | True | none detected | 1 |
| `archestra_agent_tool_exclusions` | True | True | none detected | 1 |
| `archestra_api_key` | True | True | none detected | 5 |
| `archestra_environment` | True | True | none detected | 2 |
| `archestra_identity_provider` | True | True | none detected | 7 |
| `archestra_knowledge_base` | True | True | none detected | 3 |
| `archestra_limit` | True | True | none detected | 4 |
| `archestra_llm_model` | True | True | none detected | 4 |
| `archestra_llm_provider_api_key` | True | True | none detected | 5 |
| `archestra_mcp_gateway` | True | True | none detected | 2 |
| `archestra_mcp_registry_catalog_item` | True | True | none detected | 18 |
| `archestra_mcp_server_installation` | True | True | none detected | 3 |
| `archestra_organization_settings` | True | True | none detected | 8 |
| `archestra_project` | True | True | none detected | 2 |
| `archestra_resource_permission_policy` | True | True | none detected | 1 |
| `archestra_role` | True | True | none detected | 3 |
| `archestra_runtime_credential` | True | True | none detected | 3 |
| `archestra_schedule_trigger` | True | True | none detected | 4 |
| `archestra_team` | True | True | none detected | 2 |
| `archestra_team_external_group` | True | True | none detected | 1 |

Unreferenced mutations are review candidates, not an automatically inferred resource denominator.
RPCs, inference, OAuth flows, logs and transient executions must not become managed resources just to increase a percentage.
