package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ModelProviderDataSource{}

func NewModelProviderDataSource() datasource.DataSource {
	return &ModelProviderDataSource{}
}

// ModelProviderDataSource finds a provider key that exists already, by name or
// as a provider's primary key, so configuration can pin it without a pasted
// UUID. It never returns the key's secret.
type ModelProviderDataSource struct {
	client *client.ClientWithResponses
}

type ModelProviderDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	LLMProvider types.String `tfsdk:"llm_provider"`
	IsPrimary   types.Bool   `tfsdk:"is_primary"`
	Scope       types.String `tfsdk:"scope"`
	TeamID      types.String `tfsdk:"team_id"`
	BaseUrl     types.String `tfsdk:"base_url"`
}

func (d *ModelProviderDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_provider"
}

func (d *ModelProviderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Finds an LLM provider key that exists already, by `name`, by `llm_provider` and `is_primary`, or both. Exactly one key must match. Use `id` wherever a provider key ID is expected, such as `llm_api_key_id` on `archestra_agent`. The key's secret is never read.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the provider key.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Exact name of the key. Set this, `llm_provider`, or both.",
				Optional:            true,
				Computed:            true,
			},
			"llm_provider": schema.StringAttribute{
				MarkdownDescription: "Provider the key belongs to, such as `anthropic`.",
				Optional:            true,
				Computed:            true,
			},
			"is_primary": schema.BoolAttribute{
				MarkdownDescription: "When set, match only keys that are (`true`) or are not (`false`) their provider's primary key. With `llm_provider`, `true` selects that provider's primary key.",
				Optional:            true,
				Computed:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "Who the key is shared with: `personal`, `team` or `org`.",
				Computed:            true,
			},
			"team_id": schema.StringAttribute{
				MarkdownDescription: "Team the key belongs to, for a team key.",
				Computed:            true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Custom base URL of the key, when it has one.",
				Computed:            true,
			},
		},
	}
}

func (d *ModelProviderDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = dataSourceClient(req, resp)
}

func (d *ModelProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ModelProviderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name, provider := data.Name.ValueString(), data.LLMProvider.ValueString()
	if name == "" && provider == "" {
		resp.Diagnostics.AddError("Missing Lookup", "Set name, llm_provider, or both.")
		return
	}

	params := &client.GetLlmProviderApiKeysParams{}
	if provider != "" {
		wireProvider := client.GetLlmProviderApiKeysParamsProvider(provider)
		params.Provider = &wireProvider
	}
	keys, err := d.client.GetLlmProviderApiKeysWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list provider keys: %s", err))
		return
	}
	if keys.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d", keys.StatusCode()))
		return
	}

	found := -1
	for index, key := range *keys.JSON200 {
		if (name != "" && key.Name != name) || (provider != "" && string(key.Provider) != provider) {
			continue
		}
		if !data.IsPrimary.IsNull() && !data.IsPrimary.IsUnknown() && data.IsPrimary.ValueBool() != key.IsPrimary {
			continue
		}
		if found >= 0 {
			resp.Diagnostics.AddError("Ambiguous Provider Key", "Several provider keys match. Narrow the lookup with name, llm_provider or is_primary.")
			return
		}
		found = index
	}
	if found < 0 {
		resp.Diagnostics.AddError("Provider Key Not Found", "No provider key visible to this API key matches the lookup.")
		return
	}

	key := (*keys.JSON200)[found]
	data.ID = types.StringValue(key.Id.String())
	data.Name = types.StringValue(key.Name)
	data.LLMProvider = types.StringValue(string(key.Provider))
	data.IsPrimary = types.BoolValue(key.IsPrimary)
	data.Scope = types.StringValue(string(key.Scope))
	data.TeamID = types.StringPointerValue(key.TeamId)
	data.BaseUrl = types.StringPointerValue(key.BaseUrl)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
