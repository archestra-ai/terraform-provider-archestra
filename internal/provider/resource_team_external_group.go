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
}

/* ---------------- Metadata ---------------- */

func (r *TeamExternalGroupResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_team_external_group"
}

/* ---------------- Configure ---------------- */

func (r *TeamExternalGroupResource) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data",
			fmt.Sprintf("Expected *client.ClientWithResponses, got %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

/* ---------------- Schema ---------------- */

func (r *TeamExternalGroupResource) Schema(
	ctx context.Context,
	req resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages external identity provider group sync for a team.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},

			"team_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"external_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "External IdP group identifier (LDAP DN, OIDC group name, SAML attribute, etc).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

/* ---------------- Create ---------------- */

func (r *TeamExternalGroupResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var data TeamExternalGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.AddTeamExternalGroupWithResponse(
		ctx,
		data.TeamID.ValueString(),
		client.AddTeamExternalGroupJSONRequestBody{
			GroupIdentifier: data.ExternalGroupID.ValueString(),
		},
	)

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got %d", apiResp.StatusCode()),
		)
		return
	}

	data.ID = types.StringValue(
		fmt.Sprintf("%s/%s", apiResp.JSON200.TeamId, apiResp.JSON200.Id),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

/* ---------------- Read ---------------- */

func (r *TeamExternalGroupResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var data TeamExternalGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetTeamExternalGroupsWithResponse(
		ctx,
		data.TeamID.ValueString(),
	)
	if err != nil || apiResp.JSON200 == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	idParts := strings.Split(data.ID.ValueString(), "/")
	mappingID := idParts[len(idParts)-1]

	for _, g := range *apiResp.JSON200 {
		if g.Id == mappingID {
			data.ExternalGroupID = types.StringValue(g.GroupIdentifier)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

/* ---------------- Update ---------------- */

func (r *TeamExternalGroupResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	// Not supported: RequiresReplace handles updates
}

/* ---------------- Delete ---------------- */

func (r *TeamExternalGroupResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var data TeamExternalGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idParts := strings.Split(data.ID.ValueString(), "/")
	mappingID := idParts[len(idParts)-1]

	apiResp, err := r.client.RemoveTeamExternalGroupWithResponse(
		ctx,
		data.TeamID.ValueString(),
		mappingID,
	)

	if err != nil {
		resp.Diagnostics.AddError("API Error", err.Error())
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 or 404, got %d", apiResp.StatusCode()),
		)
	}
}

/* ---------------- Import ---------------- */

func (r *TeamExternalGroupResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected format team_id/mapping_id",
		)
		return
	}

	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("team_id"), parts[0])...,
	)
	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...,
	)
}
