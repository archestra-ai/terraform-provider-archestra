package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TeamExternalGroupResource{}
var _ resource.ResourceWithImportState = &TeamExternalGroupResource{}

func NewTeamExternalGroupResource() resource.Resource {
	return &TeamExternalGroupResource{}
}

type TeamExternalGroupResource struct {
	client *client.ClientWithResponses
}

type TeamExternalGroupModel struct {
	ID              types.String `tfsdk:"id"`
	TeamID          types.String `tfsdk:"team_id"`
	ExternalGroupID types.String `tfsdk:"external_group_id"`
	ExternalName    types.String `tfsdk:"external_group_name"`
}

// ---------------------
// Metadata
// ---------------------

func (r *TeamExternalGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_external_group"
}

// ---------------------
// Configure
// ---------------------

func (r *TeamExternalGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

// ---------------------
// Schema
// ---------------------

func (r *TeamExternalGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a mapping between an Archestra team and an external identity provider group.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier in format team_id/mapping_id",
			},

			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the Archestra team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"external_group_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The external group ID (e.g., LDAP DN).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"external_group_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The external group name (for OIDC/SAML providers).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// ---------------------
// Create
// ---------------------

func (r *TeamExternalGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamExternalGroupModel

	// Load values from plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine group identifier
	var groupIdentifier string
	if !data.ExternalGroupID.IsNull() {
		groupIdentifier = data.ExternalGroupID.ValueString()
	} else if !data.ExternalName.IsNull() {
		groupIdentifier = data.ExternalName.ValueString()
	} else {
		resp.Diagnostics.AddError("Invalid Configuration",
			"Either external_group_id or external_group_name must be provided.")
		return
	}

	// Build request body
	body := client.AddTeamExternalGroupJSONRequestBody{
		GroupIdentifier: groupIdentifier,
	}

	// Call API (correct high-level method)
	apiResp, err := r.client.AddTeamExternalGroupWithResponse(
		ctx,
		data.TeamID.ValueString(),
		body,
	)

	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to add external group: %s", err))
		return
	}

	// Validate response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Set Terraform state values
	// Store composite ID for import compatibility (team_id/mapping_id)
	data.ID = types.StringValue(fmt.Sprintf("%s/%s", apiResp.JSON200.TeamId, apiResp.JSON200.Id))
	data.TeamID = types.StringValue(apiResp.JSON200.TeamId)

	// Preserve which field was originally configured to avoid state drift
	if !data.ExternalGroupID.IsNull() {
		data.ExternalGroupID = types.StringValue(apiResp.JSON200.GroupIdentifier)
	} else if !data.ExternalName.IsNull() {
		data.ExternalName = types.StringValue(apiResp.JSON200.GroupIdentifier)
	}

	// Save into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ---------------------
// Read
// ---------------------

func (r *TeamExternalGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamExternalGroupModel

	// Load existing state
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID := data.TeamID.ValueString()

	// Call API to list all external groups for this team
	apiResp, err := r.client.GetTeamExternalGroupsWithResponse(ctx, teamID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to read external groups: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		// If API returns 404, team might be deleted
		if apiResp.JSON404 != nil {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Extract mapping ID from composite ID (team_id/mapping_id)
	idParts := strings.Split(data.ID.ValueString(), "/")
	mappingID := data.ID.ValueString()
	if len(idParts) == 2 {
		mappingID = idParts[1]
	}

	// Look for THIS external group in API results
	found := false
	for _, g := range *apiResp.JSON200 {
		if g.Id == mappingID {
			// Update state with actual backend values
			// Preserve which field was originally configured to avoid state drift
			if !data.ExternalGroupID.IsNull() {
				data.ExternalGroupID = types.StringValue(g.GroupIdentifier)
			} else if !data.ExternalName.IsNull() {
				data.ExternalName = types.StringValue(g.GroupIdentifier)
			}
			data.TeamID = types.StringValue(g.TeamId)
			found = true
			break
		}
	}

	// If not found → remove from state
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ---------------------
// Delete
// ---------------------

func (r *TeamExternalGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamExternalGroupModel

	// Load from state
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamID := data.TeamID.ValueString()

	// Extract mapping ID from composite ID (team_id/mapping_id)
	idParts := strings.Split(data.ID.ValueString(), "/")
	mappingID := data.ID.ValueString()
	if len(idParts) == 2 {
		mappingID = idParts[1]
	}

	// Call API
	apiResp, err := r.client.RemoveTeamExternalGroupWithResponse(ctx, teamID, mappingID)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Failed to remove external group mapping: %s", err))
		return
	}

	// Acceptable results:
	// - 200 OK: mapping removed
	// - 404 Not Found: mapping already removed
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Terraform removes state automatically after Delete returns
}

// ---------------------
// Update
// ---------------------

func (r *TeamExternalGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Updates are not supported; RequiresReplace plan modifiers handle this.
}

// ---------------------
// ImportState
// ---------------------

func (r *TeamExternalGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expect import format: team_id/mapping_id
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected format: team_id/mapping_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
