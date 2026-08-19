package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &KnowledgeConnectorRunsDataSource{}

func NewKnowledgeConnectorRunsDataSource() datasource.DataSource {
	return &KnowledgeConnectorRunsDataSource{}
}

type KnowledgeConnectorRunsDataSource struct {
	client *client.ClientWithResponses
}

type KnowledgeConnectorRunsDataSourceModel struct {
	ConnectorID types.String `tfsdk:"connector_id"`
	MaxRecords  types.Int64  `tfsdk:"max_records"`
	Runs        types.List   `tfsdk:"runs"`
	Total       types.Int64  `tfsdk:"total"`
	Truncated   types.Bool   `tfsdk:"truncated"`
}

var connectorRunsElementType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":                  types.StringType,
	"connector_id":        types.StringType,
	"status":              types.StringType,
	"started_at":          types.StringType,
	"completed_at":        types.StringType,
	"documents_processed": types.Int64Type,
	"documents_ingested":  types.Int64Type,
	"item_errors":         types.Int64Type,
	"total_batches":       types.Int64Type,
	"completed_batches":   types.Int64Type,
	"total_items":         types.Int64Type,
	"error":               types.StringType,
	"created_at":          types.StringType,
}}

const defaultConnectorRunsMaxRecords int64 = 1000

func (d *KnowledgeConnectorRunsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_connector_runs"
}

func (d *KnowledgeConnectorRunsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sync-run history for a knowledge connector. Caps at `max_records` (default 1000) to keep state size bounded on long-running connectors; `truncated = true` signals more runs exist.",
		Attributes: map[string]schema.Attribute{
			"connector_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Connector UUID.",
			},
			"max_records": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of runs to pull into state (default 1000).",
			},
			"runs": schema.ListAttribute{
				Computed:            true,
				ElementType:         connectorRunsElementType,
				MarkdownDescription: "Sync runs ordered as the backend returns them (typically newest first).",
			},
			"total":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Total count reported by the backend."},
			"truncated": schema.BoolAttribute{Computed: true, MarkdownDescription: "True when `max_records` capped the result."},
		},
	}
}

func (d *KnowledgeConnectorRunsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KnowledgeConnectorRunsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data KnowledgeConnectorRunsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	maxRecords := defaultConnectorRunsMaxRecords
	if !data.MaxRecords.IsNull() && !data.MaxRecords.IsUnknown() {
		maxRecords = data.MaxRecords.ValueInt64()
		if maxRecords <= 0 {
			resp.Diagnostics.AddError("Invalid max_records", "max_records must be > 0")
			return
		}
	}

	limit := 100
	offset := 0
	params := &client.GetConnectorRunsParams{Limit: &limit, Offset: &offset}
	connectorID := data.ConnectorID.ValueString()

	var collected []attr.Value
	var total int64
	truncated := false
	for {
		apiResp, err := d.client.GetConnectorRunsWithResponse(ctx, connectorID, params)
		if err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read connector runs: %s", err))
			return
		}
		if IsNotFound(apiResp) {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Knowledge connector %q not found.", connectorID))
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
			obj, diags := types.ObjectValue(connectorRunsElementType.AttrTypes, map[string]attr.Value{
				"id":                  types.StringValue(row.Id.String()),
				"connector_id":        types.StringValue(row.ConnectorId.String()),
				"status":              types.StringValue(string(row.Status)),
				"started_at":          types.StringValue(row.StartedAt.Format(time.RFC3339)),
				"completed_at":        optionalTime(row.CompletedAt),
				"documents_processed": optionalInt(row.DocumentsProcessed),
				"documents_ingested":  optionalInt(row.DocumentsIngested),
				"item_errors":         optionalInt(row.ItemErrors),
				"total_batches":       optionalInt(row.TotalBatches),
				"completed_batches":   optionalInt(row.CompletedBatches),
				"total_items":         optionalInt(row.TotalItems),
				"error":               optionalString(row.Error),
				"created_at":          types.StringValue(row.CreatedAt.Format(time.RFC3339)),
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

	listValue, diags := types.ListValue(connectorRunsElementType, collected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Runs = listValue
	data.Total = types.Int64Value(total)
	data.Truncated = types.BoolValue(truncated)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// optionalInt is the nullable-int counterpart to optionalString /
// optionalTime defined alongside the virtual_api_keys data source.
func optionalInt(v *int) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}
