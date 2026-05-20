package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &KnowledgeConnectorDataSource{}

func NewKnowledgeConnectorDataSource() datasource.DataSource {
	return &KnowledgeConnectorDataSource{}
}

type KnowledgeConnectorDataSource struct {
	client *client.ClientWithResponses
}

type KnowledgeConnectorDataSourceModel struct {
	ID                types.String         `tfsdk:"id"`
	Name              types.String         `tfsdk:"name"`
	Description       types.String         `tfsdk:"description"`
	ConnectorType     types.String         `tfsdk:"connector_type"`
	Visibility        types.String         `tfsdk:"visibility"`
	TeamIDs           types.List           `tfsdk:"team_ids"`
	Config            jsontypes.Normalized `tfsdk:"config"`
	Schedule          types.String         `tfsdk:"schedule"`
	Enabled           types.Bool           `tfsdk:"enabled"`
	SecretID          types.String         `tfsdk:"secret_id"`
	LastSyncAt        types.String         `tfsdk:"last_sync_at"`
	LastSyncStatus    types.String         `tfsdk:"last_sync_status"`
	LastSyncError     types.String         `tfsdk:"last_sync_error"`
	TotalDocsIngested types.Float64        `tfsdk:"total_docs_ingested"`
	CreatedAt         types.String         `tfsdk:"created_at"`
	UpdatedAt         types.String         `tfsdk:"updated_at"`
}

func (d *KnowledgeConnectorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_knowledge_connector"
}

func (d *KnowledgeConnectorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing knowledge connector by ID. `credentials.api_token` is never exposed — secrets stay on the backend.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Required: true, MarkdownDescription: "Connector UUID."},
			"name":           schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable name."},
			"description":    schema.StringAttribute{Computed: true, MarkdownDescription: "Free-form description, if set."},
			"connector_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Type of upstream system (jira, github, notion, …)."},
			"visibility":     schema.StringAttribute{Computed: true, MarkdownDescription: "`org-wide` or `team-scoped`."},
			"team_ids": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Team IDs the connector is scoped to (empty unless `visibility = \"team-scoped\"`).",
			},
			"config": schema.StringAttribute{
				Computed:            true,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "Connector-type-specific configuration as a JSON string. The discriminator `type` field is stripped — it's encoded in `connector_type`.",
			},
			"schedule":         schema.StringAttribute{Computed: true, MarkdownDescription: "Cron expression controlling sync cadence."},
			"enabled":          schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the connector runs on its schedule."},
			"secret_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "Internal secret-manager handle for the stored credentials."},
			"last_sync_at":     schema.StringAttribute{Computed: true, MarkdownDescription: "Most recent sync attempt timestamp (RFC 3339)."},
			"last_sync_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Status of the most recent sync run."},
			"last_sync_error":  schema.StringAttribute{Computed: true, MarkdownDescription: "Error message from the most recent failed sync, or null if the last run succeeded."},
			"total_docs_ingested": schema.Float64Attribute{
				Computed:            true,
				MarkdownDescription: "Total document count ingested across all syncs (a running counter; the backend returns it as a float for paginated histories).",
			},
			"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp (RFC 3339)."},
			"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp (RFC 3339)."},
		},
	}
}

func (d *KnowledgeConnectorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *KnowledgeConnectorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data KnowledgeConnectorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.GetConnectorWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read knowledge connector: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Knowledge connector %q not found.", data.ID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	// JSON-roundtrip the body so the polymorphic `config` (broken
	// `union json.RawMessage` in the generated client) lands as a
	// plain `map[string]any` we can stringify.
	var src struct {
		Config            json.RawMessage `json:"config"`
		ConnectorType     string          `json:"connectorType"`
		CreatedAt         time.Time       `json:"createdAt"`
		Description       *string         `json:"description"`
		Enabled           bool            `json:"enabled"`
		ID                string          `json:"id"`
		LastSyncAt        *time.Time      `json:"lastSyncAt"`
		LastSyncError     *string         `json:"lastSyncError"`
		LastSyncStatus    *string         `json:"lastSyncStatus"`
		Name              string          `json:"name"`
		Schedule          *string         `json:"schedule"`
		SecretID          *string         `json:"secretId"`
		TeamIDs           []string        `json:"teamIds"`
		TotalDocsIngested float64         `json:"totalDocsIngested"`
		UpdatedAt         time.Time       `json:"updatedAt"`
		Visibility        string          `json:"visibility"`
	}
	if err := json.Unmarshal(apiResp.Body, &src); err != nil {
		resp.Diagnostics.AddError("API Response Decode Error", "Unable to decode connector response: "+err.Error())
		return
	}

	data.ID = types.StringValue(src.ID)
	data.Name = types.StringValue(src.Name)
	if src.Description != nil {
		data.Description = types.StringValue(*src.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.ConnectorType = types.StringValue(src.ConnectorType)
	data.Visibility = types.StringValue(src.Visibility)
	teamList, diags := types.ListValueFrom(ctx, types.StringType, src.TeamIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.TeamIDs = teamList

	// Strip the wire-only `type` discriminator so the config round-trips
	// the same shape the resource-side projection uses.
	if len(src.Config) > 0 {
		var cfg map[string]any
		if err := json.Unmarshal(src.Config, &cfg); err != nil {
			resp.Diagnostics.AddError("Config decode error", err.Error())
			return
		}
		delete(cfg, "type")
		encoded, err := json.Marshal(cfg)
		if err != nil {
			resp.Diagnostics.AddError("Config encode error", err.Error())
			return
		}
		data.Config = jsontypes.NewNormalizedValue(string(encoded))
	} else {
		data.Config = jsontypes.NewNormalizedNull()
	}

	if src.Schedule != nil && *src.Schedule != "" {
		data.Schedule = types.StringValue(*src.Schedule)
	} else {
		data.Schedule = types.StringNull()
	}
	data.Enabled = types.BoolValue(src.Enabled)
	if src.SecretID != nil {
		data.SecretID = types.StringValue(*src.SecretID)
	} else {
		data.SecretID = types.StringNull()
	}
	if src.LastSyncAt != nil {
		data.LastSyncAt = types.StringValue(src.LastSyncAt.Format(time.RFC3339))
	} else {
		data.LastSyncAt = types.StringNull()
	}
	if src.LastSyncStatus != nil {
		data.LastSyncStatus = types.StringValue(*src.LastSyncStatus)
	} else {
		data.LastSyncStatus = types.StringNull()
	}
	if src.LastSyncError != nil {
		data.LastSyncError = types.StringValue(*src.LastSyncError)
	} else {
		data.LastSyncError = types.StringNull()
	}
	data.TotalDocsIngested = types.Float64Value(src.TotalDocsIngested)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
