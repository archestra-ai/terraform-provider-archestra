package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
		MarkdownDescription: "Sets the unconditional default trusted-data action for a set of tools (mark trusted / untrusted / sanitize / block). For conditional rules layered on top, use `archestra_trusted_data_policy`.",

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

// Read reconciles state's `tool_ids` against the live trusted-data
// policies table — same drift-detection model as
// resource_tool_invocation_policy_default. See that file's Read for the
// detailed contract.
func (r *TrustedDataPolicyDefaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TrustedDataPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateTools := parseUUIDSet(ctx, state.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetTrustedDataPoliciesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list trusted data policies: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("List trusted data policies returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	defaults := map[openapi_types.UUID]string{}
	for _, p := range *apiResp.JSON200 {
		if len(p.Conditions) == 0 {
			defaults[p.ToolId] = string(p.Action)
		}
	}

	kept := reconcileDefaultPolicyTools(stateTools, state.Action.ValueString(), defaults)
	if len(kept) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	keptSet, d := uuidsToStringSet(kept)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ToolIDs = keptSet
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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

// Delete removes the per-tool default trusted-data policy rows this
// resource owns by listing policies, filtering to (tool_id, action,
// conditions=[]) matches, and DELETEing each by ID. Errors surface as
// `AddError` — silent leftover rows would be a security inconsistency.
func (r *TrustedDataPolicyDefaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TrustedDataPolicyDefaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	stateTools := parseUUIDSet(ctx, data.ToolIDs, &resp.Diagnostics, "tool_ids")
	if resp.Diagnostics.HasError() {
		return
	}
	stateAction := data.Action.ValueString()

	listResp, err := r.client.GetTrustedDataPoliciesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list trusted data policies for delete: %s", err))
		return
	}
	if listResp.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("List trusted data policies returned status %d: %s", listResp.StatusCode(), string(listResp.Body)))
		return
	}

	managed := map[openapi_types.UUID]struct{}{}
	for _, t := range stateTools {
		managed[t] = struct{}{}
	}
	for _, p := range *listResp.JSON200 {
		if len(p.Conditions) != 0 {
			continue
		}
		if string(p.Action) != stateAction {
			continue
		}
		if _, ok := managed[p.ToolId]; !ok {
			continue
		}
		delResp, err := r.client.DeleteTrustedDataPolicyWithResponse(ctx, p.Id)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to delete trusted data policy %s: %s", p.Id, err))
			return
		}
		// Tolerate 404 — row already gone (race, manual cleanup,
		// concurrent destroy). Anything else is a real failure.
		if delResp.JSON200 == nil && delResp.StatusCode() != 404 {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Delete trusted data policy %s returned status %d: %s", p.Id, delResp.StatusCode(), string(delResp.Body)))
			return
		}
	}
}

// ImportState accepts either the bare action name (manual import) or
// the synthetic `<action>:<hash>` ID (round-trip during test framework
// import-verify). Read fills in `tool_ids` on the next refresh by
// listing policies and selecting those whose unconditional default
// matches the imported action.
func (r *TrustedDataPolicyDefaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	action := req.ID
	if i := strings.Index(action, ":"); i >= 0 {
		action = action[:i]
	}
	switch action {
	case "mark_as_trusted", "mark_as_untrusted", "block_always", "sanitize_with_dual_llm":
	default:
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected one of mark_as_trusted | mark_as_untrusted | block_always | sanitize_with_dual_llm (optionally followed by `:<hash>`), got %q.", req.ID),
		)
		return
	}

	listResp, err := r.client.GetTrustedDataPoliciesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list trusted data policies for import: %s", err))
		return
	}
	if listResp.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("List trusted data policies returned status %d: %s", listResp.StatusCode(), string(listResp.Body)))
		return
	}

	tools := []openapi_types.UUID{}
	for _, p := range *listResp.JSON200 {
		if len(p.Conditions) == 0 && string(p.Action) == action {
			tools = append(tools, p.ToolId)
		}
	}
	if len(tools) == 0 {
		resp.Diagnostics.AddError(
			"No matching policies",
			fmt.Sprintf("No tools have %q as their unconditional default trusted-data policy on the backend; nothing to import.", action),
		)
		return
	}

	toolSet, d := uuidsToStringSet(tools)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(syntheticToolSetID(tools, action)))
	resp.State.SetAttribute(ctx, path.Root("action"), types.StringValue(action))
	resp.State.SetAttribute(ctx, path.Root("tool_ids"), toolSet)
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
