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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TrustedDataPolicyResource{}
var _ resource.ResourceWithImportState = &TrustedDataPolicyResource{}

func NewTrustedDataPolicyResource() resource.Resource {
	return &TrustedDataPolicyResource{}
}

type TrustedDataPolicyResource struct {
	client *client.ClientWithResponses
}

type TrustedDataPolicyResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ProfileToolID types.String `tfsdk:"profile_tool_id"`
	Description   types.String `tfsdk:"description"`
	AttributePath types.String `tfsdk:"attribute_path"`
	Operator      types.String `tfsdk:"operator"`
	Value         types.String `tfsdk:"value"`
	Action        types.String `tfsdk:"action"`
}

func (r *TrustedDataPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trusted_data_policy"
}

func (r *TrustedDataPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra trusted data policy.",

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
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the policy",
				Required:            true,
			},
			"attribute_path": schema.StringAttribute{
				MarkdownDescription: "The attribute path to match",
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
				MarkdownDescription: "The action to take when the policy matches. Valid values: `mark_as_trusted`, `block_always`, `sanitize_with_dual_llm` (default: `mark_as_trusted`)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("mark_as_trusted"),
			},
		},
	}
}

func (r *TrustedDataPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TrustedDataPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TrustedDataPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse ProfileToolID as UUID
	parsedProfileToolID, err := uuid.Parse(data.ProfileToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile Tool ID", fmt.Sprintf("Unable to parse profile tool ID: %s", err))
		return
	}
	profileToolID := parsedProfileToolID

	// Create request body using generated type
	description := data.Description.ValueString()
	requestBody := client.CreateTrustedDataPolicyJSONRequestBody{
		ToolId:      profileToolID,
		Action:      client.CreateTrustedDataPolicyJSONBodyAction(data.Action.ValueString()),
		Description: &description,
		Conditions: []struct {
			Key      string                                                   `json:"key"`
			Operator client.CreateTrustedDataPolicyJSONBodyConditionsOperator `json:"operator"`
			Value    string                                                   `json:"value"`
		}{
			{
				Key:      data.AttributePath.ValueString(),
				Operator: client.CreateTrustedDataPolicyJSONBodyConditionsOperator(data.Operator.ValueString()),
				Value:    data.Value.ValueString(),
			},
		},
	}

	// Call API
	apiResp, err := r.client.CreateTrustedDataPolicyWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create trusted data policy, got error: %s", err))
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
	data.ProfileToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	}
	if len(apiResp.JSON200.Conditions) > 0 {
		data.AttributePath = types.StringValue(apiResp.JSON200.Conditions[0].Key)
		data.Operator = types.StringValue(string(apiResp.JSON200.Conditions[0].Operator))
		data.Value = types.StringValue(apiResp.JSON200.Conditions[0].Value)
	}
	data.Action = types.StringValue(string(apiResp.JSON200.Action))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TrustedDataPolicyResourceModel
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
	apiResp, err := r.client.GetTrustedDataPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read trusted data policy, got error: %s", err))
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
	data.ProfileToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	}
	if len(apiResp.JSON200.Conditions) > 0 {
		data.AttributePath = types.StringValue(apiResp.JSON200.Conditions[0].Key)
		data.Operator = types.StringValue(string(apiResp.JSON200.Conditions[0].Operator))
		data.Value = types.StringValue(apiResp.JSON200.Conditions[0].Value)
	}
	data.Action = types.StringValue(string(apiResp.JSON200.Action))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TrustedDataPolicyResourceModel
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

	parsedProfileToolID, err := uuid.Parse(data.ProfileToolID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile Tool ID", fmt.Sprintf("Unable to parse profile tool ID: %s", err))
		return
	}
	profileToolID := parsedProfileToolID

	// Create request body using generated type
	description := data.Description.ValueString()
	action := client.UpdateTrustedDataPolicyJSONBodyAction(data.Action.ValueString())
	conditions := []struct {
		Key      string                                                   `json:"key"`
		Operator client.UpdateTrustedDataPolicyJSONBodyConditionsOperator `json:"operator"`
		Value    string                                                   `json:"value"`
	}{
		{
			Key:      data.AttributePath.ValueString(),
			Operator: client.UpdateTrustedDataPolicyJSONBodyConditionsOperator(data.Operator.ValueString()),
			Value:    data.Value.ValueString(),
		},
	}

	requestBody := client.UpdateTrustedDataPolicyJSONRequestBody{
		ToolId:      &profileToolID,
		Action:      &action,
		Description: &description,
		Conditions:  &conditions,
	}

	// Call API
	apiResp, err := r.client.UpdateTrustedDataPolicyWithResponse(ctx, policyID, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update trusted data policy, got error: %s", err))
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
	data.ProfileToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	}
	if len(apiResp.JSON200.Conditions) > 0 {
		data.AttributePath = types.StringValue(apiResp.JSON200.Conditions[0].Key)
		data.Operator = types.StringValue(string(apiResp.JSON200.Conditions[0].Operator))
		data.Value = types.StringValue(apiResp.JSON200.Conditions[0].Value)
	}
	data.Action = types.StringValue(string(apiResp.JSON200.Action))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TrustedDataPolicyResourceModel
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
	apiResp, err := r.client.DeleteTrustedDataPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete trusted data policy, got error: %s", err))
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

func (r *TrustedDataPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
