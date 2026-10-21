package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
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

var _ resource.ResourceWithImportState = &VirtualAPIKeyResource{}

func NewVirtualAPIKeyResource() resource.Resource { return &VirtualAPIKeyResource{} }

type VirtualAPIKeyResource struct{ client *client.ClientWithResponses }
type VirtualAPIKeyResourceModel struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	KeyType                   types.String `tfsdk:"key_type"`
	ProviderAPIKeys           types.Set    `tfsdk:"provider_api_keys"`
	IncludeFutureProviders    types.Bool   `tfsdk:"include_future_providers"`
	Labels                    types.Set    `tfsdk:"labels"`
	SpendCap                  types.Object `tfsdk:"spend_cap"`
	BillingTeamID             types.String `tfsdk:"billing_team_id"`
	BillingTeamName           types.String `tfsdk:"billing_team_name"`
	ExpiresAt                 types.String `tfsdk:"expires_at"`
	OwnerID                   types.String `tfsdk:"owner_id"`
	InitialGrants             types.Set    `tfsdk:"permissions"`
	RotationNonce             types.String `tfsdk:"rotation_nonce"`
	Value                     types.String `tfsdk:"value"`
	SecretID                  types.String `tfsdk:"secret_id"`
	TokenStart                types.String `tfsdk:"token_start"`
	Scope                     types.String `tfsdk:"scope"`
	AuthorID                  types.String `tfsdk:"author_id"`
	AuthorName                types.String `tfsdk:"author_name"`
	CreatedBy                 types.String `tfsdk:"created_by"`
	CreatedByServiceAccountID types.String `tfsdk:"created_by_service_account_id"`
	OrganizationID            types.String `tfsdk:"organization_id"`
	Teams                     types.String `tfsdk:"teams"`
	CreatedAt                 types.String `tfsdk:"created_at"`
	LastUsedAt                types.String `tfsdk:"last_used_at"`
}

var virtualAPIKeyAttrSpec = []AttrSpec{
	{TFName: "value", Kind: Synthetic, Sensitive: true},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "key_type", JSONName: "keyType", Kind: Scalar},
	credentialProviderKeySpec, credentialLabelsSpec, credentialSpendCapSpec,
	{TFName: "include_future_providers", JSONName: "includeFutureProviders", Kind: Scalar, OmitOnNull: true},
	{TFName: "billing_team_id", JSONName: "billingTeamId", Kind: Scalar},
	{TFName: "expires_at", JSONName: "expiresAt", Kind: Scalar},
	{TFName: "owner_id", JSONName: "ownerId", Kind: Scalar, OmitOnNull: true}, initialGrantsAttrSpec,
	{TFName: "rotation_nonce", Kind: Synthetic},
}

func (r *VirtualAPIKeyResource) AttrSpecs() []AttrSpec { return virtualAPIKeyAttrSpec }
func (r *VirtualAPIKeyResource) APIShape() any         { return client.GetVirtualApiKeyResponse{} }

// Billing team details are split into billing_team_id and billing_team_name.
func (r *VirtualAPIKeyResource) KnownIntentionallySkipped() []string { return []string{"billingTeam"} }
func (r *VirtualAPIKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_key"
}
func (r *VirtualAPIKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Revocable virtual LLM key with explicit provider routing, billing and spend cap. The sensitive value is saved only on creation; import adopts metadata without retrieving a bearer credential. Change rotation_nonce to replace and revoke the key. Passthrough keys carry no provider credentials.", Attributes: map[string]schema.Attribute{
		"id":                schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Virtual key UUID."},
		"name":              schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 256)}, MarkdownDescription: "Key display name."},
		"key_type":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("standard"), Validators: []validator.String{stringvalidator.OneOf("standard", "passthrough")}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Immutable standard or passthrough key type."},
		"provider_api_keys": credentialProviderKeysAttribute(), "labels": credentialLabelsAttribute(), "spend_cap": credentialSpendCapAttribute(),
		"include_future_providers":      schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Use current and future shared primary provider keys, with explicit mappings as overrides."},
		"billing_team_id":               schema.StringAttribute{Optional: true, MarkdownDescription: "Team billed for this key; removing the value clears team billing."},
		"billing_team_name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Billing team display name."},
		"expires_at":                    schema.StringAttribute{Optional: true, MarkdownDescription: "Absolute RFC 3339 expiry; omission removes expiry."},
		"owner_id":                      schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Creation-only owner selector, defaulting to the creator."},
		"permissions":                   initialGrantsAttribute(),
		"rotation_nonce":                schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Change this value to replace the key and revoke its old credential."},
		"value":                         schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Bearer credential returned on creation and retained in sensitive state."},
		"secret_id":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Stored credential identifier."},
		"token_start":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Non-secret identifying prefix."},
		"scope":                         schema.StringAttribute{Computed: true, MarkdownDescription: "Legacy visibility projection; permissions are managed separately."},
		"author_id":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Owning identity."},
		"author_name":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Author display name."},
		"created_by":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Creator metadata as JSON."},
		"created_by_service_account_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Creating service account UUID."},
		"organization_id":               schema.StringAttribute{Computed: true, MarkdownDescription: "Organization identifier."},
		"teams":                         schema.StringAttribute{Computed: true, MarkdownDescription: "Legacy team visibility metadata as JSON."},
		"created_at":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
		"last_used_at":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Most recent use timestamp."},
	}}
}
func (r *VirtualAPIKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *VirtualAPIKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer finishParentCreate(ctx, r.client, "llmVirtualKey", "id", resp)

	var data VirtualAPIKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), virtualAPIKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_virtual_key Create", patch, virtualAPIKeyAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateVirtualApiKeyWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	data.Value = types.StringValue(result.JSON200.Value)
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resolveInitialGrants(&data.InitialGrants)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *VirtualAPIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VirtualAPIKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.GetVirtualApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	readParentPermissions(ctx, r.client, "llmVirtualKey", data.ID.ValueString(), &data.InitialGrants, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *VirtualAPIKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	finishPermissions := beginParentPermissionUpdate(ctx, r.client, "llmVirtualKey", "id", req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	defer finishPermissions()

	var data, prior VirtualAPIKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, virtualAPIKeyAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	delete(patch, "initialGrants")
	delete(patch, "ownerId")
	// The PATCH contract validates a complete routing body and clears omitted expiry.
	patch["name"] = data.Name.ValueString()
	patch["keyType"] = data.KeyType.ValueString()
	if _, present := patch["providerApiKeys"]; !present {
		patch["providerApiKeys"] = credentialProviderRequest(ctx, prior.ProviderAPIKeys, &resp.Diagnostics)
	}
	if _, present := patch["includeFutureProviders"]; !present {
		patch["includeFutureProviders"] = prior.IncludeFutureProviders.ValueBool()
	}
	if _, present := patch["expiresAt"]; !present {
		patch["expiresAt"] = nil
		if !prior.ExpiresAt.IsNull() {
			patch["expiresAt"] = prior.ExpiresAt.ValueString()
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_virtual_key Update", patch, virtualAPIKeyAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	id, err := uuid.Parse(prior.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.UpdateVirtualApiKeyWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	data.Value = prior.Value
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *VirtualAPIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VirtualAPIKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteVirtualApiKeyWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *VirtualAPIKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *VirtualAPIKeyResource) flatten(ctx context.Context, data *VirtualAPIKeyResourceModel, body []byte, diags *diag.Diagnostics) {
	var raw struct {
		credentialRoutingResponse
		ID                        string          `json:"id"`
		Name                      string          `json:"name"`
		KeyType                   string          `json:"keyType"`
		IncludeFutureProviders    bool            `json:"includeFutureProviders"`
		ExpiresAt                 *time.Time      `json:"expiresAt"`
		CreatedAt                 *time.Time      `json:"createdAt"`
		LastUsedAt                *time.Time      `json:"lastUsedAt"`
		SecretID                  string          `json:"secretId"`
		TokenStart                string          `json:"tokenStart"`
		Scope                     string          `json:"scope"`
		AuthorID                  *string         `json:"authorId"`
		AuthorName                *string         `json:"authorName"`
		CreatedBy                 json.RawMessage `json:"createdBy"`
		CreatedByServiceAccountID *string         `json:"createdByServiceAccountId"`
		OrganizationID            string          `json:"organizationId"`
		Teams                     json.RawMessage `json:"teams"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringValue(raw.ID)
	data.Name = types.StringValue(raw.Name)
	data.KeyType = types.StringValue(raw.KeyType)
	data.IncludeFutureProviders = types.BoolValue(raw.IncludeFutureProviders)
	data.ExpiresAt = credentialConfiguredTimestamp(data.ExpiresAt, raw.ExpiresAt)
	data.CreatedAt = canonicalAPITimestamp(raw.CreatedAt)
	data.LastUsedAt = canonicalAPITimestamp(raw.LastUsedAt)
	data.SecretID = types.StringValue(raw.SecretID)
	data.TokenStart = types.StringValue(raw.TokenStart)
	data.Scope = types.StringValue(raw.Scope)
	data.AuthorID = types.StringPointerValue(raw.AuthorID)
	data.AuthorName = types.StringPointerValue(raw.AuthorName)
	data.CreatedBy = credentialJSONValue(raw.CreatedBy, diags)
	data.CreatedByServiceAccountID = types.StringPointerValue(raw.CreatedByServiceAccountID)
	data.OrganizationID = types.StringValue(raw.OrganizationID)
	data.Teams = credentialJSONValue(raw.Teams, diags)
	flattenCredentialRouting(ctx, raw.credentialRoutingResponse, &data.ProviderAPIKeys, &data.Labels, &data.SpendCap, &data.BillingTeamID, &data.BillingTeamName, diags)
}
func credentialConfiguredTimestamp(prior types.String, value *time.Time) types.String {
	if value == nil {
		return types.StringNull()
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		if configured, err := time.Parse(time.RFC3339Nano, prior.ValueString()); err == nil && configured.Equal(*value) {
			return prior
		}
	}
	return canonicalAPITimestamp(value)
}
