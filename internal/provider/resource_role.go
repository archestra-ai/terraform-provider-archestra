package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
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
	ID             types.String          `tfsdk:"id"`
	Name           types.String          `tfsdk:"name"`
	Permissions    map[string]types.List `tfsdk:"permissions"`
	Predefined     types.Bool            `tfsdk:"predefined"`
	OrganizationID types.String          `tfsdk:"organization_id"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom RBAC role in Archestra. Roles define permissions that can be assigned to users.",

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
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Map of resource names to lists of allowed actions. Valid actions: admin, cancel, create, delete, read, update",
				Required:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a predefined system role",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Organization ID this role belongs to",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = apiClient
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert permissions from Terraform types to API format
	permissions := make(map[string][]client.CreateRoleJSONBodyPermission)
	for resourceName, actionsList := range data.Permissions {
		var actions []string
		resp.Diagnostics.Append(actionsList.ElementsAs(ctx, &actions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		apiActions := make([]client.CreateRoleJSONBodyPermission, len(actions))
		for i, action := range actions {
			apiActions[i] = client.CreateRoleJSONBodyPermission(action)
		}
		permissions[resourceName] = apiActions
	}

	// Create request body
	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissions,
	}

	// Call API
	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON403 != nil {
			errMsg = apiResp.JSON403.Error.Message
		} else if apiResp.JSON409 != nil {
			errMsg = apiResp.JSON409.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)
	if apiResp.JSON200.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}

	// Convert permissions from API response to Terraform types
	data.Permissions, resp.Diagnostics = convertCreateRolePermissions(ctx, apiResp.JSON200.Permission, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use GetRoles (list all) since GetRole has a union type parameter that's hard to construct
	apiResp, err := r.client.GetRolesWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read roles, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Find role by ID
	roleID := data.ID.ValueString()
	var found bool
	for _, role := range *apiResp.JSON200 {
		if role.Id == roleID {
			found = true
			data.Name = types.StringValue(role.Name)
			data.Predefined = types.BoolValue(role.Predefined)
			if role.OrganizationId != nil {
				data.OrganizationID = types.StringValue(*role.OrganizationId)
			} else {
				data.OrganizationID = types.StringNull()
			}

			// Convert permissions
			data.Permissions, resp.Diagnostics = convertGetRolesPermissions(ctx, role.Permission, resp.Diagnostics)
			if resp.Diagnostics.HasError() {
				return
			}
			break
		}
	}

	if !found {
		// Role was deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Since UpdateRole has a union type parameter that's difficult to construct,
	// we delete and recreate the role instead
	// First, delete the existing role
	deleteResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role for update, got error: %s", err))
		return
	}

	if deleteResp.JSON200 == nil && deleteResp.JSON404 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK or 404, got status %d", deleteResp.StatusCode())
		if deleteResp.JSON403 != nil {
			errMsg = deleteResp.JSON403.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	// Now create a new role with the updated values
	permissions := make(map[string][]client.CreateRoleJSONBodyPermission)
	for resourceName, actionsList := range data.Permissions {
		var actions []string
		resp.Diagnostics.Append(actionsList.ElementsAs(ctx, &actions, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		apiActions := make([]client.CreateRoleJSONBodyPermission, len(actions))
		for i, action := range actions {
			apiActions[i] = client.CreateRoleJSONBodyPermission(action)
		}
		permissions[resourceName] = apiActions
	}

	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissions,
	}

	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role after delete, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON403 != nil {
			errMsg = apiResp.JSON403.Error.Message
		} else if apiResp.JSON409 != nil {
			errMsg = apiResp.JSON409.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	// Map response to Terraform state - note: ID will change!
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)
	if apiResp.JSON200.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*apiResp.JSON200.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}

	data.Permissions, resp.Diagnostics = convertCreateRolePermissions(ctx, apiResp.JSON200.Permission, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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
		errMsg := fmt.Sprintf("Expected 200 OK or 404 Not Found, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON403 != nil {
			errMsg = apiResp.JSON403.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper functions to convert API permission types to Terraform types
func convertCreateRolePermissions(ctx context.Context, perms map[string][]client.CreateRole200Permission, diagnostics diag.Diagnostics) (map[string]types.List, diag.Diagnostics) {
	result := make(map[string]types.List)
	for resourceName, actions := range perms {
		stringActions := make([]string, len(actions))
		for i, action := range actions {
			stringActions[i] = string(action)
		}
		list, listDiags := types.ListValueFrom(ctx, types.StringType, stringActions)
		diagnostics.Append(listDiags...)
		result[resourceName] = list
	}
	return result, diagnostics
}

func convertGetRolesPermissions(ctx context.Context, perms map[string][]client.GetRoles200Permission, diagnostics diag.Diagnostics) (map[string]types.List, diag.Diagnostics) {
	result := make(map[string]types.List)
	for resourceName, actions := range perms {
		stringActions := make([]string, len(actions))
		for i, action := range actions {
			stringActions[i] = string(action)
		}
		list, listDiags := types.ListValueFrom(ctx, types.StringType, stringActions)
		diagnostics.Append(listDiags...)
		result[resourceName] = list
	}
	return result, diagnostics
}
