package provider

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"strings"
)

var _ resource.ResourceWithImportState = &HookResource{}

func NewHookResource() resource.Resource { return &HookResource{} }

type HookResource struct{ client *client.ClientWithResponses }
type HookResourceModel struct {
	ID             types.String `tfsdk:"id"`
	AgentID        types.String `tfsdk:"agent_id"`
	Event          types.String `tfsdk:"event"`
	FileName       types.String `tfsdk:"file_name"`
	Content        types.String `tfsdk:"content"`
	Requirements   types.List   `tfsdk:"requirements"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

var agent_hookAttrSpec = []AttrSpec{
	{TFName: "agent_id", JSONName: "agentId", Kind: Synthetic},
	{TFName: "event", JSONName: "event", Kind: Scalar},
	{TFName: "file_name", JSONName: "fileName", Kind: Scalar},
	{TFName: "content", JSONName: "content", Kind: Scalar},
	{TFName: "requirements", JSONName: "requirements", Kind: List, OmitOnNull: true},
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar, OmitOnNull: true},
}

func (r *HookResource) AttrSpecs() []AttrSpec { return agent_hookAttrSpec }
func (r *HookResource) APIShape() any         { return client.GetHooksResponse{} }

// Permission projections, creator details and scheduler bookkeeping are owned by their respective APIs.
func (r *HookResource) KnownIntentionallySkipped() []string { return []string{} }
func (r *HookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_hook"
}
func (r *HookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Versioned hook file executed for an agent event. Destroy removes the hook configuration; applying it does not execute the hook.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Id."},
		"agent_id":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Agent id."},
		"event":           schema.StringAttribute{Required: true, MarkdownDescription: "Event."},
		"file_name":       schema.StringAttribute{Required: true, MarkdownDescription: "File name."},
		"content":         schema.StringAttribute{Required: true, MarkdownDescription: "Content."},
		"requirements":    schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: "Requirements."},
		"enabled":         schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enabled."},
		"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization id."},
		"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Created at."},
		"updated_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Updated at."},
	}}
}
func (r *HookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *HookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), agent_hookAttrSpec, &resp.Diagnostics)
	patch["agentId"] = data.AgentID.ValueString()
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_agent_hook Create", patch, agent_hookAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateHookWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *HookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	agentID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}
	result, err := r.client.GetHooksWithResponse(ctx, &client.GetHooksParams{AgentId: agentID})
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	for _, item := range *result.JSON200 {
		if item.Id.String() == strings.SplitN(data.ID.ValueString(), ":", 2)[0] {
			body, err := json.Marshal(item)
			if err != nil {
				resp.Diagnostics.AddError("Response Encoding Failed", err.Error())
				return
			}
			r.flatten(ctx, &data, body, &resp.Diagnostics)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}
func (r *HookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(strings.SplitN(data.ID.ValueString(), ":", 2)[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, agent_hookAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_agent_hook Update", patch, agent_hookAttrSpec)
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateHookWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Update Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *HookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(strings.SplitN(data.ID.ValueString(), ":", 2)[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteHookWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *HookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Use hook_uuid:agent_uuid.")
		return
	}
	for _, part := range parts {
		if _, err := uuid.Parse(part); err != nil {
			resp.Diagnostics.AddError("Invalid Import ID", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), parts[1])...)
}
func (r *HookResource) flatten(ctx context.Context, data *HookResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID             *string  `json:"id"`
		AgentID        *string  `json:"agentId"`
		Event          *string  `json:"event"`
		FileName       *string  `json:"fileName"`
		Content        *string  `json:"content"`
		Requirements   []string `json:"requirements"`
		Enabled        *bool    `json:"enabled"`
		OrganizationID *string  `json:"organizationId"`
		CreatedAt      *string  `json:"createdAt"`
		UpdatedAt      *string  `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringValue(*value.ID + ":" + *value.AgentID)
	data.AgentID = types.StringPointerValue(value.AgentID)
	data.Event = types.StringPointerValue(value.Event)
	data.FileName = types.StringPointerValue(value.FileName)
	data.Content = types.StringPointerValue(value.Content)
	var d diag.Diagnostics
	data.Requirements, d = types.ListValueFrom(ctx, types.StringType, value.Requirements)
	diags.Append(d...)
	data.Enabled = types.BoolPointerValue(value.Enabled)
	data.OrganizationID = types.StringPointerValue(value.OrganizationID)
	data.CreatedAt = types.StringPointerValue(value.CreatedAt)
	data.UpdatedAt = types.StringPointerValue(value.UpdatedAt)
}
