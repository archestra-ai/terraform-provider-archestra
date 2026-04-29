package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource = &ToolPolicyAutoConfigResource{}
)

func NewToolPolicyAutoConfigResource() resource.Resource {
	return &ToolPolicyAutoConfigResource{}
}

type ToolPolicyAutoConfigResource struct {
	client *client.ClientWithResponses
}

type ToolPolicyAutoConfigResourceModel struct {
	ID      types.String `tfsdk:"id"`
	ToolIDs types.Set    `tfsdk:"tool_ids"`
	Results types.List   `tfsdk:"results"`
}

var toolPolicyAutoConfigResultObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"tool_id":                types.StringType,
	"success":                types.BoolType,
	"error":                  types.StringType,
	"reasoning":              types.StringType,
	"tool_invocation_action": types.StringType,
	"trusted_data_action":    types.StringType,
}}

func (r *ToolPolicyAutoConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool_policy_auto_config"
}

func (r *ToolPolicyAutoConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Runs the platform's LLM-driven policy auto-configuration over a set of tools. The backend analyses each tool's name, description, and parameters and writes a default invocation + trusted-data policy plus a reasoning string.\n\n" +
			"Mirrors the frontend's *Configure with Subagent* button. Spends LLM tokens on every apply, so the resource is **one-shot**: changing `tool_ids` forces a full replacement (re-running the LLM); the `results` list is captured in state and never refreshed.\n\n" +
			"```hcl\n" +
			"resource \"archestra_tool_policy_auto_config\" \"filesystem\" {\n" +
			"  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])\n" +
			"}\n\n" +
			"output \"filesystem_policy_reasoning\" {\n" +
			"  value = { for r in archestra_tool_policy_auto_config.filesystem.results : r.tool_id => r.reasoning }\n" +
			"}\n" +
			"```\n\n" +
			"~> **Side effects in the backend.** Auto-config writes default invocation and trusted-data policies for each tool. Removing this resource does **not** delete those policies — manage their lifecycle via `archestra_tool_invocation_policy_default` / `archestra_trusted_data_policy_default` if you need to roll them back, or import them into individual `archestra_tool_invocation_policy` / `archestra_trusted_data_policy` resources for finer control.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic resource ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tool_ids": schema.SetAttribute{
				MarkdownDescription: "Set of bare tool UUIDs to feed into the LLM auto-config. Changing the set re-runs the analysis (replacement).",
				Required:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"results": schema.ListNestedAttribute{
				MarkdownDescription: "Per-tool LLM analysis result. Captured at create time and never refreshed.",
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tool_id": schema.StringAttribute{
							MarkdownDescription: "Bare tool UUID.",
							Computed:            true,
						},
						"success": schema.BoolAttribute{
							MarkdownDescription: "True when the LLM produced a valid policy for this tool.",
							Computed:            true,
						},
						"error": schema.StringAttribute{
							MarkdownDescription: "Error message when `success` is false. Null otherwise.",
							Computed:            true,
						},
						"reasoning": schema.StringAttribute{
							MarkdownDescription: "Free-form rationale the LLM produced. Useful for audit / sign-off.",
							Computed:            true,
						},
						"tool_invocation_action": schema.StringAttribute{
							MarkdownDescription: "Default invocation action the LLM chose for this tool.",
							Computed:            true,
						},
						"trusted_data_action": schema.StringAttribute{
							MarkdownDescription: "Default trusted-data action the LLM chose for this tool.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (r *ToolPolicyAutoConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *ToolPolicyAutoConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ToolPolicyAutoConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tools := parseUUIDSet(ctx, plan.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}

	body := client.AutoConfigureAgentToolPoliciesJSONRequestBody{
		ToolIds: tools,
	}
	apiResp, err := r.client.AutoConfigureAgentToolPoliciesWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("auto-configure failed: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("auto-configure returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	elems := make([]attr.Value, len(apiResp.JSON200.Results))
	for i, res := range apiResp.JSON200.Results {
		errStr := types.StringNull()
		if res.Error != nil {
			errStr = types.StringValue(*res.Error)
		}
		reasoning := types.StringNull()
		invocationAction := types.StringNull()
		trustedDataAction := types.StringNull()
		if res.Config != nil {
			reasoning = types.StringValue(res.Config.Reasoning)
			invocationAction = types.StringValue(string(res.Config.ToolInvocationAction))
			trustedDataAction = types.StringValue(string(res.Config.TrustedDataAction))
		}
		obj, diags := types.ObjectValue(toolPolicyAutoConfigResultObjectType.AttrTypes, map[string]attr.Value{
			"tool_id":                types.StringValue(res.ToolId.String()),
			"success":                types.BoolValue(res.Success),
			"error":                  errStr,
			"reasoning":              reasoning,
			"tool_invocation_action": invocationAction,
			"trusted_data_action":    trustedDataAction,
		})
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		elems[i] = obj
	}
	listValue, diags := types.ListValue(toolPolicyAutoConfigResultObjectType, elems)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	plan.Results = listValue
	plan.ID = types.StringValue(syntheticToolSetID(tools, "auto-config"))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolPolicyAutoConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// One-shot: state is the source of truth, never re-run the LLM.
	var data ToolPolicyAutoConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolPolicyAutoConfigResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All inputs are RequiresReplace — Update should never be called.
	resp.Diagnostics.AddError("Update Not Supported",
		"All `archestra_tool_policy_auto_config` inputs require replacement. This call should never have happened.")
}

func (r *ToolPolicyAutoConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: the policies the LLM wrote are persisted backend-side and
	// outlive this resource. We surface this at the moment it matters so
	// `terraform destroy` output isn't silently misleading.
	resp.Diagnostics.AddWarning(
		"Backend policies persist",
		"Default invocation + trusted-data policies the LLM wrote remain on the backend after this resource is destroyed. "+
			"Manage their lifecycle via `archestra_tool_invocation_policy_default` / `archestra_trusted_data_policy_default`, "+
			"or import them into individual `archestra_tool_invocation_policy` / `archestra_trusted_data_policy` resources for finer control.",
	)
}
