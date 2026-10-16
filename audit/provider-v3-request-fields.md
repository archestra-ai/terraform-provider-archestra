# Provider API inventory

static inventory; no runtime coverage implied

Terraform: 26; Crossplane configured: 26.

Mapping counts are configuration coverage, not successful CRUD or reconciliation coverage.

| Terraform resource | Crossplane configured | Import | Missing direct API calls / retired request fields | Request fields needing review | Acceptance tests declared |
|---|---|---|---|---|---|
| `archestra_agent` | True | True | CreateAgent: retired scope, teams, users; UpdateAgent: retired scope, teams, users | CreateAgent: activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, identityProviderId, initialGrants, isPersonalProxy, organizationId, passthroughHeaders; UpdateAgent: agentType, builtInAgentConfig, deletedAt, identityProviderId, isPersonalProxy, organizationId, passthroughHeaders | 5 |
| `archestra_agent_delegation` | True | True | none detected | SyncAgentDelegations: targetAgentIds | 1 |
| `archestra_agent_skill_policy` | True | True | none detected | PatchAgentActivationSkillPolicy: discardUnavailable, expectedRevision, operations | 1 |
| `archestra_agent_subagent_exclusions` | True | True | none detected | none detected | 1 |
| `archestra_agent_tool` | True | True | none detected | AssignToolToAgent: resolveAtCallTime | 1 |
| `archestra_agent_tool_batch` | True | True | none detected | BulkAssignTools: assignments | 1 |
| `archestra_agent_tool_exclusions` | True | True | none detected | none detected | 1 |
| `archestra_api_key` | True | True | none detected | none detected | 5 |
| `archestra_environment` | True | True | CreateEnvironment: retired restricted; UpdateEnvironment: retired restricted | none detected | 2 |
| `archestra_identity_provider` | True | True | none detected | none detected | 7 |
| `archestra_knowledge_base` | True | True | UpdateKnowledgeBase: retired teamIds, visibility | CreateKnowledgeBase: initialGrants | 4 |
| `archestra_limit` | True | True | none detected | CreateLimit: lastCleanup; UpdateLimit: lastCleanup | 4 |
| `archestra_llm_model` | True | True | UpdateModel: retired teamIds | none detected | 4 |
| `archestra_llm_provider_api_key` | True | True | none detected | CreateLlmProviderApiKey: initialGrants | 5 |
| `archestra_mcp_gateway` | True | True | CreateAgent: retired scope, teams, users; UpdateAgent: retired scope, teams, users | CreateAgent: accessAllSubagents, activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, initialGrants, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt; UpdateAgent: accessAllSubagents, agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt | 2 |
| `archestra_mcp_registry_catalog_item` | True | True | CreateInternalMcpCatalogItem: retired scope, teams; UpdateInternalMcpCatalogItem: retired scope, teams | CreateInternalMcpCatalogItem: catalogReinstallRequired, id, initialGrants, oauthConfig, serverType, serverUrl; UpdateInternalMcpCatalogItem: catalogReinstallRequired, hibernationMode, oauthConfig, serverType, serverUrl | 18 |
| `archestra_mcp_server_installation` | True | True | none detected | InstallMcpServer: accessToken, agentIds, catalogId, createdByServiceAccountId, environmentValues, isByosVault, localInstallationError, localInstallationStatus, name, ownerId, reinstallRequired, scope, secretId, teamId, userConfigValues, userId | 3 |
| `archestra_organization_settings` | True | True | UpdateConnectionSettings: retired connectionShownProviders; UpdateDefaultEnvironment: retired restricted | none detected | 9 |
| `archestra_project` | True | True | none detected | CreateProject: initialGrants | 2 |
| `archestra_resource_permission_policy` | True | True | none detected | UpdateResourcePermissions: grants, revision | 1 |
| `archestra_role` | True | True | none detected | none detected | 3 |
| `archestra_runtime_credential` | True | True | none detected | CreateRuntimeCredential: initialGrants; SetOrganizationRuntimeCredentialConnection: value | 3 |
| `archestra_schedule_trigger` | True | True | none detected | none detected | 4 |
| `archestra_skill` | True | True | none detected | CreateSkill: allowedTools, initialGrants; UpdateSkill: allowedTools, baseVersion | 1 |
| `archestra_team` | True | True | none detected | AddTeamMember: role, userId; UpdateTeamMember: role | 2 |
| `archestra_team_external_group` | True | True | none detected | AddTeamExternalGroup: groupIdentifier | 1 |

Unreferenced mutations are review candidates, not an automatically inferred resource denominator.
RPCs, inference, OAuth flows, logs and transient executions must not become managed resources just to increase a percentage.
Request fields without a declared mapping require review: derived wire values and backend-owned fields need explicit justification; a clean GET-field gate alone does not prove write coverage.
