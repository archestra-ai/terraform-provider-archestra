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

var _ resource.ResourceWithImportState = &GithubPatResource{}
var _ resource.ResourceWithModifyPlan = &GithubPatResource{}

func NewGithubPatResource() resource.Resource { return &GithubPatResource{} }

type GithubPatResource struct{ client *client.ClientWithResponses }
type GithubPatResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Secret         types.String `tfsdk:"token"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

var github_patAttrSpec = []AttrSpec{
	{TFName: "name", JSONName: "name", Kind: Scalar},
	{TFName: "token", JSONName: "token", Kind: Scalar, Sensitive: true, OmitOnNull: true},
}

func (r *GithubPatResource) AttrSpecs() []AttrSpec               { return github_patAttrSpec }
func (r *GithubPatResource) APIShape() any                       { return client.ListGithubPatsResponse{} }
func (r *GithubPatResource) KnownIntentionallySkipped() []string { return nil }
func (r *GithubPatResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_github_pat"
}
func (r *GithubPatResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Stored existing GitHub credential configuration. Applying it registers configuration inside Archestra without provisioning or contacting GitHub. Credential input is required for creation, preserved when omitted, and write-only on the API; imports adopt metadata without recovering it. Deletion revokes the Archestra registration and is rejected while synchronized resources still reference it.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Stored configuration UUID."},
		"name":            schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Credential display name."},
		"token":           schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Existing write-only credential or read-only Vault reference; changing it rotates the stored value."},
		"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization identifier."},
		"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
		"updated_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Last update timestamp."},
	}}
}
func (r *GithubPatResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *GithubPatResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if !req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var secret types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("token"), &secret)...)
	if secret.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Missing credential input", "Set an existing credential when creating this registration; importing existing metadata does not require recovering a write-only value.")
	}
}
func (r *GithubPatResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GithubPatResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Secret.IsNull() || data.Secret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Missing credential input", "A known existing credential is required when creating this registration.")
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), github_patAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	LogPatch(ctx, "archestra_github_pat Create", patch, github_patAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		resp.Diagnostics.AddError("Request Encoding Failed", err.Error())
		return
	}
	result, err := r.client.CreateGithubPatWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
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
func (r *GithubPatResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GithubPatResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	result, err := r.client.ListGithubPatsWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.JSON200 == nil {
		resp.Diagnostics.AddError("Read Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
		return
	}
	for _, item := range *result.JSON200 {
		if item.Id.String() == data.ID.ValueString() {
			body, err := json.Marshal(item)
			if err != nil {
				resp.Diagnostics.AddError("Response Encoding Failed", err.Error())
				return
			}
			r.flatten(&data, body, &resp.Diagnostics)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)

}
func (r *GithubPatResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior GithubPatResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, github_patAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	LogPatch(ctx, "archestra_github_pat Update", patch, github_patAttrSpec)
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
	result, err := r.client.UpdateGithubPatWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
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
func (r *GithubPatResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GithubPatResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", err.Error())
		return
	}
	result, err := r.client.DeleteGithubPatWithResponse(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *GithubPatResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
func (r *GithubPatResource) flatten(data *GithubPatResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID             string     `json:"id"`
		Name           string     `json:"name"`
		OrganizationID string     `json:"organizationId"`
		CreatedAt      *time.Time `json:"createdAt"`
		UpdatedAt      *time.Time `json:"updatedAt"`
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

}
