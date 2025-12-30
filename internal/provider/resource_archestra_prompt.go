package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ArchestraPromptResource{}

func NewArchestraPromptResource() resource.Resource {
	return &ArchestraPromptResource{}
}

type ArchestraPromptResource struct {
	client *client.ClientWithResponses
}

type ArchestraPromptResourceModel struct {
	ID           types.String `tfsdk:"prompt_id"`
	ProfileId    types.String `tfsdk:"profile_id"`
	Name         types.String `tfsdk:"name"`
	SystemPrompt types.String `tfsdk:"system_prompt"`
	Prompt       types.String `tfsdk:"prompt"`
	IsActive     types.Bool   `tfsdk:"is_active"`
	Version      types.Int64  `tfsdk:"version"`
}

func (r *ArchestraPromptResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (r *ArchestraPromptResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"prompt_id": schema.StringAttribute{
				Computed: true,
			},
			"profile_id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"system_prompt": schema.StringAttribute{
				Optional: true,
			},
			"prompt": schema.StringAttribute{
				Required: true,
			},
			"is_active": schema.BoolAttribute{
				Optional: true,
			},
			"version": schema.Int64Attribute{
				Computed: true,
			},
		},
	}
}

func (r *ArchestraPromptResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ArchestraPromptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ArchestraPromptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	Profile_Id, err := uuid.Parse(data.ProfileId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to get Prompt: %s", err))
		return
	}

	body := client.CreatePromptJSONRequestBody{
		AgentId:      Profile_Id,
		Name:         data.Name.ValueString(),
		SystemPrompt: data.SystemPrompt.ValueStringPointer(),
		UserPrompt:   data.Prompt.ValueStringPointer(),
		IsActive:     data.IsActive.ValueBoolPointer(),
	}

	createResp, err := r.client.CreatePromptWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	data.ID = types.StringValue(createResp.JSON200.Id.String())
	data.Version = types.Int64Value(int64(createResp.JSON200.Version))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ArchestraPromptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ArchestraPromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to get Prompt: %s", err))
		return
	}

	getResp, err := r.client.GetPromptWithResponse(ctx, ID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	data.Name = types.StringValue(getResp.JSON200.Name)
	data.SystemPrompt = types.StringPointerValue(getResp.JSON200.SystemPrompt)
	data.Prompt = types.StringPointerValue(getResp.JSON200.UserPrompt)
	data.IsActive = types.BoolValue(getResp.JSON200.IsActive)
	data.Version = types.Int64Value(int64(getResp.JSON200.Version))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ArchestraPromptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ArchestraPromptResourceModel
	var state ArchestraPromptResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	Profile_Id, err := uuid.Parse(data.ProfileId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to get Prompt: %s", err))
		return
	}

	parentID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Parent ID", fmt.Sprintf("Unable to get Prompt: %s", err))
		return
	}

	// Create new version by referencing the parent prompt
	body := client.CreatePromptJSONRequestBody{
		AgentId:        Profile_Id,
		Name:           data.Name.ValueString(),
		SystemPrompt:   data.SystemPrompt.ValueStringPointer(),
		UserPrompt:     data.Prompt.ValueStringPointer(),
		IsActive:       data.IsActive.ValueBoolPointer(),
		ParentPromptId: &parentID,
	}

	createResp, err := r.client.CreatePromptWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	// Update Terraform state to new version
	data.ID = types.StringValue(createResp.JSON200.Id.String())
	data.Version = types.Int64Value(int64(createResp.JSON200.Version))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ArchestraPromptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ArchestraPromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to Delete Prompt: %s", err))
		return
	}

	delResp, err := r.client.DeletePromptWithResponse(ctx, ID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
	}

	if delResp.StatusCode() != 200 && delResp.StatusCode() != 400 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", delResp.StatusCode()),
		)
	}
}
