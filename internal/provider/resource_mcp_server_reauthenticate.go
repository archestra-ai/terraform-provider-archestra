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

var (
	_ resource.Resource                   = &MCPServerReauthenticateResource{}
	_ resource.ResourceWithValidateConfig = &MCPServerReauthenticateResource{}
)

func NewMCPServerReauthenticateResource() resource.Resource {
	return &MCPServerReauthenticateResource{}
}

type MCPServerReauthenticateResource struct {
	client *client.ClientWithResponses
}

// MCPServerReauthenticateResourceModel models a one-shot
// `PATCH /api/mcp_server/{id}/reauthenticate`. The resource is keyed on
// `mcp_server_id` + an opaque `trigger` string; bumping `trigger`
// forces replacement and re-issues the call. Mirrors the UI's
// "Reauthenticate" button.
type MCPServerReauthenticateResourceModel struct {
	ID                types.String `tfsdk:"id"`
	McpServerID       types.String `tfsdk:"mcp_server_id"`
	Trigger           types.String `tfsdk:"trigger"`
	AccessToken       types.String `tfsdk:"access_token"`
	EnvironmentValues types.Map    `tfsdk:"environment_values"`
	UserConfigValues  types.Map    `tfsdk:"user_config_values"`
	SecretID          types.String `tfsdk:"secret_id"`
	IsByosVault       types.Bool   `tfsdk:"is_byos_vault"`
	ExecutedAt        types.String `tfsdk:"executed_at"`
}

func (r *MCPServerReauthenticateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_server_reauthenticate"
}

func (r *MCPServerReauthenticateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Re-runs credential resolution for an installed MCP server. Maps to the UI's *Reauthenticate* button (`PATCH /api/mcp_server/{id}/reauthenticate`).\n\n" +
			"~> **One-shot.** Every attribute is `RequiresReplace`. The resource has no Read-side state to refresh and no Delete-side cleanup; bumping `trigger` is the supported way to force a re-run (token rotation, OAuth refresh, etc.).\n\n" +
			"```hcl\n" +
			"resource \"archestra_mcp_server_reauthenticate\" \"github_rotation\" {\n" +
			"  mcp_server_id = archestra_mcp_server_installation.github.id\n" +
			"  access_token  = var.new_github_pat\n" +
			"  trigger       = \"2026-05-19-rotation\"\n" +
			"}\n" +
			"```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier `<mcp_server_id>:<trigger>`. Surfaced so destroy / import behave predictably.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				MarkdownDescription: "Opaque string the user bumps to force a re-run. Any value works; the suggested convention is a date or rotation tag (`2026-05-19-rotation`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "New personal-access-token to feed the server. Omit to keep the existing token.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_values": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Replacement environment variables. Omit to leave existing env unchanged.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"user_config_values": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Replacement user-config values. Omit to leave existing config unchanged.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"secret_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "BYOS vault secret UUID to swap to. Mutually exclusive with inline `environment_values` / `user_config_values` in BYOS mode.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_byos_vault": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When true, treat `environment_values` / `user_config_values` as vault references (mirrors the `archestra_mcp_server_installation` flag).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"executed_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp of the reauthenticate call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// ValidateConfig pre-empts the backend's 400
// "At least one credential field is required" so users see the
// constraint at plan time instead of mid-apply. Unknown values mean
// the field might resolve at apply — skip the check and let the
// backend handle it (Plugin Framework convention).
func (r *MCPServerReauthenticateResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data MCPServerReauthenticateResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	anyKnownNonNull := func(values ...interface { //nolint:dupword // 'attr | ...' alternation, not duplicated.
		IsNull() bool
		IsUnknown() bool
	}) bool {
		for _, v := range values {
			if !v.IsNull() && !v.IsUnknown() {
				return true
			}
			if v.IsUnknown() {
				return true // unknowns punt validation to backend
			}
		}
		return false
	}
	if !anyKnownNonNull(data.AccessToken, data.EnvironmentValues, data.UserConfigValues, data.SecretID, data.IsByosVault) {
		resp.Diagnostics.AddError(
			"At least one credential field is required",
			"`archestra_mcp_server_reauthenticate` needs at least one of `access_token`, `environment_values`, `user_config_values`, `secret_id`, or `is_byos_vault` to issue a meaningful reauthenticate call (backend rejects with 400 otherwise).",
		)
	}
}

func (r *MCPServerReauthenticateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MCPServerReauthenticateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MCPServerReauthenticateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverID, err := uuid.Parse(plan.McpServerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid mcp_server_id", err.Error())
		return
	}

	body := client.ReauthenticateMcpServerJSONRequestBody{}
	if !plan.AccessToken.IsNull() && !plan.AccessToken.IsUnknown() {
		v := plan.AccessToken.ValueString()
		body.AccessToken = &v
	}
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
	if !plan.SecretID.IsNull() && !plan.SecretID.IsUnknown() {
		sid, err := uuid.Parse(plan.SecretID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid secret_id", err.Error())
			return
		}
		body.SecretId = &sid
	}
	if !plan.IsByosVault.IsNull() && !plan.IsByosVault.IsUnknown() {
		v := plan.IsByosVault.ValueBool()
		body.IsByosVault = &v
	}

	apiResp, err := r.client.ReauthenticateMcpServerWithResponse(ctx, serverID, body)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Reauthenticate failed: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("Unexpected API Response",
			fmt.Sprintf("Reauthenticate returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.McpServerID.ValueString(), plan.Trigger.ValueString()))
	plan.ExecutedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MCPServerReauthenticateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// One-shot: state is the source of truth.
	var data MCPServerReauthenticateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MCPServerReauthenticateResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported",
		"All `archestra_mcp_server_reauthenticate` inputs are RequiresReplace. This call should never have happened.")
}

func (r *MCPServerReauthenticateResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// No-op: the underlying server install row is owned by
	// archestra_mcp_server_installation; this resource only records that
	// the reauthenticate operation ran.
}
