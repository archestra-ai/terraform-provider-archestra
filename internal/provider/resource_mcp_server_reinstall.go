package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &MCPServerReinstallResource{}

func NewMCPServerReinstallResource() resource.Resource {
	return &MCPServerReinstallResource{}
}

type MCPServerReinstallResource struct {
	client *client.ClientWithResponses
}

// MCPServerReinstallResourceModel models a one-shot
// `POST /api/mcp_server/{id}/reinstall`. The resource is keyed on
// `mcp_server_id` + an opaque `trigger` string; bumping `trigger`
// forces replacement and re-issues the call. Mirrors the UI's
// "Reinstall" action — typically used when `reinstall_required = true`
// on the parent `archestra_mcp_server_installation`.
type MCPServerReinstallResourceModel struct {
	ID                types.String `tfsdk:"id"`
	McpServerID       types.String `tfsdk:"mcp_server_id"`
	Trigger           types.String `tfsdk:"trigger"`
	EnvironmentValues types.Map    `tfsdk:"environment_values"`
	UserConfigValues  types.Map    `tfsdk:"user_config_values"`
	ServiceAccount    types.String `tfsdk:"service_account"`
	IsByosVault       types.Bool   `tfsdk:"is_byos_vault"`
	ExecutedAt        types.String `tfsdk:"executed_at"`
}

func (r *MCPServerReinstallResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_reinstall"
}

func (r *MCPServerReinstallResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Re-deploys an installed MCP server (re-runs the K8s install) without recreating the install row. Maps to `POST /api/mcp_server/{id}/reinstall` — typically run after the parent `archestra_mcp_server_installation.reinstall_required` flips to true.\n\n" +
			"~> **One-shot.** Every attribute is `RequiresReplace`. Bumping `trigger` forces a re-deploy. The `archestra_mcp_server_installation` row is not touched.\n\n" +
			"```hcl\n" +
			"resource \"archestra_mcp_server_reinstall\" \"refresh\" {\n" +
			"  mcp_server_id = archestra_mcp_server_installation.api.id\n" +
			"  trigger       = \"image-pull-2026-05-19\"\n" +
			"}\n" +
			"```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Synthetic identifier `<mcp_server_id>:<trigger>`.",
			},
			"mcp_server_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the target MCP server installation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"trigger": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Opaque string the user bumps to force a re-deploy.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_values": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Replacement env vars. Omit to leave existing env unchanged.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"user_config_values": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Replacement user-config values.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"service_account": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K8s service-account override for the MCP server pod. Omit to keep the install-time value.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_byos_vault": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When true, treat env/user-config values as vault references.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"executed_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp of the reinstall call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *MCPServerReinstallResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *MCPServerReinstallResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MCPServerReinstallResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverID, err := uuid.Parse(plan.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid mcp_server_id", err.Error())
		return
	}

	body := client.ReinstallMcpServerJSONRequestBody{}
	if !plan.EnvironmentValues.IsNull() && !plan.EnvironmentValues.IsUnknown() {
		var m map[string]string
		resp.Diagnostics.Append(plan.EnvironmentValues.ElementsAs(ctx, &m, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.EnvironmentValues = &m
	}
	if !plan.UserConfigValues.IsNull() && !plan.UserConfigValues.IsUnknown() {
		var m map[string]string
		resp.Diagnostics.Append(plan.UserConfigValues.ElementsAs(ctx, &m, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.UserConfigValues = &m
	}
	if !plan.ServiceAccount.IsNull() && !plan.ServiceAccount.IsUnknown() {
		v := plan.ServiceAccount.ValueString()
		body.ServiceAccount = &v
	}
	if !plan.IsByosVault.IsNull() && !plan.IsByosVault.IsUnknown() {
		v := plan.IsByosVault.ValueBool()
		body.IsByosVault = &v
	}

	apiResp, err := r.client.ReinstallMcpServerWithResponse(ctx, serverID, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Reinstall failed: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Reinstall returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.McpServerID.ValueString(), plan.Trigger.ValueString()))
	plan.ExecutedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MCPServerReinstallResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MCPServerReinstallResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerReinstallResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported",
		"All `archestra_mcp_server_reinstall` inputs are RequiresReplace. This call should never have happened.")
}

func (r *MCPServerReinstallResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// No-op: the install row is owned by archestra_mcp_server_installation.
}
