package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AgentDataSource{}

func NewAgentDataSource() datasource.DataSource {
	return &AgentDataSource{}
}

type AgentDataSource struct {
	client *client.ClientWithResponses
}

// AgentDataSourceModel mirrors the read-side surface of
// `archestra_agent` (and the sibling llm_proxy / mcp_gateway resources
// — the backend uses one table). Returns whichever agent matches the
// requested ID regardless of `agent_type`, so this works for all three
// resource variants.
type AgentDataSourceModel struct {
	ID                         types.String `tfsdk:"id"`
	Name                       types.String `tfsdk:"name"`
	AgentType                  types.String `tfsdk:"agent_type"`
	Description                types.String `tfsdk:"description"`
	Icon                       types.String `tfsdk:"icon"`
	SystemPrompt               types.String `tfsdk:"system_prompt"`
	LlmModel                   types.String `tfsdk:"llm_model"`
	LlmAPIKeyID                types.String `tfsdk:"llm_api_key_id"`
	Scope                      types.String `tfsdk:"scope"`
	Slug                       types.String `tfsdk:"slug"`
	BuiltIn                    types.Bool   `tfsdk:"built_in"`
	IsDefault                  types.Bool   `tfsdk:"is_default"`
	ConsiderContextUntrusted   types.Bool   `tfsdk:"consider_context_untrusted"`
	IncomingEmailEnabled       types.Bool   `tfsdk:"incoming_email_enabled"`
	IncomingEmailAllowedDomain types.String `tfsdk:"incoming_email_allowed_domain"`
	IncomingEmailSecurityMode  types.String `tfsdk:"incoming_email_security_mode"`
	KnowledgeBaseIDs           types.List   `tfsdk:"knowledge_base_ids"`
	ConnectorIDs               types.List   `tfsdk:"connector_ids"`
	Labels                     types.Map    `tfsdk:"labels"`
	TeamIDs                    types.List   `tfsdk:"team_ids"`
	CreatedAt                  types.String `tfsdk:"created_at"`
}

func (d *AgentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (d *AgentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lookup an existing agent by UUID. Works across all three agent variants (`archestra_agent`, `archestra_llm_proxy`, `archestra_mcp_gateway`) — inspect `agent_type` to discriminate. Useful for cross-stack references without `terraform import`.",
		Attributes: map[string]schema.Attribute{
			"id":                            schema.StringAttribute{Required: true, MarkdownDescription: "Agent UUID."},
			"name":                          schema.StringAttribute{Computed: true},
			"agent_type":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Variant discriminator: `agent`, `llm_proxy`, `mcp_gateway`, or `profile` (legacy)."},
			"description":                   schema.StringAttribute{Computed: true},
			"icon":                          schema.StringAttribute{Computed: true},
			"system_prompt":                 schema.StringAttribute{Computed: true},
			"llm_model":                     schema.StringAttribute{Computed: true},
			"llm_api_key_id":                schema.StringAttribute{Computed: true},
			"scope":                         schema.StringAttribute{Computed: true},
			"slug":                          schema.StringAttribute{Computed: true},
			"built_in":                      schema.BoolAttribute{Computed: true, MarkdownDescription: "True when the agent is a platform built-in."},
			"is_default":                    schema.BoolAttribute{Computed: true, MarkdownDescription: "True when this is the default agent for its `agent_type`."},
			"consider_context_untrusted":    schema.BoolAttribute{Computed: true},
			"incoming_email_enabled":        schema.BoolAttribute{Computed: true},
			"incoming_email_allowed_domain": schema.StringAttribute{Computed: true},
			"incoming_email_security_mode":  schema.StringAttribute{Computed: true},
			"knowledge_base_ids":            schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"connector_ids":                 schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"labels": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Flat `key → value` label map. Backend stores labels as a list of `{key, value}` pairs but this data source projects them into a map for HCL ergonomics — duplicate keys collapse to the last-seen value.",
			},
			"team_ids":   schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "UUIDs of teams this agent is assigned to (only populated when `scope = \"team\"`)."},
			"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC 3339 creation timestamp."},
		},
	}
}

func (d *AgentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AgentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentID, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid id", err.Error())
		return
	}
	apiResp, err := d.client.GetAgentWithResponse(ctx, agentID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Agent %q not found.", data.ID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}
	row := apiResp.JSON200
	data.Name = types.StringValue(row.Name)
	data.AgentType = types.StringValue(string(row.AgentType))
	if row.Description != nil {
		data.Description = types.StringValue(*row.Description)
	} else {
		data.Description = types.StringNull()
	}
	if row.Icon != nil {
		data.Icon = types.StringValue(*row.Icon)
	} else {
		data.Icon = types.StringNull()
	}
	if row.SystemPrompt != nil {
		data.SystemPrompt = types.StringValue(*row.SystemPrompt)
	} else {
		data.SystemPrompt = types.StringNull()
	}
	if row.LlmModel != nil {
		data.LlmModel = types.StringValue(*row.LlmModel)
	} else {
		data.LlmModel = types.StringNull()
	}
	if row.LlmApiKeyId != nil {
		data.LlmAPIKeyID = types.StringValue(row.LlmApiKeyId.String())
	} else {
		data.LlmAPIKeyID = types.StringNull()
	}
	data.Scope = types.StringValue(string(row.Scope))
	if row.Slug != nil {
		data.Slug = types.StringValue(*row.Slug)
	} else {
		data.Slug = types.StringNull()
	}
	if row.BuiltIn != nil {
		data.BuiltIn = types.BoolValue(*row.BuiltIn)
	} else {
		data.BuiltIn = types.BoolValue(false)
	}
	data.IsDefault = types.BoolValue(row.IsDefault)
	data.ConsiderContextUntrusted = types.BoolValue(row.ConsiderContextUntrusted)
	data.IncomingEmailEnabled = types.BoolValue(row.IncomingEmailEnabled)
	if row.IncomingEmailAllowedDomain != nil {
		data.IncomingEmailAllowedDomain = types.StringValue(*row.IncomingEmailAllowedDomain)
	} else {
		data.IncomingEmailAllowedDomain = types.StringNull()
	}
	data.IncomingEmailSecurityMode = types.StringValue(string(row.IncomingEmailSecurityMode))
	data.CreatedAt = types.StringValue(row.CreatedAt.Format(time.RFC3339))

	kbElems := make([]attr.Value, len(row.KnowledgeBaseIds))
	for i, s := range row.KnowledgeBaseIds {
		kbElems[i] = types.StringValue(s)
	}
	kbList, diags := types.ListValue(types.StringType, kbElems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.KnowledgeBaseIDs = kbList

	connElems := make([]attr.Value, len(row.ConnectorIds))
	for i, s := range row.ConnectorIds {
		connElems[i] = types.StringValue(s)
	}
	connList, diags := types.ListValue(types.StringType, connElems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ConnectorIDs = connList

	teamElems := make([]attr.Value, len(row.Teams))
	for i, t := range row.Teams {
		teamElems[i] = types.StringValue(t.Id)
	}
	teamList, diags := types.ListValue(types.StringType, teamElems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.TeamIDs = teamList

	labelMap := make(map[string]attr.Value, len(row.Labels))
	for _, l := range row.Labels {
		labelMap[l.Key] = types.StringValue(l.Value)
	}
	labelValue, diags := types.MapValue(types.StringType, labelMap)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Labels = labelValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
