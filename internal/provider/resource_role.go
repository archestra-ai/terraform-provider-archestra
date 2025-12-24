package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *client.ClientWithResponses
}

type RolePermissionModel struct {
	Resource    types.String   `tfsdk:"resource"`
	Permissions []types.String `tfsdk:"permissions"`
}

type RoleResourceModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	Permissions []RolePermissionModel `tfsdk:"permissions"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom RBAC role in Archestra.",

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
			"permissions": schema.ListNestedAttribute{
				MarkdownDescription: "List of permissions for this role",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource": schema.StringAttribute{
							MarkdownDescription: "Resource type (e.g., 'agents', 'mcp_servers', 'teams')",
							Required:            true,
						},
						"permissions": schema.ListAttribute{
							MarkdownDescription: "List of permissions for this resource (e.g., 'read', 'write', 'delete', 'admin')",
							Required:            true,
							ElementType:         types.StringType,
						},
					},
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build permissions map
	permissionsMap := make(map[string][]client.CreateRoleJSONBodyPermission)
	for _, perm := range data.Permissions {
		var perms []client.CreateRoleJSONBodyPermission
		for _, p := range perm.Permissions {
			perms = append(perms, client.CreateRoleJSONBodyPermission(p.ValueString()))
		}
		permissionsMap[perm.Resource.ValueString()] = perms
	}

	// Create request body
	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissionsMap,
	}

	// Call API
	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)

	// Map permissions back
	var tfPermissions []RolePermissionModel
	for resource, perms := range apiResp.JSON200.Permission {
		var tfPerms []types.String
		for _, p := range perms {
			tfPerms = append(tfPerms, types.StringValue(string(p)))
		}
		tfPermissions = append(tfPermissions, RolePermissionModel{
			Resource:    types.StringValue(resource),
			Permissions: tfPerms,
		})
	}
	data.Permissions = tfPermissions

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build roleId parameter - it's a union type, try using the ID as string
	roleIdParam := struct{ Union json.RawMessage }{Union: json.RawMessage(data.ID.ValueString())}

	// Call API
	apiResp, err := r.client.GetRoleWithResponse(ctx, roleIdParam)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	// Handle not found
	if apiResp.JSON404 != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)

	// Map permissions
	var tfPermissions []RolePermissionModel
	for resource, perms := range apiResp.JSON200.Permission {
		var tfPerms []types.String
		for _, p := range perms {
			tfPerms = append(tfPerms, types.StringValue(string(p)))
		}
		tfPermissions = append(tfPermissions, RolePermissionModel{
			Resource:    types.StringValue(resource),
			Permissions: tfPerms,
		})
	}
	data.Permissions = tfPermissions

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build permissions map
	permissionsMap := make(map[string][]client.UpdateRoleJSONBodyPermission)
	for _, perm := range data.Permissions {
		var perms []client.UpdateRoleJSONBodyPermission
		for _, p := range perm.Permissions {
			perms = append(perms, client.UpdateRoleJSONBodyPermission(p.ValueString()))
		}
		permissionsMap[perm.Resource.ValueString()] = perms
	}

	// Create request body
	name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: &permissionsMap,
	}

	// Build roleId parameter
	roleIdParam := struct{ Union json.RawMessage }{Union: json.RawMessage(data.ID.ValueString())}

	// Call API
	apiResp, err := r.client.UpdateRoleWithResponse(ctx, roleIdParam, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role, got error: %s", err))
		return
	}

	// Check response
	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)

	// Map permissions back
	var tfPermissions []RolePermissionModel
	for resource, perms := range apiResp.JSON200.Permission {
		var tfPerms []types.String
		for _, p := range perms {
			tfPerms = append(tfPerms, types.StringValue(string(p)))
		}
		tfPermissions = append(tfPermissions, RolePermissionModel{
			Resource:    types.StringValue(resource),
			Permissions: tfPerms,
		})
	}
	data.Permissions = tfPermissions

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API
	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}

	// Check response (200 or 404 are both acceptable for delete)
	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
