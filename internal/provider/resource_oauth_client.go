package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"time"
)

var _ resource.ResourceWithImportState = &OauthClientResource{}

func NewOauthClientResource() resource.Resource { return &OauthClientResource{} }

type OauthClientResource struct{ client *client.ClientWithResponses }
type OauthClientResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	ClientID               types.String `tfsdk:"client_id"`
	ClientSecret           types.String `tfsdk:"client_secret"`
	GrantType              types.String `tfsdk:"grant_type"`
	Scopes                 types.Set    `tfsdk:"scopes"`
	AllowedGatewayIDs      types.Set    `tfsdk:"allowed_gateway_ids"`
	ProviderAPIKeys        types.Set    `tfsdk:"provider_api_keys"`
	IncludeFutureProviders types.Bool   `tfsdk:"include_future_providers"`
	RedirectURIs           types.Set    `tfsdk:"redirect_uris"`
	ServiceAccountID       types.String `tfsdk:"service_account_id"`
	Labels                 types.Set    `tfsdk:"labels"`
	SpendCap               types.Object `tfsdk:"spend_cap"`
	BillingTeamID          types.String `tfsdk:"billing_team_id"`
	BillingTeamName        types.String `tfsdk:"billing_team_name"`
	InitialGrants          types.Set    `tfsdk:"initial_grants"`
	RotationNonce          types.String `tfsdk:"rotation_nonce"`
	Disabled               types.Bool   `tfsdk:"disabled"`
	AuthorID               types.String `tfsdk:"author_id"`
	AuthorName             types.String `tfsdk:"author_name"`
	OrganizationID         types.String `tfsdk:"organization_id"`
	CreatedBy              types.String `tfsdk:"created_by"`
	CreatedAt              types.String `tfsdk:"created_at"`
	UpdatedAt              types.String `tfsdk:"updated_at"`
}

var oauthClientAttrSpec = []AttrSpec{
	{TFName: "client_secret", Kind: Synthetic, Sensitive: true},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "grant_type", JSONName: "grantType", Kind: Scalar},
	{TFName: "scopes", JSONName: "scopes", Kind: Set},
	{TFName: "allowed_gateway_ids", JSONName: "allowedGatewayIds", Kind: Set, OmitOnNull: true},
	{TFName: "redirect_uris", JSONName: "redirectUris", Kind: Set, OmitOnNull: true},
	{TFName: "include_future_providers", JSONName: "includeFutureProviders", Kind: Scalar, OmitOnNull: true},
	{TFName: "service_account_id", JSONName: "serviceAccountId", Kind: Scalar},
	{TFName: "billing_team_id", JSONName: "billingTeamId", Kind: Scalar},
	credentialProviderKeySpec, credentialLabelsSpec, credentialSpendCapSpec, initialGrantsAttrSpec,
	{TFName: "rotation_nonce", Kind: Synthetic},
}

func (r *OauthClientResource) AttrSpecs() []AttrSpec { return oauthClientAttrSpec }
func (r *OauthClientResource) APIShape() any         { return client.GetOauthClientsResponse{} }

// Billing team details are split into billing_team_id and billing_team_name.
func (r *OauthClientResource) KnownIntentionallySkipped() []string { return []string{"billingTeam"} }
func (r *OauthClientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth_client"
}
func (r *OauthClientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Unified managed OAuth client for MCP, A2A and LLM proxy capabilities. Client credentials and authorization-code modes share one resource; grant type changes replace the client. A nonempty rotation_nonce change rotates the secret in place and preserves client identity. Import does not recover the secret. User authorization, consent and token redemption are separate interactive operations.", Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Managed client identifier."},
		"name":                schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 256)}, MarkdownDescription: "Client display name."},
		"client_id":           schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Public OAuth client identifier."},
		"client_secret":       schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Secret returned only on creation or provider-managed rotation."},
		"grant_type":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("client_credentials"), Validators: []validator.String{stringvalidator.OneOf("client_credentials", "authorization_code")}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Immutable OAuth grant type."},
		"scopes":              schema.SetAttribute{Required: true, ElementType: types.StringType, Validators: []validator.Set{setvalidator.SizeBetween(1, 3), setvalidator.ValueStringsAre(stringvalidator.OneOf("mcp", "a2a", "llm:proxy"))}, MarkdownDescription: "Enabled API capabilities: mcp, a2a and llm:proxy."},
		"allowed_gateway_ids": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: "Allowed MCP gateway and A2A agent UUIDs; an empty set clears bindings."},
		"redirect_uris":       schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: "Authorization-code redirect URIs; an empty set clears them when the selected mode permits it."},
		"provider_api_keys":   credentialProviderKeysAttribute(), "labels": credentialLabelsAttribute(), "spend_cap": credentialSpendCapAttribute(),
		"include_future_providers": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Use current and future shared primary provider keys with explicit mappings as overrides."},
		"service_account_id":       schema.StringAttribute{Optional: true, MarkdownDescription: "Service account this client acts as; removing the value clears the binding."},
		"billing_team_id":          schema.StringAttribute{Optional: true, MarkdownDescription: "Team billed for client-credentials usage; removing the value clears billing."},
		"billing_team_name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Billing team display name."},
		"initial_grants":           initialGrantsAttribute(),
		"rotation_nonce":           schema.StringAttribute{Optional: true, MarkdownDescription: "Set or change a nonempty value to rotate the secret while preserving client identity."},
		"disabled":                 schema.BoolAttribute{Computed: true, MarkdownDescription: "Server-reported disabled state."},
		"author_id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Author identity."},
		"author_name":              schema.StringAttribute{Computed: true, MarkdownDescription: "Author display name."},
		"organization_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Organization identifier."},
		"created_by":               schema.StringAttribute{Computed: true, MarkdownDescription: "Creator metadata as JSON."},
		"created_at":               schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
		"updated_at":               schema.StringAttribute{Computed: true, MarkdownDescription: "Last update timestamp."},
	}}
}
func (r *OauthClientResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *OauthClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OauthClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), oauthClientAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_oauth_client Create", patch, oauthClientAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateOauthClientWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	data.ClientSecret = types.StringValue(result.JSON200.ClientSecret)
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *OauthClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OauthClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	// The unified API exposes a paginated list, without a detail GET.
	limit := 100
	for offset := 0; ; {
		result, err := r.client.GetOauthClientsWithResponse(ctx, &client.GetOauthClientsParams{Limit: &limit, Offset: &offset})
		if err != nil {
			resp.Diagnostics.AddError("API Error", err.Error())
			return
		}
		if result.JSON200 == nil {
			resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
			return
		}
		var page struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(result.Body, &page); err != nil {
			resp.Diagnostics.AddError("Invalid API Response", err.Error())
			return
		}
		for _, raw := range page.Data {
			var identity struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &identity); err != nil {
				resp.Diagnostics.AddError("Invalid API Response", err.Error())
				return
			}
			if identity.ID == data.ID.ValueString() {
				r.flatten(ctx, &data, raw, &resp.Diagnostics)
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}
		if len(page.Data) < limit {
			break
		}
		offset += len(page.Data)
	}
	resp.State.RemoveResource(ctx)
}
func (r *OauthClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior OauthClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, oauthClientAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(patch, "initialGrants")
	data.ClientSecret = prior.ClientSecret
	if len(patch) > 0 {
		// Name/scopes are required and the API defaults a missing grantType to client_credentials.
		patch["name"] = data.Name.ValueString()
		patch["grantType"] = data.GrantType.ValueString()
		var scopes []string
		resp.Diagnostics.Append(data.Scopes.ElementsAs(ctx, &scopes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		patch["scopes"] = scopes
		LogPatch(ctx, "archestra_oauth_client Update", patch, oauthClientAttrSpec)
		body, err := json.Marshal(patch)
		if err != nil {
			resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
			return
		}
		result, err := r.client.UpdateOauthClientWithBodyWithResponse(ctx, prior.ID.ValueString(), "application/json", bytes.NewReader(body))
		if err != nil {
			resp.Diagnostics.AddError("API Error", err.Error())
			return
		}
		if result.JSON200 == nil {
			resp.Diagnostics.AddError("Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
			return
		}
		r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	} else {
		// Preserve observations when only the synthetic rotation input changed.
		nonce := data.RotationNonce
		data = prior
		data.RotationNonce = nonce
	}
	desiredNonce := data.RotationNonce
	rotate := !desiredNonce.IsNull() && desiredNonce.ValueString() != "" && !desiredNonce.Equal(prior.RotationNonce)
	if rotate {
		// Keep the old nonce until the secret is saved so failures remain retryable.
		data.RotationNonce = prior.RotationNonce
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if rotate {
		result, err := r.client.RotateOauthClientSecretWithResponse(ctx, prior.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("API Error", err.Error())
			return
		}
		if result.JSON200 == nil {
			resp.Diagnostics.AddError("Secret Rotation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
			return
		}
		data.ClientSecret = types.StringValue(result.JSON200.ClientSecret)
		data.RotationNonce = desiredNonce
		r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *OauthClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OauthClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.DeleteOauthClientWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *OauthClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *OauthClientResource) flatten(ctx context.Context, data *OauthClientResourceModel, body []byte, diags *diag.Diagnostics) {
	var raw struct {
		credentialRoutingResponse
		ID                     string          `json:"id"`
		Name                   string          `json:"name"`
		ClientID               string          `json:"clientId"`
		GrantType              string          `json:"grantType"`
		Scopes                 []string        `json:"scopes"`
		AllowedGatewayIDs      []string        `json:"allowedGatewayIds"`
		RedirectURIs           []string        `json:"redirectUris"`
		IncludeFutureProviders bool            `json:"includeFutureProviders"`
		ServiceAccountID       *string         `json:"serviceAccountId"`
		Disabled               bool            `json:"disabled"`
		AuthorID               *string         `json:"authorId"`
		AuthorName             *string         `json:"authorName"`
		OrganizationID         string          `json:"organizationId"`
		CreatedBy              json.RawMessage `json:"createdBy"`
		CreatedAt              *time.Time      `json:"createdAt"`
		UpdatedAt              *time.Time      `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringValue(raw.ID)
	data.Name = types.StringValue(raw.Name)
	data.ClientID = types.StringValue(raw.ClientID)
	data.GrantType = types.StringValue(raw.GrantType)
	data.IncludeFutureProviders = types.BoolValue(raw.IncludeFutureProviders)
	var d diag.Diagnostics
	data.Scopes, d = types.SetValueFrom(ctx, types.StringType, raw.Scopes)
	diags.Append(d...)
	data.AllowedGatewayIDs, d = types.SetValueFrom(ctx, types.StringType, raw.AllowedGatewayIDs)
	diags.Append(d...)
	data.RedirectURIs, d = types.SetValueFrom(ctx, types.StringType, raw.RedirectURIs)
	diags.Append(d...)
	data.ServiceAccountID = types.StringPointerValue(raw.ServiceAccountID)
	data.Disabled = types.BoolValue(raw.Disabled)
	data.AuthorID = types.StringPointerValue(raw.AuthorID)
	data.AuthorName = types.StringPointerValue(raw.AuthorName)
	data.OrganizationID = types.StringValue(raw.OrganizationID)
	data.CreatedBy = credentialJSONValue(raw.CreatedBy, diags)
	data.CreatedAt = canonicalAPITimestamp(raw.CreatedAt)
	data.UpdatedAt = canonicalAPITimestamp(raw.UpdatedAt)
	flattenCredentialRouting(ctx, raw.credentialRoutingResponse, &data.ProviderAPIKeys, &data.Labels, &data.SpendCap, &data.BillingTeamID, &data.BillingTeamName, diags)
}
