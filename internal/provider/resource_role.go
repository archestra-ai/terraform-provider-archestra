package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
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
	ID             types.String            `tfsdk:"id"`
	Name           types.String            `tfsdk:"name"`
	Role           types.String            `tfsdk:"role"`
	Permissions    map[string]types.List   `tfsdk:"permissions"`
	Predefined     types.Bool              `tfsdk:"predefined"`
	OrganizationID types.String            `tfsdk:"organization_id"`
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
				MarkdownDescription: "Role identifier (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the role",
				Required:            true,
			},
			"role": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role slug/identifier (auto-generated from name)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Map of resource types to permission actions. Example: {\"agents\": [\"read\", \"create\"], \"mcp_servers\": [\"read\"]}",
				Required:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"predefined": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this is a system-defined role (always false for custom roles)",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The organization ID this role belongs to",
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

	// Convert permissions from Terraform types to API format
	permissions := make(map[string][]client.CreateRoleJSONBodyPermission)
	for resourceType, permList := range data.Permissions {
		var perms []string
		resp.Diagnostics.Append(permList.ElementsAs(ctx, &perms, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiPerms := make([]client.CreateRoleJSONBodyPermission, len(perms))
		for i, p := range perms {
			apiPerms[i] = client.CreateRoleJSONBodyPermission(p)
		}
		permissions[resourceType] = apiPerms
	}

	requestBody := client.CreateRoleJSONRequestBody{
		Name:       data.Name.ValueString(),
		Permission: permissions,
	}

	apiResp, err := r.client.CreateRoleWithResponse(ctx, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON409 != nil {
			errMsg = apiResp.JSON409.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	role := apiResp.JSON200
	data.ID = types.StringValue(role.Id)
	data.Role = types.StringValue(role.Role)
	data.Predefined = types.BoolValue(role.Predefined)
	if role.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*role.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}

	// Convert permissions back to Terraform types
	tfPermissions := make(map[string]types.List)
	for resourceType, perms := range role.Permission {
		permStrings := make([]string, len(perms))
		for i, p := range perms {
			permStrings[i] = string(p)
		}
		permList, diags := types.ListValueFrom(ctx, types.StringType, permStrings)
		resp.Diagnostics.Append(diags...)
		tfPermissions[resourceType] = permList
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

	// Get all roles and find ours
	apiResp, err := r.client.GetRolesWithResponse(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()))
		return
	}

	// Find the role by ID
	var found bool
	for _, role := range *apiResp.JSON200 {
		if role.Id == data.ID.ValueString() {
			found = true
			data.Name = types.StringValue(role.Name)
			data.Role = types.StringValue(role.Role)
			data.Predefined = types.BoolValue(role.Predefined)
			if role.OrganizationId != nil {
				data.OrganizationID = types.StringValue(*role.OrganizationId)
			} else {
				data.OrganizationID = types.StringNull()
			}

			// Convert permissions
			tfPermissions := make(map[string]types.List)
			for resourceType, perms := range role.Permission {
				permStrings := make([]string, len(perms))
				for i, p := range perms {
					permStrings[i] = string(p)
				}
				permList, diags := types.ListValueFrom(ctx, types.StringType, permStrings)
				resp.Diagnostics.Append(diags...)
				tfPermissions[resourceType] = permList
			}
			data.Permissions = tfPermissions
			break
		}
	}

	if !found {
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

	// Convert permissions from Terraform types to API format
	permissions := make(map[string][]client.UpdateRoleJSONBodyPermission)
	for resourceType, permList := range data.Permissions {
		var perms []string
		resp.Diagnostics.Append(permList.ElementsAs(ctx, &perms, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiPerms := make([]client.UpdateRoleJSONBodyPermission, len(perms))
		for i, p := range perms {
			apiPerms[i] = client.UpdateRoleJSONBodyPermission(p)
		}
		permissions[resourceType] = apiPerms
	}

	name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: &permissions,
	}

	// Build the role ID parameter using the helper
	roleIdParam := client.RoleIdParam(data.ID.ValueString())

	apiResp, err := r.client.UpdateRoleWithResponse(ctx, roleIdParam, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		} else if apiResp.JSON404 != nil {
			errMsg = apiResp.JSON404.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}

	role := apiResp.JSON200
	data.Name = types.StringValue(role.Name)
	data.Role = types.StringValue(role.Role)
	data.Predefined = types.BoolValue(role.Predefined)
	if role.OrganizationId != nil {
		data.OrganizationID = types.StringValue(*role.OrganizationId)
	} else {
		data.OrganizationID = types.StringNull()
	}

	// Convert permissions back to Terraform types
	tfPermissions := make(map[string]types.List)
	for resourceType, perms := range role.Permission {
		permStrings := make([]string, len(perms))
		for i, p := range perms {
			permStrings[i] = string(p)
		}
		permList, diags := types.ListValueFrom(ctx, types.StringType, permStrings)
		resp.Diagnostics.Append(diags...)
		tfPermissions[resourceType] = permList
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

	apiResp, err := r.client.DeleteRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete role, got error: %s", err))
		return
	}

	if apiResp.JSON200 == nil && apiResp.JSON404 == nil {
		errMsg := fmt.Sprintf("Expected 200 OK or 404, got status %d", apiResp.StatusCode())
		if apiResp.JSON400 != nil {
			errMsg = apiResp.JSON400.Error.Message
		}
		resp.Diagnostics.AddError("API Error", errMsg)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
