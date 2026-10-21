package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	_ resource.Resource                = &AgentKnowledgeSourceExclusionsResource{}
	_ resource.ResourceWithImportState = &AgentKnowledgeSourceExclusionsResource{}
)

func NewAgentKnowledgeSourceExclusionsResource() resource.Resource {
	return &AgentKnowledgeSourceExclusionsResource{}
}

type AgentKnowledgeSourceExclusionsResource struct {
	client *client.ClientWithResponses
}

// AgentKnowledgeSourceExclusionsResourceModel models the FULL Auto-mode knowledge-source exclusion
// list of one agent. The backend endpoint is a full replace, so the resource
// is authoritative: at most one per agent, and out-of-band additions are
// reverted on the next apply.
type AgentKnowledgeSourceExclusionsResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	AgentID              types.String `tfsdk:"agent_id"`
	ExcludedConnectorIds types.Set    `tfsdk:"excluded_connector_ids"`
}

func (r *AgentKnowledgeSourceExclusionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_knowledge_source_exclusions"
}

func (r *AgentKnowledgeSourceExclusionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Auto-mode knowledge-source exclusions for one agent: connectors omitted from its knowledge queries while `access_all_tools` is on (exclusions persist when it is off). **Authoritative over the agent's entire exclusion list** — every apply replaces the full set, and destroy clears it. Declare at most one per agent.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `agent_id` — the exclusion list has no id of its own",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Agent UUID. Pass the `id` from `archestra_agent`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"excluded_connector_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge connector UUIDs excluded from the agent's Auto-mode knowledge queries. An empty set is valid and clears all exclusions.",
			},
		},
	}
}

func (r *AgentKnowledgeSourceExclusionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *AgentKnowledgeSourceExclusionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentKnowledgeSourceExclusionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceExclusions(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentKnowledgeSourceExclusionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentKnowledgeSourceExclusionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentKnowledgeSourceExclusionsWithResponse(ctx, agentUUID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent knowledge source exclusions: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	ids := make([]string, len(apiResp.JSON200.ExcludedConnectorIds))
	for i, id := range apiResp.JSON200.ExcludedConnectorIds {
		ids[i] = id.String()
	}
	set, d := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(d...)
	data.ExcludedConnectorIds = set
	data.ID = data.AgentID

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentKnowledgeSourceExclusionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AgentKnowledgeSourceExclusionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.replaceExclusions(ctx, &plan, &resp.Diagnostics) {
		return
	}
	plan.ID = plan.AgentID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete clears the agent's exclusion list — the resource owns the whole set,
// so destroy means "no exclusions", not "stop managing".
func (r *AgentKnowledgeSourceExclusionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentKnowledgeSourceExclusionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentUUID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return
	}

	apiResp, err := r.client.UpdateAgentKnowledgeSourceExclusionsWithResponse(ctx, agentUUID,
		client.UpdateAgentKnowledgeSourceExclusionsJSONRequestBody{ExcludedConnectorIds: []openapi_types.UUID{}})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to clear agent knowledge source exclusions: %s", err))
		return
	}
	// Agent already gone → nothing to clear.
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState imports by agent UUID (the list has no id of its own).
func (r *AgentKnowledgeSourceExclusionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}

// AttrSpecs implements resourceWithAttrSpec (see specdrift_test.go). Both
// fields ride a typed body / the URL path rather than a merge patch, but the
// declaration keeps the schema↔spec drift lint active.
func (r *AgentKnowledgeSourceExclusionsResource) AttrSpecs() []AttrSpec {
	return agentKnowledgeSourceExclusionsAttrSpec
}

func (r *AgentKnowledgeSourceExclusionsResource) APIShape() any {
	return client.GetAgentKnowledgeSourceExclusionsResponse{}
}

func (r *AgentKnowledgeSourceExclusionsResource) KnownIntentionallySkipped() []string { return nil }

var agentKnowledgeSourceExclusionsAttrSpec = []AttrSpec{
	{TFName: "agent_id", Kind: Synthetic},
	{TFName: "excluded_connector_ids", JSONName: "excludedConnectorIds", Kind: Set},
}

// replaceExclusions PUTs the full plan-side exclusion set.
func (r *AgentKnowledgeSourceExclusionsResource) replaceExclusions(ctx context.Context, plan *AgentKnowledgeSourceExclusionsResourceModel, diags *diag.Diagnostics) bool {
	agentUUID, err := uuid.Parse(plan.AgentID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", fmt.Sprintf("Unable to parse agent ID: %s", err))
		return false
	}

	var rawIds []string
	diags.Append(plan.ExcludedConnectorIds.ElementsAs(ctx, &rawIds, false)...)
	if diags.HasError() {
		return false
	}
	connectorIds := make([]openapi_types.UUID, len(rawIds))
	for i, raw := range rawIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			diags.AddError("Invalid Connector ID", fmt.Sprintf("Unable to parse connector ID %q: %s", raw, err))
			return false
		}
		connectorIds[i] = id
	}

	apiResp, err := r.client.UpdateAgentKnowledgeSourceExclusionsWithResponse(ctx, agentUUID,
		client.UpdateAgentKnowledgeSourceExclusionsJSONRequestBody{ExcludedConnectorIds: connectorIds})
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to replace agent knowledge source exclusions: %s", err))
		return false
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return false
	}

	persisted := make([]string, len(apiResp.JSON200.ExcludedConnectorIds))
	for i, id := range apiResp.JSON200.ExcludedConnectorIds {
		persisted[i] = id.String()
	}
	actual, d := types.SetValueFrom(ctx, types.StringType, persisted)
	diags.Append(d...)
	if !actual.Equal(plan.ExcludedConnectorIds) {
		diags.AddError("Invalid Knowledge Source Exclusions", "The API rejected missing or cross-organization connectors; verify every excluded connector before applying.")
		return false
	}
	return !diags.HasError()
}
