package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &AgentEmailAddressDataSource{}

func NewAgentEmailAddressDataSource() datasource.DataSource {
	return &AgentEmailAddressDataSource{}
}

type AgentEmailAddressDataSource struct {
	client *client.ClientWithResponses
}

type AgentEmailAddressDataSourceModel struct {
	AgentID                   types.String `tfsdk:"agent_id"`
	EmailAddress              types.String `tfsdk:"email_address"`
	AgentAllowedDomain        types.String `tfsdk:"agent_allowed_domain"`
	AgentIncomingEmailEnabled types.Bool   `tfsdk:"agent_incoming_email_enabled"`
	AgentSecurityMode         types.String `tfsdk:"agent_security_mode"`
	ProviderEnabled           types.Bool   `tfsdk:"provider_enabled"`
}

func (d *AgentEmailAddressDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_email_address"
}

func (d *AgentEmailAddressDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Inbound email address provisioned for an agent (when the org's email-incoming feature is enabled). Use this to wire downstream SMTP/forwarding rules without copying the address by hand.",
		Attributes: map[string]schema.Attribute{
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Target agent UUID.",
			},
			"email_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The inbound email address the platform routes to this agent; null when incoming email is disabled at either the org or agent level.",
			},
			"agent_allowed_domain": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Sender-domain allowlist enforced on inbound mail; null when no domain restriction is configured.",
			},
			"agent_incoming_email_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True when the agent itself accepts inbound email.",
			},
			"agent_security_mode": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Inbound-email security posture for this agent (e.g. `dmarc_only`, `sender_allowlist`).",
			},
			"provider_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True when the org-level email provider is configured. False here means the agent address cannot be resolved regardless of agent-level settings.",
			},
		},
	}
}

func (d *AgentEmailAddressDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AgentEmailAddressDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentEmailAddressDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentID, err := uuid.Parse(data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid agent_id", err.Error())
		return
	}

	apiResp, err := d.client.GetAgentEmailAddressWithResponse(ctx, agentID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read agent email address: %s", err))
		return
	}
	if apiResp.JSON404 != nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Agent %q not found.", data.AgentID.ValueString()))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response", fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	body := apiResp.JSON200
	if body.EmailAddress != nil {
		data.EmailAddress = types.StringValue(*body.EmailAddress)
	} else {
		data.EmailAddress = types.StringNull()
	}
	if body.AgentAllowedDomain != nil {
		data.AgentAllowedDomain = types.StringValue(*body.AgentAllowedDomain)
	} else {
		data.AgentAllowedDomain = types.StringNull()
	}
	data.AgentIncomingEmailEnabled = types.BoolValue(body.AgentIncomingEmailEnabled)
	data.AgentSecurityMode = types.StringValue(string(body.AgentSecurityMode))
	data.ProviderEnabled = types.BoolValue(body.ProviderEnabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
