package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AgentLabelValuesDataSource{}

func NewAgentLabelValuesDataSource() datasource.DataSource {
	return &AgentLabelValuesDataSource{}
}

type AgentLabelValuesDataSource struct {
	client *client.ClientWithResponses
}

type AgentLabelValuesDataSourceModel struct {
	Key    types.String `tfsdk:"key"`
	Values types.List   `tfsdk:"values"`
}

func (d *AgentLabelValuesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_label_values"
}

func (d *AgentLabelValuesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "All label values currently in use across agents, optionally filtered by `key`. Pair with `archestra_agent_label_keys` to drive policy-condition input validation in HCL.",
		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional label key to scope results.",
			},
			"values": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Distinct label values across agents (filtered by `key` when set).",
			},
		},
	}
}

func (d *AgentLabelValuesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *AgentLabelValuesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentLabelValuesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &client.GetLabelValuesParams{}
	if !data.Key.IsNull() && !data.Key.IsUnknown() {
		s := data.Key.ValueString()
		params.Key = &s
	}

	apiResp, err := d.client.GetLabelValuesWithResponse(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list agent label values: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	values := make([]attr.Value, len(*apiResp.JSON200))
	for i, v := range *apiResp.JSON200 {
		values[i] = types.StringValue(v)
	}
	listVal, diags := types.ListValue(types.StringType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Values = listVal
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
