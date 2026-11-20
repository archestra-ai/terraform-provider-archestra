package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The ChatOps integrations are one configuration slot each: the API reads and
// writes the whole slot, merges omitted fields with what is stored, and never
// returns a secret. One implementation serves them all, driven by their field
// lists.

func NewChatOpsSlackResource() resource.Resource {
	return &ChatOpsResource{channel: chatOpsChannel{
		typeName: "chatops_slack",
		id:       "slack",
		description: "The organization's shared Slack app: one bot that answers as whichever agent a channel or direct message is bound to. People pick the agent on their first message, and reach another one with `Agent > message` or the `list` command. One per organization.\n\n" +
			"To give an agent a Slack bot of its own, which people @mention by name and which always answers as that agent, use `archestra_chatops_slack_agent_bot`. Several such bots can work in the same channel and thread, and they run alongside this app.\n\n" +
			"Secrets are write-only: the API never returns them, so a change made outside Terraform is not detected. Destroying the resource turns the app off and keeps the stored credentials. Enabling it checks the bot token with Slack.",
		fields: []chatOpsField{
			{tf: "bot_token", json: "botToken", secret: true, desc: "Slack bot token (`xoxb-…`)."},
			{tf: "signing_secret", json: "signingSecret", secret: true, desc: "Slack signing secret, used in webhook mode."},
			{tf: "app_level_token", json: "appLevelToken", secret: true, desc: "Slack app-level token (`xapp-…`), used in socket mode."},
			{tf: "app_id", json: "appId", desc: "Slack app ID."},
			{tf: "connection_mode", json: "connectionMode", desc: "How Slack reaches Archestra: `socket` or `webhook`.", oneOf: []string{"socket", "webhook"}},
		},
		apiShape: client.GetSlackChatOpsConfigResponse{},
		get: func(ctx context.Context, c *client.ClientWithResponses) (int, []byte, error) {
			r, err := c.GetSlackChatOpsConfig(ctx)
			return chatOpsRaw(r, err)
		},
		put: func(ctx context.Context, c *client.ClientWithResponses, body io.Reader) (int, []byte, error) {
			r, err := c.UpdateSlackChatOpsConfigWithBody(ctx, "application/json", body)
			return chatOpsRaw(r, err)
		},
	}}
}

func NewChatOpsMsTeamsResource() resource.Resource {
	return &ChatOpsResource{channel: chatOpsChannel{
		typeName:    "chatops_ms_teams",
		id:          "ms-teams",
		description: "The organization's Microsoft Teams integration, so people can chat with agents in Teams. One per organization. The app secret is write-only: the API never returns it, so a change made outside Terraform is not detected. Destroying the resource turns the integration off and keeps the stored credentials. Enabling it checks the credentials with Microsoft Entra ID.",
		fields: []chatOpsField{
			{tf: "app_id", json: "appId", desc: "Microsoft Entra application (client) ID of the bot."},
			{tf: "app_secret", json: "appSecret", secret: true, desc: "Client secret of the bot's application."},
			{tf: "tenant_id", json: "tenantId", desc: "Microsoft Entra tenant ID."},
			{tf: "group_chat_file_sign_in", json: "groupChatFileSignIn", boolean: true, desc: "Whether people sign in to share files from group chats."},
			{tf: "sign_in_redirect_uri", json: "signInRedirectUri", computedOnly: true, desc: "Redirect URI to register on the application for sign-in."},
		},
		apiShape: client.GetMsTeamsChatOpsConfigResponse{},
		get: func(ctx context.Context, c *client.ClientWithResponses) (int, []byte, error) {
			r, err := c.GetMsTeamsChatOpsConfig(ctx)
			return chatOpsRaw(r, err)
		},
		put: func(ctx context.Context, c *client.ClientWithResponses, body io.Reader) (int, []byte, error) {
			r, err := c.UpdateChatOpsConfigInQuickstartWithBody(ctx, "application/json", body)
			return chatOpsRaw(r, err)
		},
	}}
}

func NewChatOpsTelegramResource() resource.Resource {
	return &ChatOpsResource{channel: chatOpsChannel{
		typeName:    "chatops_telegram",
		id:          "telegram",
		description: "The organization's Telegram integration, so people can chat with agents in Telegram. One per organization. The bot token is write-only: the API never returns it, so a change made outside Terraform is not detected. Destroying the resource turns the integration off and keeps the stored token. Enabling it checks the token with Telegram.",
		fields: []chatOpsField{
			{tf: "bot_token", json: "botToken", secret: true, desc: "Telegram bot token from BotFather."},
		},
		apiShape: client.GetTelegramChatOpsConfigResponse{},
		get: func(ctx context.Context, c *client.ClientWithResponses) (int, []byte, error) {
			r, err := c.GetTelegramChatOpsConfig(ctx)
			return chatOpsRaw(r, err)
		},
		put: func(ctx context.Context, c *client.ClientWithResponses, body io.Reader) (int, []byte, error) {
			r, err := c.UpdateTelegramChatOpsConfigWithBody(ctx, "application/json", body)
			return chatOpsRaw(r, err)
		},
	}}
}

var _ resource.ResourceWithImportState = &ChatOpsResource{}

type ChatOpsResource struct {
	client  *client.ClientWithResponses
	channel chatOpsChannel
}

type chatOpsChannel struct {
	typeName    string
	id          string
	description string
	fields      []chatOpsField
	apiShape    any
	get         func(context.Context, *client.ClientWithResponses) (int, []byte, error)
	put         func(context.Context, *client.ClientWithResponses, io.Reader) (int, []byte, error)
}

type chatOpsField struct {
	tf, json, desc string
	secret         bool
	boolean        bool
	computedOnly   bool
	oneOf          []string
}

func (r *ChatOpsResource) AttrSpecs() []AttrSpec {
	specs := []AttrSpec{{TFName: "enabled", JSONName: "enabled", Kind: Scalar}}
	for _, f := range r.channel.fields {
		if f.computedOnly {
			continue
		}
		specs = append(specs, AttrSpec{TFName: f.tf, JSONName: f.json, Kind: Scalar, Sensitive: f.secret, OmitOnNull: true})
	}
	return specs
}

func (r *ChatOpsResource) APIShape() any                       { return r.channel.apiShape }
func (r *ChatOpsResource) KnownIntentionallySkipped() []string { return nil }

func (r *ChatOpsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.channel.typeName
}

func (r *ChatOpsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Integration name."},
		"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the integration is on."},
		"configured": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the stored configuration is complete."},
		"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Time the configuration was last saved."},
	}
	for _, f := range r.channel.fields {
		switch {
		case f.computedOnly:
			attrs[f.tf] = schema.StringAttribute{Computed: true, MarkdownDescription: f.desc}
		case f.boolean:
			attrs[f.tf] = schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, MarkdownDescription: f.desc}
		case f.secret:
			attrs[f.tf] = schema.StringAttribute{Optional: true, Sensitive: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: f.desc}
		default:
			validators := []validator.String{stringvalidator.LengthAtLeast(1)}
			if len(f.oneOf) > 0 {
				validators = append(validators, stringvalidator.OneOf(f.oneOf...))
			}
			attrs[f.tf] = schema.StringAttribute{Optional: true, Computed: true, Validators: validators, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: f.desc}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: r.channel.description, Attributes: attrs}
}

func (r *ChatOpsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = resourceClient(req, resp)
}

func (r *ChatOpsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), r.AttrSpecs(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.Raw = req.Plan.Raw
	r.write(ctx, patch, &resp.State, &resp.Diagnostics)
}

func (r *ChatOpsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Nothing adopted yet: Create writes the organization's configuration.
	if id.IsNull() || id.IsUnknown() || id.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	r.refresh(ctx, &resp.State, &resp.Diagnostics)
}

func (r *ChatOpsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, r.AttrSpecs(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.Raw = req.Plan.Raw
	r.write(ctx, patch, &resp.State, &resp.Diagnostics)
}

func (r *ChatOpsResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	status, body, err := r.channel.put(ctx, r.client, bytes.NewReader([]byte(`{"enabled":false}`)))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if status != http.StatusOK {
		resp.Diagnostics.AddError(chatOpsStatusError("Disable Failed", status, body))
	}
}

func (r *ChatOpsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), r.channel.id)...)
}

func (r *ChatOpsResource) write(ctx context.Context, patch map[string]any, state *tfsdk.State, diags *diag.Diagnostics) {
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return
	}
	status, respBody, err := r.channel.put(ctx, r.client, bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return
	}
	if status != http.StatusOK {
		diags.AddError(chatOpsStatusError("Save Failed", status, respBody))
		return
	}
	r.refresh(ctx, state, diags)
}

// refresh copies the stored, non-secret values into state. Secrets stay as
// planned or as last applied.
func (r *ChatOpsResource) refresh(ctx context.Context, state *tfsdk.State, diags *diag.Diagnostics) {
	status, body, err := r.channel.get(ctx, r.client)
	if err != nil {
		diags.AddError("API Error", err.Error())
		return
	}
	if status != http.StatusOK {
		diags.AddError(chatOpsStatusError("Read Failed", status, body))
		return
	}
	var stored map[string]any
	if err := json.Unmarshal(body, &stored); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	set := func(name string, value any) { diags.Append(state.SetAttribute(ctx, path.Root(name), value)...) }
	set("id", r.channel.id)
	set("enabled", chatOpsBool(stored["enabled"]))
	set("configured", chatOpsBool(stored["configured"]))
	set("updated_at", chatOpsString(stored["updatedAt"]))
	for _, f := range r.channel.fields {
		switch {
		case f.secret:
		case f.boolean:
			set(f.tf, chatOpsBool(stored[f.json]))
		default:
			set(f.tf, chatOpsString(stored[f.json]))
		}
	}
}

func chatOpsBool(value any) types.Bool {
	if b, ok := value.(bool); ok {
		return types.BoolValue(b)
	}
	return types.BoolValue(false)
}

func chatOpsString(value any) types.String {
	if s, ok := value.(string); ok && s != "" {
		return types.StringValue(s)
	}
	return types.StringNull()
}

// The raw client: the generated response parser rejects the error body an
// Archestra release without these routes sends with its 404.
func chatOpsRaw(resp *http.Response, err error) (int, []byte, error) {
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

const chatOpsMinimumAPIVersion = "1.5.0-rc.1"

func chatOpsStatusError(summary string, status int, body []byte) (string, string) {
	if status == http.StatusNotFound {
		return summary, fmt.Sprintf("This Archestra server has no ChatOps configuration API; it needs Archestra %s or later.", chatOpsMinimumAPIVersion)
	}
	return summary, fmt.Sprintf("API returned status %d: %s", status, string(body))
}
