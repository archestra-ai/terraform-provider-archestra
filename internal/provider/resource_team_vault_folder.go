package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
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
)

var _ resource.Resource = &TeamVaultFolderResource{}
var _ resource.ResourceWithImportState = &TeamVaultFolderResource{}

func NewTeamVaultFolderResource() resource.Resource {
	return &TeamVaultFolderResource{}
}

type TeamVaultFolderResource struct {
	client *client.ClientWithResponses
}

type TeamVaultFolderResourceModel struct {
	ID        types.String `tfsdk:"id"`
	TeamID    types.String `tfsdk:"team_id"`
	VaultPath types.String `tfsdk:"vault_path"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

type teamVaultFolderAPIResponse struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
	TeamID    string    `json:"teamId"`
	UpdatedAt time.Time `json:"updatedAt"`
	VaultPath string    `json:"vaultPath"`
}

func (r *TeamVaultFolderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_vault_folder"
}

func (r *TeamVaultFolderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Maps an Archestra team to an external HashiCorp Vault folder path; the team can only read secrets under that path. Enterprise Edition only.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Vault folder mapping identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the team this folder grants access to. Each team can have at most one folder; changing this forces a new resource.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"vault_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Vault KV path the team can read from (e.g. `secret/data/engineering`).",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					// Pre-empts the backend 400 "Invalid Vault path"
					// rejecting `..`, leading `/`, or trailing `/`.
					vaultPathFormatValidator{},
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last-update timestamp (RFC 3339).",
			},
		},
	}
}

func (r *TeamVaultFolderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

// setFolder is the shared POST path used by both Create and Update —
// the backend's `SetTeamVaultFolder` route is idempotent (upserts on
// (teamId, vaultPath)).
func (r *TeamVaultFolderResource) setFolder(ctx context.Context, data *TeamVaultFolderResourceModel, diags *diag.Diagnostics) {
	body := client.SetTeamVaultFolderJSONRequestBody{VaultPath: data.VaultPath.ValueString()}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		diags.AddError("Marshal Error", err.Error())
		return
	}
	apiResp, err := r.client.SetTeamVaultFolderWithBodyWithResponse(ctx, data.TeamID.ValueString(), "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to set team vault folder: %s", err))
		return
	}
	if apiResp.JSON200 == nil {
		diags.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}
	flattenTeamVaultFolderBody(data, apiResp.Body, diags)
}

func (r *TeamVaultFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.setFolder(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-emit the patch we'd have sent for debug logging, even though
	// the wire request was made via the typed client above. Keeps the
	// merge-patch flow consistent with other resources for traceability.
	priorNull := tftypes.NewValue(req.Plan.Schema.Type().TerraformType(ctx), nil)
	patch := MergePatch(ctx, req.Plan.Raw, priorNull, teamVaultFolderAttrSpec, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		LogPatch(ctx, "archestra_team_vault_folder Create", patch, teamVaultFolderAttrSpec)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamVaultFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetTeamVaultFolderWithResponse(ctx, data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read team vault folder: %s", err))
		return
	}
	if IsNotFound(apiResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	if !flattenTeamVaultFolderBody(&data, apiResp.Body, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamVaultFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TeamVaultFolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only `vault_path` can change in-place (team_id RequiresReplace).
	// SetTeamVaultFolder is idempotent — calling it with the same path
	// is a no-op on the backend, so we always send.
	r.setFolder(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
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

	apiResp, err := r.client.DeleteTeamVaultFolderWithResponse(ctx, data.TeamID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete team vault folder: %s", err))
		return
	}
	if apiResp.JSON200 == nil && !IsNotFound(apiResp) {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
	}
}

// ImportState uses just the team_id since the (team_id, vault_folder)
// relationship is one-to-one — the folder is uniquely identified by
// its team. Read populates the rest.
func (r *TeamVaultFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), req.ID)...)
}

func flattenTeamVaultFolderBody(data *TeamVaultFolderResourceModel, body []byte, diags *diag.Diagnostics) bool {
	var src teamVaultFolderAPIResponse
	if err := json.Unmarshal(body, &src); err != nil {
		diags.AddError("API Response Decode Error", "Unable to decode team vault folder response: "+err.Error())
		return false
	}
	data.ID = types.StringValue(src.ID)
	data.TeamID = types.StringValue(src.TeamID)
	data.VaultPath = types.StringValue(src.VaultPath)
	data.CreatedAt = types.StringValue(src.CreatedAt.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(src.UpdatedAt.Format(time.RFC3339))
	return true
}
