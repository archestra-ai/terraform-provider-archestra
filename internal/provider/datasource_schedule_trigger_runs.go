package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ScheduleTriggerRunsDataSource{}

func NewScheduleTriggerRunsDataSource() datasource.DataSource {
	return &ScheduleTriggerRunsDataSource{}
}

type ScheduleTriggerRunsDataSource struct {
	client *client.ClientWithResponses
}

type ScheduleTriggerRunsDataSourceModel struct {
	TriggerID  types.String `tfsdk:"trigger_id"`
	Status     types.String `tfsdk:"status"`
	MaxRecords types.Int64  `tfsdk:"max_records"`
	Runs       types.List   `tfsdk:"runs"`
	Total      types.Int64  `tfsdk:"total"`
	Truncated  types.Bool   `tfsdk:"truncated"`
}

var scheduleTriggerRunsElementType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                   types.StringType,
	"trigger_id":           types.StringType,
	"run_kind":             types.StringType,
	"status":               types.StringType,
	"initiated_by_user_id": types.StringType,
	"chat_conversation_id": types.StringType,
	"artifact":             types.StringType,
	"started_at":           types.StringType,
	"completed_at":         types.StringType,
	"error":                types.StringType,
	"created_at":           types.StringType,
}}

const defaultScheduleTriggerRunsMaxRecords int64 = 1000

func (d *ScheduleTriggerRunsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_trigger_runs"
}

func (d *ScheduleTriggerRunsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Execution history for an `archestra_schedule_trigger`. Caps at `max_records` (default 1000); `truncated = true` signals more runs exist.",
		Attributes: map[string]schema.Attribute{
			"trigger_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Schedule trigger UUID.",
			},
			"status": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional filter on run status: `running`, `success`, or `failed`.",
				Validators: []validator.String{
					stringvalidator.OneOf("running", "success", "failed"),
				},
			},
			"max_records": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of runs to pull into state (default 1000).",
			},
			"runs": schema.ListAttribute{
				Computed:            true,
				ElementType:         scheduleTriggerRunsElementType,
				MarkdownDescription: "Trigger runs ordered as the backend returns them (typically newest first).",
			},
			"total":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Total count reported by the backend."},
			"truncated": schema.BoolAttribute{Computed: true, MarkdownDescription: "True when `max_records` capped the result."},
		},
	}
}

func (d *ScheduleTriggerRunsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ScheduleTriggerRunsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ScheduleTriggerRunsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	triggerID, err := uuid.Parse(data.TriggerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid trigger_id", err.Error())
		return
	}

	maxRecords := defaultScheduleTriggerRunsMaxRecords
	if !data.MaxRecords.IsNull() && !data.MaxRecords.IsUnknown() {
		maxRecords = data.MaxRecords.ValueInt64()
		if maxRecords <= 0 {
			resp.Diagnostics.AddError("Invalid max_records", "max_records must be > 0")
			return
		}
	}

	limit := 100
	offset := 0
	params := &client.GetScheduleTriggerRunsParams{Limit: &limit, Offset: &offset}
	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		s := client.GetScheduleTriggerRunsParamsStatus(data.Status.ValueString())
		params.Status = &s
	}

	var collected []attr.Value
	var total int64
	truncated := false
	for {
		apiResp, err := d.client.GetScheduleTriggerRunsWithResponse(ctx, triggerID, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read trigger runs: %s", err))
			return
		}
		if IsNotFound(apiResp) {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Schedule trigger %q not found.", data.TriggerID.ValueString()))
			return
		}
		if apiResp.JSON200 == nil {
			resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
			return
		}
		total = int64(apiResp.JSON200.Pagination.Total)
		for i := range apiResp.JSON200.Data {
			if int64(len(collected)) >= maxRecords {
				truncated = true
				break
			}
			row := &apiResp.JSON200.Data[i]
			chatConv := types.StringNull()
			if row.ChatConversationId != nil {
				chatConv = types.StringValue(row.ChatConversationId.String())
			}
			startedAt := types.StringNull()
			if row.StartedAt != nil {
				startedAt = types.StringValue(row.StartedAt.Format(time.RFC3339))
			}
			obj, diags := types.ObjectValue(scheduleTriggerRunsElementType.AttrTypes, map[string]attr.Value{
				"id":                   types.StringValue(row.Id.String()),
				"trigger_id":           types.StringValue(row.TriggerId.String()),
				"run_kind":             types.StringValue(string(row.RunKind)),
				"status":               types.StringValue(string(row.Status)),
				"initiated_by_user_id": optionalString(row.InitiatedByUserId),
				"chat_conversation_id": chatConv,
				"artifact":             optionalString(row.Artifact),
				"started_at":           startedAt,
				"completed_at":         optionalTime(row.CompletedAt),
				"error":                optionalString(row.Error),
				"created_at":           types.StringValue(row.CreatedAt.Format(time.RFC3339)),
			})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			collected = append(collected, obj)
		}
		if truncated || !apiResp.JSON200.Pagination.HasNext {
			break
		}
		offset += limit
		params.Offset = &offset
	}
	if !truncated && total > int64(len(collected)) {
		truncated = true
	}

	listValue, diags := types.ListValue(scheduleTriggerRunsElementType, collected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Runs = listValue
	data.Total = types.Int64Value(total)
	data.Truncated = types.BoolValue(truncated)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
