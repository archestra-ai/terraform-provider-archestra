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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
	ID         types.String           `tfsdk:"id"`
	ToolID     types.String           `tfsdk:"tool_id"`
	Conditions []PolicyConditionModel `tfsdk:"conditions"`
	Action     types.String           `tfsdk:"action"`
	Reason     types.String           `tfsdk:"reason"`
}

// PolicyConditionModel mirrors the wire `{key, operator, value}` triple shared
// by tool_invocation_policy and trusted_data_policy.
type PolicyConditionModel struct {
	Key      types.String `tfsdk:"key"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

func (r *ToolInvocationPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool_invocation_policy"
}

func (r *ToolInvocationPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Conditional tool-invocation policy — fires `action` when ALL of `conditions` match the tool-call arguments. `conditions` must be non-empty; for the unconditional default, use `archestra_tool_invocation_policy_default`.",

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
			"conditions": schema.ListNestedAttribute{
				MarkdownDescription: "Conditions evaluated against tool-call arguments. ALL must match for `action` to fire.",
				Required:            true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Argument name to match.",
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
				MarkdownDescription: "Action to take when the policy matches. One of `allow_when_context_is_untrusted`, `block_when_context_is_untrusted`, `block_always`, `require_approval`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("allow_when_context_is_untrusted", "block_when_context_is_untrusted", "block_always", "require_approval"),
				},
			},
			"reason": schema.StringAttribute{
				MarkdownDescription: "Optional reason describing why this policy exists.",
				Optional:            true,
			},
		},
	}
}

func (r *ToolInvocationPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ToolInvocationPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	prior := tftypes.NewValue(req.Plan.Raw.Type(), nil)
	patch := MergePatch(ctx, req.Plan.Raw, prior, toolInvocationPolicyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_tool_invocation_policy Create", patch, toolInvocationPolicyAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.CreateToolInvocationPolicyWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create tool invocation policy: %s", err))
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
	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	} else {
		data.Reason = types.StringNull()
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

func (r *ToolInvocationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}

	apiResp, err := r.client.GetToolInvocationPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read tool invocation policy: %s", err))
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
	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	} else {
		data.Reason = types.StringNull()
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

func (r *ToolInvocationPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}

	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, toolInvocationPolicyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	LogPatch(ctx, "archestra_tool_invocation_policy Update", patch, toolInvocationPolicyAttrSpec)

	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Marshal Error", fmt.Sprintf("Unable to marshal request body: %s", err))
		return
	}
	apiResp, err := r.client.UpdateToolInvocationPolicyWithBodyWithResponse(ctx, policyID, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update tool invocation policy: %s", err))
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
	if apiResp.JSON200.Reason != nil {
		data.Reason = types.StringValue(*apiResp.JSON200.Reason)
	} else {
		data.Reason = types.StringNull()
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

func (r *ToolInvocationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ToolInvocationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse policy ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteToolInvocationPolicyWithResponse(ctx, policyID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete tool invocation policy: %s", err))
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

func (r *ToolInvocationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
