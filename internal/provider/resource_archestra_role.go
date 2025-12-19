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

var _ resource.Resource = &ArchestraRoleResource{}
var _ resource.ResourceWithImportState = &ArchestraRoleResource{}

func NewArchestraRoleResource() resource.Resource {
	return &ArchestraRoleResource{}
}

type ArchestraRoleResource struct {
	client *client.ClientWithResponses
}

type ArchestraRoleResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Permissions []types.String `tfsdk:"permissions"`
}

func (r *ArchestraRoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *ArchestraRoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra RBAC role.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role",
				Required:            true,
			},
			"permissions": schema.SetAttribute{
				MarkdownDescription: "List of permissions in 'resource:action' format (e.g., 'agents:read')",
				Required:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *ArchestraRoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ArchestraRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ArchestraRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Map permissions from flat list "resource:action" to map[string][]Permission
	permissionsMap := make(map[string][]client.CreateRoleJSONBodyPermission)
	for _, p := range data.Permissions {
		parts := strings.Split(p.ValueString(), ":")
		if len(parts) != 2 {
			resp.Diagnostics.AddError("Invalid Permission Format", fmt.Sprintf("Permission '%s' must be in 'resource:action' format", p.ValueString()))
			return
		}
		res, action := parts[0], parts[1]
		permissionsMap[res] = append(permissionsMap[res], client.CreateRoleJSONBodyPermission(action))
	}

	// Create request body
	requestBody := client.CreateRoleJSONBody{
		Name:       data.Name.ValueString(),
		Permission: permissionsMap,
	}

	// Call API
	apiResp, err := r.client.CreateRoleWithResponse(ctx, client.CreateRoleJSONRequestBody(requestBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	role := apiResp.JSON200

	// Set state
	data.ID = types.StringValue(role.Id)
	data.Name = types.StringValue(role.Name)

	// Flatten permissions
	var flattenedPerms []types.String
	if role.Permission != nil {
		for res, actions := range role.Permission {
			for _, action := range actions {
				flattenedPerms = append(flattenedPerms, types.StringValue(fmt.Sprintf("%s:%s", res, action)))
			}
		}
	}
	data.Permissions = flattenedPerms

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ArchestraRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ArchestraRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.GetRoleById(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	role := apiResp.JSON200
	data.Name = types.StringValue(role.Name)

	var flattenedPerms []types.String
	if role.Permission != nil {
		for res, actions := range role.Permission {
			for _, action := range actions {
				flattenedPerms = append(flattenedPerms, types.StringValue(fmt.Sprintf("%s:%s", res, action)))
			}
		}
	}
	data.Permissions = flattenedPerms

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ArchestraRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ArchestraRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	permissionsMap := make(map[string][]client.UpdateRoleJSONBodyPermission)
	for _, p := range data.Permissions {
		parts := strings.Split(p.ValueString(), ":")
		if len(parts) != 2 {
			resp.Diagnostics.AddError("Invalid Permission Format", fmt.Sprintf("Permission '%s' must be in 'resource:action' format", p.ValueString()))
			return
		}
		res, action := parts[0], parts[1]
		permissionsMap[res] = append(permissionsMap[res], client.UpdateRoleJSONBodyPermission(action))
	}

	requestBody := client.UpdateRoleJSONBody{
		Permission: &permissionsMap,
	}

	name := data.Name.ValueString()
	requestBody.Name = &name

	apiResp, err := r.client.UpdateRoleById(ctx, data.ID.ValueString(), client.UpdateRoleJSONRequestBody(requestBody))
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Update state
	role := apiResp.JSON200
	data.Name = types.StringValue(role.Name)

	var flattenedPerms []types.String
	if role.Permission != nil {
		for res, actions := range role.Permission {
			for _, action := range actions {
				flattenedPerms = append(flattenedPerms, types.StringValue(fmt.Sprintf("%s:%s", res, action)))
			}
		}
	}
	data.Permissions = flattenedPerms

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ArchestraRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ArchestraRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// DeleteRoleWithResponse takes string ID, not struct! (Verified in previous checks)
	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *ArchestraRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
