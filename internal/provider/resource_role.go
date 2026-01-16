package provider

import (
	"context"
	"fmt"

	"github.com/archestra-ai/archestra/terraform-provider-archestra/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Permissions types.Map    `tfsdk:"permissions"`
	Predefined  types.Bool   `tfsdk:"predefined"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Archestra role with permissions.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Role identifier. If provided, manages an existing role instead of creating a new one.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the role",
				Optional:            true,
			},
			"permissions": schema.MapAttribute{
				MarkdownDescription: "Permissions for the role, as a map of resource types to allowed actions. Valid actions: admin, cancel, create, delete, read, update",
				Required:            true,
				ElementType: types.ListType{
					ElemType: types.StringType,
				},
			},
			"predefined": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a predefined (built-in) role",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
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

	// If ID is provided, manage an existing role (update instead of create)
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		r.updateExistingRole(ctx, &data, resp)
		return
	}

	// Convert permissions from Terraform types to API types
	permissions, diags := r.convertPermissionsToAPI(ctx, data.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build request body
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
		resp.Diagnostics.AddError(
			"Unexpected API Response",
			fmt.Sprintf("Expected 200 OK, got status %d", apiResp.StatusCode()),
		)
		return
	}

	// Map response to Terraform state
	data.ID = types.StringValue(apiResp.JSON200.Id)
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	// Note: The API does not support the description field.
	// We preserve the user's configured value from the plan.

	// Convert permissions from API response
	tfPerms, diags := r.convertPermissionsFromAPI(ctx, apiResp.JSON200.Permission)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = tfPerms

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// updateExistingRole handles the case where an ID is provided in config
func (r *RoleResource) updateExistingRole(ctx context.Context, data *RoleResourceModel, resp *resource.CreateResponse) {
	// Convert permissions from Terraform types to API types
	permissions, diags := r.convertPermissionsToAPIForUpdate(ctx, data.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build request body
	name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: permissions,
	}

	// Call API
	apiResp, err := r.client.UpdateRoleWithResponse(ctx, data.ID.ValueString(), requestBody)
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

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	// Note: The API does not support the description field.
	// We preserve the user's configured value from the plan.

	// Convert permissions from API response
	tfPerms, permDiags := r.convertPermissionsFromAPIForUpdate(ctx, apiResp.JSON200.Permission)
	resp.Diagnostics.Append(permDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = tfPerms

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call API
	apiResp, err := r.client.GetRoleWithResponse(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read role, got error: %s", err))
		return
	}

	// Handle not found
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

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	// Note: The API does not support the description field.
	// We preserve the value from state (already loaded above).

	// Convert permissions from API response
	tfPerms, permDiags := r.convertPermissionsFromGetAPI(ctx, apiResp.JSON200.Permission)
	resp.Diagnostics.Append(permDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = tfPerms

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert permissions from Terraform types to API types
	permissions, diags := r.convertPermissionsToAPIForUpdate(ctx, data.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build request body
	name := data.Name.ValueString()
	requestBody := client.UpdateRoleJSONRequestBody{
		Name:       &name,
		Permission: permissions,
	}

	// Call API
	apiResp, err := r.client.UpdateRoleWithResponse(ctx, data.ID.ValueString(), requestBody)
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

	// Map response to Terraform state
	data.Name = types.StringValue(apiResp.JSON200.Name)
	data.Predefined = types.BoolValue(apiResp.JSON200.Predefined)

	// Note: The API does not support the description field.
	// We preserve the user's configured value from the plan.

	// Convert permissions from API response
	tfPerms, diags := r.convertPermissionsFromAPIForUpdate(ctx, apiResp.JSON200.Permission)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = tfPerms

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

// Helper functions for permission conversion

func (r *RoleResource) convertPermissionsToAPI(ctx context.Context, tfPerms types.Map) (map[string][]client.CreateRoleJSONBodyPermission, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make(map[string][]client.CreateRoleJSONBodyPermission)

	if tfPerms.IsNull() || tfPerms.IsUnknown() {
		return result, diags
	}

	elements := tfPerms.Elements()
	for key, val := range elements {
		listVal, ok := val.(types.List)
		if !ok {
			diags.AddError("Type Error", fmt.Sprintf("Expected list for permission key %s", key))
			continue
		}

		var actions []client.CreateRoleJSONBodyPermission
		for _, elem := range listVal.Elements() {
			strVal, ok := elem.(types.String)
			if !ok {
				diags.AddError("Type Error", fmt.Sprintf("Expected string in permission list for key %s", key))
				continue
			}
			actions = append(actions, client.CreateRoleJSONBodyPermission(strVal.ValueString()))
		}
		result[key] = actions
	}

	return result, diags
}

func (r *RoleResource) convertPermissionsToAPIForUpdate(ctx context.Context, tfPerms types.Map) (*map[string][]client.UpdateRoleJSONBodyPermission, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make(map[string][]client.UpdateRoleJSONBodyPermission)

	if tfPerms.IsNull() || tfPerms.IsUnknown() {
		return &result, diags
	}

	elements := tfPerms.Elements()
	for key, val := range elements {
		listVal, ok := val.(types.List)
		if !ok {
			diags.AddError("Type Error", fmt.Sprintf("Expected list for permission key %s", key))
			continue
		}

		var actions []client.UpdateRoleJSONBodyPermission
		for _, elem := range listVal.Elements() {
			strVal, ok := elem.(types.String)
			if !ok {
				diags.AddError("Type Error", fmt.Sprintf("Expected string in permission list for key %s", key))
				continue
			}
			actions = append(actions, client.UpdateRoleJSONBodyPermission(strVal.ValueString()))
		}
		result[key] = actions
	}

	return &result, diags
}

func (r *RoleResource) convertPermissionsFromAPI(ctx context.Context, apiPerms map[string][]client.CreateRole200Permission) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	if apiPerms == nil {
		return types.MapNull(types.ListType{ElemType: types.StringType}), diags
	}

	result := make(map[string]attr.Value)
	for key, actions := range apiPerms {
		actionValues := make([]attr.Value, len(actions))
		for i, action := range actions {
			actionValues[i] = types.StringValue(string(action))
		}
		listVal, listDiags := types.ListValue(types.StringType, actionValues)
		diags.Append(listDiags...)
		result[key] = listVal
	}

	mapVal, mapDiags := types.MapValue(types.ListType{ElemType: types.StringType}, result)
	diags.Append(mapDiags...)
	return mapVal, diags
}

func (r *RoleResource) convertPermissionsFromGetAPI(ctx context.Context, apiPerms map[string][]client.GetRole200Permission) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	if apiPerms == nil {
		return types.MapNull(types.ListType{ElemType: types.StringType}), diags
	}

	result := make(map[string]attr.Value)
	for key, actions := range apiPerms {
		actionValues := make([]attr.Value, len(actions))
		for i, action := range actions {
			actionValues[i] = types.StringValue(string(action))
		}
		listVal, listDiags := types.ListValue(types.StringType, actionValues)
		diags.Append(listDiags...)
		result[key] = listVal
	}

	mapVal, mapDiags := types.MapValue(types.ListType{ElemType: types.StringType}, result)
	diags.Append(mapDiags...)
	return mapVal, diags
}

func (r *RoleResource) convertPermissionsFromAPIForUpdate(ctx context.Context, apiPerms map[string][]client.UpdateRole200Permission) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	if apiPerms == nil {
		return types.MapNull(types.ListType{ElemType: types.StringType}), diags
	}

	result := make(map[string]attr.Value)
	for key, actions := range apiPerms {
		actionValues := make([]attr.Value, len(actions))
		for i, action := range actions {
			actionValues[i] = types.StringValue(string(action))
		}
		listVal, listDiags := types.ListValue(types.StringType, actionValues)
		diags.Append(listDiags...)
		result[key] = listVal
	}

	mapVal, mapDiags := types.MapValue(types.ListType{ElemType: types.StringType}, result)
	diags.Append(mapDiags...)
	return mapVal, diags
}
