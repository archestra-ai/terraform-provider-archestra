package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func NewServiceAccountTokenResource() resource.Resource { return &ServiceAccountTokenResource{} }

type ServiceAccountTokenResource struct{ client *client.ClientWithResponses }
type ServiceAccountTokenResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ServiceAccountID   types.String `tfsdk:"service_account_id"`
	Name               types.String `tfsdk:"name"`
	Token              types.String `tfsdk:"token"`
	TokenStart         types.String `tfsdk:"token_start"`
	ExpiresIn          types.Int64  `tfsdk:"expires_in"`
	ExpiresAt          types.String `tfsdk:"expires_at"`
	EffectiveExpiresAt types.String `tfsdk:"effective_expires_at"`
	Disabled           types.Bool   `tfsdk:"disabled"`
	LastUsedAt         types.String `tfsdk:"last_used_at"`
	CreatedAt          types.String `tfsdk:"created_at"`
	RotationNonce      types.String `tfsdk:"rotation_nonce"`
}

var serviceAccountTokenAttrSpec = []AttrSpec{
	{TFName: "token", Kind: Synthetic, Sensitive: true},
	{TFName: "service_account_id", Kind: Synthetic},
	{TFName: "rotation_nonce", Kind: Synthetic},
	{TFName: "expires_in", JSONName: "expiresIn", Kind: Scalar, OmitOnNull: true},
	{TFName: "expires_at", JSONName: "expiresAt", Kind: Scalar},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "disabled", JSONName: "disabled", Kind: Scalar, OmitOnNull: true},
}

func (r *ServiceAccountTokenResource) AttrSpecs() []AttrSpec { return serviceAccountTokenAttrSpec }

// The parent detail embeds these token metadata fields; the create projection additionally includes the secret-once value.
func (r *ServiceAccountTokenResource) APIShape() any {
	return client.CreateServiceAccountTokenResponse{}
}
func (r *ServiceAccountTokenResource) KnownIntentionallySkipped() []string { return nil }
func (r *ServiceAccountTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_api_key"
}
func (r *ServiceAccountTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Revocable credential for a service account; the secret is returned only on creation and stored in sensitive Terraform state. Import adopts metadata and cannot recover the secret. Destroy revokes this token.", Attributes: map[string]schema.Attribute{
		"id":                   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Composite service-account UUID and token UUID."},
		"service_account_id":   schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Service account that owns the token."},
		"name":                 schema.StringAttribute{Required: true, MarkdownDescription: "Token display name."},
		"token":                schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Secret returned only on creation."},
		"token_start":          schema.StringAttribute{Computed: true, MarkdownDescription: "Non-secret prefix identifying the token."},
		"expires_in":           schema.Int64Attribute{Optional: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}, MarkdownDescription: "Creation-time lifetime in seconds; changing it issues a replacement token."},
		"expires_at":           schema.StringAttribute{Optional: true, MarkdownDescription: "Explicit absolute expiry timestamp; removing a previously configured value clears the expiry."},
		"effective_expires_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Expiry timestamp currently stored by the API."},
		"disabled":             schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether this credential is disabled."},
		"last_used_at":         schema.StringAttribute{Computed: true, MarkdownDescription: "Most recent authentication time."},
		"created_at":           schema.StringAttribute{Computed: true, MarkdownDescription: "Token creation timestamp."},
		"rotation_nonce":       schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Change this declaration to revoke and replace the credential."},
	}}
}
func (r *ServiceAccountTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ServiceAccountTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	account, err := uuid.Parse(data.ServiceAccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid service_account_id", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), serviceAccountTokenAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(patch, "expiresAt")
	delete(patch, "disabled")
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateServiceAccountTokenWithBodyWithResponse(ctx, account, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Token Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	expiry, disabled := data.ExpiresAt, data.Disabled
	r.flatten(&data, result.Body, &resp.Diagnostics)
	data.ID = types.StringValue(account.String() + ":" + result.JSON200.Id.String())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	followup := map[string]any{}
	if !expiry.IsNull() && !expiry.IsUnknown() {
		followup["expiresAt"] = expiry.ValueString()
	}
	if !disabled.IsNull() && !disabled.IsUnknown() && disabled.ValueBool() != data.Disabled.ValueBool() {
		followup["disabled"] = disabled.ValueBool()
	}
	if len(followup) > 0 {
		if !r.update(ctx, &data, followup, &resp.Diagnostics) {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *ServiceAccountTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	account, token, err := serviceAccountTokenIDs(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.GetServiceAccountWithResponse(ctx, account)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Token Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	for _, item := range result.JSON200.Tokens {
		if item.Id == token {
			body, err := json.Marshal(item)
			if err != nil {
				resp.Diagnostics.AddError("Response Encoding Failed", err.Error())
				return
			}
			data.ServiceAccountID = types.StringValue(account.String())
			r.flatten(&data, body, &resp.Diagnostics)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}
func (r *ServiceAccountTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceAccountTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, serviceAccountTokenAttrSpec, &resp.Diagnostics)
	delete(patch, "expiresIn")
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	if r.update(ctx, &data, patch, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *ServiceAccountTokenResource) update(ctx context.Context, data *ServiceAccountTokenResourceModel, patch map[string]any, diags *diag.Diagnostics) bool {
	account, token, err := serviceAccountTokenIDs(data.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid ID", err.Error())
		return false
	}
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return false
	}
	result, err := r.client.UpdateServiceAccountTokenWithBodyWithResponse(ctx, account, token, "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Token Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return false
	}
	r.flatten(data, result.Body, diags)
	return !diags.HasError()
}
func (r *ServiceAccountTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	account, token, err := serviceAccountTokenIDs(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteServiceAccountTokenWithResponse(ctx, account, token)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Token Revocation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *ServiceAccountTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, _, err := serviceAccountTokenIDs(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func serviceAccountTokenIDs(id string) (uuid.UUID, uuid.UUID, error) {
	account, token, found := strings.Cut(id, ":")
	if !found {
		return uuid.Nil, uuid.Nil, fmt.Errorf("expected service_account_uuid:token_uuid")
	}
	a, err := uuid.Parse(account)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	t, err := uuid.Parse(token)
	return a, t, err
}
func (r *ServiceAccountTokenResource) flatten(data *ServiceAccountTokenResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		Name       string  `json:"name"`
		Token      *string `json:"token"`
		TokenStart string  `json:"tokenStart"`
		ExpiresAt  *string `json:"expiresAt"`
		Disabled   bool    `json:"disabled"`
		LastUsedAt *string `json:"lastUsedAt"`
		CreatedAt  string  `json:"createdAt"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid Token Response", err.Error())
		return
	}
	data.Name = types.StringValue(value.Name)
	data.TokenStart = types.StringValue(value.TokenStart)
	data.Disabled = types.BoolValue(value.Disabled)
	data.EffectiveExpiresAt = canonicalTokenTime(value.ExpiresAt)
	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		planned, pe := time.Parse(time.RFC3339Nano, data.ExpiresAt.ValueString())
		actual, ae := time.Parse(time.RFC3339Nano, data.EffectiveExpiresAt.ValueString())
		if pe != nil || ae != nil || !planned.Equal(actual) {
			data.ExpiresAt = types.StringPointerValue(value.ExpiresAt)
		}
	}
	if value.Token != nil {
		data.Token = types.StringPointerValue(value.Token)
	} else if data.Token.IsUnknown() {
		data.Token = types.StringNull()
	}
	data.CreatedAt = canonicalTokenTime(&value.CreatedAt)
	data.LastUsedAt = canonicalTokenTime(value.LastUsedAt)
}

func canonicalTokenTime(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return types.StringPointerValue(value)
	}
	return types.StringValue(parsed.Format(time.RFC3339Nano))
}
