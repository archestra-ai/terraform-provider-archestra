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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"time"
)

var _ resource.ResourceWithImportState = &GithubAppConfigResource{}
var _ resource.ResourceWithModifyPlan = &GithubAppConfigResource{}

func NewGithubAppConfigResource() resource.Resource { return &GithubAppConfigResource{} }

type GithubAppConfigResource struct{ client *client.ClientWithResponses }
type GithubAppConfigResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Secret         types.String `tfsdk:"private_key"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	GithubURL      types.String `tfsdk:"github_url"`
	AppID          types.String `tfsdk:"app_id"`
	InstallationID types.String `tfsdk:"installation_id"`
}

var github_app_configAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "private_key", JSONName: "privateKey", Kind: Scalar, Sensitive: true, OmitOnNull: true},
	{TFName: "github_url", JSONName: "githubUrl", Kind: Scalar, OmitOnNull: true},
	{TFName: "app_id", JSONName: "appId", Kind: Scalar, OmitOnNull: true},
	{TFName: "installation_id", JSONName: "installationId", Kind: Scalar, OmitOnNull: true},
}

func (r *GithubAppConfigResource) AttrSpecs() []AttrSpec               { return github_app_configAttrSpec }
func (r *GithubAppConfigResource) APIShape() any                       { return client.GetGithubAppConfigResponse{} }
func (r *GithubAppConfigResource) KnownIntentionallySkipped() []string { return nil }
func (r *GithubAppConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_github_app"
}
func (r *GithubAppConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Stored existing GitHub credential configuration. Applying it registers configuration inside Archestra without provisioning or contacting GitHub. Credential input is required for creation, preserved when omitted, and write-only on the API; imports adopt metadata without recovering it. Deletion revokes the Archestra registration and is rejected while synchronized resources still reference it.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Stored configuration UUID."},
		"name":            schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Credential display name."},
		"private_key":     schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Existing write-only credential or read-only Vault reference; changing it rotates the stored value."},
		"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization identifier."},
		"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
		"updated_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Last update timestamp."},
		"github_url":      schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "API base URL of the existing GitHub server."},
		"app_id":          schema.StringAttribute{Required: true, MarkdownDescription: "ID of the existing GitHub App."},
		"installation_id": schema.StringAttribute{Required: true, MarkdownDescription: "ID of the existing authorized installation."},
	}}
}
func (r *GithubAppConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *GithubAppConfigResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if !req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var secret types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("private_key"), &secret)...)
	if secret.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("private_key"), "Missing credential input", "Set an existing credential when creating this registration; importing existing metadata does not require recovering a write-only value.")
	}
}
func (r *GithubAppConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GithubAppConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Secret.IsNull() || data.Secret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("private_key"), "Missing credential input", "A known existing credential is required when creating this registration.")
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), github_app_configAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_github_app Create", patch, github_app_configAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateGithubAppConfigWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Creation Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	r.flatten(&data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *GithubAppConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GithubAppConfigResourceModel
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
	result, err := r.client.GetGithubAppConfigWithResponse(ctx, id)
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
	r.flatten(&data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}
func (r *GithubAppConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior GithubAppConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, github_app_configAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	LogPatch(ctx, "archestra_github_app Update", patch, github_app_configAttrSpec)
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
	result, err := r.client.UpdateGithubAppConfigWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Update Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	if data.Secret.IsUnknown() || data.Secret.IsNull() {
		data.Secret = prior.Secret
	}
	r.flatten(&data, result.Body, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *GithubAppConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GithubAppConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteGithubAppConfigWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *GithubAppConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *GithubAppConfigResource) flatten(data *GithubAppConfigResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID             string     `json:"id"`
		Name           string     `json:"name"`
		OrganizationID string     `json:"organizationId"`
		CreatedAt      *time.Time `json:"createdAt"`
		UpdatedAt      *time.Time `json:"updatedAt"`
		GithubURL      string     `json:"githubUrl"`
		AppID          string     `json:"appId"`
		InstallationID string     `json:"installationId"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringValue(value.ID)
	data.Name = types.StringValue(value.Name)
	data.OrganizationID = types.StringValue(value.OrganizationID)
	data.CreatedAt = canonicalAPITimestamp(value.CreatedAt)
	data.UpdatedAt = canonicalAPITimestamp(value.UpdatedAt)
	data.GithubURL = types.StringValue(value.GithubURL)
	data.AppID = types.StringValue(value.AppID)
	data.InstallationID = types.StringValue(value.InstallationID)

}
