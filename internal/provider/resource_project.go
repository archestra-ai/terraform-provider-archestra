package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource { return &ProjectResource{} }

type ProjectResource struct{ client *client.ClientWithResponses }
type ProjectResourceModel struct {
	ID                types.String      `tfsdk:"id"`
	Name              types.String      `tfsdk:"name"`
	Description       types.String      `tfsdk:"description"`
	Icon              types.String      `tfsdk:"icon"`
	DefaultAgentID    types.String      `tfsdk:"default_agent_id"`
	Labels            []AgentLabelModel `tfsdk:"labels"`
	ViewerRole        types.String      `tfsdk:"viewer_role"`
	OwnerName         types.String      `tfsdk:"owner_name"`
	ConversationCount types.Int64       `tfsdk:"conversation_count"`
	CreatedAt         types.String      `tfsdk:"created_at"`
}

var projectAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "description", JSONName: "description", Kind: Scalar},
	{TFName: "icon", JSONName: "icon", Kind: Scalar},
	{TFName: "default_agent_id", JSONName: "defaultAgentId", Kind: Scalar},
	{TFName: "labels", JSONName: "labels", Kind: Set, Children: []AttrSpec{{TFName: "key", JSONName: "key", Kind: Scalar}, {TFName: "value", JSONName: "value", Kind: Scalar}}},
}

func (r *ProjectResource) AttrSpecs() []AttrSpec { return projectAttrSpec }
func (r *ProjectResource) APIShape() any         { return client.GetProjectResponse{} }

// Grants own sharing; pinning is per-viewer UI state. Creator metadata and soft
// deletion are backend-managed. defaultAgent is represented by default_agent_id.
func (r *ProjectResource) KnownIntentionallySkipped() []string {
	return []string{"createdBy", "visibility", "shareTeamNames", "shareUserNames", "shareTeamIds", "shareUserIds", "pinnedAt", "deletedAt", "defaultAgent"}
}
func (r *ProjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}
func (r *ProjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "A project containing conversations and schedules. Configure sharing with a separate archestra_resource_permission_policy. Destroy uses the project's soft-delete API; retain backups of project content before deleting it.", Attributes: map[string]schema.Attribute{
		"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":             schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 64)}},
		"description":      schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(200)}},
		"icon":             schema.StringAttribute{Optional: true},
		"default_agent_id": schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.RegexMatches(uuidRegexp, "must be a UUID")}},
		"labels":           schema.SetNestedAttribute{Optional: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"key": schema.StringAttribute{Required: true}, "value": schema.StringAttribute{Required: true}}}},
		"viewer_role":      schema.StringAttribute{Computed: true}, "owner_name": schema.StringAttribute{Computed: true}, "conversation_count": schema.Int64Attribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *ProjectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), projectAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	created, err := r.client.CreateProjectWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if created.StatusCode() != 200 && created.StatusCode() != 201 {
		resp.Diagnostics.AddError("Project Creation Failed", fmt.Sprintf("API returned status %d.", created.StatusCode()))
		return
	}
	var wire struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body, &wire); err != nil || wire.ID == "" {
		resp.Diagnostics.AddError("Invalid Project Response", "The API did not return the created project's ID.")
		return
	}
	data.ID = types.StringValue(wire.ID)
	data.ViewerRole = types.StringNull()
	data.OwnerName = types.StringNull()
	data.ConversationCount = types.Int64Null()
	data.CreatedAt = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, projectAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Project ID", err.Error())
		return
	}
	updated, err := r.client.UpdateProjectWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if updated.StatusCode() != 200 {
		resp.Diagnostics.AddError("Project Update Failed", fmt.Sprintf("API returned status %d. Refresh before retrying.", updated.StatusCode()))
		return
	}
	if !r.refresh(ctx, &data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Project ID", err.Error())
		return
	}
	deleted, err := r.client.DeleteProjectWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if deleted.StatusCode() != 200 && deleted.StatusCode() != 204 && deleted.StatusCode() != 404 {
		resp.Diagnostics.AddError("Project Deletion Failed", fmt.Sprintf("API returned status %d.", deleted.StatusCode()))
	}
}
func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Project ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *ProjectResource) refresh(ctx context.Context, data *ProjectResourceModel, ds *diag.Diagnostics) bool {
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		return false
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		ds.AddError("Invalid Project ID", err.Error())
		return false
	}
	response, err := r.client.GetProjectWithResponse(ctx, id)
	if err != nil {
		ds.AddError("API Error", err.Error())
		return false
	}
	if response.StatusCode() == 404 {
		return false
	}
	if response.JSON200 == nil {
		ds.AddError("Project Read Failed", fmt.Sprintf("API returned status %d.", response.StatusCode()))
		return false
	}
	wire := response.JSON200
	data.Name = types.StringValue(wire.Name)
	data.Description = types.StringPointerValue(wire.Description)
	data.Icon = types.StringPointerValue(wire.Icon)
	data.DefaultAgentID = types.StringNull()
	if wire.DefaultAgent != nil {
		data.DefaultAgentID = types.StringValue(wire.DefaultAgent.Id.String())
	}
	data.Labels = flattenAgentLabels(data.Labels, wire.Labels)
	data.ViewerRole = types.StringValue(string(wire.ViewerRole))
	data.OwnerName = types.StringPointerValue(wire.OwnerName)
	data.ConversationCount = types.Int64Value(int64(wire.ConversationCount))
	data.CreatedAt = types.StringValue(wire.CreatedAt.Format(time.RFC3339Nano))
	return true
}
