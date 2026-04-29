package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	_ resource.Resource                = &TrustedDataPolicyDefaultResource{}
	_ resource.ResourceWithImportState = &TrustedDataPolicyDefaultResource{}
)

func NewTrustedDataPolicyDefaultResource() resource.Resource {
	return &TrustedDataPolicyDefaultResource{}
}

type TrustedDataPolicyDefaultResource struct {
	client *client.ClientWithResponses
}

type TrustedDataPolicyDefaultResourceModel struct {
	ID      types.String `tfsdk:"id"`
	ToolIDs types.Set    `tfsdk:"tool_ids"`
	Action  types.String `tfsdk:"action"`
}

func (r *TrustedDataPolicyDefaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trusted_data_policy_default"
}

func (r *TrustedDataPolicyDefaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets the **default** (unconditional) trusted-data policy for a set of tools in a single API call. The trusted-data policy controls how the tool's *result* is treated as it flows back into the LLM context — verbose, summarised, sanitised, or blocked.\n\n" +
			"Equivalent to writing `N` `archestra_trusted_data_policy` resources with empty `conditions = []`, but uses the `bulk-default` upsert endpoint.\n\n" +
			"```hcl\n" +
			"resource \"archestra_trusted_data_policy_default\" \"sanitise_filesystem\" {\n" +
			"  tool_ids = toset([for t in archestra_mcp_server_installation.filesystem.tools : t.id])\n" +
			"  action   = \"sanitize_with_dual_llm\"\n" +
			"}\n" +
			"```",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic resource ID. Not a backend identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tool_ids": schema.SetAttribute{
				MarkdownDescription: "Set of bare tool UUIDs to apply the default trusted-data policy to.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Default action for tool results when no conditional trusted-data policy matches. One of:\n\n" +
					"- `mark_as_trusted` — flow the result into the LLM context as-is.\n" +
					"- `mark_as_untrusted` — let downstream policies treat it as untrusted but don't sanitise.\n" +
					"- `sanitize_with_dual_llm` — pre-process the result through a dual-LLM sanitiser before flowing.\n" +
					"- `block_always` — discard the result entirely.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("mark_as_trusted", "mark_as_untrusted", "block_always", "sanitize_with_dual_llm"),
				},
			},
		},
	}
}

func (r *TrustedDataPolicyDefaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TrustedDataPolicyDefaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TrustedDataPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tools := parseUUIDSet(ctx, plan.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, tools, plan.Action.ValueString()); err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	plan.ID = types.StringValue(syntheticToolSetID(tools, plan.Action.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TrustedDataPolicyDefaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Same fire-and-forget rationale as the call-policy default resource:
	// the bulk-upsert endpoint is one-way, so we don't try to reconcile
	// from the policies table.
	var data TrustedDataPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TrustedDataPolicyDefaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state TrustedDataPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tools := parseUUIDSet(ctx, plan.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, tools, plan.Action.ValueString()); err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	// Preserve the ID from prior state — see same comment in
	// resource_tool_invocation_policy_default.go.
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TrustedDataPolicyDefaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Reset to mark_as_trusted on delete — matches the implicit
	// permissive behaviour when no default trusted-data policy exists.
	var data TrustedDataPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tools := parseUUIDSet(ctx, data.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, tools, "mark_as_trusted"); err != nil {
		resp.Diagnostics.AddWarning("Default policy reset failed",
			fmt.Sprintf("Could not reset default trusted-data policies on delete: %s. Remove via `archestra_trusted_data_policy` or directly via the API.", err))
	}
}

func (r *TrustedDataPolicyDefaultResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError("Import not supported",
		"`archestra_trusted_data_policy_default` cannot be imported because the bulk-default endpoint is upsert-only. Recreate the resource in HCL and run `terraform apply`.")
}

func (r *TrustedDataPolicyDefaultResource) upsert(ctx context.Context, tools []openapi_types.UUID, action string) error {
	body := client.BulkUpsertDefaultResultPolicyJSONRequestBody{
		Action:  client.BulkUpsertDefaultResultPolicyJSONBodyAction(action),
		ToolIds: tools,
	}
	resp, err := r.client.BulkUpsertDefaultResultPolicyWithResponse(ctx, body)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("bulk-upsert-default-result-policy returned status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return nil
}
