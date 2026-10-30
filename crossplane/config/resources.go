package config

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

const agentGroup = "agent"
const mcpGroup = "mcp"
const credentialGroup = "credential"

// ConfigureResources applies the same API names and references in both scopes.
func ConfigureResources(p *ujconfig.Provider) {
	p.AddResourceConfigurator("archestra_app", func(r *ujconfig.Resource) {
		r.ShortGroup = "app"
		r.Kind = "App"
		r.References["environment_id"] = ujconfig.Reference{TerraformName: "archestra_environment"}
		r.References["tools.mcp_server_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_server"}
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "html", "slug", "browser_permissions", "enabled", "locked", "open_in_fullscreen", "tools", "permissions", "labels", "environment_id", "description", "icon")
	})

	p.AddResourceConfigurator("archestra_knowledge_file", func(r *ujconfig.Resource) {
		r.ShortGroup = "knowledge"
		r.Kind = "File"
		r.References["directory_id"] = ujconfig.Reference{TerraformName: "archestra_knowledge_directory"}
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "permissions", "labels", "directory_id")
	})

	p.AddResourceConfigurator("archestra_organization_settings", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "environment_defaults", "default_user_limits", "default_permissions")
		for _, name := range []string{"agents", "mcp_gateways", "mcp_registry", "apps", "knowledge_connectors"} {
			r.References["environment_defaults."+name] = ujconfig.Reference{TerraformName: "archestra_environment"}
		}
	})

	p.AddResourceConfigurator("archestra_project_file", func(r *ujconfig.Resource) {
		r.ShortGroup = "project"
		r.Kind = "File"
		r.References["project_id"] = ujconfig.Reference{TerraformName: "archestra_project"}
	})
	p.AddResourceConfigurator("archestra_share_link", func(r *ujconfig.Resource) {
		r.ShortGroup = "marketplace"
		r.Kind = "ShareLink"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "name", "plugin_platform", "expires_at", "rotation_nonce")
		r.References["skill_ids"] = ujconfig.Reference{TerraformName: "archestra_skill"}
		r.References["plugin_ids"] = ujconfig.Reference{TerraformName: "archestra_plugin"}
	})
	p.AddResourceConfigurator("archestra_plugin", func(r *ujconfig.Resource) {
		r.ShortGroup = "plugin"
		r.Kind = "Plugin"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "labels", "git_source", "files")
		r.References["git_source.github_app_id"] = ujconfig.Reference{TerraformName: "archestra_github_app"}
		r.References["git_source.github_pat_id"] = ujconfig.Reference{TerraformName: "archestra_github_pat"}
		r.References["git_source.gitlab_token_id"] = ujconfig.Reference{TerraformName: "archestra_credential", Extractor: `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("definition_id", true)`}
	})
	p.AddResourceConfigurator("archestra_github_pat", func(r *ujconfig.Resource) { r.ShortGroup = credentialGroup; r.Kind = "GitHubPAT" })
	p.AddResourceConfigurator("archestra_github_app", func(r *ujconfig.Resource) { r.ShortGroup = credentialGroup; r.Kind = "GitHubApp" })
	p.AddResourceConfigurator("archestra_virtual_key", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "VirtualKey"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "billing_team_id", "spend_cap", "expires_at", "provider_api_keys", "labels")
		r.References["billing_team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
		r.References["provider_api_keys.provider_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_model_provider"}
	})
	p.AddResourceConfigurator("archestra_oauth_client", func(r *ujconfig.Resource) {
		r.ShortGroup = credentialGroup
		r.Kind = "OAuthClient"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "service_account_id", "billing_team_id", "spend_cap", "provider_api_keys", "labels", "allowed_gateway_ids", "redirect_uris", "model_pool_ids")
		r.References["service_account_id"] = ujconfig.Reference{TerraformName: "archestra_service_account"}
		r.References["billing_team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
		r.References["provider_api_keys.provider_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_model_provider"}
	})
	p.AddResourceConfigurator("archestra_remote_agent", func(r *ujconfig.Resource) { r.ShortGroup = "a2a"; r.Kind = "RemoteAgent" })

	for _, name := range []string{"archestra_environment", "archestra_model", "archestra_plugin", "archestra_virtual_key", "archestra_oauth_client", "archestra_remote_agent", "archestra_service_account", "archestra_agent", "archestra_mcp_gateway", "archestra_project", "archestra_knowledge_base", "archestra_mcp_registry_entry", "archestra_skill", "archestra_credential", "archestra_model_provider"} {
		p.AddResourceConfigurator(name, func(r *ujconfig.Resource) {
			// Leave sharing unmanaged when omitted; forProvider manages ongoing sharing.
			r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "permissions")
		})
	}
	p.AddResourceConfigurator("archestra_skill", func(r *ujconfig.Resource) {
		r.ShortGroup = "skill"
		r.Kind = "Skill"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "git_source", "name", "description", "content", "license", "compatibility", "agent_name", "templated", "metadata", "allowed_tools", "files")
		r.References["git_source.github_app_id"] = ujconfig.Reference{TerraformName: "archestra_github_app"}
		r.References["git_source.github_pat_id"] = ujconfig.Reference{TerraformName: "archestra_github_pat"}
		r.References["git_source.gitlab_token_id"] = ujconfig.Reference{TerraformName: "archestra_credential", Extractor: `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("definition_id", true)`}
		r.References["environment_ids"] = ujconfig.Reference{TerraformName: "archestra_environment"}
	})
	p.AddResourceConfigurator("archestra_project", func(r *ujconfig.Resource) {
		r.ShortGroup = "project"
		r.Kind = "Project"
		r.References["default_agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["app_ids"] = ujconfig.Reference{TerraformName: "archestra_app"}
		// Omitted app links stay unmanaged instead of being adopted into forProvider.
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "app_ids")
	})
	p.AddResourceConfigurator("archestra_knowledge_base", func(r *ujconfig.Resource) { r.ShortGroup = "knowledge"; r.Kind = "KnowledgeBase" })
	p.AddResourceConfigurator("archestra_role", func(r *ujconfig.Resource) { r.ShortGroup = "identity"; r.Kind = "Role" })
	p.AddResourceConfigurator("archestra_api_key", func(r *ujconfig.Resource) { r.ShortGroup = credentialGroup; r.Kind = "APIKey" })
	p.AddResourceConfigurator("archestra_schedule", func(r *ujconfig.Resource) {
		r.ShortGroup = "schedule"
		r.Kind = "Schedule"
		r.References["agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["project_id"] = ujconfig.Reference{TerraformName: "archestra_project"}
	})

	p.AddResourceConfigurator("archestra_knowledge_directory", func(r *ujconfig.Resource) { r.ShortGroup = "knowledge"; r.Kind = "Directory" })
	p.AddResourceConfigurator("archestra_service_account", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "team_id")
		r.ShortGroup = "identity"
		r.Kind = "ServiceAccount"
		r.References["team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
	})
	p.AddResourceConfigurator("archestra_service_account_api_key", func(r *ujconfig.Resource) {
		r.ShortGroup = credentialGroup
		r.Kind = "ServiceAccountAPIKey"
		r.References["service_account_id"] = ujconfig.Reference{TerraformName: "archestra_service_account"}
	})
	p.AddResourceConfigurator("archestra_environment", func(r *ujconfig.Resource) { r.ShortGroup = "environment"; r.Kind = "Environment" })
	p.AddResourceConfigurator("archestra_agent", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "subagent_ids", "remote_agent_ids", "excluded_subagent_ids", "excluded_connector_ids", "skills", "hooks", "tools", "excluded_tool_ids", "model_pool_id", "model_id", "llm_api_key_id")
		r.References["tools.mcp_server_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_server"}
		r.References["subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["excluded_subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["remote_agent_ids"] = ujconfig.Reference{TerraformName: "archestra_remote_agent"}
		r.References["skills.allowed.skill_id"] = ujconfig.Reference{TerraformName: "archestra_skill"}
		r.References["skills.excluded.skill_id"] = ujconfig.Reference{TerraformName: "archestra_skill"}

		r.ShortGroup = agentGroup
		r.Kind = "Agent"
		r.References["knowledge_base_ids"] = ujconfig.Reference{TerraformName: "archestra_knowledge_base"}
		r.References["environment_id"] = ujconfig.Reference{TerraformName: "archestra_environment"}
		r.References["llm_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_model_provider"}
	})
	p.AddResourceConfigurator("archestra_identity_provider", func(r *ujconfig.Resource) {
		r.ShortGroup = "identity"
		r.Kind = "Provider"
	})
	p.AddResourceConfigurator("archestra_limit", func(r *ujconfig.Resource) {
		r.ShortGroup = "limit"
		r.Kind = "Limit"
	})
	p.AddResourceConfigurator("archestra_model_pool", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "ModelPool"
		r.References["members.provider_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_model_provider"}
	})
	p.AddResourceConfigurator("archestra_model", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "Model"
	})
	p.AddResourceConfigurator("archestra_model_provider", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "ModelProvider"
	})
	p.AddResourceConfigurator("archestra_mcp_gateway", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "subagent_ids", "remote_agent_ids", "excluded_subagent_ids", "excluded_connector_ids", "tools", "excluded_tool_ids", "skills")
		r.References["subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["remote_agent_ids"] = ujconfig.Reference{TerraformName: "archestra_remote_agent"}
		r.References["excluded_subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["tools.mcp_server_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_server"}
		r.References["skills.ids"] = ujconfig.Reference{TerraformName: "archestra_skill"}
		r.References["skills.excluded_ids"] = ujconfig.Reference{TerraformName: "archestra_skill"}

		// Keep gateway-to-agent references separate from agent-to-MCP-server references.
		r.ShortGroup = "gateway"
		r.Kind = "Gateway"
		r.References["identity_provider_id"] = ujconfig.Reference{TerraformName: "archestra_identity_provider"}
	})
	p.AddResourceConfigurator("archestra_mcp_registry_entry", func(r *ujconfig.Resource) {
		r.ShortGroup = mcpGroup
		r.Kind = "RegistryEntry"
	})
	p.AddResourceConfigurator("archestra_mcp_server", func(r *ujconfig.Resource) {
		r.ShortGroup = mcpGroup
		r.Kind = "Server"
		r.References["catalog_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_registry_entry"}
	})
	p.AddResourceConfigurator("archestra_organization_settings", func(r *ujconfig.Resource) {
		r.ShortGroup = "organization"
		r.Kind = "Settings"
	})

	p.AddResourceConfigurator("archestra_credential", func(r *ujconfig.Resource) {
		r.ShortGroup = credentialGroup
		r.Kind = "Credential"
	})
	p.AddResourceConfigurator("archestra_team", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "external_group_ids", "roles")
		r.ShortGroup = "team"
		r.Kind = "Team"
		r.References["parent_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
		r.References["roles"] = ujconfig.Reference{TerraformName: "archestra_role", Extractor: roleSlugExtractor}
	})
	configureAdditionalRelationships(p)
}

const roleSlugExtractor = `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("role", true)`

func configureAdditionalRelationships(p *ujconfig.Provider) {
	for owner, targets := range map[string]map[string]string{
		"archestra_agent": {
			"model_pool_id": "archestra_model_pool", "model_id": "archestra_model",
			"fast_model_id": "archestra_model", "smart_model_id": "archestra_model",
			"skills.allowed.mcp_server_id": "archestra_mcp_server", "skills.excluded.mcp_server_id": "archestra_mcp_server",
			"skills.allowed.plugin_id": "archestra_plugin", "skills.excluded.plugin_id": "archestra_plugin",
		},
		"archestra_mcp_gateway": {
			"environment_id": "archestra_environment", "knowledge_base_ids": "archestra_knowledge_base",
		},
		"archestra_mcp_registry_entry": {
			"environment_id": "archestra_environment", "dynamic_connection_mcp_server_id": "archestra_mcp_server",
			"enterprise_managed_config.identity_provider_id": "archestra_identity_provider",
		},
		"archestra_mcp_server":   {"team_id": "archestra_team"},
		"archestra_oauth_client": {"model_pool_ids": "archestra_model_pool"},
		"archestra_organization_settings": {
			"default_agent_id": "archestra_agent", "default_llm_api_key_id": "archestra_model_provider",
			"default_model_id": "archestra_model", "default_fast_model_id": "archestra_model", "default_smart_model_id": "archestra_model",
			"embedding_chat_api_key_id": "archestra_model_provider", "ocr_chat_api_key_id": "archestra_model_provider", "reranker_chat_api_key_id": "archestra_model_provider",
			"connection_default_mcp_gateway_id": "archestra_mcp_gateway",
		},
	} {
		p.AddResourceConfigurator(owner, func(r *ujconfig.Resource) {
			for field, target := range targets {
				r.References[field] = ujconfig.Reference{TerraformName: target}
			}
		})
	}
	p.AddResourceConfigurator("archestra_service_account", func(r *ujconfig.Resource) {
		r.References["role"] = ujconfig.Reference{TerraformName: "archestra_role", Extractor: roleSlugExtractor}
	})
}
