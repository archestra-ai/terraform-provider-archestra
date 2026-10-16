package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ resource.ResourceWithImportState = &TeamVaultFolderResource{}

func NewTeamVaultFolderResource() resource.Resource { return &TeamVaultFolderResource{} }

type TeamVaultFolderResource struct{ client *client.ClientWithResponses }
type TeamVaultFolderResourceModel struct {
	ID        types.String `tfsdk:"id"`
	TeamID    types.String `tfsdk:"team_id"`
	FolderID  types.String `tfsdk:"folder_id"`
	VaultPath types.String `tfsdk:"vault_path"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

var teamVaultFolderAttrSpec = []AttrSpec{
	{TFName: "team_id", Kind: Synthetic},
	{TFName: "vault_path", JSONName: "vaultPath", Kind: Scalar},
}

func (r *TeamVaultFolderResource) AttrSpecs() []AttrSpec               { return teamVaultFolderAttrSpec }
func (r *TeamVaultFolderResource) APIShape() any                       { return client.GetTeamVaultFolderResponse{} }
func (r *TeamVaultFolderResource) KnownIntentionallySkipped() []string { return nil }
func (r *TeamVaultFolderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_vault_folder"
}
func (r *TeamVaultFolderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "One team's mapping to an existing Vault folder. Requires read-only Vault and the relevant enterprise entitlement. Declare one mapping per team; destroy removes the mapping without changing Vault folders or secrets. Import by team ID.", Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Team ID identifying the mapping."},
		"team_id":    schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Team ID."},
		"folder_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "Server-generated mapping row ID."},
		"vault_path": schema.StringAttribute{Required: true, MarkdownDescription: "Existing Vault folder path."},
		"created_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Mapping creation timestamp."},
		"updated_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Mapping update timestamp."},
	}}
}
func (r *TeamVaultFolderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *TeamVaultFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), teamVaultFolderAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.set(ctx, &data, patch, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *TeamVaultFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	result, err := r.client.GetTeamVaultFolderWithResponse(ctx, data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if IsNotFound(result) || (result.StatusCode() == 200 && bytes.Equal(bytes.TrimSpace(result.Body), []byte("null"))) {
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
func (r *TeamVaultFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := MergePatch(ctx, req.Plan.Raw, req.State.Raw, teamVaultFolderAttrSpec, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(patch) == 0 {
		resp.State = req.State
		return
	}
	if !r.set(ctx, &data, patch, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *TeamVaultFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.DeleteTeamVaultFolderWithResponse(ctx, data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}
	if result.StatusCode() != 200 && result.StatusCode() != 204 && !IsNotFound(result) {
		resp.Diagnostics.AddError("Delete Failed", fmt.Sprintf("API returned status %d", result.StatusCode()))
	}
}
func (r *TeamVaultFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), req.ID)...)
}
func (r *TeamVaultFolderResource) set(ctx context.Context, data *TeamVaultFolderResourceModel, patch map[string]any, diags *diag.Diagnostics) bool {
	LogPatch(ctx, "archestra_team_vault_folder", patch, teamVaultFolderAttrSpec)
	body, err := json.Marshal(patch)
	if err != nil {
		diags.AddError("Request Encoding Failed", err.Error())
		return false
	}
	result, err := r.client.SetTeamVaultFolderWithBodyWithResponse(ctx, data.TeamID.ValueString(), "application/json", bytes.NewReader(body))
	if err != nil {
		diags.AddError("API Error", err.Error())
		return false
	}
	if result.JSON200 == nil {
		diags.AddError("Mapping Update Failed", fmt.Sprintf("API returned status %d: %s", result.StatusCode(), result.Body))
		return false
	}
	r.flatten(data, result.Body, diags)
	return !diags.HasError()
}
func (r *TeamVaultFolderResource) flatten(data *TeamVaultFolderResourceModel, body []byte, diags *diag.Diagnostics) {
	var value struct {
		ID        string `json:"id"`
		TeamID    string `json:"teamId"`
		VaultPath string `json:"vaultPath"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		diags.AddError("Invalid API Response", err.Error())
		return
	}
	data.ID = types.StringValue(value.TeamID)
	data.TeamID = data.ID
	data.FolderID = types.StringValue(value.ID)
	data.VaultPath = types.StringValue(value.VaultPath)
	data.CreatedAt = types.StringValue(value.CreatedAt)
	data.UpdatedAt = types.StringValue(value.UpdatedAt)
}
