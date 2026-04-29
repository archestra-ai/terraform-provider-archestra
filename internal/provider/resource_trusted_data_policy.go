package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
	ID          types.String           `tfsdk:"id"`
	ToolID      types.String           `tfsdk:"tool_id"`
	Description types.String           `tfsdk:"description"`
	Conditions  []PolicyConditionModel `tfsdk:"conditions"`
	Action      types.String           `tfsdk:"action"`
}

func (r *TrustedDataPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trusted_data_policy"
}

func (r *TrustedDataPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Conditional trusted-data policy — fires `action` when ALL of `conditions` match the tool's *result*. `conditions` must be non-empty; for the unconditional default, use `archestra_trusted_data_policy_default`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Policy identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tool_id": schema.StringAttribute{
				MarkdownDescription: "Bare tool UUID this policy applies to. **Not** the agent-tool assignment composite ID.\n\n" +
					"Preferred lookup is `archestra_mcp_server_installation.<n>.tool_id_by_name[\"<server>__<short>\"]` — one line, no extra data source. Fallbacks: `archestra_agent_tool.<n>.tool_id` (when the assignment is also Terraform-managed) or `data.archestra_mcp_server_tool.<n>.id` (for installs not managed by Terraform).",
				Required: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(uuidRegexp, "tool_id must be a UUID (use a tool data source's `id` or `tool_id` field, not the agent-tool assignment ID)"),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the policy",
				Required:            true,
			},
			"conditions": schema.ListNestedAttribute{
				MarkdownDescription: "Conditions evaluated against the data attribute. ALL must match for `action` to fire. Use `key` for the JSON path of the attribute being matched.",
				Required:            true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Attribute path to match (e.g., `payload.role`).",
							Required:            true,
						},
						"operator": schema.StringAttribute{
							MarkdownDescription: "Comparison operator. One of `equal`, `notEqual`, `contains`, `notContains`, `startsWith`, `endsWith`, `regex`.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("equal", "notEqual", "contains", "notContains", "startsWith", "endsWith", "regex"),
							},
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Value to compare against.",
							Required:            true,
						},
					},
				},
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Action to take when the policy matches. One of `mark_as_trusted`, `mark_as_untrusted`, `block_always`, `sanitize_with_dual_llm`. Default `mark_as_trusted`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("mark_as_trusted"),
				Validators: []validator.String{
					stringvalidator.OneOf("mark_as_trusted", "mark_as_untrusted", "block_always", "sanitize_with_dual_llm"),
				},
			},
		},
	}
}

func (r *TrustedDataPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *TrustedDataPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TrustedDataPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	patch := MergePatch(ctx, req.Plan.Raw, prior, trustedDataPolicyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_trusted_data_policy Create", patch, trustedDataPolicyAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateTrustedDataPolicyWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create trusted data policy: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.ToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	data.Action = types.StringValue(string(apiResp.JSON200.Action))
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	}
	data.Conditions = make([]PolicyConditionModel, len(apiResp.JSON200.Conditions))
	for i, c := range apiResp.JSON200.Conditions {
		data.Conditions[i] = PolicyConditionModel{
			Key:      types.StringValue(c.Key),
			Operator: types.StringValue(string(c.Operator)),
			Value:    types.StringValue(c.Value),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TrustedDataPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}

	apiResp, err := r.client.GetTrustedDataPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read trusted data policy: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.ToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	data.Action = types.StringValue(string(apiResp.JSON200.Action))
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.Conditions = make([]PolicyConditionModel, len(apiResp.JSON200.Conditions))
	for i, c := range apiResp.JSON200.Conditions {
		data.Conditions[i] = PolicyConditionModel{
			Key:      types.StringValue(c.Key),
			Operator: types.StringValue(string(c.Operator)),
			Value:    types.StringValue(c.Value),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TrustedDataPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, trustedDataPolicyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_trusted_data_policy Update", patch, trustedDataPolicyAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.UpdateTrustedDataPolicyWithBodyWithResponse(ctx, policyID, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update trusted data policy: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	data.ToolID = types.StringValue(apiResp.JSON200.ToolId.String())
	data.Action = types.StringValue(string(apiResp.JSON200.Action))
	if apiResp.JSON200.Description != nil {
		data.Description = types.StringValue(*apiResp.JSON200.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.Conditions = make([]PolicyConditionModel, len(apiResp.JSON200.Conditions))
	for i, c := range apiResp.JSON200.Conditions {
		data.Conditions[i] = PolicyConditionModel{
			Key:      types.StringValue(c.Key),
			Operator: types.StringValue(string(c.Operator)),
			Value:    types.StringValue(c.Value),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TrustedDataPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteTrustedDataPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete trusted data policy: %s", err))
		return
	}
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
