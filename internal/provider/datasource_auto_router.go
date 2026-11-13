package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AutoRouterDataSource{}

func NewAutoRouterDataSource() datasource.DataSource { return &AutoRouterDataSource{} }

type AutoRouterDataSource struct{ client *client.AutoRouterClient }

func (d *AutoRouterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auto_router"
}

func (d *AutoRouterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	tier := func(name string) schema.SingleNestedAttribute {
		return schema.SingleNestedAttribute{Computed: true, MarkdownDescription: fmt.Sprintf("The %s tier; null when its model or key was deleted.", name), Attributes: map[string]schema.Attribute{
			"model_id":            schema.StringAttribute{Computed: true, MarkdownDescription: "Database UUID of the tier's model row."},
			"provider_api_key_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Provider key UUID that serves the tier."},
		}}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "ALPHA. Looks up an auto router by the model name clients request (`auto` or `auto:<name>`), for example to reference a router created in the UI. Requires API 1.5.0-rc.29 or later with `ARCHESTRA_AUTO_ROUTING_ENABLED=true` (or `ARCHESTRA_BETA=true`).",
		Attributes: map[string]schema.Attribute{
			"name":               schema.StringAttribute{Required: true, MarkdownDescription: "Router model name: `auto` or `auto:<name>`."},
			"id":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Auto router UUID."},
			"budget_usd_per_day": schema.Float64Attribute{Computed: true, MarkdownDescription: "Custom daily budget in USD, if any."},
			"budget_preset":      schema.StringAttribute{Computed: true, MarkdownDescription: "Budget preset, if any."},
			"tiers": schema.SingleNestedAttribute{Computed: true, MarkdownDescription: "The router's fast, normal and smart tiers.", Attributes: map[string]schema.Attribute{
				"fast": tier("fast"), "normal": tier("normal"), "smart": tier("smart"),
			}},
			"decision_model": schema.SingleNestedAttribute{Computed: true, MarkdownDescription: "Model that scores sessions.", Attributes: map[string]schema.Attribute{
				"api_key_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Provider key for the decision model; null uses the organization's primary decisions key."},
				"model":      schema.StringAttribute{Computed: true, MarkdownDescription: "Provider-facing model name of the decision model."},
			}},
			"enabled":             schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the router currently routes sessions."},
			"virtual_api_key_ids": schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Virtual keys granted this router."},
		},
	}
}

func (d *AutoRouterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	var err error
	d.client, err = client.NewAutoRouterClient(api)
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Client Configuration Failed", err.Error())
	}
}

func (d *AutoRouterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AutoRouterResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	routers, err := d.client.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Auto Router Read Failed", err.Error())
		return
	}
	for i := range routers {
		if routers[i].Name == data.Name.ValueString() {
			flattenAutoRouter(ctx, &data, &routers[i], &resp.Diagnostics)
			if !resp.Diagnostics.HasError() {
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			}
			return
		}
	}
	resp.Diagnostics.AddError("Auto Router Not Found", fmt.Sprintf("No auto router is named %q.", data.Name.ValueString()))
}
