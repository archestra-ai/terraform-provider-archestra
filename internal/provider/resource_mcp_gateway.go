package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &McpGatewayResource{}
var _ resource.ResourceWithImportState = &McpGatewayResource{}

func NewMcpGatewayResource() resource.Resource { return &McpGatewayResource{} }

type McpGatewayResource struct {
	client *client.ClientWithResponses
}

// McpGatewayResourceModel is the schema for an Archestra MCP gateway. The
// gateway exposes a unified MCP endpoint backed by tool installations and
// optional knowledge sources.
type McpGatewayResourceModel struct {
	ID                       types.String      `tfsdk:"id"`
	Name                     types.String      `tfsdk:"name"`
	Description              types.String      `tfsdk:"description"`
	Icon                     types.String      `tfsdk:"icon"`
	KnowledgeBaseIds         types.List        `tfsdk:"knowledge_base_ids"`
	ConnectorIds             types.List        `tfsdk:"connector_ids"`
	PassthroughHeaders       types.List        `tfsdk:"passthrough_headers"`
	IdentityProviderId       types.String      `tfsdk:"identity_provider_id"`
	ConsiderContextUntrusted types.Bool        `tfsdk:"consider_context_untrusted"`
	IsDefault                types.Bool        `tfsdk:"is_default"`
	Scope                    types.String      `tfsdk:"scope"`
	Teams                    types.List        `tfsdk:"teams"`
	Labels                   []AgentLabelModel `tfsdk:"labels"`
}

func (r *McpGatewayResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_gateway"
}

func (r *McpGatewayResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra MCP gateway — a unified MCP endpoint that aggregates installed tools and (optionally) knowledge sources, with optional inbound JWT auth via `identity_provider_id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MCP gateway identifier",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Gateway name"},
			"description": schema.StringAttribute{Optional: true, MarkdownDescription: "Human-readable description"},
			"icon":        schema.StringAttribute{Optional: true, MarkdownDescription: "Emoji or base64 image data URL"},
			"knowledge_base_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge base IDs the gateway has access to",
			},
			"connector_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Knowledge connector IDs the gateway has access to",
			},
			"passthrough_headers": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Allowlist of HTTP header names to forward from gateway requests to downstream MCP servers",
			},
			"identity_provider_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Identity provider used to validate inbound JWTs. Reference an `archestra_sso_provider`. Omit to disable JWT auth.",
			},
			"consider_context_untrusted": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the gateway context is treated as untrusted",
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is the default MCP gateway",
			},
			"scope": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Ownership scope: `personal`, `team`, or `org` (default: `org`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"teams": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Team IDs this gateway is assigned to. Required when `scope = \"team\"`.",
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"labels": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Key/value labels for organizing gateways",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Required: true},
						"value": schema.StringAttribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *McpGatewayResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *McpGatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope := client.CreateAgentJSONBodyScopeOrg
	if !data.Scope.IsNull() && !data.Scope.IsUnknown() {
		scope = client.CreateAgentJSONBodyScope(data.Scope.ValueString())
	}
	teams := []string{}
	if !data.Teams.IsNull() && !data.Teams.IsUnknown() {
		resp.Diagnostics.Append(data.Teams.ElementsAs(ctx, &teams, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	labels := buildAgentLabelsCreate(data.Labels)
	requestBody := client.CreateAgentJSONRequestBody{
		Name:   data.Name.ValueString(),
		Scope:  scope,
		Teams:  &teams,
		Labels: &labels,
	}
	at := client.CreateAgentJSONBodyAgentType("mcp_gateway")
	requestBody.AgentType = &at

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		requestBody.Description = &v
	}
	if !data.Icon.IsNull() && !data.Icon.IsUnknown() {
		v := data.Icon.ValueString()
		requestBody.Icon = &v
	}
	if !data.KnowledgeBaseIds.IsNull() && !data.KnowledgeBaseIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.KnowledgeBaseIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.KnowledgeBaseIds = &ids
	}
	if !data.ConnectorIds.IsNull() && !data.ConnectorIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.ConnectorIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.ConnectorIds = &ids
	}
	if !data.PassthroughHeaders.IsNull() && !data.PassthroughHeaders.IsUnknown() {
		var headers []string
		resp.Diagnostics.Append(data.PassthroughHeaders.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.PassthroughHeaders = &headers
	}
	if !data.IdentityProviderId.IsNull() && !data.IdentityProviderId.IsUnknown() {
		v := data.IdentityProviderId.ValueString()
		requestBody.IdentityProviderId = &v
	}
	if !data.ConsiderContextUntrusted.IsNull() && !data.ConsiderContextUntrusted.IsUnknown() {
		v := data.ConsiderContextUntrusted.ValueBool()
		requestBody.ConsiderContextUntrusted = &v
	}
	if !data.IsDefault.IsNull() && !data.IsDefault.IsUnknown() {
		v := data.IsDefault.ValueBool()
		requestBody.IsDefault = &v
	}

	apiResp, createErr := r.client.CreateAgentWithResponse(ctx, requestBody)
	if createErr != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create MCP gateway, got error: %s", createErr))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpGatewayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP gateway ID: %s", err))
		return
	}

	apiResp, err := r.client.GetAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read MCP gateway, got error: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpGatewayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP gateway ID: %s", err))
		return
	}

	labels := buildAgentLabelsUpdate(data.Labels)
	name := data.Name.ValueString()
	requestBody := client.UpdateAgentJSONRequestBody{
		Name:   &name,
		Labels: &labels,
	}
	at := client.UpdateAgentJSONBodyAgentType("mcp_gateway")
	requestBody.AgentType = &at

	if !data.Scope.IsNull() && !data.Scope.IsUnknown() {
		s := client.UpdateAgentJSONBodyScope(data.Scope.ValueString())
		requestBody.Scope = &s
	}
	if !data.Teams.IsNull() && !data.Teams.IsUnknown() {
		var teams []string
		resp.Diagnostics.Append(data.Teams.ElementsAs(ctx, &teams, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.Teams = &teams
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		requestBody.Description = &v
	}
	if !data.Icon.IsNull() && !data.Icon.IsUnknown() {
		v := data.Icon.ValueString()
		requestBody.Icon = &v
	}
	if !data.KnowledgeBaseIds.IsNull() && !data.KnowledgeBaseIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.KnowledgeBaseIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.KnowledgeBaseIds = &ids
	}
	if !data.ConnectorIds.IsNull() && !data.ConnectorIds.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(data.ConnectorIds.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.ConnectorIds = &ids
	}
	if !data.PassthroughHeaders.IsNull() && !data.PassthroughHeaders.IsUnknown() {
		var headers []string
		resp.Diagnostics.Append(data.PassthroughHeaders.ElementsAs(ctx, &headers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		requestBody.PassthroughHeaders = &headers
	}
	if !data.IdentityProviderId.IsNull() && !data.IdentityProviderId.IsUnknown() {
		v := data.IdentityProviderId.ValueString()
		requestBody.IdentityProviderId = &v
	}
	if !data.ConsiderContextUntrusted.IsNull() && !data.ConsiderContextUntrusted.IsUnknown() {
		v := data.ConsiderContextUntrusted.ValueBool()
		requestBody.ConsiderContextUntrusted = &v
	}
	if !data.IsDefault.IsNull() && !data.IsDefault.IsUnknown() {
		v := data.IsDefault.ValueBool()
		requestBody.IsDefault = &v
	}

	apiResp, updateErr := r.client.UpdateAgentWithResponse(ctx, id, requestBody)
	if updateErr != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update MCP gateway, got error: %s", updateErr))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	r.flatten(ctx, &data, apiResp.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *McpGatewayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data McpGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Unable to parse MCP gateway ID: %s", err))
		return
	}

	apiResp, err := r.client.DeleteAgentWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete MCP gateway, got error: %s", err))
		return
	}
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
	}
}

func (r *McpGatewayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *McpGatewayResource) flatten(ctx context.Context, data *McpGatewayResourceModel, body []byte, diags *diag.Diagnostics) {
	resp := parseAgentResponse(body, diags)
	if resp == nil {
		return
	}

	data.ID = types.StringValue(resp.Id.String())
	data.Name = types.StringValue(resp.Name)
	optionalStringFromAPI(&data.Description, resp.Description)
	optionalStringFromAPI(&data.Icon, resp.Icon)
	optionalStringFromAPI(&data.IdentityProviderId, resp.IdentityProviderId)

	stringListFromAPI(ctx, &data.KnowledgeBaseIds, resp.KnowledgeBaseIds, diags)
	stringListFromAPI(ctx, &data.ConnectorIds, resp.ConnectorIds, diags)

	if resp.PassthroughHeaders != nil {
		list, d := types.ListValueFrom(ctx, types.StringType, *resp.PassthroughHeaders)
		diags.Append(d...)
		data.PassthroughHeaders = list
	} else if !data.PassthroughHeaders.IsNull() {
		data.PassthroughHeaders = types.ListNull(types.StringType)
	}

	data.ConsiderContextUntrusted = types.BoolValue(resp.ConsiderContextUntrusted)
	data.IsDefault = types.BoolValue(resp.IsDefault)
	data.Scope = types.StringValue(resp.Scope)
	data.Teams = teamsListFromAPI(ctx, resp.Teams, diags)

	data.Labels = flattenAgentLabels(data.Labels, resp.Labels)
}
