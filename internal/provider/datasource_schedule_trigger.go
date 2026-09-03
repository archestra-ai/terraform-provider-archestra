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
)

var _ datasource.DataSource = &ScheduleTriggerDataSource{}

func NewScheduleTriggerDataSource() datasource.DataSource {
	return &ScheduleTriggerDataSource{}
}

type ScheduleTriggerDataSource struct {
	client *client.ClientWithResponses
}

type ScheduleTriggerDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	AgentID         types.String `tfsdk:"agent_id"`
	CronExpression  types.String `tfsdk:"cron_expression"`
	Timezone        types.String `tfsdk:"timezone"`
	MessageTemplate types.String `tfsdk:"message_template"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	LastExecutedAt  types.String `tfsdk:"last_executed_at"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func (d *ScheduleTriggerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_trigger"
}

func (d *ScheduleTriggerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lookup an existing `archestra_schedule_trigger` by UUID. Pair with `data.archestra_schedule_trigger_runs` to inspect execution history without managing the trigger itself.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Required: true, MarkdownDescription: "Schedule trigger UUID."},
			"name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Trigger name."},
			"agent_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Target agent UUID."},
			"cron_expression":  schema.StringAttribute{Computed: true, MarkdownDescription: "Cron expression that drives scheduling."},
			"timezone":         schema.StringAttribute{Computed: true, MarkdownDescription: "IANA timezone the cron expression is evaluated against."},
			"message_template": schema.StringAttribute{Computed: true, MarkdownDescription: "Prompt template fed to the agent on each tick."},
			"enabled":          schema.BoolAttribute{Computed: true, MarkdownDescription: "True when the trigger is scheduling runs."},
			"last_executed_at": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 timestamp of the most recent execution; null if never run."},
			"created_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 timestamp of trigger creation."},
		},
	}
}

func (d *ScheduleTriggerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *ScheduleTriggerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ScheduleTriggerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	triggerID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", err.Error())
		return
	}
	apiResp, err := d.client.GetScheduleTriggerWithResponse(ctx, triggerID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read schedule trigger: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Schedule trigger %q not found.", data.ID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	row := apiResp.JSON200
	data.Name = types.StringValue(row.Name)
	data.AgentID = types.StringValue(row.AgentId.String())
	data.CronExpression = types.StringValue(row.CronExpression)
	data.Timezone = types.StringValue(row.Timezone)
	data.MessageTemplate = types.StringValue(row.MessageTemplate)
	data.Enabled = types.BoolValue(row.Enabled)
	if row.LastExecutedAt != nil {
		data.LastExecutedAt = types.StringValue(row.LastExecutedAt.Format(time.RFC3339))
	} else {
		data.LastExecutedAt = types.StringNull()
	}
	data.CreatedAt = types.StringValue(row.CreatedAt.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
