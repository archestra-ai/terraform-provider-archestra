package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PromptResource{}
var _ resource.ResourceWithImportState = &PromptResource{}
var _ resource.ResourceWithModifyPlan = &PromptResource{}

func NewPromptResource() resource.Resource {
	return &PromptResource{}
}

// PromptResource defines the resource implementation.
type PromptResource struct {
	client *client.ClientWithResponses
}

// PromptResourceModel is now redundant but kept for type safety if needed elsewhere,
// though we'll use PromptModel from prompt_shared.go.
type PromptResourceModel = PromptModel

func (r *PromptResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (r *PromptResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a prompt in the Archestra private prompt registry.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Prompt identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_id": schema.StringAttribute{
				MarkdownDescription: "The profile identifier this prompt belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the prompt",
				Required:            true,
			},
			"system_prompt": schema.StringAttribute{
				MarkdownDescription: "The system prompt template",
				Optional:            true,
			},
			"user_prompt": schema.StringAttribute{
				MarkdownDescription: "The user prompt template",
				Optional:            true,
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "The version of the prompt",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the prompt was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the prompt was last updated",
				Computed:            true,
			},
		},
	}
}

func (r *PromptResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *PromptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PromptResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile UUID: %s", err))
		return
	}

	requestBody := client.CreatePromptJSONRequestBody{
		AgentId: agentID,
		Name:    data.Name.ValueString(),
	}

	if !data.SystemPrompt.IsNull() && !data.SystemPrompt.IsUnknown() {
		val := data.SystemPrompt.ValueString()
		requestBody.SystemPrompt = &val
	}
	if !data.UserPrompt.IsNull() && !data.UserPrompt.IsUnknown() {
		val := data.UserPrompt.ValueString()
		requestBody.UserPrompt = &val
	}

	apiResp, err := r.client.CreatePromptWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create prompt, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	respItem := apiResp.JSON200
	mapPromptResponseToModel(&struct {
		AgentId        openapi_types.UUID `json:"agentId"`
		CreatedAt      time.Time          `json:"createdAt"`
		Id             openapi_types.UUID `json:"id"`
		Name           string             `json:"name"`
		OrganizationId string             `json:"organizationId"`
		SystemPrompt   *string            `json:"systemPrompt"`
		UpdatedAt      time.Time          `json:"updatedAt"`
		UserPrompt     *string            `json:"userPrompt"`
		Version        int                `json:"version"`
	}{
		AgentId:        respItem.AgentId,
		CreatedAt:      respItem.CreatedAt,
		Id:             respItem.Id,
		Name:           respItem.Name,
		OrganizationId: respItem.OrganizationId,
		SystemPrompt:   respItem.SystemPrompt,
		UpdatedAt:      respItem.UpdatedAt,
		UserPrompt:     respItem.UserPrompt,
		Version:        respItem.Version,
	}, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PromptResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	promptID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse prompt ID: %s", err))
		return
	}

	apiResp, err := r.client.GetPromptWithResponse(ctx, promptID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read prompt, got error: %s", err))
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

	respItem := apiResp.JSON200
	mapPromptResponseToModel(&struct {
		AgentId        openapi_types.UUID `json:"agentId"`
		CreatedAt      time.Time          `json:"createdAt"`
		Id             openapi_types.UUID `json:"id"`
		Name           string             `json:"name"`
		OrganizationId string             `json:"organizationId"`
		SystemPrompt   *string            `json:"systemPrompt"`
		UpdatedAt      time.Time          `json:"updatedAt"`
		UserPrompt     *string            `json:"userPrompt"`
		Version        int                `json:"version"`
	}{
		AgentId:        respItem.AgentId,
		CreatedAt:      respItem.CreatedAt,
		Id:             respItem.Id,
		Name:           respItem.Name,
		OrganizationId: respItem.OrganizationId,
		SystemPrompt:   respItem.SystemPrompt,
		UpdatedAt:      respItem.UpdatedAt,
		UserPrompt:     respItem.UserPrompt,
		Version:        respItem.Version,
	}, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, state PromptResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	promptID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse prompt ID from state: %s", err))
		return
	}

	agentID, err := uuid.Parse(data.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Profile ID", fmt.Sprintf("Unable to parse profile UUID: %s", err))
		return
	}

	name := data.Name.ValueString()
	requestBody := client.UpdatePromptJSONRequestBody{
		AgentId: &agentID,
		Name:    &name,
	}

	if !data.SystemPrompt.IsNull() && !data.SystemPrompt.IsUnknown() {
		val := data.SystemPrompt.ValueString()
		requestBody.SystemPrompt = &val
	}
	if !data.UserPrompt.IsNull() && !data.UserPrompt.IsUnknown() {
		val := data.UserPrompt.ValueString()
		requestBody.UserPrompt = &val
	}

	apiResp, err := r.client.UpdatePromptWithResponse(ctx, promptID, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update prompt, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	respItem := apiResp.JSON200
	mapPromptResponseToModel(&struct {
		AgentId        openapi_types.UUID `json:"agentId"`
		CreatedAt      time.Time          `json:"createdAt"`
		Id             openapi_types.UUID `json:"id"`
		Name           string             `json:"name"`
		OrganizationId string             `json:"organizationId"`
		SystemPrompt   *string            `json:"systemPrompt"`
		UpdatedAt      time.Time          `json:"updatedAt"`
		UserPrompt     *string            `json:"userPrompt"`
		Version        int                `json:"version"`
	}{
		AgentId:        respItem.AgentId,
		CreatedAt:      respItem.CreatedAt,
		Id:             respItem.Id,
		Name:           respItem.Name,
		OrganizationId: respItem.OrganizationId,
		SystemPrompt:   respItem.SystemPrompt,
		UpdatedAt:      respItem.UpdatedAt,
		UserPrompt:     respItem.UserPrompt,
		Version:        respItem.Version,
	}, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PromptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PromptResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	promptID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse prompt ID: %s", err))
		return
	}

	apiResp, err := r.client.DeletePromptWithResponse(ctx, promptID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete prompt, got error: %s", err))
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

func (r *PromptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ModifyPlan handles the versioning behavior of Archestra prompts.
// Whenever the name, system_prompt, or user_prompt is changed, the Archestra API
// creates a new version of the prompt, resulting in a new ID. We mark the ID
// as unknown in the plan to inform Terraform that a new ID will be generated.
func (r *PromptResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() {
		return
	}

	if req.Plan.Raw.IsNull() {
		return
	}

	var state, plan PromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Name.Equal(state.Name) ||
		!plan.SystemPrompt.Equal(state.SystemPrompt) ||
		!plan.UserPrompt.Equal(state.UserPrompt) {
		plan.ID = types.StringUnknown()
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	}
}
