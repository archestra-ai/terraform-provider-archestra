# Request field review

All 90 raw occurrences from the b180fee baseline have an explicit decision. Repeated create/update fields remain separate audit rows. Static mappings alone neither prove missing capability nor runtime behavior.

| Classification | Occurrences |
|---|---:|
| authentication_context | 4 |
| derived | 5 |
| implemented_gap | 3 |
| legacy_alias | 1 |
| lifecycle_owned | 4 |
| manual_encoding | 30 |
| other_subtype | 22 |
| related_resource | 3 |
| server_owned | 18 |

New justified fields found in this review: gateway `access_all_subagents` (two baseline occurrences), personal installation `user_id` (one), and installation `hibernation_mode` (a separate previously unreferenced reinstall operation, outside the 90-row baseline). Lifecycle results are recorded separately.

| Resource / operation | Field | Decision | Rationale |
|---|---|---|---|
| `archestra_agent` / CreateAgent | `activationSkillPolicy` | related_resource | archestra_agent_skill_policy owns activation policy and revision semantics. Gateway/profile rows reject an activation policy. |
| `archestra_agent` / CreateAgent | `agentType` | derived | The resource constructor selects the agent-table subtype; changing subtype would change the managed identity. |
| `archestra_agent` / CreateAgent | `builtInAgentConfig` | server_owned | The create route strips this trust attribute; ordinary updates ignore it. Existing built-in configuration is reported as computed metadata. |
| `archestra_agent` / CreateAgent | `deletedAt` | lifecycle_owned | Deletion and restoration own soft-delete timestamps; setting a timestamp through a generic update is not a separate declarative capability. |
| `archestra_agent` / CreateAgent | `identityProviderId` | other_subtype | Inbound gateway JWT authentication and forwarded client headers belong to external gateway/profile resources, not internal execution agents. |
| `archestra_agent` / CreateAgent | `isPersonalProxy` | server_owned | Legacy platform-owned personal LLM proxy marker; generic proxy CRUD is rejected and the managed profile resource does not create singleton proxies. |
| `archestra_agent` / CreateAgent | `organizationId` | authentication_context | Organization identity comes from the authenticated provider connection; cross-organization writes are not a resource selector. |
| `archestra_agent` / CreateAgent | `passthroughHeaders` | other_subtype | Inbound gateway JWT authentication and forwarded client headers belong to external gateway/profile resources, not internal execution agents. |
| `archestra_agent` / UpdateAgent | `agentType` | derived | The resource constructor selects the agent-table subtype; changing subtype would change the managed identity. |
| `archestra_agent` / UpdateAgent | `builtInAgentConfig` | server_owned | The create route strips this trust attribute; ordinary updates ignore it. Existing built-in configuration is reported as computed metadata. |
| `archestra_agent` / UpdateAgent | `deletedAt` | lifecycle_owned | Deletion and restoration own soft-delete timestamps; setting a timestamp through a generic update is not a separate declarative capability. |
| `archestra_agent` / UpdateAgent | `identityProviderId` | other_subtype | Inbound gateway JWT authentication and forwarded client headers belong to external gateway/profile resources, not internal execution agents. |
| `archestra_agent` / UpdateAgent | `isPersonalProxy` | server_owned | Legacy platform-owned personal LLM proxy marker; generic proxy CRUD is rejected and the managed profile resource does not create singleton proxies. |
| `archestra_agent` / UpdateAgent | `organizationId` | authentication_context | Organization identity comes from the authenticated provider connection; cross-organization writes are not a resource selector. |
| `archestra_agent` / UpdateAgent | `passthroughHeaders` | other_subtype | Inbound gateway JWT authentication and forwarded client headers belong to external gateway/profile resources, not internal execution agents. |
| `archestra_agent_skill_policy` / PatchAgentActivationSkillPolicy | `discardUnavailable` | manual_encoding | The revision-checked operation builder derives incremental operations and expectedRevision from current state; discardUnavailable is a deliberate repair choice, not persisted policy. |
| `archestra_agent_skill_policy` / PatchAgentActivationSkillPolicy | `expectedRevision` | manual_encoding | The revision-checked operation builder derives incremental operations and expectedRevision from current state; discardUnavailable is a deliberate repair choice, not persisted policy. |
| `archestra_agent_skill_policy` / PatchAgentActivationSkillPolicy | `operations` | manual_encoding | The revision-checked operation builder derives incremental operations and expectedRevision from current state; discardUnavailable is a deliberate repair choice, not persisted policy. |
| `archestra_agent_tool` / AssignToolToAgent | `resolveAtCallTime` | legacy_alias | The backend normalizes this legacy boolean into supported credential_resolution_mode=dynamic/static and never stores or returns the alias. |
| `archestra_agent_tool_batch` / BulkAssignTools | `assignments` | manual_encoding | The resource expands installation tools into the assignments request from the configured installation and credential mode. |
| `archestra_limit` / CreateLimit | `lastCleanup` | server_owned | Cleanup scheduler progress is observed metadata; callers must not reset accounting through configuration. |
| `archestra_limit` / UpdateLimit | `lastCleanup` | server_owned | Cleanup scheduler progress is observed metadata; callers must not reset accounting through configuration. |
| `archestra_mcp_gateway` / CreateAgent | `accessAllSubagents` | implemented_gap | Gateway callers can delegate dynamically. Added access_all_subagents with drift-honest reads and an in-place acceptance regression. |
| `archestra_mcp_gateway` / CreateAgent | `activationSkillPolicy` | related_resource | archestra_agent_skill_policy owns activation policy and revision semantics. Gateway/profile rows reject an activation policy. |
| `archestra_mcp_gateway` / CreateAgent | `agentType` | derived | The resource constructor selects the agent-table subtype; changing subtype would change the managed identity. |
| `archestra_mcp_gateway` / CreateAgent | `builtInAgentConfig` | server_owned | The create route strips this trust attribute; ordinary updates ignore it. Existing built-in configuration is reported as computed metadata. |
| `archestra_mcp_gateway` / CreateAgent | `deletedAt` | lifecycle_owned | Deletion and restoration own soft-delete timestamps; setting a timestamp through a generic update is not a separate declarative capability. |
| `archestra_mcp_gateway` / CreateAgent | `incomingEmailAllowedDomain` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `incomingEmailEnabled` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `incomingEmailSecurityMode` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `isPersonalProxy` | server_owned | Legacy platform-owned personal LLM proxy marker; generic proxy CRUD is rejected and the managed profile resource does not create singleton proxies. |
| `archestra_mcp_gateway` / CreateAgent | `llmApiKeyId` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `llmModel` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `modelId` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `organizationId` | authentication_context | Organization identity comes from the authenticated provider connection; cross-organization writes are not a resource selector. |
| `archestra_mcp_gateway` / CreateAgent | `runtime` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `suggestedPrompts` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / CreateAgent | `systemPrompt` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `accessAllSubagents` | implemented_gap | Gateway callers can delegate dynamically. Added access_all_subagents with drift-honest reads and an in-place acceptance regression. |
| `archestra_mcp_gateway` / UpdateAgent | `agentType` | derived | The resource constructor selects the agent-table subtype; changing subtype would change the managed identity. |
| `archestra_mcp_gateway` / UpdateAgent | `builtInAgentConfig` | server_owned | The create route strips this trust attribute; ordinary updates ignore it. Existing built-in configuration is reported as computed metadata. |
| `archestra_mcp_gateway` / UpdateAgent | `deletedAt` | lifecycle_owned | Deletion and restoration own soft-delete timestamps; setting a timestamp through a generic update is not a separate declarative capability. |
| `archestra_mcp_gateway` / UpdateAgent | `incomingEmailAllowedDomain` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `incomingEmailEnabled` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `incomingEmailSecurityMode` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `isPersonalProxy` | server_owned | Legacy platform-owned personal LLM proxy marker; generic proxy CRUD is rejected and the managed profile resource does not create singleton proxies. |
| `archestra_mcp_gateway` / UpdateAgent | `llmApiKeyId` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `llmModel` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `modelId` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `organizationId` | authentication_context | Organization identity comes from the authenticated provider connection; cross-organization writes are not a resource selector. |
| `archestra_mcp_gateway` / UpdateAgent | `runtime` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `suggestedPrompts` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_gateway` / UpdateAgent | `systemPrompt` | other_subtype | Internal execution, prompt, model and email-invocation controls belong to archestra_agent; a gateway does not run inference or an agent runtime. |
| `archestra_mcp_registry_catalog_item` / CreateInternalMcpCatalogItem | `catalogReinstallRequired` | server_owned | Runtime reinstall bookkeeping reports whether installations need a refresh; users manage configuration rather than forcing diagnostic state. |
| `archestra_mcp_registry_catalog_item` / CreateInternalMcpCatalogItem | `id` | server_owned | The create route generates catalog identity; import adopts an existing UUID instead of assigning a new one. |
| `archestra_mcp_registry_catalog_item` / CreateInternalMcpCatalogItem | `oauthConfig` | manual_encoding | Typed local_config/remote_config blocks encode and read these top-level wire values; server type is inferred from the selected configuration. |
| `archestra_mcp_registry_catalog_item` / CreateInternalMcpCatalogItem | `serverType` | manual_encoding | Typed local_config/remote_config blocks encode and read these top-level wire values; server type is inferred from the selected configuration. |
| `archestra_mcp_registry_catalog_item` / CreateInternalMcpCatalogItem | `serverUrl` | manual_encoding | Typed local_config/remote_config blocks encode and read these top-level wire values; server type is inferred from the selected configuration. |
| `archestra_mcp_registry_catalog_item` / UpdateInternalMcpCatalogItem | `catalogReinstallRequired` | server_owned | Runtime reinstall bookkeeping reports whether installations need a refresh; users manage configuration rather than forcing diagnostic state. |
| `archestra_mcp_registry_catalog_item` / UpdateInternalMcpCatalogItem | `hibernationMode` | related_resource | This catalog command cascades onto every installation and is not a readable catalog property. The new installation hibernation_mode manages the persisted override with explicit ownership. |
| `archestra_mcp_registry_catalog_item` / UpdateInternalMcpCatalogItem | `oauthConfig` | manual_encoding | Typed local_config/remote_config blocks encode and read these top-level wire values; server type is inferred from the selected configuration. |
| `archestra_mcp_registry_catalog_item` / UpdateInternalMcpCatalogItem | `serverType` | manual_encoding | Typed local_config/remote_config blocks encode and read these top-level wire values; server type is inferred from the selected configuration. |
| `archestra_mcp_registry_catalog_item` / UpdateInternalMcpCatalogItem | `serverUrl` | manual_encoding | Typed local_config/remote_config blocks encode and read these top-level wire values; server type is inferred from the selected configuration. |
| `archestra_mcp_server_installation` / InstallMcpServer | `accessToken` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `agentIds` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `catalogId` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `createdByServiceAccountId` | server_owned | The route derives creator and owner from the authenticated identity and validated userId/scope; raw ownership bookkeeping is not independently configurable. |
| `archestra_mcp_server_installation` / InstallMcpServer | `environmentValues` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `isByosVault` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `localInstallationError` | server_owned | Installation workers own progress/error/reinstall status; desired configuration is supplied through the installation and reinstall inputs. |
| `archestra_mcp_server_installation` / InstallMcpServer | `localInstallationStatus` | server_owned | Installation workers own progress/error/reinstall status; desired configuration is supplied through the installation and reinstall inputs. |
| `archestra_mcp_server_installation` / InstallMcpServer | `name` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `ownerId` | server_owned | The route derives creator and owner from the authenticated identity and validated userId/scope; raw ownership bookkeeping is not independently configurable. |
| `archestra_mcp_server_installation` / InstallMcpServer | `reinstallRequired` | server_owned | Installation workers own progress/error/reinstall status; desired configuration is supplied through the installation and reinstall inputs. |
| `archestra_mcp_server_installation` / InstallMcpServer | `scope` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `secretId` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `teamId` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `userConfigValues` | manual_encoding | The generated install request and scope wrapper explicitly encode this existing schema field; secret inputs remain sensitive and write-only. |
| `archestra_mcp_server_installation` / InstallMcpServer | `userId` | implemented_gap | Added immutable user_id for administrator-selected personal installation ownership, read from the persisted owner. |
| `archestra_project` / SetProjectInstructions | `content` | manual_encoding | The instructions attribute writes the separate content endpoint and preserves omitted instructions. |
| `archestra_resource_permission_policy` / UpdateResourcePermissions | `grants` | manual_encoding | The policy converts principal/action grants and supplies the current optimistic concurrency revision. |
| `archestra_resource_permission_policy` / UpdateResourcePermissions | `revision` | manual_encoding | The policy converts principal/action grants and supplies the current optimistic concurrency revision. |
| `archestra_runtime_credential` / SetOrganizationRuntimeCredentialConnection | `value` | manual_encoding | The sensitive organization_value attribute owns the separate connection value endpoint. |
| `archestra_skill` / CreateSkill | `allowedTools` | manual_encoding | allowed_tools is rendered into canonical manifest frontmatter and read back from the versioned skill. |
| `archestra_skill` / UpdateSkill | `allowedTools` | manual_encoding | allowed_tools is rendered into canonical manifest frontmatter and read back from the versioned skill. |
| `archestra_skill` / UpdateSkill | `baseVersion` | derived | Version concurrency is derived from the refreshed latest_version; users cannot select a stale write revision. |
| `archestra_team` / AddTeamMember | `role` | manual_encoding | The team membership map supplies user identity and role to dedicated member mutation endpoints. |
| `archestra_team` / AddTeamMember | `userId` | manual_encoding | The team membership map supplies user identity and role to dedicated member mutation endpoints. |
| `archestra_team` / UpdateTeamMember | `role` | manual_encoding | The team membership map supplies user identity and role to dedicated member mutation endpoints. |
| `archestra_team_external_group` / AddTeamExternalGroup | `groupIdentifier` | manual_encoding | group_identifier is explicitly passed in the dedicated external-group request. |
