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

var _ datasource.DataSource = &AgentLabelKeysDataSource{}

func NewAgentLabelKeysDataSource() datasource.DataSource {
	return &AgentLabelKeysDataSource{}
}

type AgentLabelKeysDataSource struct {
	client *client.ClientWithResponses
}

type AgentLabelKeysDataSourceModel struct {
	Keys types.List `tfsdk:"keys"`
}

func (d *AgentLabelKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_label_keys"
}

func (d *AgentLabelKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "All label keys currently in use across agents in the organization. Useful for validating that a desired key already exists before referencing it in a tool-invocation policy condition.",
		Attributes: map[string]schema.Attribute{
			"keys": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Distinct label keys present on at least one agent.",
			},
		},
	}
}

func (d *AgentLabelKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AgentLabelKeysDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.GetLabelKeysWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list agent label keys: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	keys := make([]attr.Value, len(*apiResp.JSON200))
	for i, k := range *apiResp.JSON200 {
		keys[i] = types.StringValue(k)
	}
	listVal, diags := types.ListValue(types.StringType, keys)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := AgentLabelKeysDataSourceModel{Keys: listVal}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
