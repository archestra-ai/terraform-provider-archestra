package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func NewServiceAccountFederatedCredentialResource() resource.Resource {
	return &ServiceAccountFederatedCredentialResource{}
}

type ServiceAccountFederatedCredentialResource struct{ client *client.ClientWithResponses }
type ServiceAccountFederatedCredentialResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	Name             types.String `tfsdk:"name"`
	Issuer           types.String `tfsdk:"issuer"`
	Audience         types.String `tfsdk:"audience"`
	Subject          types.String `tfsdk:"subject"`
	ClaimConditions  types.Map    `tfsdk:"claim_conditions"`
	LastUsedAt       types.String `tfsdk:"last_used_at"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

var serviceAccountFederatedCredentialAttrSpec = []AttrSpec{
	{TFName: "service_account_id", Kind: Synthetic},
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "issuer", JSONName: "issuer", Kind: Scalar},
	{TFName: "audience", JSONName: "audience", Kind: Scalar},
	{TFName: "subject", JSONName: "subject", Kind: Scalar},
	{TFName: "claim_conditions", JSONName: "claimConditions", Kind: Map},
}

func (r *ServiceAccountFederatedCredentialResource) AttrSpecs() []AttrSpec {
	return serviceAccountFederatedCredentialAttrSpec
}

func (r *ServiceAccountFederatedCredentialResource) APIShape() any {
	return client.CreateServiceAccountFederatedCredentialResponse{}
}
func (r *ServiceAccountFederatedCredentialResource) KnownIntentionallySkipped() []string { return nil }
func (r *ServiceAccountFederatedCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_federated_credential"
}
func (r *ServiceAccountFederatedCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Workload identity federation trust for a service account: an OIDC token whose issuer, audience, subject and listed claims all match exactly can be exchanged at `/api/workload-identity/token` for a short-lived token of the account, so a CI job or workload holds no API key. Changing the issuer, audience, subject or claim conditions revokes the tokens the credential already issued, as does destroying it. Requires API 1.5.0-rc.26 or later with an Enterprise license.", Attributes: map[string]schema.Attribute{
		"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Composite service-account UUID and credential UUID."},
		"service_account_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Service account the exchanged token acts as."},
		"name":               schema.StringAttribute{Required: true, MarkdownDescription: "Credential display name, unique within the service account."},
		"issuer":             schema.StringAttribute{Required: true, MarkdownDescription: "OIDC issuer URL, exactly as the token's `iss` claim carries it (for GitHub Actions, `https://token.actions.githubusercontent.com`). Must be reachable over https at a public address."},
		"audience":           schema.StringAttribute{Required: true, MarkdownDescription: "Value the token's `aud` claim must carry, usually the Archestra URL."},
		"subject":            schema.StringAttribute{Required: true, MarkdownDescription: "Value the token's `sub` claim must equal (for GitHub Actions, for example `repo:acme/infra:ref:refs/heads/main`). No wildcards."},
		"claim_conditions":   schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})), MarkdownDescription: "Other claims the token must carry, each with the exact value required (for example `job_workflow_ref` or `repository_id`). Defaults to none."},
		"last_used_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Most recent successful token exchange."},
		"created_at":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Creation timestamp."},
		"updated_at":         schema.StringAttribute{Computed: true, MarkdownDescription: "Last-update timestamp."},
	}}
}
func (r *ServiceAccountFederatedCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ServiceAccountFederatedCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceAccountFederatedCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	account, err := uuid.Parse(data.ServiceAccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid service_account_id", err.Error())
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), serviceAccountFederatedCredentialAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateServiceAccountFederatedCredentialWithBodyWithResponse(ctx, account, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Federated Credential Creation Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ServiceAccountFederatedCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceAccountFederatedCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	account, credential, err := serviceAccountTokenIDs(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", "expected service_account_uuid:credential_uuid: "+err.Error())
		return
	}
	// The API has no single-credential read; the account's list is small (at most 20).
	result, err := r.client.GetServiceAccountFederatedCredentialsWithResponse(ctx, account)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) {
		resp.State.RemoveResource(ctx)
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Federated Credential Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	for _, item := range *result.JSON200 {
		if item.Id != credential {
			continue
		}
		body, err := json.Marshal(item)
		if err != nil {
			resp.Diagnostics.AddError("Response Encoding Failed", err.Error())
			return
		}
		r.flatten(ctx, &data, body, &resp.Diagnostics)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	resp.State.RemoveResource(ctx)
}
func (r *ServiceAccountFederatedCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceAccountFederatedCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, serviceAccountFederatedCredentialAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	account, credential, err := serviceAccountTokenIDs(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.UpdateServiceAccountFederatedCredentialWithBodyWithResponse(ctx, account, credential, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Federated Credential Update Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return
	}
	r.flatten(ctx, &data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *ServiceAccountFederatedCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceAccountFederatedCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	account, credential, err := serviceAccountTokenIDs(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteServiceAccountFederatedCredentialWithResponse(ctx, account, credential)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Federated Credential Deletion Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *ServiceAccountFederatedCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, _, err := serviceAccountTokenIDs(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", "expected service_account_uuid:credential_uuid: "+err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *ServiceAccountFederatedCredentialResource) flatten(ctx context.Context, data *ServiceAccountFederatedCredentialResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID               string            `json:"id"`
		ServiceAccountID string            `json:"serviceAccountId"`
		Name             string            `json:"name"`
		Issuer           string            `json:"issuer"`
		Audience         string            `json:"audience"`
		Subject          string            `json:"subject"`
		ClaimConditions  map[string]string `json:"claimConditions"`
		LastUsedAt       *string           `json:"lastUsedAt"`
		CreatedAt        string            `json:"createdAt"`
		UpdatedAt        string            `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid Federated Credential Response", err.Error())
		return
	}
	data.ID = types.StringValue(value.ServiceAccountID + ":" + value.ID)
	data.ServiceAccountID = types.StringValue(value.ServiceAccountID)
	data.Name = types.StringValue(value.Name)
	data.Issuer = types.StringValue(value.Issuer)
	data.Audience = types.StringValue(value.Audience)
	data.Subject = types.StringValue(value.Subject)
	if value.ClaimConditions == nil {
		value.ClaimConditions = map[string]string{}
	}
	conditions, d := types.MapValueFrom(ctx, types.StringType, value.ClaimConditions)
	diags.Append(d...)
	data.ClaimConditions = conditions
	data.LastUsedAt = canonicalTokenTime(value.LastUsedAt)
	data.CreatedAt = canonicalTokenTime(&value.CreatedAt)
	data.UpdatedAt = canonicalTokenTime(&value.UpdatedAt)
}
