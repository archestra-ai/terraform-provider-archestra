# Provider API inventory

static inventory; no runtime coverage implied

Terraform: 29; Crossplane configured: 29.

Mapping counts are configuration coverage, not successful CRUD or reconciliation coverage.

| Terraform resource | Crossplane configured | Import | Missing direct API calls / retired request fields | Request fields needing review | Acceptance tests declared |
|---|---|---|---|---|---|
| `archestra_agent` | True | True | none detected | BulkAssignTools: assignments; CreateAgent: activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, identityProviderId, isPersonalProxy, organizationId, passthroughHeaders; CreateHook: agentId, content, enabled, event, fileName, requirements; PatchAgentActivationSkillPolicy: discardUnavailable, expectedRevision, mode, operations; SyncAgentA2aDelegations: connectionIds; SyncAgentDelegations: targetAgentIds; UpdateAgent: agentType, builtInAgentConfig, deletedAt, identityProviderId, isPersonalProxy, organizationId, passthroughHeaders; UpdateAgentKnowledgeSourceExclusions: excludedConnectorIds; UpdateAgentSubagentExclusions: excludedSubagentIds; UpdateAgentToolExclusions: excludedToolIds; UpdateHook: content, enabled, event, fileName, requirements; UpdateResourcePermissions: grants, revision | 5 |
| `archestra_api_key` | True | True | none detected | none detected | 5 |
| `archestra_credential` | True | True | none detected | SetOrganizationRuntimeCredentialConnection: value; UpdateResourcePermissions: grants, revision | 3 |
| `archestra_environment` | True | True | none detected | UpdateResourcePermissions: grants, revision | 2 |
| `archestra_github_app` | True | True | none detected | none detected | 0 |
| `archestra_github_pat` | True | True | none detected | none detected | 0 |
| `archestra_identity_provider` | True | True | none detected | none detected | 7 |
| `archestra_knowledge_base` | True | True | none detected | UpdateResourcePermissions: grants, revision | 4 |
| `archestra_knowledge_directory` | True | True | none detected | none detected | 0 |
| `archestra_limit` | True | True | none detected | CreateLimit: lastCleanup; UpdateLimit: lastCleanup | 4 |
| `archestra_mcp_gateway` | True | True | none detected | BulkAssignTools: assignments; CreateAgent: activationSkillPolicy, agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt; SyncAgentA2aDelegations: connectionIds; SyncAgentDelegations: targetAgentIds; UpdateAgent: agentType, builtInAgentConfig, deletedAt, incomingEmailAllowedDomain, incomingEmailEnabled, incomingEmailSecurityMode, isPersonalProxy, llmApiKeyId, llmModel, modelId, organizationId, runtime, suggestedPrompts, systemPrompt; UpdateAgentKnowledgeSourceExclusions: excludedConnectorIds; UpdateAgentSkillExclusions: excludedSkillIds; UpdateAgentSkills: accessAllSkills, skillIds; UpdateAgentSubagentExclusions: excludedSubagentIds; UpdateAgentToolExclusions: excludedToolIds; UpdateResourcePermissions: grants, revision | 2 |
| `archestra_mcp_registry_entry` | True | True | none detected | CreateInternalMcpCatalogItem: catalogReinstallRequired, id, oauthConfig, serverType, serverUrl; UpdateInternalMcpCatalogItem: catalogReinstallRequired, hibernationMode, oauthConfig, serverType, serverUrl; UpdateResourcePermissions: grants, revision | 18 |
| `archestra_mcp_server` | True | True | none detected | InstallMcpServer: accessToken, agentIds, catalogId, createdByServiceAccountId, environmentValues, isByosVault, localInstallationError, localInstallationStatus, name, ownerId, reinstallRequired, scope, secretId, teamId, userConfigValues, userId; ReinstallMcpServer: environmentValues, hibernationMode, isByosVault, userConfigValues | 3 |
| `archestra_model` | True | True | none detected | UpdateResourcePermissions: grants, revision | 4 |
| `archestra_model_provider` | True | True | none detected | UpdateResourcePermissions: grants, revision | 5 |
| `archestra_oauth_client` | True | True | none detected | UpdateResourcePermissions: grants, revision | 0 |
| `archestra_organization_settings` | True | True | none detected | CreateDefaultUserLimit: cleanupInterval, environmentId, limitValue, model; UpdateDefaultUserLimit: cleanupInterval, limitValue, model; UpdateEnvironmentResourceDefaults: agent, app, knowledgeSource, mcpGateway, mcpRegistry; UpdateResourcePermissions: grants, revision | 9 |
| `archestra_plugin` | True | True | none detected | ImportGithubPlugin: approvedCommitSha, exclude, githubAppConfigId, githubPatId, githubToken, gitlabTokenId, provider, ref, repoUrl, subdir, trackingRef; UpdateResourcePermissions: grants, revision | 2 |
| `archestra_project` | True | True | none detected | SetProjectInstructions: content; UpdateResourcePermissions: grants, revision | 2 |
| `archestra_project_file` | True | True | none detected | UpdateSkillSandboxArtifactContent: content | 4 |
| `archestra_remote_agent` | True | True | none detected | UpdateResourcePermissions: grants, revision | 2 |
| `archestra_role` | True | True | none detected | none detected | 3 |
| `archestra_schedule` | True | True | none detected | none detected | 4 |
| `archestra_service_account` | True | True | none detected | UpdateResourcePermissions: grants, revision | 3 |
| `archestra_service_account_api_key` | True | True | none detected | none detected | 0 |
| `archestra_share_link` | True | True | none detected | none detected | 3 |
| `archestra_skill` | True | True | none detected | CreateSkill: allowedTools; ImportGithubSkills: githubAppConfigId, githubPatId, githubToken, gitlabTokenId, path, provider, repoUrl, skillPaths, sync; UpdateResourcePermissions: grants, revision; UpdateSkill: allowedTools, baseVersion; UpdateSkillGithubSync: disconnect, interval, source, syncNow | 2 |
| `archestra_team` | True | True | none detected | AddTeamExternalGroup: groupIdentifier; AddTeamMember: role, userId; SetTeamVaultFolder: vaultPath; UpdateTeamMember: role | 2 |
| `archestra_virtual_key` | True | True | none detected | UpdateResourcePermissions: grants, revision | 0 |

Unreferenced mutations are review candidates, not an automatically inferred resource denominator.
RPCs, inference, OAuth flows, logs and transient executions must not become managed resources just to increase a percentage.
Request fields without a declared mapping require review: derived wire values and backend-owned fields need explicit justification; a clean GET-field gate alone does not prove write coverage.
