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

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *client.ClientWithResponses
}

type RoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Permissions types.List   `tfsdk:"permissions"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom RBAC role in Archestra. Roles define sets of permissions that can be assigned to users.",

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
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of what this role allows",
				Optional:            true,
			},
			"permissions": schema.ListAttribute{
				MarkdownDescription: "List of permissions in format 'resource:action' (e.g., 'agents:read', 'agents:write', 'mcp_servers:read')",
				Required:            true,
				ElementType:         types.StringType,
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

// Convert list of "resource:action" strings to map[string][]Permission
func permissionsListToMap(permissions []string) map[string][]client.CreateRoleJSONBodyPermission {
	result := make(map[string][]client.CreateRoleJSONBodyPermission)
	
	for _, perm := range permissions {
		parts := strings.Split(perm, ":")
		if len(parts) == 2 {
			resource := parts[0]
			action := client.CreateRoleJSONBodyPermission(parts[1])
			result[resource] = append(result[resource], action)
		}
	}
	
	return result
}

// Convert map[string][]Permission to list of "resource:action" strings for GetRole
func permissionsMapToListGet(permMap map[string][]client.GetRole200Permission) []string {
	var result []string
	
	for resource, actions := range permMap {
		for _, action := range actions {
			result = append(result, fmt.Sprintf("%s:%s", resource, string(action)))
		}
	}
	
	return result
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract permissions list
	var permissionsList []string
	resp.Diagnostics.Append(data.Permissions.ElementsAs(ctx, &permissionsList, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert to API format
	permissionsMap := permissionsListToMap(permissionsList)

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
			fmt.Sprintf("Expected 200 OK, got status %d. Body: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Update state with response data
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get all roles and find ours
	// The GetRole API has a union type issue, so we use GetRoles instead
	apiResp, err := r.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read roles, got error: %s", err))
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

	// Find our role by ID
	var found bool
	var foundName string
	var foundPermissions map[string][]client.GetRoles200Permission
	
	for _, role := range *apiResp.JSON200 {
		if role.Id == data.ID.ValueString() {
			found = true
			foundName = role.Name
			foundPermissions = role.Permission
			break
		}
	}

	// Handle not found - resource deleted
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update state
	data.Name = types.StringValue(foundName)
	
	// Convert permissions map back to list
	permList := []string{}
	for resource, actions := range foundPermissions {
		for _, action := range actions {
			permList = append(permList, fmt.Sprintf("%s:%s", resource, string(action)))
		}
	}
	
	permListValue, diags := types.ListValueFrom(ctx, types.StringType, permList)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = permListValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract permissions list
	var permissionsList []string
	resp.Diagnostics.Append(data.Permissions.ElementsAs(ctx, &permissionsList, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert to API format (UpdateRole uses different type)
	permissionsMap := make(map[string][]client.UpdateRoleJSONBodyPermission)
	for _, perm := range permissionsList {
		parts := strings.Split(perm, ":")
		if len(parts) == 2 {
			resource := parts[0]
			action := client.UpdateRoleJSONBodyPermission(parts[1])
			permissionsMap[resource] = append(permissionsMap[resource], action)
		}
	}

	// Create update body
	name := data.Name.ValueString()
	updateBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: &permissionsMap,
	}

	// Update uses DeleteRole + CreateRole as workaround for union type
	// First delete the old role
	_, deleteErr := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if deleteErr != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role during update, got error: %s", deleteErr))
		return
	}

	// Then create with new values - convert permissions back to CreateRole type
	createPerms := make(map[string][]client.CreateRoleJSONBodyPermission)
	for resource, actions := range *updateBody.Permission {
		for _, action := range actions {
			createPerms[resource] = append(createPerms[resource], client.CreateRoleJSONBodyPermission(action))
		}
	}
	
	createBody := client.CreateRoleJSONRequestBody{
		Name:       *updateBody.Name,
		Permission: createPerms,
	}

	createResp, createErr := r.client.CreateRoleWithResponse(ctx, createBody)
	if createErr != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to recreate role during update, got error: %s", createErr))
		return
	}

	// Check response
	if createResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", createResp.StatusCode()),
		)
		return
	}

	// Update ID in case it changed
	data.ID = types.StringValue(createResp.JSON200.Id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete via API
	apiResp, deleteErr := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if deleteErr != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", deleteErr))
		return
	}

	// Check response (204 No Content or 200 OK expected)
	if apiResp.StatusCode() != 200 && apiResp.StatusCode() != 204 {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 or 204, got status %d", apiResp.StatusCode()),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
