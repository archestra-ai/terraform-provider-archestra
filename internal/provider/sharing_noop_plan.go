package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Computed sharing reads must not turn unchanged metadata into an update.
func (r *EnvironmentResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *LlmModelResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *KnowledgeBaseResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *ServiceAccountResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *RuntimeCredentialResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *ProjectResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *SkillResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *LLMProviderApiKeyResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *VirtualAPIKeyResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *OauthClientResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
func (r *RemoteAgentResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}

func (r *MCPServerRegistryResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveManagedNoopPlan(req, resp)
}
