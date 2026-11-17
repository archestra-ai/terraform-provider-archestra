package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &LlmModelDataSource{}

func NewLlmModelDataSource() datasource.DataSource {
	return &LlmModelDataSource{}
}

// LlmModelDataSource finds a model the platform discovered, by the name the
// provider knows it by, so configuration can pin it without a pasted UUID.
type LlmModelDataSource struct {
	client *client.ClientWithResponses
}

type LlmModelDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	ModelID       types.String `tfsdk:"model_id"`
	LLMProvider   types.String `tfsdk:"llm_provider"`
	Description   types.String `tfsdk:"description"`
	ContextLength types.Int64  `tfsdk:"context_length"`
	ApiKeyIDs     types.List   `tfsdk:"api_key_ids"`
}

func (d *LlmModelDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (d *LlmModelDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds an LLM model the platform discovered from a provider key, by its provider-facing name. Use `id` wherever a model's database UUID is expected, such as `model_id` on `archestra_agent`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Database UUID of the model.",
				Computed:            true,
			},
			"model_id": schema.StringAttribute{
				MarkdownDescription: "Provider-facing model name, such as `claude-sonnet-4-5`.",
				Required:            true,
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "Provider that serves the model, such as `anthropic`. Required when several providers expose the same `model_id`.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the model, when the provider gives one.",
				Computed:            true,
			},
			"context_length": schema.Int64Attribute{
				MarkdownDescription: "Context window in tokens, when known.",
				Computed:            true,
			},
			"api_key_ids": schema.ListAttribute{
				MarkdownDescription: "IDs of the provider keys the model is reachable through, in the order the API lists them.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *LlmModelDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = apiClient
}

func (d *LlmModelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data LlmModelDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	includeAssignments := false
	models, err := d.client.GetModelsWithApiKeysWithResponse(ctx, &client.GetModelsWithApiKeysParams{IncludeAssignments: &includeAssignments})
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list models: %s", err))
		return
	}
	if models.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", models.StatusCode()))
		return
	}

	wanted := data.ModelID.ValueString()
	provider := data.LLMProvider.ValueString()
	found := -1
	for index, model := range *models.JSON200 {
		if model.ModelId != wanted || (provider != "" && provider != string(model.Provider)) {
			continue
		}
		if found >= 0 {
			resp.Diagnostics.AddError("Ambiguous Model", fmt.Sprintf("Several providers expose the model %q. Set llm_provider to select one.", wanted))
			return
		}
		found = index
	}
	if found < 0 {
		resp.Diagnostics.AddError("Model Not Found", fmt.Sprintf("No model %q is visible to this API key. Models are discovered from configured provider keys.", wanted))
		return
	}

	model := (*models.JSON200)[found]
	data.ID = types.StringValue(model.Id.String())
	data.LLMProvider = types.StringValue(string(model.Provider))
	data.Description = types.StringPointerValue(model.Description)
	data.ContextLength = types.Int64Null()
	if model.ContextLength != nil {
		data.ContextLength = types.Int64Value(int64(*model.ContextLength))
	}
	keyIDs := make([]string, 0, len(model.ApiKeys))
	for _, key := range model.ApiKeys {
		keyIDs = append(keyIDs, key.Id.String())
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, keyIDs)
	resp.Diagnostics.Append(diags...)
	data.ApiKeyIDs = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
