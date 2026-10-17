package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func NewMarketplaceShareLinkResource() resource.Resource { return &MarketplaceShareLinkResource{} }

type MarketplaceShareLinkResource struct{ client *client.ClientWithResponses }
type MarketplaceShareLinkResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	SkillIDs         types.Set    `tfsdk:"skill_ids"`
	PluginIDs        types.Set    `tfsdk:"plugin_ids"`
	PluginPlatform   types.String `tfsdk:"plugin_platform"`
	ExpiresAt        types.String `tfsdk:"expires_at"`
	RotationNonce    types.String `tfsdk:"rotation_nonce"`
	RawToken         types.String `tfsdk:"raw_token"`
	CloneURL         types.String `tfsdk:"clone_url"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	CreatedByUserID  types.String `tfsdk:"created_by_user_id"`
	TokenStart       types.String `tfsdk:"token_start"`
	MarketplaceName  types.String `tfsdk:"marketplace_name"`
	PluginClientType types.String `tfsdk:"plugin_client_type"`
	RevokedAt        types.String `tfsdk:"revoked_at"`
	LastUsedAt       types.String `tfsdk:"last_used_at"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	Status           types.String `tfsdk:"status"`
	Skills           types.String `tfsdk:"skills"`
	Plugins          types.String `tfsdk:"plugins"`
}

var marketplaceShareLinkAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar, OmitOnNull: true},
	{TFName: "skill_ids", JSONName: "skillIds", Kind: Set},
	{TFName: "plugin_ids", JSONName: "pluginIds", Kind: Set},
	{TFName: "plugin_platform", JSONName: "pluginPlatform", Kind: Scalar, OmitOnNull: true},
	{TFName: "expires_at", JSONName: "expiresAt", Kind: Scalar},
	{TFName: "rotation_nonce", Kind: Synthetic},
	{TFName: "raw_token", Kind: Synthetic, Sensitive: true},
	{TFName: "clone_url", Kind: Synthetic, Sensitive: true},
}

func (r *MarketplaceShareLinkResource) AttrSpecs() []AttrSpec               { return marketplaceShareLinkAttrSpec }
func (r *MarketplaceShareLinkResource) APIShape() any                       { return client.GetSkillShareLinksResponse{} }
func (r *MarketplaceShareLinkResource) KnownIntentionallySkipped() []string { return nil }
func (r *MarketplaceShareLinkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_share_link"
}
func (r *MarketplaceShareLinkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{MarkdownDescription: "Revocable marketplace link exposing an explicit skill/plugin set through a secret-once clone URL. Membership, metadata, expiry and explicit rotation changes replace the link and revoke its predecessor. Expiration remains observed state; out-of-band revocation triggers recreation. Import adopts metadata without recovering its token.", Attributes: map[string]schema.Attribute{
		"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Share-link UUID."},
		"name":               schema.StringAttribute{Optional: true, PlanModifiers: replace, Validators: []validator.String{stringvalidator.LengthBetween(1, 200), stringvalidator.RegexMatches(regexp.MustCompile(`^\S(?:.*\S)?$`), "Use a name without surrounding whitespace.")}, MarkdownDescription: "Optional link label."},
		"skill_ids":          schema.SetAttribute{Required: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()}, Validators: []validator.Set{setvalidator.SizeAtMost(1111), setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical skill UUID."))}, MarkdownDescription: "Explicit skill membership, including an empty set for plugin-only links."},
		"plugin_ids":         schema.SetAttribute{Required: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()}, Validators: []validator.Set{setvalidator.SizeAtMost(555), setvalidator.ValueStringsAre(stringvalidator.RegexMatches(canonicalUUIDRegexp, "Use a lowercase canonical plugin UUID."))}, MarkdownDescription: "Explicit approved-plugin membership, including an empty set for skill-only links."},
		"plugin_platform":    schema.StringAttribute{Optional: true, PlanModifiers: replace, Validators: []validator.String{stringvalidator.OneOf("posix", "windows")}, MarkdownDescription: "Required target operating-system family when sharing plugins."},
		"expires_at":         schema.StringAttribute{Optional: true, PlanModifiers: replace, Validators: []validator.String{shareLinkTimestampValidator{}}, MarkdownDescription: "Canonical UTC expiry with millisecond precision; required for plugin links."},
		"rotation_nonce":     schema.StringAttribute{Optional: true, PlanModifiers: replace, MarkdownDescription: "Change this value to revoke the old link and create a replacement secret."},
		"raw_token":          schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Secret returned once on creation and preserved through reads."},
		"clone_url":          schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Secret-once Git clone URL containing the raw token."},
		"organization_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Owning organization UUID."},
		"created_by_user_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Creating user or service-account actor."},
		"token_start":        schema.StringAttribute{Computed: true, MarkdownDescription: "Display prefix of the link token."},
		"marketplace_name":   schema.StringAttribute{Computed: true, MarkdownDescription: "Marketplace name frozen at link creation."},
		"plugin_client_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Observed client format shared by all attached plugins."},
		"revoked_at":         schema.StringAttribute{Computed: true, MarkdownDescription: "Observed revocation timestamp."},
		"last_used_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Last observed anonymous link use."},
		"created_at":         schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
		"updated_at":         schema.StringAttribute{Computed: true, MarkdownDescription: "Last metadata timestamp."},
		"status":             schema.StringAttribute{Computed: true, MarkdownDescription: "Observed active, expired or revoked lifecycle state."},
		"skills":             schema.StringAttribute{Computed: true, MarkdownDescription: "Observed skill summaries as JSON."},
		"plugins":            schema.StringAttribute{Computed: true, MarkdownDescription: "Observed plugin summaries as JSON."},
	}}
}
func (r *MarketplaceShareLinkResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data MarketplaceShareLinkResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.SkillIDs.IsUnknown() && !data.PluginIDs.IsUnknown() && len(data.SkillIDs.Elements())+len(data.PluginIDs.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("skill_ids"), "Empty marketplace membership", "Declare at least one skill or plugin.")
	}
	if !data.PluginIDs.IsUnknown() && len(data.PluginIDs.Elements()) > 0 {
		if data.PluginPlatform.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("plugin_platform"), "Missing plugin platform", "Plugin links require a target platform.")
		}
		if data.ExpiresAt.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Missing plugin expiration", "Plugin links require an expiration timestamp.")
		}
	}
}
func (r *MarketplaceShareLinkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected API client, got %T", req.ProviderData))
		return
	}
	r.client = c
}
func (r *MarketplaceShareLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MarketplaceShareLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), marketplaceShareLinkAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "Marketplace share link Create", patch, marketplaceShareLinkAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateSkillShareLinkWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	var envelope struct {
		Link     json.RawMessage `json:"link"`
		RawToken string          `json:"rawToken"`
		CloneURL string          `json:"cloneUrl"`
	}
	if err := json.Unmarshal(result.Body, &envelope); err != nil {
		resp.Diagnostics.AddError("Invalid API Response", err.Error())
		return
	}
	r.flatten(ctx, &data, envelope.Link, &resp.Diagnostics)
	data.RawToken = types.StringValue(envelope.RawToken)
	data.CloneURL = types.StringValue(envelope.CloneURL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *MarketplaceShareLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MarketplaceShareLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.GetSkillShareLinksWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	for _, item := range result.JSON200.Links {
		if item.Id.String() != data.ID.ValueString() {
			continue
		}
		body, err := json.Marshal(item)
		if err != nil {
			resp.Diagnostics.AddError("Response Encoding Failed", err.Error())
			return
		}
		r.flatten(ctx, &data, body, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		if data.Status.ValueString() == "revoked" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	resp.State.RemoveResource(ctx)
}
func (r *MarketplaceShareLinkResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Immutable share link", "Changing link inputs requires replacement and revocation of its predecessor.")
}
func (r *MarketplaceShareLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MarketplaceShareLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.RevokeSkillShareLinkWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Revocation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *MarketplaceShareLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *MarketplaceShareLinkResource) flatten(ctx context.Context, data *MarketplaceShareLinkResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID               string          `json:"id"`
		Name             *string         `json:"name"`
		PluginPlatform   *string         `json:"pluginPlatform"`
		ExpiresAt        *string         `json:"expiresAt"`
		OrganizationID   string          `json:"organizationId"`
		CreatedByUserID  string          `json:"createdByUserId"`
		TokenStart       string          `json:"tokenStart"`
		MarketplaceName  string          `json:"marketplaceName"`
		PluginClientType *string         `json:"pluginClientType"`
		RevokedAt        *string         `json:"revokedAt"`
		LastUsedAt       *string         `json:"lastUsedAt"`
		CreatedAt        *string         `json:"createdAt"`
		UpdatedAt        *string         `json:"updatedAt"`
		Status           string          `json:"status"`
		Skills           json.RawMessage `json:"skills"`
		Plugins          json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	var skills, plugins []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(value.Skills, &skills); err != nil {
		diags.AddError("Invalid skill membership", err.Error())
		return
	}
	if err := json.Unmarshal(value.Plugins, &plugins); err != nil {
		diags.AddError("Invalid plugin membership", err.Error())
		return
	}
	skillIDs := make([]string, len(skills))
	for i, item := range skills {
		skillIDs[i] = item.ID
	}
	pluginIDs := make([]string, len(plugins))
	for i, item := range plugins {
		pluginIDs[i] = item.ID
	}
	var d diag.Diagnostics
	data.SkillIDs, d = types.SetValueFrom(ctx, types.StringType, skillIDs)
	diags.Append(d...)
	data.PluginIDs, d = types.SetValueFrom(ctx, types.StringType, pluginIDs)
	diags.Append(d...)
	data.ID = types.StringValue(value.ID)
	data.Name = types.StringPointerValue(value.Name)
	data.PluginPlatform = types.StringPointerValue(value.PluginPlatform)
	data.ExpiresAt = canonicalTokenTime(value.ExpiresAt)
	data.OrganizationID = types.StringValue(value.OrganizationID)
	data.CreatedByUserID = types.StringValue(value.CreatedByUserID)
	data.TokenStart = types.StringValue(value.TokenStart)
	data.MarketplaceName = types.StringValue(value.MarketplaceName)
	data.PluginClientType = types.StringPointerValue(value.PluginClientType)
	data.RevokedAt = canonicalTokenTime(value.RevokedAt)
	data.LastUsedAt = canonicalTokenTime(value.LastUsedAt)
	data.CreatedAt = canonicalTokenTime(value.CreatedAt)
	data.UpdatedAt = canonicalTokenTime(value.UpdatedAt)
	data.Status = types.StringValue(value.Status)
	data.Skills = credentialJSONValue(value.Skills, diags)
	data.Plugins = credentialJSONValue(value.Plugins, diags)
}

type shareLinkTimestampValidator struct{}

func (shareLinkTimestampValidator) Description(context.Context) string {
	return "Use canonical RFC3339 UTC with millisecond precision and no redundant fractional zeros."
}
func (v shareLinkTimestampValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (shareLinkTimestampValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !strings.HasSuffix(value, "Z") || parsed.Nanosecond()%int(time.Millisecond) != 0 || parsed.UTC().Format(time.RFC3339Nano) != value {
		resp.Diagnostics.AddAttributeError(req.Path, "Noncanonical share-link timestamp", "Use canonical UTC with millisecond precision, for example 2040-01-01T00:00:00Z or 2040-01-01T00:00:00.123Z.")
	}
}
