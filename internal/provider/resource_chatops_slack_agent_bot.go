package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &ChatOpsSlackAgentBotResource{}

func NewChatOpsSlackAgentBotResource() resource.Resource { return &ChatOpsSlackAgentBotResource{} }

// ChatOpsSlackAgentBotResource connects a Slack app whose bot always answers
// as one agent. Each agent has at most one such bot, alongside the
// organization's own Slack app (archestra_chatops_slack).
type ChatOpsSlackAgentBotResource struct{ client *client.ClientWithResponses }

var chatOpsSlackAgentBotAttrSpec = []AttrSpec{
	{TFName: "agent_id", Kind: Synthetic}, // Path parameter.
	{TFName: "enabled", JSONName: "enabled", Kind: Scalar},
	{TFName: "bot_token", JSONName: "botToken", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "signing_secret", JSONName: "signingSecret", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "app_level_token", JSONName: "appLevelToken", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "app_id", JSONName: "appId", Kind: Scalar, OmitOnNull: true},
	{TFName: "connection_mode", JSONName: "connectionMode", Kind: Scalar, OmitOnNull: true},
}

func (r *ChatOpsSlackAgentBotResource) AttrSpecs() []AttrSpec { return chatOpsSlackAgentBotAttrSpec }

func (r *ChatOpsSlackAgentBotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chatops_slack_agent_bot"
}

func (r *ChatOpsSlackAgentBotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	sticky := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Slack app of its own for one agent. Its bot always answers as that agent, whatever channel it is in, and people @mention it by name, for example `@archestra_marketing`. Several agents' bots can work in the same channel and thread; mentioning one does not wake another. Each agent has at most one bot, and each bot needs its own Slack app.\n\n" +
			"This is separate from the organization's shared Slack app (`archestra_chatops_slack`), one bot that answers as whichever agent a channel or direct message is bound to. Use either or both.\n\n" +
			"Secrets are write-only: the API never returns them, so a change made outside Terraform is not detected. Destroying the resource disconnects the bot. Enabling it checks the bot token with Slack.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: sticky, MarkdownDescription: "Agent ID."},
			"agent_id":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "ID of the agent the bot answers as."},
			"enabled":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the bot is on."},
			"bot_token":       schema.StringAttribute{Optional: true, Sensitive: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Slack bot token (`xoxb-…`)."},
			"signing_secret":  schema.StringAttribute{Optional: true, Sensitive: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Slack signing secret, used in webhook mode."},
			"app_level_token": schema.StringAttribute{Optional: true, Sensitive: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Slack app-level token (`xapp-…`), used in socket mode."},
			"app_id":          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: sticky, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Slack app ID. It must differ from the organization's app and from every other agent's bot."},
			"connection_mode": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: sticky, Validators: []validator.String{stringvalidator.OneOf("socket", "webhook")}, MarkdownDescription: "How Slack reaches Archestra: `socket` or `webhook`."},
			"connected":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the bot is connected to Slack."},
			"bot_user_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Slack user ID of the bot."},
			"team_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Slack workspace ID."},
		},
	}
}

func (r *ChatOpsSlackAgentBotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = resourceClient(req, resp)
}

func (r *ChatOpsSlackAgentBotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.State.Raw = req.Plan.Raw
	r.save(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), &resp.State, &resp.Diagnostics)
}

func (r *ChatOpsSlackAgentBotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	agentID := chatOpsAgentID(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if agentID == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	if !r.refresh(ctx, agentID, &resp.State, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.State.RemoveResource(ctx)
	}
}

func (r *ChatOpsSlackAgentBotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.Plan.Raw
	r.save(ctx, req.Plan.Raw, req.State.Raw, &resp.State, &resp.Diagnostics)
}

func (r *ChatOpsSlackAgentBotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	agentID := chatOpsAgentID(ctx, req.State, &resp.Diagnostics)
	id, err := uuid.Parse(agentID)
	if resp.Diagnostics.HasError() || err != nil {
		return
	}
	result, err := r.client.DeleteSlackAgentBot(ctx, id)
	status, body, err := chatOpsRaw(result, err)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if status != http.StatusOK && status != http.StatusNotFound {
		resp.Diagnostics.AddError("Disconnect Failed", fmt.Sprintf("API returned status %d: %s", status, string(body)))
	}
}

func (r *ChatOpsSlackAgentBotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_id"), req.ID)...)
}

func (r *ChatOpsSlackAgentBotResource) save(ctx context.Context, plan, prior tftypes.Value, state *tfsdk.State, diags *diag.Diagnostics) {
	patch := MergePatch(ctx, plan, prior, r.AttrSpecs(), diags)
	agentID := chatOpsAgentID(ctx, tfsdk.State{Schema: state.Schema, Raw: plan}, diags)
	if diags.HasError() {
		return
	}
	id, err := uuid.Parse(agentID)
	if err != nil {
		diags.AddAttributeError(path.Root("agent_id"), "Invalid Agent ID", err.Error())
		return
	}
	// The API turns the bot on when `enabled` is omitted, so always send it.
	var enabled types.Bool
	diags.Append(tfsdk.State{Schema: state.Schema, Raw: plan}.GetAttribute(ctx, path.Root("enabled"), &enabled)...)
	if diags.HasError() {
		return
	}
	if patch == nil {
		patch = map[string]any{}
	}
	patch["enabled"] = enabled.ValueBool()
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateSlackAgentBotWithBody(ctx, id, "application/json", bytes.NewReader(body))
	status, respBody, err := chatOpsRaw(result, err)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return
	}
	if status != http.StatusOK {
		diags.AddError("Save Failed", fmt.Sprintf("API returned status %d: %s", status, string(respBody)))
		return
	}
	if !r.refresh(ctx, agentID, state, diags) && !diags.HasError() {
		diags.AddError("Bot Not Found", "The bot was saved, but the API does not list it.")
	}
}

// refresh copies the stored, non-secret values into state, and reports
// whether the agent has a bot.
func (r *ChatOpsSlackAgentBotResource) refresh(ctx context.Context, agentID string, state *tfsdk.State, diags *diag.Diagnostics) bool {
	result, err := r.client.ListSlackAgentBots(ctx)
	status, body, err := chatOpsRaw(result, err)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if status != http.StatusOK {
		diags.AddError("Read Failed", fmt.Sprintf("API returned status %d: %s", status, string(body)))
		return false
	}
	var list struct {
		Bots []map[string]any `json:"bots"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return false
	}
	for _, bot := range list.Bots {
		if bot["agentId"] != agentID {
			continue
		}
		set := func(name string, value any) { diags.Append(state.SetAttribute(ctx, path.Root(name), value)...) }
		set("id", agentID)
		set("agent_id", agentID)
		set("enabled", chatOpsBool(bot["enabled"]))
		set("connected", chatOpsBool(bot["connected"]))
		set("app_id", chatOpsString(bot["appId"]))
		set("connection_mode", chatOpsString(bot["connectionMode"]))
		set("bot_user_id", chatOpsString(bot["botUserId"]))
		set("team_id", chatOpsString(bot["teamId"]))
		return true
	}
	return false
}

func chatOpsAgentID(ctx context.Context, state tfsdk.State, diags *diag.Diagnostics) string {
	var id types.String
	diags.Append(state.GetAttribute(ctx, path.Root("agent_id"), &id)...)
	if id.IsNull() || id.IsUnknown() || id.ValueString() == "" {
		diags.Append(state.GetAttribute(ctx, path.Root("id"), &id)...)
	}
	if id.IsNull() || id.IsUnknown() {
		return ""
	}
	return id.ValueString()
}
