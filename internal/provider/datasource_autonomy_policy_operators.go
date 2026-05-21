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

var _ datasource.DataSource = &AutonomyPolicyOperatorsDataSource{}

func NewAutonomyPolicyOperatorsDataSource() datasource.DataSource {
	return &AutonomyPolicyOperatorsDataSource{}
}

type AutonomyPolicyOperatorsDataSource struct {
	client *client.ClientWithResponses
}

type AutonomyPolicyOperatorsDataSourceModel struct {
	Operators types.List `tfsdk:"operators"`
}

var autonomyPolicyOperatorObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"value": types.StringType,
	"label": types.StringType,
}}

func (d *AutonomyPolicyOperatorsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autonomy_policy_operators"
}

func (d *AutonomyPolicyOperatorsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Operators supported by `archestra_tool_invocation_policy` and `archestra_trusted_data_policy` condition rules — pulled live from the backend so HCL stays in sync with the platform version.",
		Attributes: map[string]schema.Attribute{
			"operators": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Supported operators with their wire `value` (use this in `conditions[*].operator`) and a human-friendly `label`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"value": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Operator wire identifier (e.g. `equal`, `contains`, `startsWith`).",
						},
						"label": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable label (e.g. `Starts With`).",
						},
					},
				},
			},
		},
	}
}

func (d *AutonomyPolicyOperatorsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AutonomyPolicyOperatorsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.GetOperatorsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list policy operators: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	elems := make([]attr.Value, len(*apiResp.JSON200))
	for i, op := range *apiResp.JSON200 {
		obj, diags := types.ObjectValue(autonomyPolicyOperatorObjectType.AttrTypes, map[string]attr.Value{
			"value": types.StringValue(string(op.Value)),
			"label": types.StringValue(op.Label),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		elems[i] = obj
	}
	listVal, diags := types.ListValue(autonomyPolicyOperatorObjectType, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := AutonomyPolicyOperatorsDataSourceModel{Operators: listVal}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
