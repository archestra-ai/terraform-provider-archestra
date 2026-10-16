# Provider API inventory

static inventory; no runtime coverage implied

Terraform: 22; Crossplane configured: 5.

Mapping counts are configuration coverage, not successful CRUD or reconciliation coverage.

| Terraform resource | Crossplane configured | Import | Missing direct API calls / retired request fields | Acceptance tests declared |
|---|---|---|---|---|
| `archestra_agent` | True | True | CreateAgent: retired scope, teams, users; UpdateAgent: retired scope, teams, users | 7 |
| `archestra_agent_delegation` | False | True | none detected | 1 |
| `archestra_agent_tool` | False | True | none detected | 1 |
| `archestra_agent_tool_batch` | True | True | none detected | 1 |
| `archestra_agent_tool_exclusions` | False | True | none detected | 1 |
| `archestra_identity_provider` | False | True | none detected | 7 |
| `archestra_limit` | False | True | none detected | 4 |
| `archestra_llm_model` | False | True | UpdateModel: retired teamIds | 4 |
| `archestra_llm_provider_api_key` | False | True | none detected | 5 |
| `archestra_mcp_gateway` | False | True | CreateAgent: retired scope, teams, users; UpdateAgent: retired scope, teams, users | 2 |
| `archestra_mcp_registry_catalog_item` | True | True | CreateInternalMcpCatalogItem: retired scope, teams; UpdateInternalMcpCatalogItem: retired scope, teams | 18 |
| `archestra_mcp_server_installation` | True | True | none detected | 3 |
| `archestra_optimization_rule` | False | True | POST /api/optimization-rules; DELETE /api/optimization-rules/{}; GET /api/optimization-rules; PUT /api/optimization-rules/{} | 4 |
| `archestra_organization_settings` | False | True | PATCH /api/organization/llm-settings | 8 |
| `archestra_runtime_credential` | False | True | none detected | 3 |
| `archestra_team` | False | True | none detected | 5 |
| `archestra_team_external_group` | False | True | none detected | 1 |
| `archestra_tool_invocation_policy` | False | True | POST /api/autonomy-policies/tool-invocation; DELETE /api/autonomy-policies/tool-invocation/{}; GET /api/autonomy-policies/tool-invocation/{}; PUT /api/autonomy-policies/tool-invocation/{} | 4 |
| `archestra_tool_invocation_policy_default` | True | True | POST /api/tool-invocation/bulk-default; DELETE /api/autonomy-policies/tool-invocation/{}; GET /api/autonomy-policies/tool-invocation | 1 |
| `archestra_tool_policy_auto_config` | False | False | POST /api/agent-tools/auto-configure-policies | 1 |
| `archestra_trusted_data_policy` | False | True | POST /api/trusted-data-policies; DELETE /api/trusted-data-policies/{}; GET /api/trusted-data-policies/{}; PUT /api/trusted-data-policies/{} | 3 |
| `archestra_trusted_data_policy_default` | False | True | POST /api/trusted-data-policies/bulk-default; DELETE /api/trusted-data-policies/{}; GET /api/trusted-data-policies | 1 |

Unreferenced mutations are review candidates, not an automatically inferred resource denominator.
RPCs, inference, OAuth flows, logs and transient executions must not become managed resources just to increase a percentage.
