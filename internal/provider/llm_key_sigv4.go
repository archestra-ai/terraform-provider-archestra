package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func sigV4Attribute(description string, required ...string) schema.StringAttribute {
	paths := make([]path.Expression, 0, len(required))
	for _, name := range required {
		paths = append(paths, path.MatchRoot(name))
	}
	return schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: description, Validators: []validator.String{
		stringvalidator.LengthAtLeast(1),
		stringvalidator.AlsoRequires(paths...),
		stringvalidator.ConflictsWith(path.MatchRoot("api_key")),
	}}
}

var _ resource.ResourceWithValidateConfig = &LLMProviderApiKeyResource{}

func (r *LLMProviderApiKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data LLMProviderApiKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.AWSAccessKeyID.IsNull() && !data.LLMProvider.IsUnknown() && data.LLMProvider.ValueString() != "bedrock" {
		resp.Diagnostics.AddAttributeError(path.Root("aws_access_key_id"), "Invalid Credential Provider", "AWS SigV4 credentials require llm_provider = bedrock.")
	}
}

// The API replaces the complete SigV4 secret. A session-token-only rotation
// must resend the pair; ordinary metadata updates must never resend secrets.
func prepareSigV4Patch(plan, prior *LLMProviderApiKeyResourceModel, patch map[string]any) {
	if plan.AWSAccessKeyID.Equal(prior.AWSAccessKeyID) && plan.AWSSecretAccessKey.Equal(prior.AWSSecretAccessKey) && plan.AWSSessionToken.Equal(prior.AWSSessionToken) {
		return
	}
	if plan.AWSAccessKeyID.IsNull() || plan.AWSAccessKeyID.IsUnknown() || plan.AWSSecretAccessKey.IsNull() || plan.AWSSecretAccessKey.IsUnknown() {
		return
	}
	patch["awsAccessKeyId"] = plan.AWSAccessKeyID.ValueString()
	patch["awsSecretAccessKey"] = plan.AWSSecretAccessKey.ValueString()
	if !plan.AWSSessionToken.IsNull() && !plan.AWSSessionToken.IsUnknown() {
		patch["awsSessionToken"] = plan.AWSSessionToken.ValueString()
	} else {
		delete(patch, "awsSessionToken")
	}
}
