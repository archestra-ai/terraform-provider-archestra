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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DualLlmConfigResource{}
var _ resource.ResourceWithImportState = &DualLlmConfigResource{}

func NewDualLlmConfigResource() resource.Resource {
	return &DualLlmConfigResource{}
}

// DualLlmConfigResource defines the resource implementation.
type DualLlmConfigResource struct {
	client *client.ClientWithResponses
}

// DualLlmConfigResourceModel describes the resource data model.
type DualLlmConfigResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	MainAgentPrompt        types.String `tfsdk:"main_agent_prompt"`
	MaxRounds              types.Int64  `tfsdk:"max_rounds"`
	QuarantinedAgentPrompt types.String `tfsdk:"quarantined_agent_prompt"`
	SummaryPrompt          types.String `tfsdk:"summary_prompt"`
	CreatedAt              types.String `tfsdk:"created_at"`
	UpdatedAt              types.String `tfsdk:"updated_at"`
}

func (r *DualLlmConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dual_llm_config"
}

func (r *DualLlmConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Dual LLM Security Config in Archestra.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Dual LLM Config identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the dual LLM config is enabled",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"main_agent_prompt": schema.StringAttribute{
				MarkdownDescription: "Prompt for the main agent",
				Required:            true,
			},
			"max_rounds": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of rounds",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"quarantined_agent_prompt": schema.StringAttribute{
				MarkdownDescription: "Prompt for the quarantined agent",
				Required:            true,
			},
			"summary_prompt": schema.StringAttribute{
				MarkdownDescription: "Prompt for the summary",
				Required:            true,
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the config was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the config was last updated",
			},
		},
	}
}

func (r *DualLlmConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DualLlmConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DualLlmConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := client.CreateDualLlmConfigJSONRequestBody{
		MainAgentPrompt:        data.MainAgentPrompt.ValueString(),
		QuarantinedAgentPrompt: data.QuarantinedAgentPrompt.ValueString(),
		SummaryPrompt:          data.SummaryPrompt.ValueString(),
	}

	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		enabled := data.Enabled.ValueBool()
		requestBody.Enabled = &enabled
	}

	if !data.MaxRounds.IsNull() && !data.MaxRounds.IsUnknown() {
		maxRounds := int(data.MaxRounds.ValueInt64())
		requestBody.MaxRounds = &maxRounds
	}

	apiResp, err := r.client.CreateDualLlmConfigWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create dual LLM config, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.ID = types.StringValue(apiResp.JSON200.Id.String())
	data.Enabled = types.BoolValue(apiResp.JSON200.Enabled)
	data.MainAgentPrompt = types.StringValue(apiResp.JSON200.MainAgentPrompt)
	data.MaxRounds = types.Int64Value(int64(apiResp.JSON200.MaxRounds))
	data.QuarantinedAgentPrompt = types.StringValue(apiResp.JSON200.QuarantinedAgentPrompt)
	data.SummaryPrompt = types.StringValue(apiResp.JSON200.SummaryPrompt)
	data.CreatedAt = types.StringValue(apiResp.JSON200.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(apiResp.JSON200.UpdatedAt.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DualLlmConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DualLlmConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	configID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse dual LLM config ID: %s", err))
		return
	}

	apiResp, err := r.client.GetDualLlmConfigWithResponse(ctx, configID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read dual LLM config, got error: %s", err))
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

	data.Enabled = types.BoolValue(apiResp.JSON200.Enabled)
	data.MainAgentPrompt = types.StringValue(apiResp.JSON200.MainAgentPrompt)
	data.MaxRounds = types.Int64Value(int64(apiResp.JSON200.MaxRounds))
	data.QuarantinedAgentPrompt = types.StringValue(apiResp.JSON200.QuarantinedAgentPrompt)
	data.SummaryPrompt = types.StringValue(apiResp.JSON200.SummaryPrompt)
	data.CreatedAt = types.StringValue(apiResp.JSON200.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(apiResp.JSON200.UpdatedAt.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DualLlmConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DualLlmConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	configID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse dual LLM config ID: %s", err))
		return
	}

	requestBody := client.UpdateDualLlmConfigJSONRequestBody{}

	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		enabled := data.Enabled.ValueBool()
		requestBody.Enabled = &enabled
	}

	mainAgentPrompt := data.MainAgentPrompt.ValueString()
	requestBody.MainAgentPrompt = &mainAgentPrompt

	if !data.MaxRounds.IsNull() && !data.MaxRounds.IsUnknown() {
		maxRounds := int(data.MaxRounds.ValueInt64())
		requestBody.MaxRounds = &maxRounds
	}

	quarantinedAgentPrompt := data.QuarantinedAgentPrompt.ValueString()
	requestBody.QuarantinedAgentPrompt = &quarantinedAgentPrompt

	summaryPrompt := data.SummaryPrompt.ValueString()
	requestBody.SummaryPrompt = &summaryPrompt

	// Call API
	apiResp, err := r.client.UpdateDualLlmConfigWithResponse(ctx, configID, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update dual LLM config, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	data.Enabled = types.BoolValue(apiResp.JSON200.Enabled)
	data.MainAgentPrompt = types.StringValue(apiResp.JSON200.MainAgentPrompt)
	data.MaxRounds = types.Int64Value(int64(apiResp.JSON200.MaxRounds))
	data.QuarantinedAgentPrompt = types.StringValue(apiResp.JSON200.QuarantinedAgentPrompt)
	data.SummaryPrompt = types.StringValue(apiResp.JSON200.SummaryPrompt)
	data.CreatedAt = types.StringValue(apiResp.JSON200.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(apiResp.JSON200.UpdatedAt.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DualLlmConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DualLlmConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	configID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse dual LLM config ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteDualLlmConfigWithResponse(ctx, configID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete dual LLM config, got error: %s", err))
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

func (r *DualLlmConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
