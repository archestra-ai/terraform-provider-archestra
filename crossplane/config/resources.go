package config

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

const agentGroup = "agent"
const mcpGroup = "mcp"
const credentialGroup = "credential"

// ConfigureResources applies the same API names and references in both scopes.
func ConfigureResources(p *ujconfig.Provider) {
	p.AddResourceConfigurator("archestra_project_file", func(r *ujconfig.Resource) {
		r.ShortGroup = "project"
		r.Kind = "File"
		r.References["project_id"] = ujconfig.Reference{TerraformName: "archestra_project"}
	})
	p.AddResourceConfigurator("archestra_marketplace_share_link", func(r *ujconfig.Resource) {
		r.ShortGroup = "marketplace"
		r.Kind = "ShareLink"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "name", "plugin_platform", "expires_at", "rotation_nonce")
		r.References["skill_ids"] = ujconfig.Reference{TerraformName: "archestra_skill"}
		r.References["plugin_ids"] = ujconfig.Reference{TerraformName: "archestra_plugin"}
	})
	p.AddResourceConfigurator("archestra_git_plugin", func(r *ujconfig.Resource) {
		r.ShortGroup = "plugin"
		r.Kind = "RepositoryPackage"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "labels", "github_sync_ref", "github_sync_interval", "github_app_config_id", "github_pat_id", "gitlab_token_id")
		r.References["github_app_config_id"] = ujconfig.Reference{TerraformName: "archestra_github_app_config"}
		r.References["github_pat_id"] = ujconfig.Reference{TerraformName: "archestra_github_pat"}
		r.References["gitlab_token_id"] = ujconfig.Reference{TerraformName: "archestra_runtime_credential"}
	})
	p.AddResourceConfigurator("archestra_plugin", func(r *ujconfig.Resource) {
		r.ShortGroup = "plugin"
		r.Kind = "ManualPackage"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "labels")
	})
	p.AddResourceConfigurator("archestra_github_pat", func(r *ujconfig.Resource) { r.ShortGroup = credentialGroup; r.Kind = "GitHubPAT" })
	p.AddResourceConfigurator("archestra_github_app_config", func(r *ujconfig.Resource) { r.ShortGroup = credentialGroup; r.Kind = "GitHubAppConfig" })
	p.AddResourceConfigurator("archestra_virtual_api_key", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "VirtualKey"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "billing_team_id", "spend_cap", "expires_at", "provider_api_keys", "labels")
		r.References["billing_team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
		r.References["provider_api_keys.provider_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_llm_provider_api_key"}
	})
	p.AddResourceConfigurator("archestra_oauth_client", func(r *ujconfig.Resource) {
		r.ShortGroup = credentialGroup
		r.Kind = "OAuthClient"
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "service_account_id", "billing_team_id", "spend_cap", "provider_api_keys", "labels", "allowed_gateway_ids", "redirect_uris")
		r.References["service_account_id"] = ujconfig.Reference{TerraformName: "archestra_service_account"}
		r.References["billing_team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
		r.References["provider_api_keys.provider_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_llm_provider_api_key"}
	})
	p.AddResourceConfigurator("archestra_remote_agent", func(r *ujconfig.Resource) { r.ShortGroup = "a2a"; r.Kind = "RemoteAgent" })
	p.AddResourceConfigurator("archestra_environment_default", func(r *ujconfig.Resource) {
		r.ShortGroup = "environment"
		r.Kind = "ResourceDefault"
		r.References["environment_id"] = ujconfig.Reference{TerraformName: "archestra_environment"}
	})
	p.AddResourceConfigurator("archestra_team_vault_folder", func(r *ujconfig.Resource) {
		r.ShortGroup = "team"
		r.Kind = "VaultFolder"
		r.References["team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
	})
	for _, name := range []string{"archestra_git_plugin", "archestra_plugin", "archestra_virtual_api_key", "archestra_oauth_client", "archestra_remote_agent", "archestra_service_account", "archestra_agent", "archestra_mcp_gateway", "archestra_project", "archestra_knowledge_base", "archestra_mcp_registry_catalog_item", "archestra_skill", "archestra_runtime_credential", "archestra_llm_provider_api_key"} {
		p.AddResourceConfigurator(name, func(r *ujconfig.Resource) {
			// Keep creation inputs in initProvider so later changes stay ignored.
			r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "initial_grants")
		})
	}
	p.AddResourceConfigurator("archestra_skill", func(r *ujconfig.Resource) {
		r.ShortGroup = "skill"
		r.Kind = "Skill"
		r.References["environment_ids"] = ujconfig.Reference{TerraformName: "archestra_environment"}
	})
	p.AddResourceConfigurator("archestra_project", func(r *ujconfig.Resource) {
		r.ShortGroup = "project"
		r.Kind = "Project"
		r.References["default_agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
	})
	p.AddResourceConfigurator("archestra_knowledge_base", func(r *ujconfig.Resource) { r.ShortGroup = "knowledge"; r.Kind = "KnowledgeBase" })
	p.AddResourceConfigurator("archestra_role", func(r *ujconfig.Resource) { r.ShortGroup = "identity"; r.Kind = "Role" })
	p.AddResourceConfigurator("archestra_api_key", func(r *ujconfig.Resource) { r.ShortGroup = credentialGroup; r.Kind = "APIKey" })
	p.AddResourceConfigurator("archestra_schedule_trigger", func(r *ujconfig.Resource) {
		r.ShortGroup = "schedule"
		r.Kind = "Trigger"
		r.References["agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["project_id"] = ujconfig.Reference{TerraformName: "archestra_project"}
	})

	p.AddResourceConfigurator("archestra_default_user_limit", func(r *ujconfig.Resource) {
		r.ShortGroup = "limit"
		r.Kind = "DefaultUserLimit"
		r.References["environment_id"] = ujconfig.Reference{TerraformName: "archestra_environment"}
	})
	p.AddResourceConfigurator("archestra_knowledge_directory", func(r *ujconfig.Resource) { r.ShortGroup = "knowledge"; r.Kind = "Directory" })
	p.AddResourceConfigurator("archestra_service_account", func(r *ujconfig.Resource) {
		r.ShortGroup = "identity"
		r.Kind = "ServiceAccount"
		r.References["team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
	})
	p.AddResourceConfigurator("archestra_service_account_token", func(r *ujconfig.Resource) {
		r.ShortGroup = credentialGroup
		r.Kind = "ServiceAccountToken"
		r.References["service_account_id"] = ujconfig.Reference{TerraformName: "archestra_service_account"}
	})
	p.AddResourceConfigurator("archestra_environment", func(r *ujconfig.Resource) { r.ShortGroup = "environment"; r.Kind = "Environment" })
	p.AddResourceConfigurator("archestra_agent", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "subagent_ids", "remote_agent_ids", "excluded_subagent_ids", "excluded_connector_ids", "skills", "hooks", "tools", "excluded_tool_ids")
		r.References["tools.mcp_server_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_server_installation"}
		r.References["subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["excluded_subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["remote_agent_ids"] = ujconfig.Reference{TerraformName: "archestra_remote_agent"}
		r.References["skills.allowed.skill_id"] = ujconfig.Reference{TerraformName: "archestra_skill"}
		r.References["skills.excluded.skill_id"] = ujconfig.Reference{TerraformName: "archestra_skill"}

		r.ShortGroup = agentGroup
		r.Kind = "Agent"
		r.References["knowledge_base_ids"] = ujconfig.Reference{TerraformName: "archestra_knowledge_base"}
		r.References["environment_id"] = ujconfig.Reference{TerraformName: "archestra_environment"}
		r.References["llm_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_llm_provider_api_key"}
	})
	p.AddResourceConfigurator("archestra_identity_provider", func(r *ujconfig.Resource) {
		r.ShortGroup = "identity"
		r.Kind = "Provider"
	})
	p.AddResourceConfigurator("archestra_limit", func(r *ujconfig.Resource) {
		r.ShortGroup = "limit"
		r.Kind = "Limit"
	})
	p.AddResourceConfigurator("archestra_llm_model", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "ModelOverride"
	})
	p.AddResourceConfigurator("archestra_llm_provider_api_key", func(r *ujconfig.Resource) {
		r.ShortGroup = "llm"
		r.Kind = "ProviderKey"
	})
	p.AddResourceConfigurator("archestra_mcp_gateway", func(r *ujconfig.Resource) {
		r.LateInitializer.IgnoredFields = append(r.LateInitializer.IgnoredFields, "subagent_ids", "remote_agent_ids", "excluded_subagent_ids", "excluded_connector_ids", "tools", "excluded_tool_ids", "skills")
		r.References["subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["remote_agent_ids"] = ujconfig.Reference{TerraformName: "archestra_remote_agent"}
		r.References["excluded_subagent_ids"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["tools.mcp_server_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_server_installation"}
		r.References["skills.ids"] = ujconfig.Reference{TerraformName: "archestra_skill"}
		r.References["skills.excluded_ids"] = ujconfig.Reference{TerraformName: "archestra_skill"}

		// Keep gateway-to-agent references separate from agent-to-MCP-server references.
		r.ShortGroup = "gateway"
		r.Kind = "Gateway"
		r.References["identity_provider_id"] = ujconfig.Reference{TerraformName: "archestra_identity_provider"}
	})
	p.AddResourceConfigurator("archestra_mcp_registry_catalog_item", func(r *ujconfig.Resource) {
		r.ShortGroup = mcpGroup
		r.Kind = "RegistryCatalogItem"
	})
	p.AddResourceConfigurator("archestra_mcp_server_installation", func(r *ujconfig.Resource) {
		r.ShortGroup = mcpGroup
		r.Kind = "ServerInstallation"
		r.References["catalog_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_registry_catalog_item"}
	})
	p.AddResourceConfigurator("archestra_organization_settings", func(r *ujconfig.Resource) {
		r.ShortGroup = "organization"
		r.Kind = "Settings"
	})
	p.AddResourceConfigurator("archestra_resource_permission_policy", func(r *ujconfig.Resource) {
		r.ShortGroup = "permission"
		r.Kind = "Policy"
	})
	p.AddResourceConfigurator("archestra_runtime_credential", func(r *ujconfig.Resource) {
		r.ShortGroup = credentialGroup
		r.Kind = "RuntimeCredential"
	})
	p.AddResourceConfigurator("archestra_team", func(r *ujconfig.Resource) {
		r.ShortGroup = "team"
		r.Kind = "Team"
	})
	p.AddResourceConfigurator("archestra_team_external_group", func(r *ujconfig.Resource) {
		r.ShortGroup = "team"
		r.Kind = "ExternalGroup"
		r.References["team_id"] = ujconfig.Reference{TerraformName: "archestra_team"}
	})
}
