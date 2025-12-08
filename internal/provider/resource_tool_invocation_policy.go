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
	AgentToolID   types.String `tfsdk:"agent_tool_id"`
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

		// NOTE: it would be nice to "automatically have"
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
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_tool_id": schema.StringAttribute{
				MarkdownDescription: "The agent tool ID this policy applies to (deprecated: use profile_tool_id)",
				Optional:            true,
				Computed:            true,
				DeprecationMessage:  "This attribute is deprecated. Please use profile_tool_id instead.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"argument_name": schema.StringAttribute{
				MarkdownDescription: "The argument name to match",
				Required:            true,
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "The comparison operator. Valid values: `equal`, `notEqual`, `contains`, `notContains`, `startsWith`, `endsWith`, `regex`",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The value to compare against",
				Required:            true,
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "The action to take when the policy matches. Valid values: `allow_when_context_is_untrusted`, `block_always`",
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

	// Resolve Tool ID
	var toolIDStr string
	if !data.ProfileToolID.IsNull() {
		toolIDStr = data.ProfileToolID.ValueString()
	} else if !data.AgentToolID.IsNull() {
		toolIDStr = data.AgentToolID.ValueString()
	} else {
		resp.Diagnostics.AddError("Missing Tool ID", "Either profile_tool_id or agent_tool_id must be specified")
		return
	}

	// Parse ToolID as UUID
	parsedToolID, err := uuid.Parse(toolIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}
	toolID := parsedToolID

	// Create request body using generated type
	requestBody := client.CreateToolInvocationPolicyJSONRequestBody{
		AgentToolId:  toolID,
		ArgumentName: data.ArgumentName.ValueString(),
		Operator:     client.CreateToolInvocationPolicyJSONBodyOperator(data.Operator.ValueString()),
		Value:        data.Value.ValueString(),
		Action:       client.CreateToolInvocationPolicyJSONBodyAction(data.Action.ValueString()),
	}

	if !data.Reason.IsNull() {
		reason := data.Reason.ValueString()
		requestBody.Reason = &reason
	}

	// Call API
	apiResp, err := r.client.CreateToolInvocationPolicyWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create tool invocation policy, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id.String())

	respID := apiResp.JSON200.AgentToolId.String()
	if !data.ProfileToolID.IsNull() {
		data.ProfileToolID = types.StringValue(respID)
		data.AgentToolID = types.StringNull()
	} else {
		data.AgentToolID = types.StringValue(respID)
		data.ProfileToolID = types.StringNull()
	}

	data.ArgumentName = types.StringValue(apiResp.JSON200.ArgumentName)
	data.Operator = types.StringValue(string(apiResp.JSON200.Operator))
	data.Value = types.StringValue(apiResp.JSON200.Value)
	data.Action = types.StringValue(string(apiResp.JSON200.Action))
	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolInvocationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	parsedID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}
	policyID := parsedID

	// Call API
	apiResp, err := r.client.GetToolInvocationPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read tool invocation policy, got error: %s", err))
		return
	}

	// Handle not found
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	respID := apiResp.JSON200.AgentToolId.String()

	// Preserve which field was used in state
	if !data.ProfileToolID.IsNull() {
		data.ProfileToolID = types.StringValue(respID)
	}
	if !data.AgentToolID.IsNull() {
		data.AgentToolID = types.StringValue(respID)
	}

	data.ArgumentName = types.StringValue(apiResp.JSON200.ArgumentName)
	data.Operator = types.StringValue(string(apiResp.JSON200.Operator))
	data.Value = types.StringValue(apiResp.JSON200.Value)
	data.Action = types.StringValue(string(apiResp.JSON200.Action))
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

	// Parse UUIDs from state
	parsedID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}
	policyID := parsedID

	// Resolve Tool ID
	var toolIDStr string
	if !data.ProfileToolID.IsNull() {
		toolIDStr = data.ProfileToolID.ValueString()
	} else if !data.AgentToolID.IsNull() {
		toolIDStr = data.AgentToolID.ValueString()
	} else {
		resp.Diagnostics.AddError("Missing Tool ID", "Either profile_tool_id or agent_tool_id must be specified")
		return
	}

	parsedToolID, err := uuid.Parse(toolIDStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Tool ID", fmt.Sprintf("Unable to parse tool ID: %s", err))
		return
	}
	toolID := parsedToolID

	// Create request body using generated type
	argumentName := data.ArgumentName.ValueString()
	operator := client.UpdateToolInvocationPolicyJSONBodyOperator(data.Operator.ValueString())
	value := data.Value.ValueString()
	action := client.UpdateToolInvocationPolicyJSONBodyAction(data.Action.ValueString())

	requestBody := client.UpdateToolInvocationPolicyJSONRequestBody{
		AgentToolId:  &toolID,
		ArgumentName: &argumentName,
		Operator:     &operator,
		Value:        &value,
		Action:       &action,
	}

	if !data.Reason.IsNull() {
		reason := data.Reason.ValueString()
		requestBody.Reason = &reason
	}

	// Call API
	apiResp, err := r.client.UpdateToolInvocationPolicyWithResponse(ctx, policyID, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update tool invocation policy, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	respID := apiResp.JSON200.AgentToolId.String()
	if !data.ProfileToolID.IsNull() {
		data.ProfileToolID = types.StringValue(respID)
		data.AgentToolID = types.StringNull()
	} else {
		data.AgentToolID = types.StringValue(respID)
		data.ProfileToolID = types.StringNull()
	}

	data.ArgumentName = types.StringValue(apiResp.JSON200.ArgumentName)
	data.Operator = types.StringValue(string(apiResp.JSON200.Operator))
	data.Value = types.StringValue(apiResp.JSON200.Value)
	data.Action = types.StringValue(string(apiResp.JSON200.Action))
	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolInvocationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse UUID from state
	parsedID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}
	policyID := parsedID

	// Call API
	apiResp, err := r.client.DeleteToolInvocationPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete tool invocation policy, got error: %s", err))
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *ToolInvocationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
