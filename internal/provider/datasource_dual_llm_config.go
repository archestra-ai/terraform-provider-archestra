package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ datasource.DataSource = &DualLlmConfigDataSource{}

func NewDualLlmConfigDataSource() datasource.DataSource {
	return &DualLlmConfigDataSource{}
}

type DualLlmConfigDataSource struct {
	client *client.ClientWithResponses
}

type DualLlmConfigDataSourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	MainAgentPrompt        types.String `tfsdk:"main_agent_prompt"`
	MaxRounds              types.Int64  `tfsdk:"max_rounds"`
	QuarantinedAgentPrompt types.String `tfsdk:"quarantined_agent_prompt"`
	SummaryPrompt          types.String `tfsdk:"summary_prompt"`
	CreatedAt              types.String `tfsdk:"created_at"`
	UpdatedAt              types.String `tfsdk:"updated_at"`
}

func (d *DualLlmConfigDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dual_llm_config"
}

func (d *DualLlmConfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Dual LLM Security Config by ID. If name is provided, lists all configs and filters by name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Dual LLM Config identifier. Either id or name must be provided.",
				Optional:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the Dual LLM Config. Either id or name must be provided. Note: Name filtering is not currently supported by the API, so this field is reserved for future use.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the dual LLM config is enabled",
				Computed:            true,
			},
			"main_agent_prompt": schema.StringAttribute{
				MarkdownDescription: "Prompt for the main agent",
				Computed:            true,
			},
			"max_rounds": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of rounds",
				Computed:            true,
			},
			"quarantined_agent_prompt": schema.StringAttribute{
				MarkdownDescription: "Prompt for the quarantined agent",
				Computed:            true,
			},
			"summary_prompt": schema.StringAttribute{
				MarkdownDescription: "Prompt for the summary",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the config was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the config was last updated",
				Computed:            true,
			},
		},
	}
}

func (d *DualLlmConfigDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *DualLlmConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DualLlmConfigDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'name' must be provided to look up a dual LLM config.",
		)
		return
	}

	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		configID, err := uuid.Parse(data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse dual LLM config ID: %s", err))
			return
		}

		apiResp, err := d.client.GetDualLlmConfigWithResponse(ctx, configID)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read dual LLM config, got error: %s", err))
			return
		}

		if apiResp.JSON404 != nil {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Dual LLM config with ID %s not found", data.ID.ValueString()))
			return
		}

		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
			return
		}

		d.mapAPIResponseToModel(apiResp.JSON200, &data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		// List all configs
		configsResp, err := d.client.GetDualLlmConfigsWithResponse(ctx)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list dual LLM configs, got error: %s", err))
			return
		}

		if configsResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", configsResp.StatusCode()))
			return
		}

		targetName := data.Name.ValueString()

		resp.Diagnostics.AddError(
			"Name Lookup Not Supported",
			fmt.Sprintf("Name-based lookup is not currently supported because the API does not expose a name field for dual LLM configs. Please use 'id' instead. (Requested name: %s)", targetName),
		)
		return
	}
}

func (d *DualLlmConfigDataSource) mapAPIResponseToModel(apiConfig *struct {
	CreatedAt              time.Time          `json:"createdAt"`
	Enabled                bool               `json:"enabled"`
	Id                     openapi_types.UUID `json:"id"`
	MainAgentPrompt        string             `json:"mainAgentPrompt"`
	MaxRounds              int                `json:"maxRounds"`
	QuarantinedAgentPrompt string             `json:"quarantinedAgentPrompt"`
	SummaryPrompt          string             `json:"summaryPrompt"`
	UpdatedAt              time.Time          `json:"updatedAt"`
}, data *DualLlmConfigDataSourceModel) {
	data.ID = types.StringValue(apiConfig.Id.String())
	data.Enabled = types.BoolValue(apiConfig.Enabled)
	data.MainAgentPrompt = types.StringValue(apiConfig.MainAgentPrompt)
	data.MaxRounds = types.Int64Value(int64(apiConfig.MaxRounds))
	data.QuarantinedAgentPrompt = types.StringValue(apiConfig.QuarantinedAgentPrompt)
	data.SummaryPrompt = types.StringValue(apiConfig.SummaryPrompt)
	data.CreatedAt = types.StringValue(apiConfig.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(apiConfig.UpdatedAt.Format(time.RFC3339))
}
