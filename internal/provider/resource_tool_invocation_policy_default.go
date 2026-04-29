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
	_ resource.Resource                = &ToolInvocationPolicyDefaultResource{}
	_ resource.ResourceWithImportState = &ToolInvocationPolicyDefaultResource{}
)

func NewToolInvocationPolicyDefaultResource() resource.Resource {
	return &ToolInvocationPolicyDefaultResource{}
}

type ToolInvocationPolicyDefaultResource struct {
	client *client.ClientWithResponses
}

type ToolInvocationPolicyDefaultResourceModel struct {
	ID      types.String `tfsdk:"id"`
	ToolIDs types.Set    `tfsdk:"tool_ids"`
	Action  types.String `tfsdk:"action"`
}

func (r *ToolInvocationPolicyDefaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool_invocation_policy_default"
}

func (r *ToolInvocationPolicyDefaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets the default invocation action for a set of tools. Maps to the **`DEFAULT` row** in the Guardrails UI's Tool Call Policies section (allow / allow-in-safe-context / require-approval / block).\n\n" +
			"~> **For per-tool conditional rules** (the UI's \"Add Policy\" button), use [`archestra_tool_invocation_policy`](tool_invocation_policy). That sibling resource layers on top of this one — conditional rules evaluate first, this default fires when none match.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic resource ID. Not a backend identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tool_ids": schema.SetAttribute{
				MarkdownDescription: "Set of bare tool UUIDs to apply the default policy to. Typically `[for t in archestra_mcp_server_installation.<n>.tools : t.id]`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Default action when no conditional policy matches. One of:\n\n" +
					"- `allow_when_context_is_untrusted` — let the call through even with untrusted context (most permissive).\n" +
					"- `block_when_context_is_untrusted` — let the call through only when context is trusted.\n" +
					"- `require_approval` — surface for human approval before executing.\n" +
					"- `block_always` — never execute (most restrictive).",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("allow_when_context_is_untrusted", "block_when_context_is_untrusted", "block_always", "require_approval"),
				},
			},
		},
	}
}

func (r *ToolInvocationPolicyDefaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ToolInvocationPolicyDefaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ToolInvocationPolicyDefaultResourceModel
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

func (r *ToolInvocationPolicyDefaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Default policies are stored on the backend as conditions=[] entries
	// in the tool-invocation-policies table. Round-tripping the exact set
	// would require listing all policies and filtering — and the bulk-
	// default endpoint is upsert-only, so any out-of-band manual change
	// can't be reliably reconciled. Treat the resource as fire-and-forget
	// for UX simplicity: trust state, never drift.
	var data ToolInvocationPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolInvocationPolicyDefaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ToolInvocationPolicyDefaultResourceModel
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
	// Preserve the ID from prior state — recomputing from plan tools/action
	// would break Terraform's "id is stable across the resource's
	// lifetime" invariant when `tool_ids` or `action` changes in-place.
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolInvocationPolicyDefaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Resetting to allow_when_context_is_untrusted is the most permissive
	// fallback and matches the implicit behaviour the platform applies
	// when no default policy is defined. We deliberately avoid hunting
	// down and DELETEing the per-tool default rows by ID — the bulk
	// endpoint is upsert-only, and matching individual policy IDs back
	// would require a list+filter dance with no atomicity guarantees.
	var data ToolInvocationPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tools := parseUUIDSet(ctx, data.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, tools, "allow_when_context_is_untrusted"); err != nil {
		resp.Diagnostics.AddWarning("Default policy reset failed",
			fmt.Sprintf("Could not reset default invocation policies on delete: %s. The policies may still exist in the backend; remove them via `archestra_tool_invocation_policy` or directly via the API.", err))
	}
}

func (r *ToolInvocationPolicyDefaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError("Import not supported",
		"`archestra_tool_invocation_policy_default` cannot be imported because the bulk-default endpoint is upsert-only. Recreate the resource in HCL with the desired tool_ids and action, then run `terraform apply`.")
}

func (r *ToolInvocationPolicyDefaultResource) upsert(ctx context.Context, tools []openapi_types.UUID, action string) error {
	body := client.BulkUpsertDefaultCallPolicyJSONRequestBody{
		Action:  client.BulkUpsertDefaultCallPolicyJSONBodyAction(action),
		ToolIds: tools,
	}
	resp, err := r.client.BulkUpsertDefaultCallPolicyWithResponse(ctx, body)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("bulk-upsert-default-call-policy returned status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return nil
}
