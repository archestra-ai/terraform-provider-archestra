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
	"time"
)

var _ resource.ResourceWithImportState = &KnowledgeDirectoryResource{}

func NewKnowledgeDirectoryResource() resource.Resource { return &KnowledgeDirectoryResource{} }

type KnowledgeDirectoryResource struct{ client *client.ClientWithResponses }
type KnowledgeDirectoryResourceModel struct {
	ID             types.String  `tfsdk:"id"`
	Name           types.String  `tfsdk:"name"`
	Visibility     types.String  `tfsdk:"visibility"`
	FileCount      types.Float64 `tfsdk:"file_count"`
	OrganizationID types.String  `tfsdk:"organization_id"`
	CreatedAt      types.String  `tfsdk:"created_at"`
	UpdatedAt      types.String  `tfsdk:"updated_at"`
}

var knowledge_directoryAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
}

func (r *KnowledgeDirectoryResource) AttrSpecs() []AttrSpec { return knowledge_directoryAttrSpec }
func (r *KnowledgeDirectoryResource) APIShape() any         { return client.GetKnowledgeDirectoriesResponse{} }

// Permission projections, creator details and scheduler bookkeeping are owned by their respective APIs.
func (r *KnowledgeDirectoryResource) KnownIntentionallySkipped() []string {
	return []string{"createdBy", "createdByServiceAccountId", "teamIds"}
}
func (r *KnowledgeDirectoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_directory"
}
func (r *KnowledgeDirectoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Named directory for uploaded knowledge files. Destroy deletes the directory and moves its files to the repository root.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Id."},
		"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Name."},
		"visibility":      schema.StringAttribute{Computed: true, MarkdownDescription: "Visibility."},
		"file_count":      schema.Float64Attribute{Computed: true, MarkdownDescription: "File count."},
		"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization id."},
		"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Created at."},
		"updated_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Updated at."},
	}}
}
func (r *KnowledgeDirectoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *KnowledgeDirectoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data KnowledgeDirectoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), knowledge_directoryAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_knowledge_directory Create", patch, knowledge_directoryAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateKnowledgeDirectoryWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
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
func (r *KnowledgeDirectoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KnowledgeDirectoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.GetKnowledgeDirectoriesWithResponse(ctx)
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
		if item.Id.String() == data.ID.ValueString() {
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
func (r *KnowledgeDirectoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data KnowledgeDirectoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, knowledge_directoryAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_knowledge_directory Update", patch, knowledge_directoryAttrSpec)
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateKnowledgeDirectoryWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
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
func (r *KnowledgeDirectoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KnowledgeDirectoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteKnowledgeDirectoryWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *KnowledgeDirectoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *KnowledgeDirectoryResource) flatten(_ context.Context, data *KnowledgeDirectoryResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID             *string    `json:"id"`
		Name           *string    `json:"name"`
		Visibility     *string    `json:"visibility"`
		FileCount      *float64   `json:"fileCount"`
		OrganizationID *string    `json:"organizationId"`
		CreatedAt      *time.Time `json:"createdAt"`
		UpdatedAt      *time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringPointerValue(value.ID)
	data.Name = types.StringPointerValue(value.Name)
	data.Visibility = types.StringPointerValue(value.Visibility)
	data.FileCount = types.Float64PointerValue(value.FileCount)
	data.OrganizationID = types.StringPointerValue(value.OrganizationID)
	data.CreatedAt = canonicalAPITimestamp(value.CreatedAt)
	data.UpdatedAt = canonicalAPITimestamp(value.UpdatedAt)
}
