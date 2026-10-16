# Provider API inventory

static inventory; no runtime coverage implied

Terraform: 39; Crossplane configured: 39.

Mapping counts are configuration coverage, not successful CRUD or reconciliation coverage.

| Terraform resource | Crossplane configured | Import | Missing direct API calls / retired request fields | Request fields needing review | Acceptance tests declared |
|---|---|---|---|---|---|
| `archestra_agent` | True | True | none detected | CreateAgent: activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, identityProviderId, isPersonalProxy, organizationId, passthroughHeaders; UpdateAgent: agentType, builtInAgentConfig, deletedAt, identityProviderId, isPersonalProxy, organizationId, passthroughHeaders | 5 |
| `archestra_agent_delegation` | True | True | none detected | none detected | 1 |
| `archestra_agent_hook` | True | True | none detected | CreateHook: agentId | 0 |
| `archestra_agent_knowledge_source_exclusions` | True | True | none detected | none detected | 1 |
| `archestra_agent_remote_delegations` | True | True | none detected | none detected | 0 |
| `archestra_agent_skill_policy` | True | True | none detected | PatchAgentActivationSkillPolicy: discardUnavailable, expectedRevision, operations | 1 |
| `archestra_agent_subagent_exclusions` | True | True | none detected | none detected | 1 |
| `archestra_agent_tool` | True | True | none detected | AssignToolToAgent: resolveAtCallTime | 1 |
| `archestra_agent_tool_batch` | True | True | none detected | BulkAssignTools: assignments | 1 |
| `archestra_agent_tool_exclusions` | True | True | none detected | none detected | 1 |
| `archestra_api_key` | True | True | none detected | none detected | 5 |
| `archestra_default_user_limit` | True | True | none detected | none detected | 0 |
| `archestra_environment` | True | True | none detected | none detected | 2 |
| `archestra_environment_default` | True | True | none detected | UpdateEnvironmentResourceDefaults: agent, app, knowledgeSource, mcpGateway, mcpRegistry | 0 |
| `archestra_identity_provider` | True | True | none detected | none detected | 7 |
| `archestra_knowledge_base` | True | True | none detected | none detected | 4 |
| `archestra_knowledge_directory` | True | True | none detected | none detected | 0 |
| `archestra_limit` | True | True | none detected | CreateLimit: lastCleanup; UpdateLimit: lastCleanup | 4 |
| `archestra_llm_model` | True | True | none detected | none detected | 4 |
| `archestra_llm_provider_api_key` | True | True | none detected | none detected | 5 |
| `archestra_llm_proxy_profile` | True | True | none detected | CreateAgent: activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt; UpdateAgent: agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt | 2 |
| `archestra_mcp_gateway` | True | True | none detected | CreateAgent: activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt; UpdateAgent: agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt | 2 |
| `archestra_mcp_gateway_skill_exclusions` | True | True | none detected | none detected | 0 |
| `archestra_mcp_gateway_skill_publication` | True | True | none detected | none detected | 1 |
| `archestra_mcp_registry_catalog_item` | True | True | none detected | CreateInternalMcpCatalogItem: catalogReinstallRequired, id, oauthConfig, serverType, serverUrl; UpdateInternalMcpCatalogItem: catalogReinstallRequired, hibernationMode, oauthConfig, serverType, serverUrl | 18 |
| `archestra_mcp_server_installation` | True | True | none detected | InstallMcpServer: accessToken, agentIds, catalogId, createdByServiceAccountId, environmentValues, isByosVault, localInstallationError, localInstallationStatus, name, ownerId, reinstallRequired, scope, secretId, teamId, userConfigValues, userId; ReinstallMcpServer: environmentValues, hibernationMode, isByosVault, userConfigValues | 3 |
| `archestra_organization_settings` | True | True | none detected | none detected | 9 |
| `archestra_project` | True | True | none detected | SetProjectInstructions: content | 2 |
| `archestra_remote_agent` | True | True | none detected | none detected | 3 |
| `archestra_resource_permission_policy` | True | True | none detected | UpdateResourcePermissions: grants, revision | 1 |
| `archestra_role` | True | True | none detected | none detected | 3 |
| `archestra_runtime_credential` | True | True | none detected | SetOrganizationRuntimeCredentialConnection: value | 3 |
| `archestra_schedule_trigger` | True | True | none detected | none detected | 4 |
| `archestra_service_account` | True | True | none detected | none detected | 2 |
| `archestra_service_account_token` | True | True | none detected | none detected | 0 |
| `archestra_skill` | True | True | none detected | CreateSkill: allowedTools; UpdateSkill: allowedTools, baseVersion | 1 |
| `archestra_team` | True | True | none detected | AddTeamMember: role, userId; UpdateTeamMember: role | 2 |
| `archestra_team_external_group` | True | True | none detected | AddTeamExternalGroup: groupIdentifier | 1 |
| `archestra_team_vault_folder` | True | True | none detected | none detected | 0 |

Unreferenced mutations are review candidates, not an automatically inferred resource denominator.
RPCs, inference, OAuth flows, logs and transient executions must not become managed resources just to increase a percentage.
Request fields without a declared mapping require review: derived wire values and backend-owned fields need explicit justification; a clean GET-field gate alone does not prove write coverage.
