package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ToolInvocationPolicyResource{}
var _ resource.ResourceWithImportState = &ToolInvocationPolicyResource{}

func NewToolInvocationPolicyResource() resource.Resource {
	return &ToolInvocationPolicyResource{}
}

type ToolInvocationPolicyResource struct {
	client *client.ClientWithResponses
}

type ToolInvocationPolicyResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ProfileToolID types.String `tfsdk:"profile_tool_id"`
	ArgumentName  types.String `tfsdk:"argument_name"`
	Operator      types.String `tfsdk:"operator"`
	Value         types.String `tfsdk:"value"`
	Action        types.String `tfsdk:"action"`
	Reason        types.String `tfsdk:"reason"`
}

func (r *ToolInvocationPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool_invocation_policy"
}

func (r *ToolInvocationPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra tool invocation policy.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Policy identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_tool_id": schema.StringAttribute{
				MarkdownDescription: "The profile tool ID this policy applies to",
				Required:            true,
			},
			"argument_name": schema.StringAttribute{
				MarkdownDescription: "The argument name to match",
				Required:            true,
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "The comparison operator. Valid values: equal, notEqual, contains, notContains, startsWith, endsWith, regex",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The value to compare against",
				Required:            true,
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "The action to take when the policy matches. Valid values: allow_when_context_is_untrusted, block_always",
				Required:            true,
			},
			"reason": schema.StringAttribute{
				MarkdownDescription: "Optional reason for the policy",
				Optional:            true,
			},
		},
	}
}

func (r *ToolInvocationPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ToolInvocationPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	toolID, err := uuid.Parse(data.ProfileToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid profile_tool_id", err.Error())
		return
	}

	request := client.CreateToolInvocationPolicyJSONRequestBody{
		ToolId: toolID,
		Action: client.CreateToolInvocationPolicyJSONBodyAction(data.Action.ValueString()),
		Conditions: []struct {
			Key      string                                              `json:"key"`
			Operator client.CreateToolInvocationPolicyJSONBodyConditionsOperator `json:"operator"`
			Value    string                                              `json:"value"`
		}{
			{
				Key:      data.ArgumentName.ValueString(),
				Operator: client.CreateToolInvocationPolicyJSONBodyConditionsOperator(data.Operator.ValueString()),
				Value:    data.Value.ValueString(),
			},
		},
	}

	if !data.Reason.IsNull() {
		r := data.Reason.ValueString()
		request.Reason = &r
	}

	apiResp, err := r.client.CreateToolInvocationPolicyWithResponse(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Create failed", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Map response
	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.ProfileToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	data.Action = types.StringValue(string(apiResp.JSON200.Action))

	if len(apiResp.JSON200.Conditions) > 0 {
		data.ArgumentName = types.StringValue(apiResp.JSON200.Conditions[0].Key)
		data.Operator = types.StringValue(string(apiResp.JSON200.Conditions[0].Operator))
		data.Value = types.StringValue(apiResp.JSON200.Conditions[0].Value)
	}

	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	} else {
		data.Reason = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolInvocationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.GetToolInvocationPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Read failed", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Map response
	data.ProfileToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	data.Action = types.StringValue(string(apiResp.JSON200.Action))

	if len(apiResp.JSON200.Conditions) > 0 {
		data.ArgumentName = types.StringValue(apiResp.JSON200.Conditions[0].Key)
		data.Operator = types.StringValue(string(apiResp.JSON200.Conditions[0].Operator))
		data.Value = types.StringValue(apiResp.JSON200.Conditions[0].Value)
	}

	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	} else {
		data.Reason = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolInvocationPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	
	toolID, err := uuid.Parse(data.ProfileToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid profile_tool_id", err.Error())
		return
	}

	action := client.UpdateToolInvocationPolicyJSONBodyAction(data.Action.ValueString())
	request := client.UpdateToolInvocationPolicyJSONRequestBody{
		ToolId: &toolID,
		Action: &action,
		Conditions: &[]struct {
			Key      string                                              `json:"key"`
			Operator client.UpdateToolInvocationPolicyJSONBodyConditionsOperator `json:"operator"`
			Value    string                                              `json:"value"`
		}{
			{
				Key:      data.ArgumentName.ValueString(),
				Operator: client.UpdateToolInvocationPolicyJSONBodyConditionsOperator(data.Operator.ValueString()),
				Value:    data.Value.ValueString(),
			},
		},
	}

	if !data.Reason.IsNull() {
		r := data.Reason.ValueString()
		request.Reason = &r
	}

	apiResp, err := r.client.UpdateToolInvocationPolicyWithResponse(ctx, policyID, request)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Update failed", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Map response
	data.ProfileToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	data.Action = types.StringValue(string(apiResp.JSON200.Action))

	if len(apiResp.JSON200.Conditions) > 0 {
		data.ArgumentName = types.StringValue(apiResp.JSON200.Conditions[0].Key)
		data.Operator = types.StringValue(string(apiResp.JSON200.Conditions[0].Operator))
		data.Value = types.StringValue(apiResp.JSON200.Conditions[0].Value)
	}

	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	} else {
		data.Reason = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolInvocationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}

	apiResp, err := r.client.DeleteToolInvocationPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError("Delete failed", fmt.Sprintf("Expected 200 or 404, got status %d", apiResp.StatusCode()))
		return
	}
}

func (r *ToolInvocationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
