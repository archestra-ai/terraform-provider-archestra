package config

import ujconfig "github.com/crossplane/upjet/v2/pkg/config"

// ConfigureResources applies the same API names and references in both scopes.
func ConfigureResources(p *ujconfig.Provider) {
	p.AddResourceConfigurator("archestra_environment", func(r *ujconfig.Resource) { r.ShortGroup = "environment"; r.Kind = "Environment" })
	p.AddResourceConfigurator("archestra_agent", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "Agent"
		r.References["environment_id"] = ujconfig.Reference{TerraformName: "archestra_environment"}
		r.References["llm_api_key_id"] = ujconfig.Reference{TerraformName: "archestra_llm_provider_api_key"}
	})
	p.AddResourceConfigurator("archestra_agent_delegation", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "Delegation"
		r.References["agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["target_agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
	})
	p.AddResourceConfigurator("archestra_agent_tool", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "Tool"
		r.References["agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
	})
	p.AddResourceConfigurator("archestra_agent_tool_batch", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "ToolBatch"
		r.References["agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
		r.References["mcp_server_id"] = ujconfig.Reference{TerraformName: "archestra_mcp_server_installation"}
	})
	p.AddResourceConfigurator("archestra_agent_tool_exclusions", func(r *ujconfig.Resource) {
		r.ShortGroup = "agent"
		r.Kind = "ToolExclusions"
		r.References["agent_id"] = ujconfig.Reference{TerraformName: "archestra_agent"}
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
		r.ShortGroup = "mcp"
		r.Kind = "Gateway"
		r.References["identity_provider_id"] = ujconfig.Reference{TerraformName: "archestra_identity_provider"}
	})
	p.AddResourceConfigurator("archestra_mcp_registry_catalog_item", func(r *ujconfig.Resource) {
		r.ShortGroup = "mcp"
		r.Kind = "RegistryCatalogItem"
	})
	p.AddResourceConfigurator("archestra_mcp_server_installation", func(r *ujconfig.Resource) {
		r.ShortGroup = "mcp"
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
		r.ShortGroup = "credential"
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
