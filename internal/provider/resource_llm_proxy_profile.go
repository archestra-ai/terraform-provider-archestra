package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

// Profiles route external LLM traffic. The platform-owned llm_proxy singleton is not a profile.
func NewLLMProxyProfileResource() resource.Resource {
	return &McpGatewayResource{agentType: "profile"}
}
func (r *McpGatewayResource) apiAgentType() string {
	if r.agentType == "profile" {
		return "profile"
	}
	return "mcp_gateway"
}
func (r *McpGatewayResource) resourceSuffix() string {
	if r.agentType == "profile" {
		return "llm_proxy_profile"
	}
	return "mcp_gateway"
}
